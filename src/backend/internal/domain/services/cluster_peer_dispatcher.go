package service

import (
	"context"
	"errors"

	"github.com/BeanYa/b-ui/src/backend/internal/infra/db/model"
	"github.com/gofrs/uuid/v5"
)

const (
	PeerCategoryCommand  = "command"
	PeerCategoryEvent    = "event"
	PeerCategoryQuery    = "query"
	PeerCategoryResponse = "response"
)

const PeerActionDomainClusterChanged = "domain.cluster.changed"

const PeerActionDomainPanelUpdateAvailable = "domain.panel.update.available"

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
	claimed, err := store.ClaimProcessing(state.MessageID)
	if err != nil {
		return err
	}
	if !claimed {
		return nil
	}

	if validTarget, reason, err := d.validateInboundRouteTarget(message); err != nil {
		_ = store.MarkEventState(state.MessageID, PeerEventStatusFailed, err.Error())
		return err
	} else if !validTarget {
		return store.MarkEventState(state.MessageID, PeerEventStatusIgnored, reason)
	}

	if message.Category == PeerCategoryEvent && message.Action == PeerActionDomainClusterChanged {
		if err := d.handleDomainClusterChanged(ctx, domain, source, message); err != nil {
			forwardedNextStep, chainErr := d.completeChainStep(ctx, domain, message, PeerEventStatusFailed, err.Error())
			if chainErr != nil {
				_ = store.MarkEventState(state.MessageID, PeerEventStatusFailed, chainErr.Error())
				return chainErr
			}
			status := PeerEventStatusFailed
			errorMessage := err.Error()
			if forwardedNextStep {
				status = PeerEventStatusSucceeded
				errorMessage = ""
			}
			if markErr := store.MarkEventState(state.MessageID, status, errorMessage); markErr != nil {
				return markErr
			}
			if forwardedNextStep {
				return nil
			}
			return err
		}
		if _, err := d.completeChainStep(ctx, domain, message, PeerEventStatusSucceeded, ""); err != nil {
			_ = store.MarkEventState(state.MessageID, PeerEventStatusFailed, err.Error())
			return err
		}
		return store.MarkEventState(state.MessageID, PeerEventStatusSucceeded, "")
	}

	if message.Category == PeerCategoryEvent && message.Action == PeerActionDomainPanelUpdateAvailable {
		if err := d.handleDomainPanelUpdateAvailable(ctx, domain, source, message); err != nil {
			forwardedNextStep, chainErr := d.completeChainStep(ctx, domain, message, PeerEventStatusFailed, err.Error())
			if chainErr != nil {
				_ = store.MarkEventState(state.MessageID, PeerEventStatusFailed, chainErr.Error())
				return chainErr
			}
			status := PeerEventStatusFailed
			errorMessage := err.Error()
			if forwardedNextStep {
				status = PeerEventStatusSucceeded
				errorMessage = ""
			}
			if markErr := store.MarkEventState(state.MessageID, status, errorMessage); markErr != nil {
				return markErr
			}
			if forwardedNextStep {
				return nil
			}
			return err
		}
		if _, err := d.completeChainStep(ctx, domain, message, PeerEventStatusSucceeded, ""); err != nil {
			_ = store.MarkEventState(state.MessageID, PeerEventStatusFailed, err.Error())
			return err
		}
		return store.MarkEventState(state.MessageID, PeerEventStatusSucceeded, "")
	}

	if message.Category == PeerCategoryEvent {
		if _, err := d.completeChainStep(ctx, domain, message, PeerEventStatusUnsupported, ""); err != nil {
			_ = store.MarkEventState(state.MessageID, PeerEventStatusFailed, err.Error())
			return err
		}
		return store.MarkEventState(state.MessageID, PeerEventStatusUnsupported, "")
	}

	if message.Category == PeerCategoryResponse {
		return store.MarkEventState(state.MessageID, PeerEventStatusIgnored, "response_unhandled")
	}

	err = errors.New("unsupported_action")
	forwardedNextStep, chainErr := d.completeChainStep(ctx, domain, message, PeerEventStatusFailed, err.Error())
	if chainErr != nil {
		_ = store.MarkEventState(state.MessageID, PeerEventStatusFailed, chainErr.Error())
		return chainErr
	}
	status := PeerEventStatusFailed
	errorMessage := err.Error()
	if forwardedNextStep {
		status = PeerEventStatusSucceeded
		errorMessage = ""
	}
	if markErr := store.MarkEventState(state.MessageID, status, errorMessage); markErr != nil {
		return markErr
	}
	if forwardedNextStep {
		return nil
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

func (d *ClusterPeerDispatcher) handleDomainPanelUpdateAvailable(ctx context.Context, domain *model.ClusterDomain, source *model.ClusterMember, message *PeerMessage) error {
	targetVersion, ok := message.Payload["target_version"].(string)
	if !ok || targetVersion == "" {
		return errors.New("invalid_payload_target_version")
	}
	_, err := d.getSyncService().HandlePanelUpdateAvailable(ctx, domain, targetVersion)
	return err
}

func (d *ClusterPeerDispatcher) validateInboundRouteTarget(message *PeerMessage) (bool, string, error) {
	switch message.Route.Mode {
	case "":
		return true, "", nil
	case RouteModeDirect:
		local, err := d.identity.GetOrCreate()
		if err != nil {
			return false, "", err
		}
		if len(message.Route.Targets) != 1 {
			return false, "direct_route_malformed", nil
		}
		if message.Route.Targets[0] != local.NodeID {
			return false, "direct_route_target_mismatch", nil
		}
		return validateInboundRouteSelector(message.Route.Selector, local.NodeID, "direct_route")
	case RouteModeMulticast:
		local, err := d.identity.GetOrCreate()
		if err != nil {
			return false, "", err
		}
		if !containsClusterNodeID(message.Route.Targets, local.NodeID) {
			return false, "multicast_route_target_mismatch", nil
		}
		return validateInboundRouteSelector(message.Route.Selector, local.NodeID, "multicast_route")
	case RouteModeBroadcast, RouteModeScheduledBroadcast:
		local, err := d.identity.GetOrCreate()
		if err != nil {
			return false, "", err
		}
		return validateInboundRouteSelector(message.Route.Selector, local.NodeID, "broadcast_route")
	case RouteModeChain:
		if message.WorkflowID == "" {
			return false, "chain_workflow_id_required", nil
		}
		if message.StepID == "" {
			return false, "chain_step_id_required", nil
		}
		step, ok := clusterPeerChainRouteStep(message.Route, message.StepID)
		if !ok {
			return false, "chain_step_not_found", nil
		}
		local, err := d.identity.GetOrCreate()
		if err != nil {
			return false, "", err
		}
		if step.NodeID != local.NodeID {
			return false, "chain_step_target_mismatch", nil
		}
		return true, "", nil
	default:
		return false, "route_mode_unknown", nil
	}
}

func validateInboundRouteSelector(selector *TargetSelector, localNodeID string, reasonPrefix string) (bool, string, error) {
	if selector == nil {
		return true, "", nil
	}
	if len(selector.CapabilityRequired) > 0 {
		return false, reasonPrefix + "_capability_unvalidated", nil
	}
	if len(selector.Include) > 0 && !containsClusterNodeID(selector.Include, localNodeID) {
		return false, reasonPrefix + "_target_mismatch", nil
	}
	if containsClusterNodeID(selector.Exclude, localNodeID) {
		return false, reasonPrefix + "_target_excluded", nil
	}
	return true, "", nil
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
		d.syncService.hubSyncer = &ClusterHubSyncer{localIdentity: &d.identity}
	}
	return d.syncService
}

