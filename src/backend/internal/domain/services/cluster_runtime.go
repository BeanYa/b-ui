package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"time"

	database "github.com/BeanYa/b-ui/src/backend/internal/infra/db"
	"github.com/BeanYa/b-ui/src/backend/internal/infra/db/model"
)

const ClusterCommunicationEndpointPath = "/_cluster"
const ClusterCommunicationProtocolVersion = "v1"

func ClusterCommunicationSupportedActions() []string {
	return []string{"domain.cluster.changed", "events", "heartbeat", "ping", "info", "action", "domain.panel.update.available"}
}

type ClusterHubSyncer struct {
	client         clusterHubClient
	store          clusterServiceStore
	secretProvider clusterSecretProvider
	localIdentity  clusterLocalIdentityProvider
	reachability   *ClusterReachabilityService
}

type clusterLocalIdentityProvider interface {
	GetOrCreate() (*model.ClusterLocalNode, error)
}

type clusterDomainMirrorRemovedError struct {
	Domain string
}

func (e *clusterDomainMirrorRemovedError) Error() string {
	if e == nil || e.Domain == "" {
		return "cluster domain info is invalid; local mirror was removed"
	}
	return fmt.Sprintf("cluster domain info is invalid for %s; local mirror was removed", e.Domain)
}

func (s *ClusterHubSyncer) getHubClient() clusterHubClient {
	if s.client != nil {
		return s.client
	}
	return &ClusterHubClient{localIdentity: s.getLocalIdentity()}
}

func (s *ClusterHubSyncer) LatestVersion(ctx context.Context, domain *model.ClusterDomain) (int64, error) {
	client := s.getHubClient()
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
		var rejectedErr *clusterHubReadRejectedError
		if errors.As(err, &rejectedErr) {
			return 0, s.removeInvalidMirror(domain)
		}
		return 0, err
	}
	return response.Version, nil
}

func (s *ClusterHubSyncer) SyncDomain(ctx context.Context, domain *model.ClusterDomain, version int64) error {
	client := s.getHubClient()
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
	local, err := s.getLocalIdentity().GetOrCreate()
	if err != nil {
		return err
	}
	snapshot, err := client.GetSnapshot(ctx, domain.HubURL, domain.Domain, domainToken)
	if err != nil {
		var rejectedErr *clusterHubReadRejectedError
		if errors.As(err, &rejectedErr) {
			return s.removeInvalidMirror(domain)
		}
		return err
	}
	members := make([]model.ClusterMember, 0, len(snapshot.Members))
	localNodePresent := false
	for _, item := range snapshot.Members {
		if item.EffectiveNodeID() == local.NodeID {
			localNodePresent = true
		}
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
			DisplayName:        item.EffectiveDisplayName(),
			BaseURL:            item.EffectiveBaseURL(),
			PublicKey:          item.EffectivePublicKey(),
			PeerTokenEncrypted: peerTokenEncrypted,
			DomainID:           domain.Id,
			LastVersion:        snapshot.Version,
			PanelVersion:       item.EffectivePanelVersion(),
			Status:             item.EffectiveStatus(),
		})
	}
	if !localNodePresent {
		if err := store.DeleteDomain(domain.Id); err != nil {
			return err
		}
		return &clusterDomainMirrorRemovedError{Domain: domain.Domain}
	}
	if err := store.ReplaceDomainMembers(domain.Id, members); err != nil {
		return err
	}
	targetNodeIDs := make([]string, 0, len(members))
	for _, member := range members {
		targetNodeIDs = append(targetNodeIDs, member.NodeID)
	}
	domain.CommunicationEndpointPath = snapshot.EffectiveCommunicationEndpointPath()
	domain.CommunicationProtocolVersion = snapshot.EffectiveCommunicationProtocolVersion()
	domain.LastVersion = snapshot.Version
	if domain.LastVersion == 0 {
		domain.LastVersion = version
	}
	if err := store.SaveDomain(domain); err != nil {
		return err
	}
	return s.getReachability().ReconcileMembers(domain.Id, targetNodeIDs)
}

func (s *ClusterHubSyncer) removeInvalidMirror(domain *model.ClusterDomain) error {
	store := s.store
	if store == nil {
		store = &dbClusterStore{}
	}
	if err := store.DeleteDomain(domain.Id); err != nil {
		return err
	}
	return &clusterDomainMirrorRemovedError{Domain: domain.Domain}
}

func (s *ClusterHubSyncer) getSecretProvider() clusterSecretProvider {
	if s.secretProvider != nil {
		return s.secretProvider
	}
	return &SettingService{}
}

func (s *ClusterHubSyncer) getReachability() *ClusterReachabilityService {
	if s.reachability != nil {
		return s.reachability
	}
	return &ClusterReachabilityService{
		store:  &dbClusterReachabilityStore{},
		policy: DefaultClusterReachabilityPolicy(),
	}
}

func (s *ClusterHubSyncer) getLocalIdentity() clusterLocalIdentityProvider {
	if s.localIdentity != nil {
		return s.localIdentity
	}
	return &ClusterLocalIdentityService{}
}

