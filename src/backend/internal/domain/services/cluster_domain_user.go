package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	clustertypes "github.com/BeanYa/b-ui/src/backend/internal/domain/services/cluster/types"
	database "github.com/BeanYa/b-ui/src/backend/internal/infra/db"
	"github.com/BeanYa/b-ui/src/backend/internal/infra/db/model"
	"github.com/BeanYa/b-ui/src/backend/internal/shared/util/common"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type clusterDomainUserIdentity interface {
	GetOrCreate() (*model.ClusterLocalNode, error)
}

type clusterDomainUserBroadcaster interface {
	BroadcastDomainUserUpsert(context.Context, *model.ClusterDomain, clustertypes.DomainUserUpsertPayload) error
	BroadcastDomainUserDelete(context.Context, *model.ClusterDomain, clustertypes.DomainUserDeletePayload) error
}

type ClusterDomainUserServiceOptions struct {
	DB          *gorm.DB
	Identity    clusterDomainUserIdentity
	Broadcaster clusterDomainUserBroadcaster
	Now         func() int64
}

type ClusterDomainUserService struct {
	db          *gorm.DB
	identity    clusterDomainUserIdentity
	broadcaster clusterDomainUserBroadcaster
	now         func() int64
}

type DomainUserUpsertResult struct {
	ClientID  uint   `json:"client_id"`
	RequestID string `json:"request_id"`
	Created   bool   `json:"created"`
}

type DomainUserDeleteResult struct {
	ClientID  uint   `json:"client_id"`
	RequestID string `json:"request_id"`
	Deleted   bool   `json:"deleted"`
}

func NewClusterDomainUserService(opts ClusterDomainUserServiceOptions) *ClusterDomainUserService {
	s := &ClusterDomainUserService{
		db:          opts.DB,
		identity:    opts.Identity,
		broadcaster: opts.Broadcaster,
		now:         opts.Now,
	}
	if s.db == nil {
		s.db = database.GetDB()
	}
	if s.identity == nil {
		s.identity = &ClusterLocalIdentityService{}
	}
	if s.now == nil {
		s.now = unixNow
	}
	return s
}

