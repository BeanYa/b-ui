package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	clustertypes "github.com/BeanYa/b-ui/src/backend/internal/domain/services/cluster/types"
	database "github.com/BeanYa/b-ui/src/backend/internal/infra/db"
	"github.com/BeanYa/b-ui/src/backend/internal/infra/db/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type ClusterDomainInboundCommandInput struct {
	GroupID string         `json:"group_id"`
	Inbound map[string]any `json:"inbound"`
}

type ClusterDomainUserCommandInput struct {
	User     clustertypes.DomainUserPayload `json:"user"`
	Inbounds []string                       `json:"inbounds,omitempty"`
}

type clusterDomainPeerSender interface {
	SendWithResult(context.Context, *PeerMessage, model.ClusterMember, string) (*clustertypes.DomainResourceCommandResult, error)
}

type ClusterDomainResourceCoordinator struct {
	DB             *gorm.DB
	OperationStore *ClusterDomainOperationStore
	PeerSender     clusterDomainPeerSender
	HubClient      clusterHubClient
	Identity       clusterDomainInboundIdentity
	SecretProvider clusterSecretProvider
	PortAllocator  func() (int, error)
}

func (c *ClusterDomainResourceCoordinator) CreateDomainInbound(ctx context.Context, domainID uint, input ClusterDomainInboundCommandInput) (*ClusterDomainOperationView, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	db := c.db()
	if db == nil {
		return nil, errors.New("cluster domain resource coordinator db is required")
	}
	domain, members, local, err := c.loadDomainContext(domainID)
	if err != nil {
		return nil, err
	}
	payload, payloadMap, err := c.domainInboundCreatePayload(domain, input)
	if err != nil {
		return nil, err
	}
	operationID := payload.RequestID
	desiredPayload, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	store := c.operationStore()
	op := &model.ClusterDomainOperation{
		OperationID:       operationID,
		DomainID:          domain.Id,
		Domain:            domain.Domain,
		ResourceKind:      ClusterDomainResourceInbound,
		ResourceID:        payload.GroupID,
		Action:            ClusterDomainOperationCreate,
		Revision:          domain.LastVersion,
		CoordinatorNodeID: local.NodeID,
		Status:            ClusterDomainOperationDispatching,
		DesiredPayload:    desiredPayload,
	}
	if err := store.SaveOperation(op); err != nil {
		return nil, err
	}

	localResult, localErr := NewClusterDomainInboundService(ClusterDomainInboundServiceOptions{
		DB:            db,
		Identity:      c.identity(),
		PortAllocator: c.PortAllocator,
	}).ApplyDomainInboundCreate(ctx, domain, payload, local.NodeID, false)
	localCommandResult := &clustertypes.DomainResourceCommandResult{
		Status:       "applied",
		OperationID:  operationID,
		NodeID:       local.NodeID,
		MemberID:     local.NodeID,
		ResourceKind: ClusterDomainResourceInbound,
		ResourceID:   payload.GroupID,
		Revision:     domain.LastVersion,
	}
	if localResult != nil {
		localCommandResult.LocalResourceID = localResult.InboundID
	}
	if err := c.saveInstanceResult(store, operationID, model.ClusterMember{NodeID: local.NodeID, DisplayName: local.NodeID}, "", localCommandResult, localErr); err != nil {
		return nil, err
	}

	commandPayload := domainResourcePeerPayload(domain, local.NodeID, operationID, payload.RequestID, ClusterDomainResourceInbound, payload.GroupID, domain.LastVersion, payload.TargetMembers, payloadMap)
	if err := c.dispatchDomainResourceTargets(ctx, store, domain, local, operationID, PeerActionDomainInboundCreate, commandPayload, eligibleDomainResourceTargets(members, local.NodeID), false); err != nil {
		return nil, err
	}

	if _, _, err := store.RecomputeStatus(operationID); err != nil {
		return nil, err
	}
	return c.reportDomainResourceOperation(ctx, store, domain, operationID)
}

