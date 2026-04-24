package service

import (
	"context"
	"errors"

	"github.com/alireza0/s-ui/src/backend/internal/infra/db/model"
)

const (
	PeerCategoryCommand  = "command"
	PeerCategoryEvent    = "event"
	PeerCategoryQuery    = "query"
	PeerCategoryResponse = "response"
)

const PeerActionDomainClusterChanged = "domain.cluster.changed"

type ClusterPeerDispatcher struct {
	eventStore       clusterPeerStore
	syncService      *ClusterSyncService
	identity         ClusterLocalIdentityService
	secretProvider   clusterSecretProvider
	delivery         *ClusterPeerDeliveryService
	saveWorkflowStep func(workflowID string, stepID string, domainID string, nodeID string, status string, resultHash string, errorMessage string) error
}

func (d *ClusterPeerDispatcher) Dispatch(ctx context.Context, domain *model.ClusterDomain, source *model.ClusterMember, message *PeerMessage) error {
	store := d.getStore()
	state, err := store.RecordReceived(message)
	if err != nil {
		return err
	}
	if isTerminalPeerEventState(state.Status) {
		return nil
	}
	if err := store.MarkEventState(message.MessageID, PeerEventStatusProcessing, ""); err != nil {
		return err
	}

	if message.Category == PeerCategoryEvent && message.Action == PeerActionDomainClusterChanged {
		if err := d.handleDomainClusterChanged(ctx, domain, source, message); err != nil {
			markErr := store.MarkEventState(message.MessageID, PeerEventStatusFailed, err.Error())
			if markErr != nil {
				return markErr
			}
			if chainErr := d.completeChainStep(ctx, domain, message, PeerEventStatusFailed, err.Error()); chainErr != nil {
				return chainErr
			}
			return err
		}
		if err := store.MarkEventState(message.MessageID, PeerEventStatusSucceeded, ""); err != nil {
			return err
		}
		return d.completeChainStep(ctx, domain, message, PeerEventStatusSucceeded, "")
	}

	if message.Category == PeerCategoryEvent {
		if err := store.MarkEventState(message.MessageID, PeerEventStatusUnsupported, ""); err != nil {
			return err
		}
		return d.completeChainStep(ctx, domain, message, PeerEventStatusUnsupported, "")
	}

	err = errors.New("unsupported_action")
	if markErr := store.MarkEventState(message.MessageID, PeerEventStatusFailed, err.Error()); markErr != nil {
		return markErr
	}
	if chainErr := d.completeChainStep(ctx, domain, message, PeerEventStatusFailed, err.Error()); chainErr != nil {
		return chainErr
	}
	return err
}

func (d *ClusterPeerDispatcher) handleDomainClusterChanged(ctx context.Context, domain *model.ClusterDomain, source *model.ClusterMember, message *PeerMessage) error {
	versionValue, ok := message.Payload["version"].(float64)
	if !ok {
		return errors.New("invalid_payload_version")
	}
	syncService := d.getSyncService()
	_, err := syncService.HandleIncomingNotifyVersion(ctx, domain.Id, source.NodeID, int64(versionValue))
	return err
}

func (d *ClusterPeerDispatcher) getStore() clusterPeerStore {
	if d.eventStore != nil {
		return d.eventStore
	}
	return newDBClusterPeerStore()
}

func (d *ClusterPeerDispatcher) getSyncService() *ClusterSyncService {
	if d.syncService == nil {
		syncService := NewRuntimeClusterSyncService()
		return &syncService
	}
	if d.syncService.store == nil {
		d.syncService.store = &dbClusterSyncStore{}
	}
	if d.syncService.hubSyncer == nil {
		d.syncService.hubSyncer = &ClusterHubSyncer{}
	}
	return d.syncService
}

func (d *ClusterPeerDispatcher) completeChainStep(ctx context.Context, domain *model.ClusterDomain, message *PeerMessage, status string, errorMessage string) error {
	if message.Route.Mode != RouteModeChain || message.WorkflowID == "" || message.StepID == "" {
		return nil
	}

	nodeID := clusterPeerChainStepNodeID(message.Route, message.StepID)
	if nodeID == "" {
		nodeID = message.SourceNodeID
	}
	if err := d.getWorkflowStepSaver()(message.WorkflowID, message.StepID, message.DomainID, nodeID, status, message.PayloadHash, errorMessage); err != nil {
		return err
	}

	nextStep, ok := NextClusterPeerChainStep(message.Route, message.StepID, status == PeerEventStatusSucceeded)
	if !ok {
		return nil
	}
	return d.dispatchNextChainStep(ctx, domain, message, nextStep)
}

func (d *ClusterPeerDispatcher) dispatchNextChainStep(ctx context.Context, domain *model.ClusterDomain, current *PeerMessage, nextStep RouteStep) error {
	local, err := d.identity.GetOrCreate()
	if err != nil {
		return err
	}
	member, err := d.getSyncService().store.GetMember(domain.Id, nextStep.NodeID)
	if err != nil {
		return err
	}
	secret, err := d.getSecretProvider().GetSecret()
	if err != nil {
		return err
	}
	token, err := DecryptClusterDomainToken(secret, member.PeerTokenEncrypted)
	if err != nil {
		return err
	}

	action := current.Action
	if nextStep.Action != "" {
		action = nextStep.Action
	}
	payload := clonePeerPayload(current.Payload)
	if nextStep.PayloadOverride != nil {
		payload = clonePeerPayload(nextStep.PayloadOverride)
	}
	sourceSeq := current.SourceSeq
	if sourceSeq > 0 {
		sourceSeq++
	}
	next, err := NewClusterPeerMessage(current.DomainID, current.MembershipVersion, local.NodeID, sourceSeq, current.Category, action, payload)
	if err != nil {
		return err
	}
	next.WorkflowID = current.WorkflowID
	next.StepID = nextStep.StepID
	next.Route = current.Route
	next.IdempotencyKey = current.IdempotencyKey
	next.CausationID = current.CausationID
	next.CorrelationID = current.CorrelationID
	next.ExpiresAt = current.ExpiresAt

	if err := SignClusterPeerMessage(local, next); err != nil {
		return err
	}
	return d.getDelivery().Send(ctx, next, *member, token)
}

func (d *ClusterPeerDispatcher) getWorkflowStepSaver() func(workflowID string, stepID string, domainID string, nodeID string, status string, resultHash string, errorMessage string) error {
	if d.saveWorkflowStep != nil {
		return d.saveWorkflowStep
	}
	return SaveClusterPeerWorkflowStep
}

func (d *ClusterPeerDispatcher) getSecretProvider() clusterSecretProvider {
	if d.secretProvider != nil {
		return d.secretProvider
	}
	return &SettingService{}
}

func (d *ClusterPeerDispatcher) getDelivery() *ClusterPeerDeliveryService {
	if d.delivery != nil {
		return d.delivery
	}
	return &ClusterPeerDeliveryService{}
}

func clusterPeerChainStepNodeID(route RoutePlan, stepID string) string {
	for _, step := range route.Chain {
		if step.StepID == stepID {
			return step.NodeID
		}
	}
	return ""
}

func clonePeerPayload(payload map[string]interface{}) map[string]interface{} {
	if payload == nil {
		return nil
	}
	clone := make(map[string]interface{}, len(payload))
	for key, value := range payload {
		clone[key] = value
	}
	return clone
}

func isTerminalPeerEventState(status string) bool {
	return status == PeerEventStatusSucceeded ||
		status == PeerEventStatusUnsupported ||
		status == PeerEventStatusIgnored
}
