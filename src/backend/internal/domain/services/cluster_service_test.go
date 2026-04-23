package service

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"testing"

	"github.com/alireza0/s-ui/src/backend/internal/infra/db/model"
)

func TestClusterServiceRegisterPersistsHubURLOnDomain(t *testing.T) {
	store := &stubClusterServiceStore{}
	hub := &stubClusterHubClient{
		registerResponse: &ClusterHubOperationResponse{OperationID: "op-register", Status: "completed"},
		snapshotResponse: &ClusterHubSnapshotResponse{Version: 4, Members: []ClusterHubMemberResponse{{NodeID: "node-a", BaseURL: "https://node-a.example.com", PeerToken: "peer-token-a"}}},
	}
	service := &ClusterService{
		secretProvider: stubClusterSecretProvider{secret: []byte("panel-secret-for-cluster-tests")},
		localIdentity:  ClusterLocalIdentityService{store: &stubClusterLocalNodeStore{}},
		store:          store,
		hubClient:      hub,
	}

	_, err := service.Register(ClusterRegisterRequest{
		HubURL:  "https://hub.example.com",
		Domain:  "edge.example.com",
		Token:   "cluster-token",
		BaseURL: "https://panel.example.com/app/",
	})
	if err != nil {
		t.Fatalf("register cluster domain: %v", err)
	}
	if hub.lastRegisterRequest.Member.BaseURL != "https://panel.example.com/app/" {
		t.Fatalf("expected register request base URL to be forwarded, got %q", hub.lastRegisterRequest.Member.BaseURL)
	}
	if hub.lastRegisterRequest.Member.Address != "https://panel.example.com/app/" {
		t.Fatalf("expected register request address to be forwarded, got %q", hub.lastRegisterRequest.Member.Address)
	}
	if len(store.savedDomains) < 1 {
		t.Fatalf("expected saved domain state, got %d", len(store.savedDomains))
	}
	lastDomain := store.savedDomains[len(store.savedDomains)-1]
	if lastDomain.HubURL != "https://hub.example.com" {
		t.Fatalf("expected persisted hub URL, got %q", lastDomain.HubURL)
	}
	if len(store.replacedMembers) != 1 || len(store.replacedMembers[0]) != 1 {
		t.Fatalf("expected one replaced member, got %#v", store.replacedMembers)
	}
	if store.replacedMembers[0][0].DomainID != lastDomain.Id {
		t.Fatalf("expected member domain id %d, got %d", lastDomain.Id, store.replacedMembers[0][0].DomainID)
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
		HubURL:  "https://hub.example.com",
		Domain:  "edge.example.com",
		Token:   "cluster-token",
		BaseURL: "https://panel.example.com/app/",
	})
	if err == nil {
		t.Fatal("expected hub registration failure")
	}
	if len(store.savedDomains) != 0 {
		t.Fatalf("expected no persisted domains on failed registration, got %d", len(store.savedDomains))
	}
}

func TestClusterServiceRegisterRequiresHubURL(t *testing.T) {
	store := &stubClusterServiceStore{}
	hub := &stubClusterHubClient{}
	service := &ClusterService{
		secretProvider: stubClusterSecretProvider{secret: []byte("panel-secret-for-cluster-tests")},
		localIdentity:  ClusterLocalIdentityService{store: &stubClusterLocalNodeStore{}},
		store:          store,
		hubClient:      hub,
	}

	_, err := service.Register(ClusterRegisterRequest{
		Domain:  "edge.example.com",
		Token:   "cluster-token",
		BaseURL: "https://panel.example.com/app/",
	})
	if err == nil {
		t.Fatal("expected missing hub URL to fail")
	}
	if hub.registerCalls != 0 {
		t.Fatalf("expected no hub register calls, got %d", hub.registerCalls)
	}
	if len(store.savedDomains) != 0 {
		t.Fatalf("expected no persisted domains, got %d", len(store.savedDomains))
	}
}

func TestClusterServiceRegisterRequiresBaseURL(t *testing.T) {
	store := &stubClusterServiceStore{}
	hub := &stubClusterHubClient{}
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
		t.Fatal("expected missing base URL to fail")
	}
	if hub.registerCalls != 0 {
		t.Fatalf("expected no hub register calls, got %d", hub.registerCalls)
	}
	if len(store.savedDomains) != 0 {
		t.Fatalf("expected no persisted domains, got %d", len(store.savedDomains))
	}
}