func (c *ClusterDomainResourceCoordinator) CreateDomainUser(ctx context.Context, domainID uint, input ClusterDomainUserCommandInput) (*ClusterDomainOperationView, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	db := c.db()
	if db == nil {
		return nil, errors.New("cluster domain resource coordinator db is required")
	}
	domain, members, local, err := c.loadDomainContext(domainID)
	if err != nil {
		return nil, err
	}
	payload, payloadMap, err := c.domainUserUpsertPayload(domain, input)
	if err != nil {
		return nil, err
	}
	operationID := payload.RequestID
	desiredPayload, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	store := c.operationStore()
	op := &model.ClusterDomainOperation{
		OperationID:       operationID,
		DomainID:          domain.Id,
		Domain:            domain.Domain,
		ResourceKind:      ClusterDomainResourceUser,
		ResourceID:        payload.User.UUID,
		Action:            ClusterDomainOperationCreate,
		Revision:          domain.LastVersion,
		CoordinatorNodeID: local.NodeID,
		Status:            ClusterDomainOperationDispatching,
		DesiredPayload:    desiredPayload,
	}
	if err := store.SaveOperation(op); err != nil {
		return nil, err
	}

	localResult, localErr := NewClusterDomainUserService(ClusterDomainUserServiceOptions{
		DB:       db,
		Identity: c.identity(),
	}).ApplyDomainUserUpsert(ctx, domain, payload, local.NodeID, false)
	localCommandResult := &clustertypes.DomainResourceCommandResult{
		Status:       "applied",
		OperationID:  operationID,
		NodeID:       local.NodeID,
		MemberID:     local.NodeID,
		ResourceKind: ClusterDomainResourceUser,
		ResourceID:   payload.User.UUID,
		Revision:     domain.LastVersion,
	}
	if localResult != nil {
		localCommandResult.LocalResourceID = localResult.ClientID
	}
	if err := c.saveInstanceResult(store, operationID, model.ClusterMember{NodeID: local.NodeID, DisplayName: local.NodeID}, "", localCommandResult, localErr); err != nil {
		return nil, err
	}

	commandPayload := domainResourcePeerPayload(domain, local.NodeID, operationID, payload.RequestID, ClusterDomainResourceUser, payload.User.UUID, domain.LastVersion, nil, payloadMap)
	if err := c.dispatchDomainResourceTargets(ctx, store, domain, local, operationID, PeerActionDomainUserUpsert, commandPayload, eligibleDomainResourceTargets(members, local.NodeID), false); err != nil {
		return nil, err
	}

	if _, _, err := store.RecomputeStatus(operationID); err != nil {
		return nil, err
	}
	return c.reportDomainResourceOperation(ctx, store, domain, operationID)
}

func (c *ClusterDomainResourceCoordinator) UpdateDomainInbound(ctx context.Context, domainID uint, groupID string, input ClusterDomainInboundCommandInput) (*ClusterDomainOperationView, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	db := c.db()
	if db == nil {
		return nil, errors.New("cluster domain resource coordinator db is required")
	}
	domain, members, local, err := c.loadDomainContext(domainID)
	if err != nil {
		return nil, err
	}
	payload, payloadMap, err := c.domainInboundUpdatePayload(domain, groupID, input)
	if err != nil {
		return nil, err
	}
	operationID := payload.RequestID
	desiredPayload, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	store := c.operationStore()
	op := &model.ClusterDomainOperation{
		OperationID:       operationID,
		DomainID:          domain.Id,
		Domain:            domain.Domain,
		ResourceKind:      ClusterDomainResourceInbound,
		ResourceID:        payload.GroupID,
		Action:            ClusterDomainOperationUpdate,
		Revision:          domain.LastVersion,
		CoordinatorNodeID: local.NodeID,
		Status:            ClusterDomainOperationDispatching,
		DesiredPayload:    desiredPayload,
	}
	if err := store.SaveOperation(op); err != nil {
		return nil, err
	}

	localResult, localErr := NewClusterDomainInboundService(ClusterDomainInboundServiceOptions{
		DB:            db,
		Identity:      c.identity(),
		PortAllocator: c.PortAllocator,
	}).ApplyDomainInboundUpdate(ctx, domain, payload, local.NodeID, false)
	localCommandResult := &clustertypes.DomainResourceCommandResult{
		Status:       "applied",
		OperationID:  operationID,
		NodeID:       local.NodeID,
		MemberID:     local.NodeID,
		ResourceKind: ClusterDomainResourceInbound,
		ResourceID:   payload.GroupID,
		Revision:     domain.LastVersion,
	}
	if localResult != nil {
		localCommandResult.LocalResourceID = localResult.InboundID
	}
	if err := c.saveInstanceResult(store, operationID, model.ClusterMember{NodeID: local.NodeID, DisplayName: local.NodeID}, "", localCommandResult, localErr); err != nil {
		return nil, err
	}

	commandPayload := domainResourcePeerPayload(domain, local.NodeID, operationID, payload.RequestID, ClusterDomainResourceInbound, payload.GroupID, domain.LastVersion, payload.TargetMembers, payloadMap)
	if err := c.dispatchDomainResourceTargets(ctx, store, domain, local, operationID, PeerActionDomainInboundUpdate, commandPayload, eligibleDomainResourceTargets(members, local.NodeID), false); err != nil {
		return nil, err
	}
	return c.finishDomainResourceOperation(ctx, store, domain, operationID)
}