func (s *ClusterDomainUserService) ApplyDomainUserUpsert(ctx context.Context, domain *model.ClusterDomain, payload clustertypes.DomainUserUpsertPayload, source string, broadcast bool) (*DomainUserUpsertResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s == nil || s.db == nil {
		return nil, errors.New("cluster domain user service is required")
	}
	if domain == nil || strings.TrimSpace(domain.Domain) == "" || domain.Id == 0 {
		return nil, errors.New("local domain is required")
	}
	payload.RequestID = strings.TrimSpace(payload.RequestID)
	payload.DomainID = strings.TrimSpace(payload.DomainID)
	payload.User.UUID = strings.TrimSpace(payload.User.UUID)
	payload.User.Name = strings.TrimSpace(payload.User.Name)
	payload.User.SubToken = strings.TrimSpace(payload.User.SubToken)
	boundInboundGroupIDs := normalizeDomainUserBoundGroups(payload.User.BoundInboundGroupIDs, payload.Inbounds)
	payload.User.BoundInboundGroupIDs = boundInboundGroupIDs
	payload.Inbounds = domainUserGroupSelectors(boundInboundGroupIDs)
	if payload.RequestID == "" {
		return nil, errors.New("request_id is required")
	}
	if payload.DomainID != "" && payload.DomainID != domain.Domain {
		return nil, fmt.Errorf("payload domain_id %q does not match local domain %q", payload.DomainID, domain.Domain)
	}
	if payload.User.UUID == "" {
		return nil, errors.New("user uuid is required")
	}
	if payload.User.Name == "" {
		return nil, errors.New("user name is required")
	}
	if len(strings.TrimSpace(string(payload.User.Config))) == 0 {
		return nil, errors.New("user config is required")
	}
	resolvedConfig, err := resolveDomainUserConfigLocalProvided(payload.User.Config)
	if err != nil {
		return nil, err
	}
	payload.User.Config = resolvedConfig

	local, err := s.identity.GetOrCreate()
	if err != nil {
		return nil, err
	}
	nodeID := ""
	if local != nil {
		nodeID = local.NodeID
	}

	result := &DomainUserUpsertResult{RequestID: payload.RequestID}
	var changedInboundIDs []uint
	err = s.db.Transaction(func(tx *gorm.DB) error {
		inboundIDs, err := s.resolveInboundIDs(tx, domain.Id, nodeID, payload.Inbounds)
		if err != nil {
			return err
		}
		changedInboundIDs = inboundIDs

		var wrapper model.ClusterClient
		err = tx.Where("domain_id = ? AND hub_user_uuid = ?", domain.Id, payload.User.UUID).First(&wrapper).Error
		created := false
		if errors.Is(err, gorm.ErrRecordNotFound) {
			created = true
		} else if err != nil {
			return err
		}
		subToken := payload.User.SubToken
		if !created && subToken == "" {
			subToken = wrapper.SubToken
		}
		if created && subToken == "" {
			subToken, err = generateDomainUserSubToken()
			if err != nil {
				return err
			}
		}
		payload.User.SubToken = subToken
		boundInboundGroupIDsJSON, err := json.Marshal(boundInboundGroupIDs)
		if err != nil {
			return err
		}

		client := model.Client{}
		if !created {
			if err := tx.First(&client, wrapper.ClientID).Error; err != nil {
				return err
			}
		}
		client.Enable = payload.User.Enable
		client.Name = payload.User.Name
		client.Desc = payload.User.Desc
		client.Group = payload.User.Group
		client.Config = cloneRawMessage(payload.User.Config)
		client.Inbounds, err = json.MarshalIndent(inboundIDs, "", "  ")
		if err != nil {
			return err
		}
		if len(client.Links) == 0 {
			client.Links = json.RawMessage(`[]`)
		}
		client.Volume = payload.User.Volume
		client.Expiry = payload.User.Expiry
		client.Down = payload.User.Down
		client.Up = payload.User.Up
		client.DelayStart = payload.User.DelayStart
		client.AutoReset = payload.User.AutoReset
		client.ResetDays = payload.User.ResetDays
		client.NextReset = payload.User.NextReset
		client.TotalUp = payload.User.TotalUp
		client.TotalDown = payload.User.TotalDown

		if err := (&ClientService{}).updateLinksWithFixedInbounds(tx, []*model.Client{&client}, domain.Domain); err != nil {
			return err
		}
		if err := tx.Save(&client).Error; err != nil {
			return err
		}

		now := s.now()
		if created {
			wrapper = model.ClusterClient{
				DomainID:             domain.Id,
				Domain:               domain.Domain,
				NodeID:               nodeID,
				MemberID:             strings.TrimSpace(source),
				ClientID:             client.Id,
				HubUserUUID:          payload.User.UUID,
				RequestID:            payload.RequestID,
				SubToken:             subToken,
				BoundInboundGroupIDs: boundInboundGroupIDsJSON,
				CreatedAt:            now,
				UpdatedAt:            now,
			}
			if wrapper.MemberID == "" {
				wrapper.MemberID = nodeID
			}
			if err := tx.Create(&wrapper).Error; err != nil {
				return err
			}
		} else {
			wrapper.NodeID = nodeID
			if source = strings.TrimSpace(source); source != "" {
				wrapper.MemberID = source
			}
			wrapper.RequestID = payload.RequestID
			wrapper.SubToken = subToken
			wrapper.BoundInboundGroupIDs = boundInboundGroupIDsJSON
			wrapper.UpdatedAt = now
			if err := tx.Save(&wrapper).Error; err != nil {
				return err
			}
		}

		if err := (&InboundService{}).RestartInbounds(tx, inboundIDs); err != nil {
			return err
		}
		result.ClientID = client.Id
		result.Created = created
		return nil
	})
	if err != nil {
		return nil, err
	}
	if result.Created && broadcast && s.broadcaster != nil {
		if err := s.broadcaster.BroadcastDomainUserUpsert(ctx, domain, payload); err != nil {
			return nil, err
		}
	}
	_ = changedInboundIDs
	return result, nil
}

func normalizeDomainUserBoundGroups(userGroups []string, selectors []string) []string {
	groups := make([]string, 0, len(userGroups)+len(selectors))
	for _, value := range append(userGroups, selectors...) {
		value = strings.TrimSpace(value)
		value = strings.TrimPrefix(value, "domain:")
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		exists := false
		for _, group := range groups {
			if group == value {
				exists = true
				break
			}
		}
		if !exists {
			groups = append(groups, value)
		}
	}
	return groups
}

