package service

import (
	"context"
	"testing"

	"github.com/alireza0/s-ui/src/backend/internal/infra/db/model"
)

func TestClusterServiceRegisterPersistsHubURLOnDomain(t *testing.T) {
	store := &stubClusterServiceStore{}
	hub := &stubClusterHubClient{
		registerResponse: &ClusterHubRegisterNodeResponse{Member: ClusterHubMemberResponse{NodeID: "node-a"}},
	}
	service := &ClusterService{
		secretProvider: stubClusterSecretProvider{secret: []byte("panel-secret-for-cluster-tests")},
		localIdentity:  ClusterLocalIdentityService{store: &stubClusterLocalNodeStore{}},
		store:          store,
		hubClient:      hub,
	}

	_, err := service.Register(ClusterRegisterRequest{
		HubURL: "https://hub.example.com",
		Domain: "edge.example.com",
		Token:  "cluster-token",
	})
	if err != nil {
		t.Fatalf("register cluster domain: %v", err)
	}
	if len(store.savedDomains) != 1 {
		t.Fatalf("expected one saved domain, got %d", len(store.savedDomains))
	}
	if store.savedDomains[0].HubURL != "https://hub.example.com" {
		t.Fatalf("expected persisted hub URL, got %q", store.savedDomains[0].HubURL)
	}
	if len(store.savedMembers) != 1 {
		t.Fatalf("expected one saved member, got %d", len(store.savedMembers))
	}
	if store.savedMembers[0].DomainID != store.savedDomains[0].Id {
		t.Fatalf("expected member domain id %d, got %d", store.savedDomains[0].Id, store.savedMembers[0].DomainID)
	}
}

func TestClusterServiceRegisterDoesNotPersistDomainWhenHubRegistrationFails(t *testing.T) {
	store := &stubClusterServiceStore{}
	hub := &stubClusterHubClient{registerErr: errTestClusterHubFailure}
	service := &ClusterService{
		secretProvider: stubClusterSecretProvider{secret: []byte("panel-secret-for-cluster-tests")},
		localIdentity:  ClusterLocalIdentityService{store: &stubClusterLocalNodeStore{}},
		store:          store,
		hubClient:      hub,
	}

	_, err := service.Register(ClusterRegisterRequest{
		HubURL: "https://hub.example.com",
		Domain: "edge.example.com",
		Token:  "cluster-token",
	})
	if err == nil {
		t.Fatal("expected hub registration failure")
	}
	if len(store.savedDomains) != 0 {
		t.Fatalf("expected no persisted domains on failed registration, got %d", len(store.savedDomains))
	}
}

type stubClusterServiceStore struct {
	domains      map[string]*model.ClusterDomain
	savedDomains []*model.ClusterDomain
	members      map[string]*model.ClusterMember
	savedMembers []*model.ClusterMember
}

func (s *stubClusterServiceStore) GetDomainByName(name string) (*model.ClusterDomain, error) {
	if s.domains == nil || s.domains[name] == nil {
		return nil, errClusterDomainNotFound
	}
	copy := *s.domains[name]
	return &copy, nil
}

func (s *stubClusterServiceStore) SaveDomain(domain *model.ClusterDomain) error {
	if s.domains == nil {
		s.domains = map[string]*model.ClusterDomain{}
	}
	copy := *domain
	if copy.Id == 0 {
		copy.Id = uint(len(s.domains) + 1)
		domain.Id = copy.Id
	}
	s.domains[copy.Domain] = &copy
	s.savedDomains = append(s.savedDomains, &copy)
	return nil
}

func (s *stubClusterServiceStore) ListDomains() ([]model.ClusterDomain, error) { return nil, nil }
func (s *stubClusterServiceStore) GetDomain(id uint) (*model.ClusterDomain, error) {
	for _, domain := range s.domains {
		if domain.Id == id {
			copy := *domain
			return &copy, nil
		}
	}
	return nil, errClusterDomainNotFound
}
func (s *stubClusterServiceStore) GetMemberByNodeID(nodeID string) (*model.ClusterMember, error) {
	if s.members == nil || s.members[nodeID] == nil {
		return nil, errClusterMemberNotFound
	}
	copy := *s.members[nodeID]
	return &copy, nil
}
func (s *stubClusterServiceStore) SaveMember(member *model.ClusterMember) error {
	copy := *member
	s.savedMembers = append(s.savedMembers, &copy)
	if s.members == nil {
		s.members = map[string]*model.ClusterMember{}
	}
	s.members[copy.NodeID] = &copy
	return nil
}
func (s *stubClusterServiceStore) ListMembers() ([]model.ClusterMember, error) { return nil, nil }
func (s *stubClusterServiceStore) DeleteMember(uint) error                     { return nil }
func (s *stubClusterServiceStore) ReplaceDomainMembers(uint, []model.ClusterMember) error {
	return nil
}

type stubClusterHubClient struct {
	registerResponse *ClusterHubRegisterNodeResponse
	registerErr      error
}

func (s *stubClusterHubClient) RegisterNode(_ context.Context, _ string, _ ClusterHubRegisterNodeRequest) (*ClusterHubRegisterNodeResponse, error) {
	if s.registerErr != nil {
		return nil, s.registerErr
	}
	return s.registerResponse, nil
}

func (s *stubClusterHubClient) GetLatestVersion(context.Context, string, string) (*ClusterHubVersionResponse, error) {
	return nil, nil
}

func (s *stubClusterHubClient) GetSnapshot(context.Context, string, string) (*ClusterHubSnapshotResponse, error) {
	return nil, nil
}

type stubClusterSecretProvider struct{ secret []byte }

func (s stubClusterSecretProvider) GetSecret() ([]byte, error) { return s.secret, nil }

var errTestClusterHubFailure = context.Canceled