func (c *ClusterDomainResourceCoordinator) DeleteDomainInbound(ctx context.Context, domainID uint, groupID string) (*ClusterDomainOperationView, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	db := c.db()
	if db == nil {
		return nil, errors.New("cluster domain resource coordinator db is required")
	}
	domain, members, local, err := c.loadDomainContext(domainID)
	if err != nil {
		return nil, err
	}
	payload, payloadMap, err := c.domainInboundDeletePayload(domain, groupID)
	if err != nil {
		return nil, err
	}
	operationID := payload.RequestID
	desiredPayload, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	store := c.operationStore()
	op := &model.ClusterDomainOperation{
		OperationID:       operationID,
		DomainID:          domain.Id,
		Domain:            domain.Domain,
		ResourceKind:      ClusterDomainResourceInbound,
		ResourceID:        payload.GroupID,
		Action:            ClusterDomainOperationDelete,
		Revision:          domain.LastVersion,
		CoordinatorNodeID: local.NodeID,
		Status:            ClusterDomainOperationDispatching,
		DesiredPayload:    desiredPayload,
	}
	if err := store.SaveOperation(op); err != nil {
		return nil, err
	}

	localErr := NewClusterDomainInboundService(ClusterDomainInboundServiceOptions{
		DB:            db,
		Identity:      c.identity(),
		PortAllocator: c.PortAllocator,
	}).ApplyDomainInboundDelete(ctx, domain, payload, local.NodeID, false)
	localCommandResult := &clustertypes.DomainResourceCommandResult{
		Status:       "applied",
		OperationID:  operationID,
		NodeID:       local.NodeID,
		MemberID:     local.NodeID,
		ResourceKind: ClusterDomainResourceInbound,
		ResourceID:   payload.GroupID,
		Revision:     domain.LastVersion,
	}
	if err := c.saveInstanceResult(store, operationID, model.ClusterMember{NodeID: local.NodeID, DisplayName: local.NodeID}, "", localCommandResult, localErr); err != nil {
		return nil, err
	}

	commandPayload := domainResourcePeerPayload(domain, local.NodeID, operationID, payload.RequestID, ClusterDomainResourceInbound, payload.GroupID, domain.LastVersion, payload.TargetMembers, payloadMap)
	if err := c.dispatchDomainResourceTargets(ctx, store, domain, local, operationID, PeerActionDomainInboundDelete, commandPayload, eligibleDomainResourceTargets(members, local.NodeID), false); err != nil {
		return nil, err
	}
	return c.finishDomainResourceOperation(ctx, store, domain, operationID)
}

func (c *ClusterDomainResourceCoordinator) UpdateDomainUser(ctx context.Context, domainID uint, userUUID string, input ClusterDomainUserCommandInput) (*ClusterDomainOperationView, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	db := c.db()
	if db == nil {
		return nil, errors.New("cluster domain resource coordinator db is required")
	}
	domain, members, local, err := c.loadDomainContext(domainID)
	if err != nil {
		return nil, err
	}
	input.User.UUID = strings.TrimSpace(userUUID)
	payload, payloadMap, err := c.domainUserUpsertPayload(domain, input)
	if err != nil {
		return nil, err
	}
	operationID := payload.RequestID
	desiredPayload, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	store := c.operationStore()
	op := &model.ClusterDomainOperation{
		OperationID:       operationID,
		DomainID:          domain.Id,
		Domain:            domain.Domain,
		ResourceKind:      ClusterDomainResourceUser,
		ResourceID:        payload.User.UUID,
		Action:            ClusterDomainOperationUpdate,
		Revision:          domain.LastVersion,
		CoordinatorNodeID: local.NodeID,
		Status:            ClusterDomainOperationDispatching,
		DesiredPayload:    desiredPayload,
	}
	if err := store.SaveOperation(op); err != nil {
		return nil, err
	}

	localResult, localErr := NewClusterDomainUserService(ClusterDomainUserServiceOptions{
		DB:       db,
		Identity: c.identity(),
	}).ApplyDomainUserUpsert(ctx, domain, payload, local.NodeID, false)
	localCommandResult := &clustertypes.DomainResourceCommandResult{
		Status:       "applied",
		OperationID:  operationID,
		NodeID:       local.NodeID,
		MemberID:     local.NodeID,
		ResourceKind: ClusterDomainResourceUser,
		ResourceID:   payload.User.UUID,
		Revision:     domain.LastVersion,
	}
	if localResult != nil {
		localCommandResult.LocalResourceID = localResult.ClientID
	}
	if err := c.saveInstanceResult(store, operationID, model.ClusterMember{NodeID: local.NodeID, DisplayName: local.NodeID}, "", localCommandResult, localErr); err != nil {
		return nil, err
	}

	commandPayload := domainResourcePeerPayload(domain, local.NodeID, operationID, payload.RequestID, ClusterDomainResourceUser, payload.User.UUID, domain.LastVersion, nil, payloadMap)
	if err := c.dispatchDomainResourceTargets(ctx, store, domain, local, operationID, PeerActionDomainUserUpsert, commandPayload, eligibleDomainResourceTargets(members, local.NodeID), false); err != nil {
		return nil, err
	}
	return c.finishDomainResourceOperation(ctx, store, domain, operationID)
}

