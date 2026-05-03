package service

import (
	"context"
	"encoding/json"

	clustertypes "github.com/BeanYa/b-ui/src/backend/internal/domain/services/cluster/types"
	"github.com/BeanYa/b-ui/src/backend/internal/infra/db/model"
)

func (b *ClusterHTTPBroadcaster) BroadcastDomainUserUpsert(ctx context.Context, domain *model.ClusterDomain, payload clustertypes.DomainUserUpsertPayload) error {
	if b == nil || domain == nil {
		return nil
	}
	return b.broadcastDomainUserCommand(ctx, domain, PeerActionDomainUserUpsert, payload)
}

func (b *ClusterHTTPBroadcaster) BroadcastDomainUserDelete(ctx context.Context, domain *model.ClusterDomain, payload clustertypes.DomainUserDeletePayload) error {
	if b == nil || domain == nil {
		return nil
	}
	return b.broadcastDomainUserCommand(ctx, domain, PeerActionDomainUserDelete, payload)
}

func (b *ClusterHTTPBroadcaster) broadcastDomainUserCommand(ctx context.Context, domain *model.ClusterDomain, action string, payload interface{}) error {
	identity, err := b.identity.GetOrCreate()
	if err != nil {
		return err
	}
	secret, err := b.getSecretProvider().GetSecret()
	if err != nil {
		return err
	}
	members, err := b.getStore().ListMembersWithDomain()
	if err != nil {
		return err
	}
	payloadMap, err := domainUserPayloadMap(payload)
	if err != nil {
		return err
	}
	delivery := &ClusterPeerDeliveryService{
		HTTPClient:     b.httpClient(),
		saveAckAttempt: b.getAckAttemptSaver(),
	}
	for _, member := range members {
		if member.Domain == nil || member.Domain.Id != domain.Id {
			continue
		}
		if member.NodeID == identity.NodeID || member.BaseURL == "" || member.PeerTokenEncrypted == "" {
			continue
		}
		token, err := DecryptClusterDomainToken(secret, member.PeerTokenEncrypted)
		if err != nil {
			continue
		}
		message, err := NewClusterPeerMessage(domain.Domain, member.LastVersion, identity.NodeID, 0, PeerCategoryCommand, action, payloadMap)
		if err != nil {
			return err
		}
		message.Route = RoutePlan{Mode: RouteModeBroadcast}
		if err := SignClusterPeerMessage(identity, message); err != nil {
			return err
		}
		if err := delivery.Send(ctx, message, member, token); err != nil {
			return err
		}
	}
	return nil
}

func domainUserPayloadMap(payload interface{}) (map[string]interface{}, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	var out map[string]interface{}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}
