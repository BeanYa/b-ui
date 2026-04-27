package service

import (
	"context"
	"testing"

	"github.com/alireza0/b-ui/src/backend/internal/infra/db/model"
)

func TestClusterHubSyncerSyncDomainPersistsEncryptedPeerTokenPerMember(t *testing.T) {
	store := &stubClusterRuntimeStore{}
	syncer := &ClusterHubSyncer{
		client: &stubClusterRuntimeHubClient{snapshot: &ClusterHubSnapshotResponse{
			Version: 7,
			Members: []ClusterHubMemberResponse{{
				NodeID:    "node-a",
				Name:      "alpha",
				BaseURL:   "https://node-a.example.com",
				PublicKey: "public-key-a",
				PeerToken: "peer-token-a",
			}},
		}},
		store:          store,
		secretProvider: stubClusterSecretProvider{secret: []byte("panel-secret-for-cluster-tests")},
		reachability:   newTestClusterRuntimeReachabilityService(10),
	}
	domain := &model.ClusterDomain{Id: 1, Domain: "edge.example.com", HubURL: "https://hub.example.com", TokenEncrypted: mustEncryptClusterToken(t, "panel-secret-for-cluster-tests", "domain-token")}

	if err := syncer.SyncDomain(context.Background(), domain, 7); err != nil {
		t.Fatalf("sync domain: %v", err)
	}
	if len(store.replaceCalls) != 1 {
		t.Fatalf("expected one replace call, got %d", len(store.replaceCalls))
	}
	member := store.replaceCalls[0].members[0]
	if member.PeerTokenEncrypted == "" {
		t.Fatal("expected encrypted peer token to be persisted")
	}
	if member.PeerTokenEncrypted == "peer-token-a" {
		t.Fatal("expected peer token to be encrypted at rest")
	}
	decrypted, err := DecryptClusterDomainToken([]byte("panel-secret-for-cluster-tests"), member.PeerTokenEncrypted)
	if err != nil {
		t.Fatalf("decrypt peer token: %v", err)
	}
	if decrypted != "peer-token-a" {
		t.Fatalf("expected decrypted peer token %q, got %q", "peer-token-a", decrypted)
	}
}

func TestClusterHubSyncerSyncDomainKeepsDistinctPeerTokensForSameNodeAcrossDomains(t *testing.T) {
	store := &stubClusterRuntimeStore{}
	syncer := &ClusterHubSyncer{
		client:         &stubClusterRuntimeHubClient{},
		store:          store,
		secretProvider: stubClusterSecretProvider{secret: []byte("panel-secret-for-cluster-tests")},
		reachability:   newTestClusterRuntimeReachabilityService(10),
	}
	domainA := &model.ClusterDomain{Id: 1, Domain: "edge-a.example.com", HubURL: "https://hub.example.com", TokenEncrypted: mustEncryptClusterToken(t, "panel-secret-for-cluster-tests", "domain-token-a")}
	domainB := &model.ClusterDomain{Id: 2, Domain: "edge-b.example.com", HubURL: "https://hub.example.com", TokenEncrypted: mustEncryptClusterToken(t, "panel-secret-for-cluster-tests", "domain-token-b")}
	syncer.client.(*stubClusterRuntimeHubClient).snapshots = map[string]*ClusterHubSnapshotResponse{
		"edge-a.example.com": {Version: 3, Members: []ClusterHubMemberResponse{{NodeID: "node-shared", PeerToken: "peer-token-a"}}},
		"edge-b.example.com": {Version: 4, Members: []ClusterHubMemberResponse{{NodeID: "node-shared", PeerToken: "peer-token-b"}}},
	}

	if err := syncer.SyncDomain(context.Background(), domainA, 3); err != nil {
		t.Fatalf("sync domain A: %v", err)
	}
	if err := syncer.SyncDomain(context.Background(), domainB, 4); err != nil {
		t.Fatalf("sync domain B: %v", err)
	}
	if len(store.membersByDomain[1]) != 1 || len(store.membersByDomain[2]) != 1 {
		t.Fatalf("expected separate stored members per domain, got %#v", store.membersByDomain)
	}
	peerA, err := DecryptClusterDomainToken([]byte("panel-secret-for-cluster-tests"), store.membersByDomain[1][0].PeerTokenEncrypted)
	if err != nil {
		t.Fatalf("decrypt domain A peer token: %v", err)
	}
	peerB, err := DecryptClusterDomainToken([]byte("panel-secret-for-cluster-tests"), store.membersByDomain[2][0].PeerTokenEncrypted)
	if err != nil {
		t.Fatalf("decrypt domain B peer token: %v", err)
	}
	if peerA != "peer-token-a" || peerB != "peer-token-b" {
		t.Fatalf("expected distinct peer tokens per domain, got %q and %q", peerA, peerB)
	}
}

