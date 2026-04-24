package service

import (
	"context"
	"testing"

	"github.com/alireza0/s-ui/src/backend/internal/infra/db/model"
)

func TestPeerDispatcherMarksUnsupportedEventWithoutError(t *testing.T) {
	store := newMemoryPeerStore()
	dispatcher := ClusterPeerDispatcher{eventStore: store}
	message := &PeerMessage{
		MessageID:   "msg-unsupported",
		DomainID:    "edge.example.com",
		PayloadHash: "hash",
		Category:    "event",
		Action:      "future.action",
	}
	if err := dispatcher.Dispatch(context.Background(), &model.ClusterDomain{Id: 1, Domain: "edge.example.com"}, &model.ClusterMember{NodeID: "node-a"}, message); err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	state, err := store.RecordReceived(message)
	if err != nil {
		t.Fatalf("state: %v", err)
	}
	if state.Status != PeerEventStatusUnsupported {
		t.Fatalf("expected unsupported, got %q", state.Status)
	}
}

func TestPeerDispatcherHandlesDomainClusterChanged(t *testing.T) {
	store := newMemoryPeerStore()
	syncer := &stubPeerSyncer{}
	dispatcher := ClusterPeerDispatcher{eventStore: store, syncService: &ClusterSyncService{hubSyncer: syncer, store: &stubPeerDispatcherSyncStore{domain: &model.ClusterDomain{Id: 1, Domain: "edge.example.com", HubURL: "https://hub.example.com"}}}}
	message := &PeerMessage{
		MessageID:   "msg-change",
		DomainID:    "edge.example.com",
		PayloadHash: "hash",
		Category:    "event",
		Action:      "domain.cluster.changed",
		Payload:     map[string]interface{}{"version": float64(9)},
	}
	if err := dispatcher.Dispatch(context.Background(), &model.ClusterDomain{Id: 1, Domain: "edge.example.com", HubURL: "https://hub.example.com"}, &model.ClusterMember{NodeID: "node-a", LastVersion: 1}, message); err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	state, err := store.RecordReceived(message)
	if err != nil {
		t.Fatalf("state: %v", err)
	}
	if state.Status != PeerEventStatusSucceeded {
		t.Fatalf("expected succeeded, got %q", state.Status)
	}
}

type stubPeerSyncer struct{}

func (s *stubPeerSyncer) LatestVersion(context.Context, *model.ClusterDomain) (int64, error) {
	return 9, nil
}

func (s *stubPeerSyncer) SyncDomain(context.Context, *model.ClusterDomain, int64) error {
	return nil
}

type stubPeerDispatcherSyncStore struct {
	domain *model.ClusterDomain
	member *model.ClusterMember
}

func (s *stubPeerDispatcherSyncStore) GetMember(uint, string) (*model.ClusterMember, error) {
	if s.member != nil {
		return s.member, nil
	}
	return &model.ClusterMember{NodeID: "node-a", LastVersion: 1}, nil
}

func (s *stubPeerDispatcherSyncStore) SaveMember(member *model.ClusterMember) error {
	s.member = member
	return nil
}

func (s *stubPeerDispatcherSyncStore) ListMembers() ([]model.ClusterMember, error) {
	if s.member == nil {
		return nil, nil
	}
	return []model.ClusterMember{*s.member}, nil
}

func (s *stubPeerDispatcherSyncStore) GetDomain(uint) (*model.ClusterDomain, error) {
	return s.domain, nil
}

func (s *stubPeerDispatcherSyncStore) SaveDomain(domain *model.ClusterDomain) error {
	s.domain = domain
	return nil
}

func (s *stubPeerDispatcherSyncStore) ListDomains() ([]model.ClusterDomain, error) {
	if s.domain == nil {
		return nil, nil
	}
	return []model.ClusterDomain{*s.domain}, nil
}