func TestClusterServiceDeleteMemberDeletesHubMembershipBeforeLocalMirror(t *testing.T) {
	secret := []byte("panel-secret-for-cluster-tests")
	store := &stubClusterServiceStore{
		domains: map[string]*model.ClusterDomain{
			"edge.example.com": {Id: 1, Domain: "edge.example.com", HubURL: "https://hub.example.com", TokenEncrypted: mustEncryptClusterToken(t, string(secret), "domain-token")},
		},
		members: map[string]*model.ClusterMember{
			serviceMemberKey(1, "node-a"): {Id: 7, NodeID: "node-a", DomainID: 1},
		},
	}
	hub := &stubClusterHubClient{}
	service := &ClusterService{
		secretProvider: stubClusterSecretProvider{secret: secret},
		store:          store,
		hubClient:      hub,
	}

	if err := service.DeleteMember(7); err != nil {
		t.Fatalf("delete member: %v", err)
	}
	if store.deletedMemberID != 7 {
		t.Fatalf("expected local member delete for id 7, got %d", store.deletedMemberID)
	}
}

func TestClusterServiceReceiveMessageRejectsWrongPeerTokenEvenWhenDomainTokenMatches(t *testing.T) {
	secret := []byte("panel-secret-for-cluster-tests")
	sourcePublicKey, sourcePrivateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate source keypair: %v", err)
	}
	store := &stubClusterServiceStore{
		domains: map[string]*model.ClusterDomain{
			"edge.example.com": {Id: 1, Domain: "edge.example.com", TokenEncrypted: mustEncryptClusterToken(t, string(secret), "domain-token")},
		},
		members: map[string]*model.ClusterMember{
			serviceMemberKey(1, "node-local"):  {NodeID: "node-local", DomainID: 1, PeerTokenEncrypted: mustEncryptClusterToken(t, string(secret), "peer-token-local")},
			serviceMemberKey(1, "node-source"): {NodeID: "node-source", DomainID: 1, PublicKey: base64.StdEncoding.EncodeToString(sourcePublicKey)},
		},
	}
	service := &ClusterService{
		secretProvider: stubClusterSecretProvider{secret: secret},
		localIdentity:  ClusterLocalIdentityService{store: &stubClusterLocalNodeStore{node: &model.ClusterLocalNode{NodeID: "node-local"}}},
		store:          store,
	}
	envelope, err := SignClusterNotifyVersionEnvelope(&model.ClusterLocalNode{NodeID: "node-source", PrivateKey: base64.StdEncoding.EncodeToString(sourcePrivateKey)}, "edge.example.com", 9, 1700000000)
	if err != nil {
		t.Fatalf("sign envelope: %v", err)
	}

	err = service.ReceiveMessage(envelope, "domain-token")
	if err == nil {
		t.Fatal("expected wrong peer token to be rejected")
	}
}

func TestClusterServiceReceiveMessageAcceptsLocalMemberPeerTokenForCorrectDomain(t *testing.T) {
	secret := []byte("panel-secret-for-cluster-tests")
	sourcePublicKey, sourcePrivateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate source keypair: %v", err)
	}
	store := &stubClusterServiceStore{
		domains: map[string]*model.ClusterDomain{
			"edge.example.com": {Id: 1, Domain: "edge.example.com", TokenEncrypted: mustEncryptClusterToken(t, string(secret), "domain-token")},
		},
		members: map[string]*model.ClusterMember{
			serviceMemberKey(1, "node-local"):  {NodeID: "node-local", DomainID: 1, PeerTokenEncrypted: mustEncryptClusterToken(t, string(secret), "peer-token-local")},
			serviceMemberKey(1, "node-source"): {NodeID: "node-source", DomainID: 1, PublicKey: base64.StdEncoding.EncodeToString(sourcePublicKey)},
		},
	}
	service := &ClusterService{
		secretProvider: stubClusterSecretProvider{secret: secret},
		localIdentity:  ClusterLocalIdentityService{store: &stubClusterLocalNodeStore{node: &model.ClusterLocalNode{NodeID: "node-local"}}},
		store:          store,
	}
	envelope, err := SignClusterNotifyVersionEnvelope(&model.ClusterLocalNode{NodeID: "node-source", PrivateKey: base64.StdEncoding.EncodeToString(sourcePrivateKey)}, "edge.example.com", 9, 1700000000)
	if err != nil {
		t.Fatalf("sign envelope: %v", err)
	}

	if err := service.ReceiveMessage(envelope, "peer-token-local"); err != nil {
		t.Fatalf("expected correct peer token to be accepted, got %v", err)
	}
}