func (c *ClusterDomainResourceCoordinator) DeleteDomainUser(ctx context.Context, domainID uint, userUUID string) (*ClusterDomainOperationView, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	db := c.db()
	if db == nil {
		return nil, errors.New("cluster domain resource coordinator db is required")
	}
	domain, members, local, err := c.loadDomainContext(domainID)
	if err != nil {
		return nil, err
	}
	payload, payloadMap, err := c.domainUserDeletePayload(domain, userUUID)
	if err != nil {
		return nil, err
	}
	operationID := payload.RequestID
	desiredPayload, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	store := c.operationStore()
	op := &model.ClusterDomainOperation{
		OperationID:       operationID,
		DomainID:          domain.Id,
		Domain:            domain.Domain,
		ResourceKind:      ClusterDomainResourceUser,
		ResourceID:        payload.UUID,
		Action:            ClusterDomainOperationDelete,
		Revision:          domain.LastVersion,
		CoordinatorNodeID: local.NodeID,
		Status:            ClusterDomainOperationDispatching,
		DesiredPayload:    desiredPayload,
	}
	if err := store.SaveOperation(op); err != nil {
		return nil, err
	}

	localResult, localErr := NewClusterDomainUserService(ClusterDomainUserServiceOptions{
		DB:       db,
		Identity: c.identity(),
	}).ApplyDomainUserDelete(ctx, domain, payload, local.NodeID, false)
	localCommandResult := &clustertypes.DomainResourceCommandResult{
		Status:       "applied",
		OperationID:  operationID,
		NodeID:       local.NodeID,
		MemberID:     local.NodeID,
		ResourceKind: ClusterDomainResourceUser,
		ResourceID:   payload.UUID,
		Revision:     domain.LastVersion,
	}
	if localResult != nil {
		localCommandResult.LocalResourceID = localResult.ClientID
	}
	if err := c.saveInstanceResult(store, operationID, model.ClusterMember{NodeID: local.NodeID, DisplayName: local.NodeID}, "", localCommandResult, localErr); err != nil {
		return nil, err
	}

	commandPayload := domainResourcePeerPayload(domain, local.NodeID, operationID, payload.RequestID, ClusterDomainResourceUser, payload.UUID, domain.LastVersion, nil, payloadMap)
	if err := c.dispatchDomainResourceTargets(ctx, store, domain, local, operationID, PeerActionDomainUserDelete, commandPayload, eligibleDomainResourceTargets(members, local.NodeID), false); err != nil {
		return nil, err
	}
	return c.finishDomainResourceOperation(ctx, store, domain, operationID)
}

