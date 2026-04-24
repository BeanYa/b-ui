package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	database "github.com/alireza0/s-ui/src/backend/internal/infra/db"
	"github.com/alireza0/s-ui/src/backend/internal/infra/db/model"
)

const ClusterCommunicationEndpointPath = "/_cluster"
const ClusterCommunicationProtocolVersion = "v1"

func ClusterCommunicationSupportedActions() []string {
	return []string{"domain.cluster.changed", "events", "heartbeat", "ping"}
}

type ClusterHubSyncer struct {
	client         clusterHubClient
	store          clusterServiceStore
	secretProvider clusterSecretProvider
}

func (s *ClusterHubSyncer) LatestVersion(ctx context.Context, domain *model.ClusterDomain) (int64, error) {
	client := s.client
	if client == nil {
		client = &ClusterHubClient{}
	}
	secret, err := s.getSecretProvider().GetSecret()
	if err != nil {
		return 0, err
	}
	domainToken, err := DecryptClusterDomainToken(secret, domain.TokenEncrypted)
	if err != nil {
		return 0, err
	}
	response, err := client.GetLatestVersion(ctx, domain.HubURL, domain.Domain, domainToken)
	if err != nil {
		return 0, err
	}
	return response.Version, nil
}

func (s *ClusterHubSyncer) SyncDomain(ctx context.Context, domain *model.ClusterDomain, version int64) error {
	client := s.client
	if client == nil {
		client = &ClusterHubClient{}
	}
	store := s.store
	if store == nil {
		store = &dbClusterStore{}
	}
	secret, err := s.getSecretProvider().GetSecret()
	if err != nil {
		return err
	}
	domainToken, err := DecryptClusterDomainToken(secret, domain.TokenEncrypted)
	if err != nil {
		return err
	}
	snapshot, err := client.GetSnapshot(ctx, domain.HubURL, domain.Domain, domainToken)
	if err != nil {
		return err
	}
	members := make([]model.ClusterMember, 0, len(snapshot.Members))
	for _, item := range snapshot.Members {
		peerTokenEncrypted := ""
		peerToken := item.EffectivePeerToken()
		if peerToken != "" {
			peerTokenEncrypted, err = EncryptClusterDomainToken(secret, peerToken)
			if err != nil {
				return err
			}
		}
		members = append(members, model.ClusterMember{
			NodeID:             item.EffectiveNodeID(),
			Name:               item.Name,
			BaseURL:            item.EffectiveBaseURL(),
			PublicKey:          item.EffectivePublicKey(),
			PeerTokenEncrypted: peerTokenEncrypted,
			DomainID:           domain.Id,
			LastVersion:        snapshot.Version,
		})
	}
	if err := store.ReplaceDomainMembers(domain.Id, members); err != nil {
		return err
	}
	domain.CommunicationEndpointPath = snapshot.EffectiveCommunicationEndpointPath()
	domain.CommunicationProtocolVersion = snapshot.EffectiveCommunicationProtocolVersion()
	domain.LastVersion = snapshot.Version
	if domain.LastVersion == 0 {
		domain.LastVersion = version
	}
	return store.SaveDomain(domain)
}

func (s *ClusterHubSyncer) getSecretProvider() clusterSecretProvider {
	if s.secretProvider != nil {
		return s.secretProvider
	}
	return &SettingService{}
}

type ClusterHTTPBroadcaster struct {
	SettingService
	identity       ClusterLocalIdentityService
	secretProvider clusterSecretProvider
	store          clusterBroadcastStore
	HTTPClient     *http.Client
}

type clusterBroadcastStore interface {
	ListMembersWithDomain() ([]model.ClusterMember, error)
}

func (b *ClusterHTTPBroadcaster) BroadcastNotifyVersion(ctx context.Context, version int64, excludeNodeID string) error {
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
	for _, member := range members {
		if member.NodeID == excludeNodeID || member.NodeID == identity.NodeID || member.BaseURL == "" || member.Domain == nil {
			continue
		}
		token, err := DecryptClusterDomainToken(secret, member.PeerTokenEncrypted)
		if err != nil {
			return err
		}
		message, err := NewClusterPeerMessage(member.Domain.Domain, version, identity.NodeID, version, PeerCategoryEvent, PeerActionDomainClusterChanged, map[string]interface{}{"version": float64(version)})
		if err != nil {
			return err
		}
		message.Route = RoutePlan{
			Mode: RouteModeBroadcast,
			Delivery: &DeliveryPolicy{
				Ack:       DeliveryAckNode,
				TimeoutMs: 10000,
				Retry: &RetryPolicy{
					MaxAttempts: 3,
					BackoffMs:   1000,
				},
			},
		}
		if err := SignClusterPeerMessage(identity, message); err != nil {
			return err
		}
		delivery := &ClusterPeerDeliveryService{HTTPClient: b.httpClient()}
		if err := delivery.Send(ctx, message, member, token); err != nil {
			envelope, legacyErr := SignClusterNotifyVersionEnvelope(identity, member.Domain.Domain, version, message.CreatedAt)
			if legacyErr != nil {
				return legacyErr
			}
			if fallbackErr := delivery.SendEnvelope(ctx, envelope, member, token); fallbackErr != nil {
				return err
			}
		}
	}
	return nil
}

func (b *ClusterHTTPBroadcaster) httpClient() *http.Client {
	if b.HTTPClient != nil {
		return b.HTTPClient
	}
	return &http.Client{Timeout: 10 * time.Second}
}

func (b *ClusterHTTPBroadcaster) getSecretProvider() clusterSecretProvider {
	if b.secretProvider != nil {
		return b.secretProvider
	}
	return &b.SettingService
}

func (b *ClusterHTTPBroadcaster) getStore() clusterBroadcastStore {
	if b.store != nil {
		return b.store
	}
	return &dbClusterBroadcastStore{}
}

func clusterPeerMessageURL(baseURL string) (string, error) {
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}
	if err := validateClusterPeerScheme(parsed); err != nil {
		return "", err
	}
	parsed.Path = strings.TrimSuffix(parsed.Path, "/") + ClusterCommunicationEndpointPath + "/" + ClusterCommunicationProtocolVersion + "/events"
	parsed.RawPath = ""
	return parsed.String(), nil
}

func validateClusterPeerScheme(parsed *url.URL) error {
	if parsed.Scheme == "https" {
		return nil
	}
	if parsed.Scheme != "http" {
		return errors.New("cluster peer URL must use http or https")
	}
	host := parsed.Hostname()
	if host == "localhost" || host == "127.0.0.1" || host == "::1" {
		return nil
	}
	if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
		return nil
	}
	return errors.New("cluster peer URL must use https for non-local addresses")
}

type dbClusterBroadcastStore struct{}

func (s *dbClusterBroadcastStore) ListMembersWithDomain() ([]model.ClusterMember, error) {
	var members []model.ClusterMember
	if err := database.GetDB().Preload("Domain").Find(&members).Error; err != nil {
		return nil, err
	}
	return members, nil
}

func requireClusterPeerSuccess(response *http.Response) error {
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return err
	}
	response.Body = io.NopCloser(bytes.NewReader(body))
	if len(bytes.TrimSpace(body)) == 0 {
		return nil
	}
	var payload struct {
		Success bool   `json:"success"`
		Msg     string `json:"msg"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil
	}
	if !payload.Success {
		if payload.Msg == "" {
			return errors.New("cluster peer notify failed")
		}
		return errors.New(payload.Msg)
	}
	return nil
}
