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

const domainResourceReportTimeout = 10 * time.Second

type ClusterDomainInboundCommandInput struct {
	GroupID         string                             `json:"group_id"`
	TagSeed         string                             `json:"tag_seed"`
	TargetMembers   []clustertypes.DomainInboundTarget `json:"target_members"`
	Prefix          string                             `json:"prefix"`
	Suffix          string                             `json:"suffix"`
	IncludeProtocol bool                               `json:"include_protocol,omitempty"`
	IncludeSecurity bool                               `json:"include_security,omitempty"`
	IncludeFlag     bool                               `json:"include_flag,omitempty"`
	Inbound         map[string]any                     `json:"inbound"`
	TLSTemplate     string                             `json:"tls_template"`
	TLS             *clustertypes.DomainInboundTLS     `json:"tls,omitempty"`
}

type ClusterDomainUserCommandInput struct {
	User                 clustertypes.DomainUserPayload `json:"user"`
	BoundInboundGroupIDs []string                       `json:"bound_inbound_group_ids,omitempty"`
	Inbounds             []string                       `json:"inbounds,omitempty"`
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
	ProxyReporter  clusterDomainInboundReporter
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
	input.TargetMembers = enrichDomainInboundTargetCountries(input.TargetMembers, members)
	payload, payloadMap, err := c.domainInboundCreatePayload(domain, input)
	if err != nil {
		return nil, err
	}
	if err := validateSelectedDomainResourceTargets(payload.TargetMembers, members, local.NodeID); err != nil {
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

	if shouldApplyLocalDomainResourceTarget(payload.TargetMembers, local.NodeID) {
		localResult, localErr := NewClusterDomainInboundService(ClusterDomainInboundServiceOptions{
			DB:            db,
			Identity:      c.identity(),
			PortAllocator: c.PortAllocator,
			Reporter:      c.ProxyReporter,
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
	}

	commandPayload := domainResourcePeerPayload(domain, local.NodeID, operationID, payload.RequestID, ClusterDomainResourceInbound, payload.GroupID, domain.LastVersion, payload.TargetMembers, payloadMap)
	if err := c.dispatchDomainResourceTargets(ctx, store, domain, local, operationID, PeerActionDomainInboundCreate, commandPayload, selectedDomainResourceTargets(members, local.NodeID, payload.TargetMembers), false); err != nil {
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
	boundGroupResources, err := c.domainUserBoundGroupResources(domain.Id, payload.User.BoundInboundGroupIDs)
	if err != nil {
		return nil, err
	}
	applyLocal, peerTargets, err := domainUserBoundGroupTargets(members, local.NodeID, payload.User.BoundInboundGroupIDs, boundGroupResources)
	if err != nil {
		return nil, err
	}
	if len(payload.User.BoundInboundGroupIDs) > 0 {
		payload.TargetMembers = domainUserTargetMembers(local.NodeID, applyLocal, peerTargets)
	}
	payloadMap, err = domainInboundPayloadMap(payload)
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

	if applyLocal {
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
			localCommandResult.UserConfig = cloneRawMessage(localResult.Config)
			localCommandResult.SubToken = localResult.SubToken
		}
		if err := c.saveInstanceResult(store, operationID, model.ClusterMember{NodeID: local.NodeID, DisplayName: local.NodeID}, "", localCommandResult, localErr); err != nil {
			return nil, err
		}
	}

	commandPayload := domainResourcePeerPayload(domain, local.NodeID, operationID, payload.RequestID, ClusterDomainResourceUser, payload.User.UUID, domain.LastVersion, payload.TargetMembers, payloadMap)
	if err := c.dispatchDomainResourceTargets(ctx, store, domain, local, operationID, PeerActionDomainUserUpsert, commandPayload, peerTargets, false); err != nil {
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
	input.TargetMembers = enrichDomainInboundTargetCountries(input.TargetMembers, members)
	payload, payloadMap, err := c.domainInboundUpdatePayload(domain, groupID, input)
	if err != nil {
		return nil, err
	}
	if err := validateSelectedDomainResourceTargets(payload.TargetMembers, members, local.NodeID); err != nil {
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

	if shouldApplyLocalDomainResourceTarget(payload.TargetMembers, local.NodeID) {
		localResult, localErr := NewClusterDomainInboundService(ClusterDomainInboundServiceOptions{
			DB:            db,
			Identity:      c.identity(),
			PortAllocator: c.PortAllocator,
			Reporter:      c.ProxyReporter,
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
	}

	commandPayload := domainResourcePeerPayload(domain, local.NodeID, operationID, payload.RequestID, ClusterDomainResourceInbound, payload.GroupID, domain.LastVersion, payload.TargetMembers, payloadMap)
	if err := c.dispatchDomainResourceTargets(ctx, store, domain, local, operationID, PeerActionDomainInboundUpdate, commandPayload, selectedDomainResourceTargets(members, local.NodeID, payload.TargetMembers), false); err != nil {
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
	if err := validateSelectedDomainResourceTargets(payload.TargetMembers, members, local.NodeID); err != nil {
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

	if shouldApplyLocalDomainResourceTarget(payload.TargetMembers, local.NodeID) {
		localErr := NewClusterDomainInboundService(ClusterDomainInboundServiceOptions{
			DB:            db,
			Identity:      c.identity(),
			PortAllocator: c.PortAllocator,
			Reporter:      c.ProxyReporter,
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
	}

	commandPayload := domainResourcePeerPayload(domain, local.NodeID, operationID, payload.RequestID, ClusterDomainResourceInbound, payload.GroupID, domain.LastVersion, payload.TargetMembers, payloadMap)
	if err := c.dispatchDomainResourceTargets(ctx, store, domain, local, operationID, PeerActionDomainInboundDelete, commandPayload, selectedDomainResourceTargets(members, local.NodeID, payload.TargetMembers), false); err != nil {
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
	boundGroupResources, err := c.domainUserBoundGroupResources(domain.Id, payload.User.BoundInboundGroupIDs)
	if err != nil {
		return nil, err
	}
	applyLocal, peerTargets, err := domainUserBoundGroupTargets(members, local.NodeID, payload.User.BoundInboundGroupIDs, boundGroupResources)
	if err != nil {
		return nil, err
	}
	if len(payload.User.BoundInboundGroupIDs) > 0 {
		payload.TargetMembers = domainUserTargetMembers(local.NodeID, applyLocal, peerTargets)
	}
	payloadMap, err = domainInboundPayloadMap(payload)
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

	if applyLocal {
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
			localCommandResult.UserConfig = cloneRawMessage(localResult.Config)
			localCommandResult.SubToken = localResult.SubToken
		}
		if err := c.saveInstanceResult(store, operationID, model.ClusterMember{NodeID: local.NodeID, DisplayName: local.NodeID}, "", localCommandResult, localErr); err != nil {
			return nil, err
		}
	}

	commandPayload := domainResourcePeerPayload(domain, local.NodeID, operationID, payload.RequestID, ClusterDomainResourceUser, payload.User.UUID, domain.LastVersion, payload.TargetMembers, payloadMap)
	if err := c.dispatchDomainResourceTargets(ctx, store, domain, local, operationID, PeerActionDomainUserUpsert, commandPayload, peerTargets, false); err != nil {
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
	resources, err := c.buildDomainResources(domain.Id)
	if err != nil {
		return nil, err
	}
	applyLocal, peerTargets := domainUserMaterializedTargets(members, local.NodeID, payload.UUID, resources.Users)
	if !applyLocal && len(peerTargets) == 0 {
		applyLocal = true
		peerTargets = eligibleDomainResourceTargets(members, local.NodeID)
	}
	payload.TargetMembers = domainUserTargetMembers(local.NodeID, applyLocal, peerTargets)
	payloadMap, err = domainInboundPayloadMap(payload)
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

	if applyLocal {
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
	}

	commandPayload := domainResourcePeerPayload(domain, local.NodeID, operationID, payload.RequestID, ClusterDomainResourceUser, payload.UUID, domain.LastVersion, payload.TargetMembers, payloadMap)
	if err := c.dispatchDomainResourceTargets(ctx, store, domain, local, operationID, PeerActionDomainUserDelete, commandPayload, peerTargets, false); err != nil {
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
	if err := c.dispatchDomainResourceTargets(ctx, store, domain, local, operationID, action, commandPayload, c.retryTargets(operationID, members, local.NodeID, targetMembers), true); err != nil {
		return nil, err
	}

	if _, _, err := store.RecomputeStatus(operationID); err != nil {
		return nil, err
	}
	return c.reportDomainResourceOperation(ctx, store, domain, operationID)
}

func (c *ClusterDomainResourceCoordinator) ListDomainResources(ctx context.Context, domainID uint) (ClusterHubDomainResources, error) {
	if err := ctx.Err(); err != nil {
		return ClusterHubDomainResources{}, err
	}
	db := c.db()
	if db == nil {
		return ClusterHubDomainResources{}, errors.New("cluster domain resource coordinator db is required")
	}
	var domain model.ClusterDomain
	if err := db.First(&domain, domainID).Error; err != nil {
		return ClusterHubDomainResources{}, err
	}
	return c.buildDomainResources(domainID)
}

func (c *ClusterDomainResourceCoordinator) retryTargets(operationID string, members []model.ClusterMember, localNodeID string, targetMembers []clustertypes.DomainInboundTarget) []model.ClusterMember {
	instances, err := c.operationStore().ListInstances(operationID)
	if err != nil || len(instances) == 0 {
		return selectedDomainResourceTargets(members, localNodeID, targetMembers)
	}
	retryNodes := map[string]struct{}{}
	for _, instance := range instances {
		switch instance.Status {
		case ClusterDomainOperationFailed, ClusterDomainOperationTimeout:
			retryNodes[instance.NodeID] = struct{}{}
		}
	}
	targets := make([]model.ClusterMember, 0, len(retryNodes))
	for _, member := range selectedDomainResourceTargets(members, localNodeID, targetMembers) {
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

// enrichDomainInboundTargetCountries copies each target's CountryCode from the
// matching ClusterMember (matched by NodeID). A CountryCode already present on a
// target is preserved; targets without a matching member, or whose member has an
// empty CountryCode, are left unchanged so the flag segment is gracefully omitted.
func enrichDomainInboundTargetCountries(targets []clustertypes.DomainInboundTarget, members []model.ClusterMember) []clustertypes.DomainInboundTarget {
	if len(targets) == 0 || len(members) == 0 {
		return targets
	}
	byNode := make(map[string]string, len(members))
	for _, member := range members {
		nodeID := strings.TrimSpace(member.NodeID)
		if nodeID == "" {
			continue
		}
		if _, exists := byNode[nodeID]; !exists {
			byNode[nodeID] = member.CountryCode
		}
	}
	for i := range targets {
		if strings.TrimSpace(targets[i].CountryCode) != "" {
			continue
		}
		if code, ok := byNode[strings.TrimSpace(targets[i].NodeID)]; ok {
			targets[i].CountryCode = code
		}
	}
	return targets
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
		RequestID:       fmt.Sprintf("domain-inbound-%s", uuid.New().String()),
		DomainID:        domain.Domain,
		GroupID:         input.GroupID,
		TagSeed:         strings.TrimSpace(input.TagSeed),
		TargetMembers:   input.TargetMembers,
		Prefix:          strings.TrimSpace(input.Prefix),
		Suffix:          strings.TrimSpace(input.Suffix),
		IncludeProtocol: input.IncludeProtocol,
		IncludeSecurity: input.IncludeSecurity,
		IncludeFlag:     input.IncludeFlag,
		Inbound:         inbound,
		TLSTemplate:     strings.TrimSpace(input.TLSTemplate),
		TLS:             input.TLS,
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
	boundGroups := normalizeDomainUserBoundGroups(input.User.BoundInboundGroupIDs, input.BoundInboundGroupIDs)
	boundGroups = normalizeDomainUserBoundGroups(boundGroups, input.Inbounds)
	input.User.BoundInboundGroupIDs = boundGroups
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
		Inbounds:  domainUserGroupSelectors(boundGroups),
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
	reportCtx, cancel := domainResourceReportContext(ctx)
	defer cancel()
	_ = c.ReportDomainResourceState(reportCtx, domain, view)
	return view, nil
}

func domainResourceReportContext(context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), domainResourceReportTimeout)
}

func (c *ClusterDomainResourceCoordinator) dispatchDomainResourceTargets(ctx context.Context, store *ClusterDomainOperationStore, domain *model.ClusterDomain, local *model.ClusterLocalNode, operationID string, action string, payloadMap map[string]interface{}, targets []model.ClusterMember, retry bool) error {
	secret, secretErr := c.secretProvider().GetSecret()
	for _, member := range targets {
		message, err := NewClusterPeerMessage(domain.Domain, member.LastVersion, local.NodeID, domainResourcePeerSourceSeq(), PeerCategoryCommand, action, payloadMap)
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

func domainResourcePeerSourceSeq() int64 {
	return 0
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
		return PeerActionDomainUserUpsert, ClusterDomainResourceUser, payload.User.UUID, payload.RequestID, payload.TargetMembers, payloadMap, err
	case op.ResourceKind == ClusterDomainResourceUser && op.Action == ClusterDomainOperationDelete:
		var payload clustertypes.DomainUserDeletePayload
		if err := json.Unmarshal(op.DesiredPayload, &payload); err != nil {
			return "", "", "", "", nil, nil, err
		}
		if strings.TrimSpace(payload.RequestID) == "" {
			payload.RequestID = op.OperationID
		}
		payloadMap, err := domainInboundPayloadMap(payload)
		return PeerActionDomainUserDelete, ClusterDomainResourceUser, payload.UUID, payload.RequestID, payload.TargetMembers, payloadMap, err
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

func selectedDomainResourceTargets(members []model.ClusterMember, localNodeID string, targetMembers []clustertypes.DomainInboundTarget) []model.ClusterMember {
	eligible := eligibleDomainResourceTargets(members, localNodeID)
	if len(targetMembers) == 0 {
		return eligible
	}

	selected := map[string]struct{}{}
	for _, target := range targetMembers {
		if nodeID := strings.TrimSpace(target.NodeID); nodeID != "" {
			selected[nodeID] = struct{}{}
		}
		if memberID := strings.TrimSpace(target.MemberID); memberID != "" {
			selected[memberID] = struct{}{}
		}
	}

	targets := make([]model.ClusterMember, 0, len(eligible))
	for _, member := range eligible {
		if _, ok := selected[member.NodeID]; ok {
			targets = append(targets, member)
		}
	}
	return targets
}

func (c *ClusterDomainResourceCoordinator) domainUserBoundGroupResources(domainID uint, boundGroups []string) ([]ClusterHubDomainResourceInbound, error) {
	if len(boundGroups) == 0 {
		return nil, nil
	}
	resources, err := c.buildDomainResources(domainID)
	if err != nil {
		return nil, err
	}
	return resources.Inbounds, nil
}

func domainUserBoundGroupTargets(members []model.ClusterMember, localNodeID string, boundGroups []string, inbounds []ClusterHubDomainResourceInbound) (bool, []model.ClusterMember, error) {
	boundGroups = normalizeDomainUserBoundGroups(boundGroups, nil)
	if len(boundGroups) == 0 {
		return true, eligibleDomainResourceTargets(members, localNodeID), nil
	}

	bound := make(map[string]struct{}, len(boundGroups))
	for _, group := range boundGroups {
		bound[group] = struct{}{}
	}

	selectedNodeGroupCounts := map[string]int{}
	for _, inbound := range inbounds {
		groupID := strings.TrimSpace(inbound.GroupID)
		if _, ok := bound[groupID]; !ok {
			continue
		}
		groupNodes := map[string]struct{}{}
		for _, instance := range inbound.Instances {
			if instance.Status != ClusterDomainOperationApplied {
				continue
			}
			nodeID := strings.TrimSpace(instance.NodeID)
			if nodeID == "" {
				continue
			}
			groupNodes[nodeID] = struct{}{}
		}
		for nodeID := range groupNodes {
			selectedNodeGroupCounts[nodeID]++
		}
	}
	selectedNodes := map[string]struct{}{}
	for nodeID, groupCount := range selectedNodeGroupCounts {
		if groupCount == len(bound) {
			selectedNodes[nodeID] = struct{}{}
		}
	}
	if len(selectedNodes) == 0 {
		return false, nil, errors.New("bound_inbound_group_ids must reference at least one applied domain inbound instance")
	}

	_, applyLocal := selectedNodes[strings.TrimSpace(localNodeID)]
	targets := make([]model.ClusterMember, 0, len(selectedNodes))
	for _, member := range eligibleDomainResourceTargets(members, localNodeID) {
		if _, ok := selectedNodes[member.NodeID]; ok {
			targets = append(targets, member)
		}
	}
	if !applyLocal && len(targets) == 0 {
		return false, nil, errors.New("bound_inbound_group_ids must reference at least one reachable domain member")
	}
	return applyLocal, targets, nil
}

func domainUserTargetMembers(localNodeID string, applyLocal bool, peerTargets []model.ClusterMember) []clustertypes.DomainInboundTarget {
	if !applyLocal && len(peerTargets) == 0 {
		return nil
	}
	targets := make([]clustertypes.DomainInboundTarget, 0, len(peerTargets)+1)
	if applyLocal {
		localNodeID = strings.TrimSpace(localNodeID)
		targets = append(targets, clustertypes.DomainInboundTarget{
			NodeID:   localNodeID,
			MemberID: localNodeID,
		})
	}
	for _, member := range peerTargets {
		targets = append(targets, clustertypes.DomainInboundTarget{
			NodeID:      member.NodeID,
			MemberID:    member.NodeID,
			DisplayName: clusterDomainMemberDisplayName(member),
		})
	}
	return targets
}

func domainUserMaterializedTargets(members []model.ClusterMember, localNodeID string, userUUID string, users []ClusterHubDomainResourceUser) (bool, []model.ClusterMember) {
	localNodeID = strings.TrimSpace(localNodeID)
	userUUID = strings.TrimSpace(userUUID)
	if userUUID == "" {
		return true, eligibleDomainResourceTargets(members, localNodeID)
	}
	materialized := map[string]struct{}{}
	for _, user := range users {
		if strings.TrimSpace(user.UUID) != userUUID {
			continue
		}
		if nodeID := strings.TrimSpace(user.NodeID); nodeID != "" {
			materialized[nodeID] = struct{}{}
		}
	}
	_, applyLocal := materialized[localNodeID]
	targets := make([]model.ClusterMember, 0, len(materialized))
	for _, member := range eligibleDomainResourceTargets(members, localNodeID) {
		if _, ok := materialized[member.NodeID]; ok {
			targets = append(targets, member)
		}
	}
	return applyLocal, targets
}

func shouldApplyLocalDomainResourceTarget(targetMembers []clustertypes.DomainInboundTarget, localNodeID string) bool {
	if len(targetMembers) == 0 {
		return true
	}
	localNodeID = strings.TrimSpace(localNodeID)
	if localNodeID == "" {
		return false
	}
	for _, target := range targetMembers {
		if strings.TrimSpace(target.NodeID) == localNodeID || strings.TrimSpace(target.MemberID) == localNodeID {
			return true
		}
	}
	return false
}

func validateSelectedDomainResourceTargets(targetMembers []clustertypes.DomainInboundTarget, members []model.ClusterMember, localNodeID string) error {
	if len(targetMembers) == 0 {
		return nil
	}
	if shouldApplyLocalDomainResourceTarget(targetMembers, localNodeID) {
		return nil
	}
	if len(selectedDomainResourceTargets(members, localNodeID, targetMembers)) > 0 {
		return nil
	}
	return errors.New("target_members must include at least one domain member")
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