func domainUserGroupSelectors(groups []string) []string {
	if len(groups) == 0 {
		return nil
	}
	selectors := make([]string, 0, len(groups))
	for _, group := range groups {
		group = strings.TrimSpace(group)
		if group == "" {
			continue
		}
		selectors = append(selectors, "domain:"+group)
	}
	if len(selectors) == 0 {
		return nil
	}
	return selectors
}

func generateDomainUserSubToken() (string, error) {
	var bytes [24]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(bytes[:]), nil
}

func (s *ClusterDomainUserService) ApplyDomainUserDelete(ctx context.Context, domain *model.ClusterDomain, payload clustertypes.DomainUserDeletePayload, source string, broadcast bool) (*DomainUserDeleteResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s == nil || s.db == nil {
		return nil, errors.New("cluster domain user service is required")
	}
	if domain == nil || strings.TrimSpace(domain.Domain) == "" || domain.Id == 0 {
		return nil, errors.New("local domain is required")
	}
	payload.RequestID = strings.TrimSpace(payload.RequestID)
	payload.DomainID = strings.TrimSpace(payload.DomainID)
	payload.UUID = strings.TrimSpace(payload.UUID)
	if payload.RequestID == "" {
		return nil, errors.New("request_id is required")
	}
	if payload.DomainID != "" && payload.DomainID != domain.Domain {
		return nil, fmt.Errorf("payload domain_id %q does not match local domain %q", payload.DomainID, domain.Domain)
	}
	if payload.UUID == "" {
		return nil, errors.New("user uuid is required")
	}

	result := &DomainUserDeleteResult{RequestID: payload.RequestID}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var wrapper model.ClusterClient
		err := tx.Where("domain_id = ? AND hub_user_uuid = ?", domain.Id, payload.UUID).First(&wrapper).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		var client model.Client
		if err := tx.First(&client, wrapper.ClientID).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		var inboundIDs []uint
		_ = json.Unmarshal(client.Inbounds, &inboundIDs)
		if err := tx.Delete(&wrapper).Error; err != nil {
			return err
		}
		if wrapper.ClientID > 0 {
			if err := tx.Delete(&model.Client{}, wrapper.ClientID).Error; err != nil {
				return err
			}
		}
		if err := (&InboundService{}).RestartInbounds(tx, inboundIDs); err != nil {
			return err
		}
		result.ClientID = wrapper.ClientID
		result.Deleted = true
		return nil
	})
	if err != nil {
		return nil, err
	}
	if result.Deleted && broadcast && s.broadcaster != nil {
		if err := s.broadcaster.BroadcastDomainUserDelete(ctx, domain, payload); err != nil {
			return nil, err
		}
	}
	return result, nil
}

const (
	localProvidedDomainUserUUID     = "DomainUserUUID"
	localProvidedDomainUserPassword = "DomainUserPassword"
	localProvidedDomainUserAuth     = "DomainUserAuth"
)

func resolveDomainUserConfigLocalProvided(raw json.RawMessage) (json.RawMessage, error) {
	var config map[string]interface{}
	if err := json.Unmarshal(raw, &config); err != nil {
		return nil, err
	}
	if err := resolveDomainUserConfigValue(config, "user.config"); err != nil {
		return nil, err
	}
	data, err := json.Marshal(config)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func resolveDomainUserConfigValue(value interface{}, path string) error {
	if kind, ok := localProvidedKind(value); ok {
		return unsupportedLocalProvided(path, kind)
	}
	switch typed := value.(type) {
	case map[string]interface{}:
		for key, child := range typed {
			nextPath := path + "." + key
			if kind, ok := localProvidedKind(child); ok {
				resolved, err := resolveDomainUserLocalProvided(kind)
				if err != nil {
					return unsupportedLocalProvided(nextPath, kind)
				}
				typed[key] = resolved
				continue
			}
			if err := resolveDomainUserConfigValue(child, nextPath); err != nil {
				return err
			}
		}
	case []interface{}:
		for index, child := range typed {
			nextPath := fmt.Sprintf("%s[%d]", path, index)
			if err := resolveDomainUserConfigValue(child, nextPath); err != nil {
				return err
			}
		}
	}
	return nil
}

func resolveDomainUserLocalProvided(kind string) (string, error) {
	switch strings.TrimSpace(kind) {
	case localProvidedDomainUserUUID:
		return uuid.New().String(), nil
	case localProvidedDomainUserPassword:
		return common.Random(16), nil
	case localProvidedDomainUserAuth:
		return common.Random(16), nil
	default:
		return "", fmt.Errorf("unsupported local provided %q", kind)
	}
}

func (s *ClusterDomainUserService) HandleDomainUserUpsert(ctx context.Context, req clustertypes.ActionRequest, payload clustertypes.DomainUserUpsertPayload) (map[string]interface{}, error) {
	domain, err := (&dbClusterStore{}).GetDomainByName(req.Domain)
	if err != nil {
		return nil, err
	}
	result, err := s.ApplyDomainUserUpsert(ctx, domain, payload, req.SourceNodeID, true)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"client_id":  result.ClientID,
		"request_id": result.RequestID,
		"created":    result.Created,
	}, nil
}