type ClusterHTTPBroadcaster struct {
	SettingService
	identity       ClusterLocalIdentityService
	secretProvider clusterSecretProvider
	store          clusterBroadcastStore
	reachability   *ClusterReachabilityService
	HTTPClient     *http.Client
	saveAckAttempt func(messageID string, targetNode string, status string, errorMessage string) error
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
	reachability := b.getReachability()
	var failures []string
	appendFailure := func(err error) {
		if err != nil {
			failures = append(failures, err.Error())
		}
	}
	for _, member := range members {
		if member.NodeID == excludeNodeID || member.NodeID == identity.NodeID || member.BaseURL == "" || member.Domain == nil {
			continue
		}
		entry, err := reachability.load(member.DomainID, member.NodeID)
		if err != nil {
			appendFailure(err)
			continue
		}
		if entry.State == ClusterReachabilityUnreachable {
			shouldRetry, err := reachability.shouldProbeWithError(entry)
			if err != nil {
				appendFailure(err)
			}
			if !shouldRetry {
				continue
			}
		}
		token, err := DecryptClusterDomainToken(secret, member.PeerTokenEncrypted)
		if err != nil {
			appendFailure(err)
			if _, recordErr := reachability.RecordTransportFailure(member.DomainID, member.NodeID, "business"); recordErr != nil {
				appendFailure(recordErr)
			}
			continue
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
		delivery := &ClusterPeerDeliveryService{HTTPClient: b.httpClient(), saveAckAttempt: b.getAckAttemptSaver()}
		if err := delivery.Send(ctx, message, member, token); err != nil {
			envelope, legacyErr := SignClusterNotifyVersionEnvelope(identity, member.Domain.Domain, version, message.CreatedAt)
			if legacyErr != nil {
				return legacyErr
			}
			if fallbackErr := delivery.SendEnvelope(ctx, envelope, member, token); fallbackErr != nil {
				return err
			}
			if ackErr := b.getAckAttemptSaver()(message.MessageID, member.NodeID, PeerAckStatusSucceeded, ""); ackErr != nil {
				return ackErr
			}
		}
	}
	return nil
}

func (b *ClusterHTTPBroadcaster) BroadcastUpdateAvailable(ctx context.Context, domainID uint, domainName string, targetVersion string, excludeNodeID string) error {
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
	reachability := b.getReachability()
	for _, member := range members {
		if member.Domain == nil || member.Domain.Id != domainID {
			continue
		}
		if member.NodeID == excludeNodeID || member.NodeID == identity.NodeID || member.BaseURL == "" {
			continue
		}
		entry, err := reachability.load(member.DomainID, member.NodeID)
		if err != nil {
			continue
		}
		if entry.State == ClusterReachabilityUnreachable {
			shouldRetry, err := reachability.shouldProbeWithError(entry)
			if err != nil || !shouldRetry {
				continue
			}
		}
		token, err := DecryptClusterDomainToken(secret, member.PeerTokenEncrypted)
		if err != nil {
			continue
		}
		message, err := NewClusterPeerMessage(domainName, 0, identity.NodeID, 0, PeerCategoryEvent, "domain.panel.update.available", map[string]interface{}{
			"target_version": targetVersion,
		})
		if err != nil {
			continue
		}
		message.Route = RoutePlan{
			Mode: RouteModeBroadcast,
			Delivery: &DeliveryPolicy{
				Ack:       DeliveryAckNone,
				TimeoutMs: 10000,
				Retry: &RetryPolicy{
					MaxAttempts: 1,
					BackoffMs:   1000,
				},
			},
		}
		if err := SignClusterPeerMessage(identity, message); err != nil {
			continue
		}
		delivery := &ClusterPeerDeliveryService{HTTPClient: b.httpClient(), saveAckAttempt: b.getAckAttemptSaver()}
		_ = delivery.Send(ctx, message, member, token)
	}
	return nil
}

func (b *ClusterHTTPBroadcaster) httpClient() *http.Client {
	if b.HTTPClient != nil {
		return b.HTTPClient
	}
	return &http.Client{Timeout: 10 * time.Second}
}

func (b *ClusterHTTPBroadcaster) getAckAttemptSaver() func(messageID string, targetNode string, status string, errorMessage string) error {
	if b.saveAckAttempt != nil {
		return b.saveAckAttempt
	}
	return SaveClusterPeerAckAttempt
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

func (b *ClusterHTTPBroadcaster) getReachability() *ClusterReachabilityService {
	if b.reachability != nil {
		return b.reachability
	}
	return &ClusterReachabilityService{
		store:  &dbClusterReachabilityStore{},
		policy: DefaultClusterReachabilityPolicy(),
	}
}

func clusterPeerMessageURL(baseURL string) (string, error) {
	return clusterPeerActionURL(
		baseURL,
		ClusterCommunicationEndpointPath,
		ClusterCommunicationProtocolVersion,
		"events",
	)
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
