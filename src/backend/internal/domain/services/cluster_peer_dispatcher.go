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
	eventStore  clusterPeerStore
	syncService *ClusterSyncService
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
			return err
		}
		return store.MarkEventState(message.MessageID, PeerEventStatusSucceeded, "")
	}

	if message.Category == PeerCategoryEvent {
		return store.MarkEventState(message.MessageID, PeerEventStatusUnsupported, "")
	}

	err = errors.New("unsupported_action")
	if markErr := store.MarkEventState(message.MessageID, PeerEventStatusFailed, err.Error()); markErr != nil {
		return markErr
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

func isTerminalPeerEventState(status string) bool {
	return status == PeerEventStatusSucceeded ||
		status == PeerEventStatusUnsupported ||
		status == PeerEventStatusIgnored
}