type stubClusterServiceStore struct {
	domains         map[string]*model.ClusterDomain
	savedDomains    []*model.ClusterDomain
	members         map[string]*model.ClusterMember
	savedMembers    []*model.ClusterMember
	replacedMembers [][]model.ClusterMember
	deletedMemberID uint
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
func (s *stubClusterServiceStore) GetMember(id uint) (*model.ClusterMember, error) {
	for _, member := range s.members {
		if member.Id == id {
			copy := *member
			return &copy, nil
		}
	}
	return nil, errClusterMemberNotFound
}
func (s *stubClusterServiceStore) GetMemberByNodeID(nodeID string) (*model.ClusterMember, error) {
	if s.members == nil {
		return nil, errClusterMemberNotFound
	}
	for _, member := range s.members {
		if member.NodeID == nodeID {
			copy := *member
			return &copy, nil
		}
	}
	return nil, errClusterMemberNotFound
}

func (s *stubClusterServiceStore) GetMemberByDomainNodeID(domainID uint, nodeID string) (*model.ClusterMember, error) {
	key := serviceMemberKey(domainID, nodeID)
	if s.members == nil || s.members[key] == nil {
		return nil, errClusterMemberNotFound
	}
	copy := *s.members[key]
	return &copy, nil
}
func (s *stubClusterServiceStore) SaveMember(member *model.ClusterMember) error {
	copy := *member
	s.savedMembers = append(s.savedMembers, &copy)
	if s.members == nil {
		s.members = map[string]*model.ClusterMember{}
	}
	s.members[serviceMemberKey(copy.DomainID, copy.NodeID)] = &copy
	return nil
}
func (s *stubClusterServiceStore) ListMembers() ([]model.ClusterMember, error) {
	members := make([]model.ClusterMember, 0, len(s.members))
	for _, member := range s.members {
		members = append(members, *member)
	}
	return members, nil
}
func (s *stubClusterServiceStore) DeleteMember(id uint) error {
	s.deletedMemberID = id
	return nil
}
func (s *stubClusterServiceStore) ReplaceDomainMembers(_ uint, members []model.ClusterMember) error {
	copyMembers := make([]model.ClusterMember, len(members))
	copy(copyMembers, members)
	s.replacedMembers = append(s.replacedMembers, copyMembers)
	return nil
}

type stubClusterHubClient struct {
	registerResponse    *ClusterHubOperationResponse
	snapshotResponse    *ClusterHubSnapshotResponse
	deleteResponse      *ClusterHubOperationResponse
	registerErr         error
	deleteErr           error
	registerCalls       int
	lastRegisterRequest ClusterHubRegisterNodeRequest
}

func (s *stubClusterHubClient) RegisterNode(_ context.Context, _ string, request ClusterHubRegisterNodeRequest) (*ClusterHubOperationResponse, error) {
	s.registerCalls++
	s.lastRegisterRequest = request
	if s.registerErr != nil {
		return nil, s.registerErr
	}
	return s.registerResponse, nil
}

func (s *stubClusterHubClient) GetLatestVersion(context.Context, string, string, string) (*ClusterHubVersionResponse, error) {
	return nil, nil
}

func (s *stubClusterHubClient) GetSnapshot(context.Context, string, string, string) (*ClusterHubSnapshotResponse, error) {
	return s.snapshotResponse, nil
}

func (s *stubClusterHubClient) DeleteMember(context.Context, string, string, string, string) (*ClusterHubOperationResponse, error) {
	if s.deleteErr != nil {
		return nil, s.deleteErr
	}
	if s.deleteResponse != nil {
		return s.deleteResponse, nil
	}
	return &ClusterHubOperationResponse{OperationID: "op-delete", Status: "completed"}, nil
}

type stubClusterSecretProvider struct{ secret []byte }

func (s stubClusterSecretProvider) GetSecret() ([]byte, error) { return s.secret, nil }

var errTestClusterHubFailure = context.Canceled

func serviceMemberKey(domainID uint, nodeID string) string {
	return fmt.Sprintf("%d:%s", domainID, nodeID)
}
