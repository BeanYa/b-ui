package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	database "github.com/alireza0/s-ui/src/backend/internal/infra/db"
	"github.com/alireza0/s-ui/src/backend/internal/infra/db/model"
)

type ClusterHubSyncer struct {
	client clusterHubClient
	store  clusterServiceStore
}

func (s *ClusterHubSyncer) LatestVersion(ctx context.Context, domain *model.ClusterDomain) (int64, error) {
	client := s.client
	if client == nil {
		client = &ClusterHubClient{}
	}
	response, err := client.GetLatestVersion(ctx, domain.HubURL, domain.Domain)
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
	snapshot, err := client.GetSnapshot(ctx, domain.HubURL, domain.Domain)
	if err != nil {
		return err
	}
	members := make([]model.ClusterMember, 0, len(snapshot.Members))
	for _, item := range snapshot.Members {
		members = append(members, model.ClusterMember{
			NodeID:      item.NodeID,
			Name:        item.Name,
			BaseURL:     item.BaseURL,
			PublicKey:   item.PublicKey,
			DomainID:    domain.Id,
			LastVersion: snapshot.Version,
		})
	}
	if err := store.ReplaceDomainMembers(domain.Id, members); err != nil {
		return err
	}
	domain.LastVersion = snapshot.Version
	if domain.LastVersion == 0 {
		domain.LastVersion = version
	}
	return store.SaveDomain(domain)
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
		if member.NodeID == excludeNodeID || member.BaseURL == "" || member.Domain == nil {
			continue
		}
		token, err := DecryptClusterDomainToken(secret, member.Domain.TokenEncrypted)
		if err != nil {
			return err
		}
		envelope, err := SignClusterNotifyVersionEnvelope(identity, member.Domain.Domain, version, time.Now().Unix())
		if err != nil {
			return err
		}
		body, err := json.Marshal(envelope)
		if err != nil {
			return err
		}
		request, err := http.NewRequestWithContext(ctx, http.MethodPost, clusterPeerMessageURL(member.BaseURL), bytes.NewReader(body))
		if err != nil {
			return err
		}
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("X-Cluster-Token", token)
		response, err := b.httpClient().Do(request)
		if err != nil {
			return err
		}
		if err := requireHTTPSuccess(response, "cluster peer notify"); err != nil {
			response.Body.Close()
			return err
		}
		if err := requireClusterPeerSuccess(response); err != nil {
			response.Body.Close()
			return err
		}
		response.Body.Close()
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

func clusterPeerMessageURL(baseURL string) string {
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return strings.TrimRight(baseURL, "/") + "/cluster/message"
	}
	parsed.Path = strings.TrimSuffix(parsed.Path, "/") + "/cluster/message"
	parsed.RawPath = ""
	return parsed.String()
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