func TestClusterHubSyncerSyncDomainClearsReachabilityForRemovedMembers(t *testing.T) {
	reachabilityStore := newStubClusterReachabilityStore()
	if err := reachabilityStore.SaveReachability(&model.ClusterPeerReachability{
		DomainID:     1,
		TargetNodeID: "node-old",
		State:        ClusterReachabilityUnreachable,
	}); err != nil {
		t.Fatalf("seed removed member reachability: %v", err)
	}

	store := &stubClusterRuntimeStore{reachabilityStore: reachabilityStore}
	syncer := &ClusterHubSyncer{
		client: &stubClusterRuntimeHubClient{
			snapshot: &ClusterHubSnapshotResponse{
				Version: 7,
				Members: []ClusterHubMemberResponse{{NodeID: "node-local"}, {NodeID: "node-new"}},
			},
		},
		store:          store,
		secretProvider: stubClusterSecretProvider{secret: []byte("panel-secret-for-cluster-tests")},
		reachability: &ClusterReachabilityService{
			store:  reachabilityStore,
			policy: DefaultClusterReachabilityPolicy(),
			now: func() int64 {
				return 10
			},
		},
	}
	domain := &model.ClusterDomain{Id: 1, Domain: "edge.example.com", HubURL: "https://hub.example.com", TokenEncrypted: mustEncryptClusterToken(t, "panel-secret-for-cluster-tests", "domain-token")}

	if err := syncer.SyncDomain(context.Background(), domain, 7); err != nil {
		t.Fatalf("sync domain: %v", err)
	}
	entry, err := reachabilityStore.GetReachability(1, "node-old")
	if err != nil {
		t.Fatalf("load removed member reachability: %v", err)
	}
	if entry != nil {
		t.Fatalf("expected removed member reachability to be cleared, got %#v", entry)
	}
}

type stubClusterRuntimeHubClient struct {
	snapshot  *ClusterHubSnapshotResponse
	snapshots map[string]*ClusterHubSnapshotResponse
}

func (s *stubClusterRuntimeHubClient) RegisterNode(context.Context, string, ClusterHubRegisterNodeRequest) (*ClusterHubOperationResponse, error) {
	return nil, nil
}

func (s *stubClusterRuntimeHubClient) GetLatestVersion(context.Context, string, string, string) (*ClusterHubVersionResponse, error) {
	return nil, nil
}

func (s *stubClusterRuntimeHubClient) GetSnapshot(_ context.Context, _ string, domain string, _ string) (*ClusterHubSnapshotResponse, error) {
	if s.snapshots != nil {
		return s.snapshots[domain], nil
	}
	return s.snapshot, nil
}

func (s *stubClusterRuntimeHubClient) DeleteMember(context.Context, string, string, string, string) (*ClusterHubOperationResponse, error) {
	return nil, nil
}

func (s *stubClusterRuntimeHubClient) ClaimUpdate(context.Context, string, string, string, string, string) (*ClusterHubClaimUpdateResponse, error) {
	return &ClusterHubClaimUpdateResponse{Proceed: true}, nil
}

func (s *stubClusterRuntimeHubClient) SetMemberStatus(context.Context, string, string, string, string, string, string, string) (*ClusterHubMemberStatusResponse, error) {
	return &ClusterHubMemberStatusResponse{OK: true}, nil
}

type stubClusterRuntimeStore struct {
	replaceCalls      []stubClusterRuntimeReplaceCall
	membersByDomain   map[uint][]model.ClusterMember
	savedDomainState  []*model.ClusterDomain
	reachabilityStore *stubClusterReachabilityStore
}

func newTestClusterRuntimeReachabilityService(now int64) *ClusterReachabilityService {
	return &ClusterReachabilityService{
		store:  newStubClusterReachabilityStore(),
		policy: DefaultClusterReachabilityPolicy(),
		now: func() int64 {
			return now
		},
	}
}

type stubClusterRuntimeReplaceCall struct {
	domainID uint
	members  []model.ClusterMember
}

func (s *stubClusterRuntimeStore) GetDomainByName(string) (*model.ClusterDomain, error) {
	return nil, nil
}
func (s *stubClusterRuntimeStore) SaveDomain(domain *model.ClusterDomain) error {
	copy := *domain
	s.savedDomainState = append(s.savedDomainState, &copy)
	return nil
}
func (s *stubClusterRuntimeStore) ListDomains() ([]model.ClusterDomain, error)  { return nil, nil }
func (s *stubClusterRuntimeStore) GetDomain(uint) (*model.ClusterDomain, error) { return nil, nil }
func (s *stubClusterRuntimeStore) GetMember(uint) (*model.ClusterMember, error) {
	return nil, errClusterMemberNotFound
}
func (s *stubClusterRuntimeStore) GetMemberByNodeID(string) (*model.ClusterMember, error) {
	return nil, errClusterMemberNotFound
}
func (s *stubClusterRuntimeStore) GetMemberByDomainNodeID(uint, string) (*model.ClusterMember, error) {
	return nil, errClusterMemberNotFound
}
func (s *stubClusterRuntimeStore) SaveMember(*model.ClusterMember) error       { return nil }
func (s *stubClusterRuntimeStore) ListMembers() ([]model.ClusterMember, error) { return nil, nil }
func (s *stubClusterRuntimeStore) DeleteMember(uint) error                     { return nil }
func (s *stubClusterRuntimeStore) DeleteDomain(uint) error                     { return nil }
func (s *stubClusterRuntimeStore) ReplaceDomainMembers(domainID uint, members []model.ClusterMember) error {
	copyMembers := make([]model.ClusterMember, len(members))
	copy(copyMembers, members)
	if s.membersByDomain == nil {
		s.membersByDomain = map[uint][]model.ClusterMember{}
	}
	s.membersByDomain[domainID] = copyMembers
	s.replaceCalls = append(s.replaceCalls, stubClusterRuntimeReplaceCall{domainID: domainID, members: copyMembers})
	return nil
}