func (s *ClusterDomainUserService) HandleDomainUserDelete(ctx context.Context, req clustertypes.ActionRequest, payload clustertypes.DomainUserDeletePayload) (map[string]interface{}, error) {
	domain, err := (&dbClusterStore{}).GetDomainByName(req.Domain)
	if err != nil {
		return nil, err
	}
	result, err := s.ApplyDomainUserDelete(ctx, domain, payload, req.SourceNodeID, true)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"client_id":  result.ClientID,
		"request_id": result.RequestID,
		"deleted":    result.Deleted,
	}, nil
}

func (s *ClusterDomainUserService) resolveInboundIDs(tx *gorm.DB, domainID uint, nodeID string, selectors []string) ([]uint, error) {
	var wrappers []model.ClusterInbound
	if err := tx.Where("domain_id = ?", domainID).Find(&wrappers).Error; err != nil {
		return nil, err
	}
	if len(selectors) == 0 {
		return clusterInboundIDs(wrappers), nil
	}

	ids := make([]uint, 0)
	for _, selector := range selectors {
		selector = strings.TrimSpace(selector)
		if selector == "" {
			continue
		}
		if strings.HasPrefix(selector, "domain:") {
			groupID := strings.TrimSpace(strings.TrimPrefix(selector, "domain:"))
			if id, ok := findClusterInboundByGroup(wrappers, groupID); ok {
				ids = appendUniqueUint(ids, id)
				continue
			}
			if id, ok := findClusterInboundByRequest(wrappers, groupID); ok {
				ids = appendUniqueUint(ids, id)
				continue
			}
			return nil, fmt.Errorf("domain inbound selector %q was not found", selector)
		}
		if strings.HasPrefix(selector, "node:") {
			parts := strings.Split(selector, ":")
			if len(parts) != 3 {
				return nil, fmt.Errorf("node inbound selector %q is invalid", selector)
			}
			if parts[1] != nodeID {
				continue
			}
			value, err := strconv.ParseUint(parts[2], 10, 64)
			if err != nil {
				return nil, fmt.Errorf("node inbound selector %q is invalid", selector)
			}
			ids = appendUniqueUint(ids, uint(value))
			continue
		}
		if value, err := strconv.ParseUint(selector, 10, 64); err == nil {
			ids = appendUniqueUint(ids, uint(value))
			continue
		}
		if id, ok := findClusterInboundByRequest(wrappers, selector); ok {
			ids = appendUniqueUint(ids, id)
			continue
		}
		return nil, fmt.Errorf("inbound selector %q was not found", selector)
	}
	return ids, nil
}

func cloneRawMessage(raw json.RawMessage) json.RawMessage {
	if raw == nil {
		return nil
	}
	out := make(json.RawMessage, len(raw))
	copy(out, raw)
	return out
}

func clusterInboundIDs(wrappers []model.ClusterInbound) []uint {
	ids := make([]uint, 0, len(wrappers))
	for _, wrapper := range wrappers {
		ids = appendUniqueUint(ids, wrapper.InboundID)
	}
	return ids
}

func findClusterInboundByRequest(wrappers []model.ClusterInbound, requestID string) (uint, bool) {
	for _, wrapper := range wrappers {
		if wrapper.RequestID == requestID {
			return wrapper.InboundID, true
		}
	}
	return 0, false
}

func findClusterInboundByGroup(wrappers []model.ClusterInbound, groupID string) (uint, bool) {
	groupID = strings.TrimSpace(groupID)
	for _, wrapper := range wrappers {
		if strings.TrimSpace(wrapper.GroupID) == groupID {
			return wrapper.InboundID, true
		}
	}
	return 0, false
}

func appendUniqueUint(values []uint, next uint) []uint {
	for _, value := range values {
		if value == next {
			return values
		}
	}
	return append(values, next)
}

func unixNow() int64 {
	return time.Now().Unix()
}