func (d *ClusterPeerDispatcher) completeChainStep(ctx context.Context, domain *model.ClusterDomain, message *PeerMessage, status string, errorMessage string) (bool, error) {
	if message.Route.Mode != RouteModeChain || message.WorkflowID == "" || message.StepID == "" {
		return false, nil
	}

	nodeID := clusterPeerChainStepNodeID(message.Route, message.StepID)
	if nodeID == "" {
		nodeID = message.SourceNodeID
	}
	if err := d.getWorkflowStepSaver()(message.WorkflowID, message.StepID, message.DomainID, nodeID, status, message.PayloadHash, errorMessage); err != nil {
		return false, err
	}

	nextStep, ok := NextClusterPeerChainStep(message.Route, message.StepID, status == PeerEventStatusSucceeded)
	if !ok {
		return false, nil
	}
	if err := d.dispatchNextChainStep(ctx, domain, message, nextStep); err != nil {
		return false, err
	}
	return true, nil
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
	stableKey := clusterPeerChainStepStableKey(current.DomainID, current.WorkflowID, nextStep.StepID)
	next.MessageID = uuid.NewV5(uuid.NamespaceURL, stableKey).String()
	next.IdempotencyKey = stableKey
	next.CausationID = current.CausationID
	next.CorrelationID = current.CorrelationID
	next.ExpiresAt = current.ExpiresAt

	if err := SignClusterPeerMessage(local, next); err != nil {
		return err
	}
	return d.getDelivery().Send(ctx, next, *member, token)
}

func clusterPeerChainStepStableKey(domainID string, workflowID string, stepID string) string {
	return "cluster-peer-chain-step:" + domainID + ":" + workflowID + ":" + stepID
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
	step, ok := clusterPeerChainRouteStep(route, stepID)
	if !ok {
		return ""
	}
	return step.NodeID
}

func clusterPeerChainRouteStep(route RoutePlan, stepID string) (RouteStep, bool) {
	for _, step := range route.Chain {
		if step.StepID == stepID {
			return step, true
		}
	}
	return RouteStep{}, false
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
		status == PeerEventStatusIgnored ||
		status == PeerEventStatusDead
}