func (c *ClusterDomainResourceCoordinator) RetryDomainOperation(ctx context.Context, operationID string) (*ClusterDomainOperationView, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	operationID = strings.TrimSpace(operationID)
	if operationID == "" {
		return nil, errors.New("operation_id is required")
	}

	store := c.operationStore()
	op, err := store.GetOperation(operationID)
	if err != nil {
		return nil, err
	}
	action, resourceKind, resourceID, requestID, targetMembers, payloadMap, err := domainOperationRetryPayload(op)
	if err != nil {
		return nil, err
	}

	domain, members, local, err := c.loadDomainContext(op.DomainID)
	if err != nil {
		return nil, err
	}
	commandPayload := domainResourcePeerPayload(domain, local.NodeID, operationID, requestID, resourceKind, resourceID, domain.LastVersion, targetMembers, payloadMap)
	if err := c.dispatchDomainResourceTargets(ctx, store, domain, local, operationID, action, commandPayload, c.retryTargets(operationID, members, local.NodeID), true); err != nil {
		return nil, err
	}

	if _, _, err := store.RecomputeStatus(operationID); err != nil {
		return nil, err
	}
	return c.reportDomainResourceOperation(ctx, store, domain, operationID)
}

func (c *ClusterDomainResourceCoordinator) retryTargets(operationID string, members []model.ClusterMember, localNodeID string) []model.ClusterMember {
	instances, err := c.operationStore().ListInstances(operationID)
	if err != nil || len(instances) == 0 {
		return eligibleDomainResourceTargets(members, localNodeID)
	}
	retryNodes := map[string]struct{}{}
	for _, instance := range instances {
		switch instance.Status {
		case ClusterDomainOperationFailed, ClusterDomainOperationTimeout:
			retryNodes[instance.NodeID] = struct{}{}
		}
	}
	targets := make([]model.ClusterMember, 0, len(retryNodes))
	for _, member := range eligibleDomainResourceTargets(members, localNodeID) {
		if _, ok := retryNodes[member.NodeID]; ok {
			targets = append(targets, member)
		}
	}
	return targets
}

func (c *ClusterDomainResourceCoordinator) loadDomainContext(domainID uint) (*model.ClusterDomain, []model.ClusterMember, *model.ClusterLocalNode, error) {
	if domainID == 0 {
		return nil, nil, nil, errors.New("domain_id is required")
	}
	domain := &model.ClusterDomain{}
	if err := c.db().First(domain, domainID).Error; err != nil {
		return nil, nil, nil, err
	}
	local, err := c.identity().GetOrCreate()
	if err != nil {
		return nil, nil, nil, err
	}
	if local == nil || strings.TrimSpace(local.NodeID) == "" {
		return nil, nil, nil, errors.New("local node identity is required")
	}
	var members []model.ClusterMember
	if err := c.db().Where("domain_id = ?", domain.Id).Order("id asc").Find(&members).Error; err != nil {
		return nil, nil, nil, err
	}
	return domain, members, local, nil
}

func (c *ClusterDomainResourceCoordinator) domainInboundCreatePayload(domain *model.ClusterDomain, input ClusterDomainInboundCommandInput) (clustertypes.DomainInboundCreatePayload, map[string]interface{}, error) {
	input.GroupID = strings.TrimSpace(input.GroupID)
	if input.GroupID == "" {
		return clustertypes.DomainInboundCreatePayload{}, nil, errors.New("group_id is required")
	}
	if len(input.Inbound) == 0 {
		return clustertypes.DomainInboundCreatePayload{}, nil, errors.New("inbound is required")
	}
	inbound, err := json.Marshal(input.Inbound)
	if err != nil {
		return clustertypes.DomainInboundCreatePayload{}, nil, err
	}
	payload := clustertypes.DomainInboundCreatePayload{
		RequestID: fmt.Sprintf("domain-inbound-%s", uuid.New().String()),
		DomainID:  domain.Domain,
		GroupID:   input.GroupID,
		Inbound:   inbound,
	}
	payloadMap, err := domainInboundPayloadMap(payload)
	if err != nil {
		return clustertypes.DomainInboundCreatePayload{}, nil, err
	}
	return payload, payloadMap, nil
}

func (c *ClusterDomainResourceCoordinator) domainInboundUpdatePayload(domain *model.ClusterDomain, groupID string, input ClusterDomainInboundCommandInput) (clustertypes.DomainInboundCreatePayload, map[string]interface{}, error) {
	input.GroupID = strings.TrimSpace(groupID)
	return c.domainInboundCreatePayload(domain, input)
}

func (c *ClusterDomainResourceCoordinator) domainInboundDeletePayload(domain *model.ClusterDomain, groupID string) (clustertypes.DomainInboundDeletePayload, map[string]interface{}, error) {
	groupID = strings.TrimSpace(groupID)
	if groupID == "" {
		return clustertypes.DomainInboundDeletePayload{}, nil, errors.New("group_id is required")
	}
	payload := clustertypes.DomainInboundDeletePayload{
		RequestID: fmt.Sprintf("domain-inbound-%s", uuid.New().String()),
		DomainID:  domain.Domain,
		GroupID:   groupID,
	}
	payloadMap, err := domainInboundPayloadMap(payload)
	if err != nil {
		return clustertypes.DomainInboundDeletePayload{}, nil, err
	}
	return payload, payloadMap, nil
}

