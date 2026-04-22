package service

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"testing"

	"github.com/alireza0/s-ui/src/backend/internal/infra/db/model"
)

func TestClusterMessageEnvelopeAcceptsSignedSyncNotifyVersionV1(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate keypair: %v", err)
	}

	local := &model.ClusterLocalNode{
		NodeID:     "node-local",
		PublicKey:  base64.StdEncoding.EncodeToString(publicKey),
		PrivateKey: base64.StdEncoding.EncodeToString(privateKey),
	}

	envelope, err := SignClusterNotifyVersionEnvelope(local, "edge.example.com", 12, 1700000000)
	if err != nil {
		t.Fatalf("sign envelope: %v", err)
	}

	message, err := VerifyClusterEnvelope(envelope, local.PublicKey)
	if err != nil {
		t.Fatalf("verify envelope: %v", err)
	}
	if message.Version != 12 {
		t.Fatalf("expected notify version 12, got %d", message.Version)
	}
	if message.Domain != "edge.example.com" {
		t.Fatalf("expected domain edge.example.com, got %q", message.Domain)
	}
}

func TestClusterSyncServiceSuppressesDuplicateNotifyVersion(t *testing.T) {
	store := &stubClusterSyncStore{
		members: map[string]*model.ClusterMember{
			"node-a": {NodeID: "node-a", LastVersion: 7},
		},
	}
	service := &ClusterSyncService{store: store}

	processed, err := service.HandleIncomingNotifyVersion(context.Background(), "node-a", 7)
	if err != nil {
		t.Fatalf("handle duplicate notify version: %v", err)
	}
	if processed {
		t.Fatal("expected duplicate notify version to be suppressed")
	}
}

func TestClusterSyncServiceDoesNotRebroadcastReceivedNotifyVersion(t *testing.T) {
	store := &stubClusterSyncStore{
		domains: map[uint]*model.ClusterDomain{
			1: {Id: 1, Domain: "edge.example.com", HubURL: "https://hub.example.com", LastVersion: 1},
		},
		members: map[string]*model.ClusterMember{
			"node-a": {NodeID: "node-a", DomainID: 1, LastVersion: 3},
			"node-b": {NodeID: "node-b", LastVersion: 1},
		},
	}
	broadcaster := &stubClusterBroadcaster{}
	hub := &stubClusterHubSyncer{}
	service := &ClusterSyncService{store: store, broadcaster: broadcaster, hubSyncer: hub}

	processed, err := service.HandleIncomingNotifyVersion(context.Background(), "node-a", 4)
	if err != nil {
		t.Fatalf("handle notify version: %v", err)
	}
	if !processed {
		t.Fatal("expected fresh notify version to trigger sync")
	}
	if hub.syncCalls != 1 {
		t.Fatalf("expected one hub sync call, got %d", hub.syncCalls)
	}
	if broadcaster.calls != 0 {
		t.Fatalf("expected no rebroadcast for received notify version, got %d", broadcaster.calls)
	}
}

func TestClusterSyncServiceVersionPollIsNoOpWhenHubVersionUnchanged(t *testing.T) {
	store := &stubClusterSyncStore{
		domains: map[uint]*model.ClusterDomain{
			1: {Id: 1, Domain: "edge.example.com", HubURL: "https://hub.example.com", LastVersion: 9},
		},
	}
	hub := &stubClusterHubSyncer{latestVersions: []int64{9, 9}}
	service := &ClusterSyncService{store: store, hubSyncer: hub}

	if err := service.PollAndNotifyVersion(context.Background()); err != nil {
		t.Fatalf("first poll: %v", err)
	}
	if err := service.PollAndNotifyVersion(context.Background()); err != nil {
		t.Fatalf("second poll: %v", err)
	}
	if hub.syncCalls != 0 {
		t.Fatalf("expected unchanged hub version to skip sync, got %d syncs", hub.syncCalls)
	}
	if hub.versionChecks != 2 {
		t.Fatalf("expected two hub version checks, got %d", hub.versionChecks)
	}
}