func (c *ClusterDomainResourceCoordinator) domainUserUpsertPayload(domain *model.ClusterDomain, input ClusterDomainUserCommandInput) (clustertypes.DomainUserUpsertPayload, map[string]interface{}, error) {
	input.User.UUID = strings.TrimSpace(input.User.UUID)
	input.User.Name = strings.TrimSpace(input.User.Name)
	if input.User.UUID == "" {
		input.User.UUID = uuid.New().String()
	}
	if input.User.Name == "" {
		return clustertypes.DomainUserUpsertPayload{}, nil, errors.New("user name is required")
	}
	if len(strings.TrimSpace(string(input.User.Config))) == 0 {
		input.User.Config = defaultDomainUserConfig(input.User.UUID)
	}
	payload := clustertypes.DomainUserUpsertPayload{
		RequestID: fmt.Sprintf("domain-user-%s", uuid.New().String()),
		DomainID:  domain.Domain,
		User:      input.User,
		Inbounds:  input.Inbounds,
	}
	payloadMap, err := domainInboundPayloadMap(payload)
	if err != nil {
		return clustertypes.DomainUserUpsertPayload{}, nil, err
	}
	return payload, payloadMap, nil
}

func defaultDomainUserConfig(seed string) json.RawMessage {
	if strings.TrimSpace(seed) == "" {
		seed = uuid.New().String()
	}
	cfg := map[string]map[string]string{
		"vless": {"uuid": seed},
		"vmess": {"uuid": uuid.New().String()},
	}
	raw, err := json.Marshal(cfg)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return raw
}

func (c *ClusterDomainResourceCoordinator) domainUserDeletePayload(domain *model.ClusterDomain, userUUID string) (clustertypes.DomainUserDeletePayload, map[string]interface{}, error) {
	userUUID = strings.TrimSpace(userUUID)
	if userUUID == "" {
		return clustertypes.DomainUserDeletePayload{}, nil, errors.New("user uuid is required")
	}
	payload := clustertypes.DomainUserDeletePayload{
		RequestID: fmt.Sprintf("domain-user-%s", uuid.New().String()),
		DomainID:  domain.Domain,
		UUID:      userUUID,
	}
	payloadMap, err := domainInboundPayloadMap(payload)
	if err != nil {
		return clustertypes.DomainUserDeletePayload{}, nil, err
	}
	return payload, payloadMap, nil
}

func (c *ClusterDomainResourceCoordinator) finishDomainResourceOperation(ctx context.Context, store *ClusterDomainOperationStore, domain *model.ClusterDomain, operationID string) (*ClusterDomainOperationView, error) {
	if _, _, err := store.RecomputeStatus(operationID); err != nil {
		return nil, err
	}
	return c.reportDomainResourceOperation(ctx, store, domain, operationID)
}

func (c *ClusterDomainResourceCoordinator) reportDomainResourceOperation(ctx context.Context, store *ClusterDomainOperationStore, domain *model.ClusterDomain, operationID string) (*ClusterDomainOperationView, error) {
	view, err := store.GetOperationView(operationID)
	if err != nil {
		return nil, err
	}
	_ = c.ReportDomainResourceState(ctx, domain, view)
	return view, nil
}

func (c *ClusterDomainResourceCoordinator) dispatchDomainResourceTargets(ctx context.Context, store *ClusterDomainOperationStore, domain *model.ClusterDomain, local *model.ClusterLocalNode, operationID string, action string, payloadMap map[string]interface{}, targets []model.ClusterMember, retry bool) error {
	secret, secretErr := c.secretProvider().GetSecret()
	for _, member := range targets {
		message, err := NewClusterPeerMessage(domain.Domain, member.LastVersion, local.NodeID, domain.LastVersion, PeerCategoryCommand, action, payloadMap)
		if err != nil {
			return err
		}
		message.Route = RoutePlan{Mode: RouteModeDirect, Targets: []string{member.NodeID}}
		message.IdempotencyKey = "domain-resource:" + operationID + ":" + member.NodeID
		if retry {
			message.IdempotencyKey += ":retry"
		}
		if err := SignClusterPeerMessage(local, message); err != nil {
			return err
		}

		var result *clustertypes.DomainResourceCommandResult
		sendErr := secretErr
		if sendErr == nil {
			token, err := DecryptClusterDomainToken(secret, member.PeerTokenEncrypted)
			if err != nil {
				sendErr = err
			} else {
				result, sendErr = c.peerSender().SendWithResult(ctx, message, member, token)
			}
		}
		if err := c.saveInstanceResult(store, operationID, member, "", result, sendErr); err != nil {
			return err
		}
	}
	return nil
}

func domainOperationRetryPayload(op *model.ClusterDomainOperation) (string, string, string, string, []clustertypes.DomainInboundTarget, map[string]interface{}, error) {
	switch {
	case op.ResourceKind == ClusterDomainResourceInbound && op.Action == ClusterDomainOperationCreate:
		var payload clustertypes.DomainInboundCreatePayload
		if err := json.Unmarshal(op.DesiredPayload, &payload); err != nil {
			return "", "", "", "", nil, nil, err
		}
		if strings.TrimSpace(payload.RequestID) == "" {
			payload.RequestID = op.OperationID
		}
		payloadMap, err := domainInboundPayloadMap(payload)
		return PeerActionDomainInboundCreate, ClusterDomainResourceInbound, payload.GroupID, payload.RequestID, payload.TargetMembers, payloadMap, err
	case op.ResourceKind == ClusterDomainResourceInbound && op.Action == ClusterDomainOperationUpdate:
		var payload clustertypes.DomainInboundUpdatePayload
		if err := json.Unmarshal(op.DesiredPayload, &payload); err != nil {
			return "", "", "", "", nil, nil, err
		}
		if strings.TrimSpace(payload.RequestID) == "" {
			payload.RequestID = op.OperationID
		}
		payloadMap, err := domainInboundPayloadMap(payload)
		return PeerActionDomainInboundUpdate, ClusterDomainResourceInbound, payload.GroupID, payload.RequestID, payload.TargetMembers, payloadMap, err
	case op.ResourceKind == ClusterDomainResourceInbound && op.Action == ClusterDomainOperationDelete:
		var payload clustertypes.DomainInboundDeletePayload
		if err := json.Unmarshal(op.DesiredPayload, &payload); err != nil {
			return "", "", "", "", nil, nil, err
		}
		if strings.TrimSpace(payload.RequestID) == "" {
			payload.RequestID = op.OperationID
		}
		payloadMap, err := domainInboundPayloadMap(payload)
		return PeerActionDomainInboundDelete, ClusterDomainResourceInbound, payload.GroupID, payload.RequestID, payload.TargetMembers, payloadMap, err
	case op.ResourceKind == ClusterDomainResourceUser && (op.Action == ClusterDomainOperationCreate || op.Action == ClusterDomainOperationUpdate):
		var payload clustertypes.DomainUserUpsertPayload
		if err := json.Unmarshal(op.DesiredPayload, &payload); err != nil {
			return "", "", "", "", nil, nil, err
		}
		if strings.TrimSpace(payload.RequestID) == "" {
			payload.RequestID = op.OperationID
		}
		payloadMap, err := domainInboundPayloadMap(payload)
		return PeerActionDomainUserUpsert, ClusterDomainResourceUser, payload.User.UUID, payload.RequestID, nil, payloadMap, err
	case op.ResourceKind == ClusterDomainResourceUser && op.Action == ClusterDomainOperationDelete:
		var payload clustertypes.DomainUserDeletePayload
		if err := json.Unmarshal(op.DesiredPayload, &payload); err != nil {
			return "", "", "", "", nil, nil, err
		}
		if strings.TrimSpace(payload.RequestID) == "" {
			payload.RequestID = op.OperationID
		}
		payloadMap, err := domainInboundPayloadMap(payload)
		return PeerActionDomainUserDelete, ClusterDomainResourceUser, payload.UUID, payload.RequestID, nil, payloadMap, err
	default:
		return "", "", "", "", nil, nil, errors.New("unsupported domain operation retry")
	}
}

func domainResourcePeerPayload(domain *model.ClusterDomain, coordinatorNodeID string, operationID string, requestID string, resourceKind string, resourceID string, revision int64, targetMembers []clustertypes.DomainInboundTarget, payload map[string]interface{}) map[string]interface{} {
	out := map[string]interface{}{
		"operation_id":        operationID,
		"request_id":          requestID,
		"domain_id":           domain.Domain,
		"resource_kind":       resourceKind,
		"resource_id":         resourceID,
		"revision":            revision,
		"coordinator_node_id": coordinatorNodeID,
		"payload":             payload,
	}
	if targetMembers != nil {
		out["target_members"] = targetMembers
	}
	return out
}