func TestClusterSyncServiceManualPollSyncsFromHubWhenRemoteVersionNewer(t *testing.T) {
	store := &stubClusterSyncStore{
		domains: map[uint]*model.ClusterDomain{
			1: {Id: 1, Domain: "edge.example.com", HubURL: "https://hub.example.com", LastVersion: 2},
		},
	}
	hub := &stubClusterHubSyncer{latestVersions: []int64{5}}
	service := &ClusterSyncService{store: store, hubSyncer: hub}

	if err := service.PollAndNotifyVersion(context.Background()); err != nil {
		t.Fatalf("poll and sync: %v", err)
	}
	if hub.syncCalls != 1 {
		t.Fatalf("expected one hub snapshot sync, got %d", hub.syncCalls)
	}
	if hub.syncedVersions[0] != 5 {
		t.Fatalf("expected synced version 5, got %d", hub.syncedVersions[0])
	}
}

type stubClusterSyncStore struct {
	domains map[uint]*model.ClusterDomain
	members map[string]*model.ClusterMember
}

func (s *stubClusterSyncStore) GetMember(nodeID string) (*model.ClusterMember, error) {
	member := s.members[nodeID]
	if member == nil {
		return nil, errClusterMemberNotFound
	}
	copy := *member
	return &copy, nil
}

func (s *stubClusterSyncStore) SaveMember(member *model.ClusterMember) error {
	copy := *member
	s.members[member.NodeID] = &copy
	return nil
}

func (s *stubClusterSyncStore) ListMembers() ([]model.ClusterMember, error) {
	members := make([]model.ClusterMember, 0, len(s.members))
	for _, member := range s.members {
		members = append(members, *member)
	}
	return members, nil
}

func (s *stubClusterSyncStore) GetDomain(id uint) (*model.ClusterDomain, error) {
	domain := s.domains[id]
	if domain == nil {
		return nil, errClusterDomainNotFound
	}
	copy := *domain
	return &copy, nil
}

func (s *stubClusterSyncStore) SaveDomain(domain *model.ClusterDomain) error {
	copy := *domain
	s.domains[domain.Id] = &copy
	return nil
}

func (s *stubClusterSyncStore) ListDomains() ([]model.ClusterDomain, error) {
	domains := make([]model.ClusterDomain, 0, len(s.domains))
	for _, domain := range s.domains {
		domains = append(domains, *domain)
	}
	return domains, nil
}

type stubClusterSyncRunner struct {
	calls    int
	nodeIDs  []string
	versions []int64
}

func (s *stubClusterSyncRunner) SyncMember(_ context.Context, nodeID string, version int64) error {
	s.calls++
	s.nodeIDs = append(s.nodeIDs, nodeID)
	s.versions = append(s.versions, version)
	return nil
}

type stubClusterBroadcaster struct {
	calls    int
	versions []int64
	excludes []string
}

func (s *stubClusterBroadcaster) BroadcastNotifyVersion(_ context.Context, version int64, excludeNodeID string) error {
	s.calls++
	s.versions = append(s.versions, version)
	s.excludes = append(s.excludes, excludeNodeID)
	return nil
}

type stubClusterVersionSource struct {
	versions []int64
	index    int
}

type stubClusterHubSyncer struct {
	latestVersions []int64
	versionChecks  int
	syncCalls      int
	syncedVersions []int64
}

func (s *stubClusterHubSyncer) LatestVersion(_ context.Context, _ *model.ClusterDomain) (int64, error) {
	s.versionChecks++
	index := s.versionChecks - 1
	if index >= len(s.latestVersions) {
		index = len(s.latestVersions) - 1
	}
	return s.latestVersions[index], nil
}

func (s *stubClusterHubSyncer) SyncDomain(_ context.Context, _ *model.ClusterDomain, version int64) error {
	s.syncCalls++
	s.syncedVersions = append(s.syncedVersions, version)
	return nil
}

func (s *stubClusterVersionSource) CurrentVersion(context.Context) (int64, error) {
	value := s.versions[s.index]
	if s.index < len(s.versions)-1 {
		s.index++
	}
	return value, nil
}