func (c *ClusterDomainResourceCoordinator) saveInstanceResult(store *ClusterDomainOperationStore, operationID string, member model.ClusterMember, targetTag string, result *clustertypes.DomainResourceCommandResult, resultErr error) error {
	status, errorMessage := clusterDomainResultStatus(result, resultErr)
	responseJSON, err := json.Marshal(result)
	if err != nil {
		return err
	}
	attemptCount := c.nextInstanceAttemptCount(store, operationID, member.NodeID)
	instance := &model.ClusterDomainOperationInstance{
		OperationID:     operationID,
		MemberID:        member.NodeID,
		NodeID:          member.NodeID,
		DisplayName:     clusterDomainMemberDisplayName(member),
		TargetTag:       targetTag,
		Status:          status,
		AttemptCount:    attemptCount,
		ResponseJSON:    responseJSON,
		Error:           errorMessage,
		LastAttemptAt:   time.Now().Unix(),
		LocalResourceID: 0,
	}
	if result != nil {
		if result.MemberID != "" {
			instance.MemberID = result.MemberID
		}
		if result.TargetTag != "" {
			instance.TargetTag = result.TargetTag
		}
		instance.LocalResourceID = result.LocalResourceID
	}
	if instance.MemberID == "" {
		instance.MemberID = member.NodeID
	}
	return store.SaveInstance(instance)
}

func (c *ClusterDomainResourceCoordinator) nextInstanceAttemptCount(store *ClusterDomainOperationStore, operationID string, nodeID string) int {
	instances, err := store.ListInstances(operationID)
	if err != nil {
		return 1
	}
	for _, instance := range instances {
		if instance.NodeID == nodeID {
			return instance.AttemptCount + 1
		}
	}
	return 1
}

func clusterDomainResultStatus(result *clustertypes.DomainResourceCommandResult, err error) (string, string) {
	if err != nil {
		return ClusterDomainOperationFailed, err.Error()
	}
	if result == nil {
		return ClusterDomainOperationApplied, ""
	}
	switch result.Status {
	case "applied", "skipped":
		return ClusterDomainOperationApplied, ""
	case "missing":
		return ClusterDomainOperationFailed, "missing"
	case "failed":
		if result.Error != "" {
			return ClusterDomainOperationFailed, result.Error
		}
		return ClusterDomainOperationFailed, "peer apply failed"
	default:
		return ClusterDomainOperationFailed, "unknown peer result status"
	}
}

func eligibleDomainResourceTargets(members []model.ClusterMember, localNodeID string) []model.ClusterMember {
	targets := make([]model.ClusterMember, 0, len(members))
	for _, member := range members {
		if member.NodeID == "" || member.NodeID == localNodeID || member.BaseURL == "" {
			continue
		}
		targets = append(targets, member)
	}
	return targets
}

func clusterDomainMemberDisplayName(member model.ClusterMember) string {
	if strings.TrimSpace(member.DisplayName) != "" {
		return member.DisplayName
	}
	if strings.TrimSpace(member.Name) != "" {
		return member.Name
	}
	return member.NodeID
}

func (c *ClusterDomainResourceCoordinator) db() *gorm.DB {
	if c != nil && c.DB != nil {
		return c.DB
	}
	return database.GetDB()
}

func (c *ClusterDomainResourceCoordinator) operationStore() *ClusterDomainOperationStore {
	if c != nil && c.OperationStore != nil {
		return c.OperationStore
	}
	return &ClusterDomainOperationStore{DB: c.db()}
}

func (c *ClusterDomainResourceCoordinator) peerSender() clusterDomainPeerSender {
	if c != nil && c.PeerSender != nil {
		return c.PeerSender
	}
	return &ClusterPeerDeliveryService{}
}

func (c *ClusterDomainResourceCoordinator) identity() clusterDomainInboundIdentity {
	if c != nil && c.Identity != nil {
		return c.Identity
	}
	return &ClusterLocalIdentityService{}
}

func (c *ClusterDomainResourceCoordinator) secretProvider() clusterSecretProvider {
	if c != nil && c.SecretProvider != nil {
		return c.SecretProvider
	}
	return &SettingService{}
}
