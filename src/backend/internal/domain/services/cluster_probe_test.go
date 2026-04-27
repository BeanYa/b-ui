package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/BeanYa/b-ui/src/backend/internal/infra/db/model"
)

type stubClusterProbeStore struct {
	localNodeID string
	members     []model.ClusterMember
}

func (s *stubClusterProbeStore) ListMembersWithDomain() ([]model.ClusterMember, error) {
	return s.members, nil
}

func (s *stubClusterProbeStore) GetLocalNodeID() (string, error) {
	return s.localNodeID, nil
}

func TestClusterPeerProbeServiceMarksIdlePeerReachableOnHeartbeatSuccess(t *testing.T) {
	var receivedPath string
	var receivedToken string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		receivedToken = r.Header.Get("X-Cluster-Token")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"status":"ok"}`))
	}))
	defer server.Close()

	reachabilityStore := newStubClusterReachabilityStore()
	entry, err := reachabilityStore.GetReachability(1, "node-b")
	if err != nil {
		t.Fatalf("seed lookup: %v", err)
	}
	if entry != nil {
		t.Fatalf("expected no reachability row before seeding, got %#v", entry)
	}
	if err := reachabilityStore.SaveReachability(&model.ClusterPeerReachability{
		DomainID:              1,
		TargetNodeID:          "node-b",
		State:                 ClusterReachabilityUnreachable,
		LastObservedAt:        time.Unix(1_700_000_000, 0).Add(-11 * time.Minute).Unix(),
		NextProbeAt:           0,
		LastObservationSource: "business",
	}); err != nil {
		t.Fatalf("seed reachability: %v", err)
	}

	prober := &ClusterPeerProbeService{
		store: &stubClusterProbeStore{
			localNodeID: "node-a",
			members: []model.ClusterMember{
				{NodeID: "node-a", DomainID: 1, Domain: &model.ClusterDomain{Id: 1, Domain: "edge.example.com", CommunicationEndpointPath: "/_cluster", CommunicationProtocolVersion: "v1"}},
				{
					NodeID:             "node-b",
					DomainID:           1,
					BaseURL:            server.URL + "/panel/",
					PeerTokenEncrypted: mustEncryptClusterToken(t, "panel-secret-for-cluster-tests", "peer-token-b"),
					Domain:             &model.ClusterDomain{Id: 1, Domain: "edge.example.com", CommunicationEndpointPath: "/_cluster", CommunicationProtocolVersion: "v1"},
				},
			},
		},
		reachability: &ClusterReachabilityService{
			store:  reachabilityStore,
			policy: DefaultClusterReachabilityPolicy(),
			now: func() int64 {
				return time.Unix(1_700_000_000, 0).Unix()
			},
		},
		secretProvider: stubClusterSecretProvider{secret: []byte("panel-secret-for-cluster-tests")},
		httpClient:     server.Client(),
	}

	if err := prober.ProbeIdlePeers(context.Background()); err != nil {
		t.Fatalf("probe idle peers: %v", err)
	}

	updated, err := reachabilityStore.GetReachability(1, "node-b")
	if err != nil {
		t.Fatalf("load updated reachability: %v", err)
	}
	if updated == nil || updated.State != ClusterReachabilityReachable {
		t.Fatalf("expected reachable after heartbeat success, got %#v", updated)
	}
	if updated.LastObservationSource != "probe" {
		t.Fatalf("expected probe observation source, got %#v", updated)
	}
	if receivedPath != "/panel/_cluster/v1/heartbeat" {
		t.Fatalf("expected heartbeat path /panel/_cluster/v1/heartbeat, got %q", receivedPath)
	}
	if receivedToken != "peer-token-b" {
		t.Fatalf("expected peer token header, got %q", receivedToken)
	}
}

func TestClusterPeerProbeServiceTreatsProtocolRejectionAsTransportSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"rejected","code":"invalid_token","nodeId":"node-b"}`))
	}))
	defer server.Close()

	reachabilityStore := newStubClusterReachabilityStore()
	if err := reachabilityStore.SaveReachability(&model.ClusterPeerReachability{
		DomainID:       1,
		TargetNodeID:   "node-b",
		State:          ClusterReachabilitySuspect,
		LastObservedAt: time.Unix(1_700_000_000, 0).Add(-11 * time.Minute).Unix(),
	}); err != nil {
		t.Fatalf("seed reachability: %v", err)
	}

	prober := &ClusterPeerProbeService{
		store: &stubClusterProbeStore{
			localNodeID: "node-a",
			members: []model.ClusterMember{
				{NodeID: "node-a", DomainID: 1, Domain: &model.ClusterDomain{Id: 1, Domain: "edge.example.com", CommunicationEndpointPath: "/_cluster", CommunicationProtocolVersion: "v1"}},
				{
					NodeID:             "node-b",
					DomainID:           1,
					BaseURL:            server.URL,
					PeerTokenEncrypted: mustEncryptClusterToken(t, "panel-secret-for-cluster-tests", "peer-token-b"),
					Domain:             &model.ClusterDomain{Id: 1, Domain: "edge.example.com", CommunicationEndpointPath: "/_cluster", CommunicationProtocolVersion: "v1"},
				},
			},
		},
		reachability: &ClusterReachabilityService{
			store:  reachabilityStore,
			policy: DefaultClusterReachabilityPolicy(),
			now: func() int64 {
				return time.Unix(1_700_000_000, 0).Unix()
			},
		},
		secretProvider: stubClusterSecretProvider{secret: []byte("panel-secret-for-cluster-tests")},
		httpClient:     server.Client(),
	}

	if err := prober.ProbeIdlePeers(context.Background()); err != nil {
		t.Fatalf("probe invalid-token response: %v", err)
	}

	updated, err := reachabilityStore.GetReachability(1, "node-b")
	if err != nil {
		t.Fatalf("load updated reachability: %v", err)
	}
	if updated == nil || updated.State != ClusterReachabilityReachable {
		t.Fatalf("expected reachable after protocol rejection transport success, got %#v", updated)
	}
}

func TestClusterPeerProbeServiceSkipsSingleNodeDomains(t *testing.T) {
	reachabilityStore := newStubClusterReachabilityStore()
	if err := reachabilityStore.SaveReachability(&model.ClusterPeerReachability{
		DomainID:     1,
		TargetNodeID: "node-b",
		State:        ClusterReachabilityUnreachable,
	}); err != nil {
		t.Fatalf("seed reachability: %v", err)
	}

	prober := &ClusterPeerProbeService{
		store: &stubClusterProbeStore{
			localNodeID: "node-a",
			members: []model.ClusterMember{
				{NodeID: "node-a", DomainID: 1, Domain: &model.ClusterDomain{Id: 1, Domain: "edge.example.com"}},
			},
		},
		reachability: &ClusterReachabilityService{
			store:  reachabilityStore,
			policy: DefaultClusterReachabilityPolicy(),
			now: func() int64 {
				return time.Unix(1_700_000_000, 0).Unix()
			},
		},
	}

	if err := prober.ProbeIdlePeers(context.Background()); err != nil {
		t.Fatalf("probe single-node domain: %v", err)
	}

	entry, err := reachabilityStore.GetReachability(1, "node-b")
	if err != nil {
		t.Fatalf("load reachability after single-node pass: %v", err)
	}
	if entry != nil {
		t.Fatalf("expected no peer reachability rows after single-node probe pass, got %#v", entry)
	}
}

func TestClusterPeerProbeServiceReconcilesRemovedPeersInMultiNodeDomain(t *testing.T) {
	reachabilityStore := newStubClusterReachabilityStore()
	if err := reachabilityStore.SaveReachability(&model.ClusterPeerReachability{
		DomainID:       1,
		TargetNodeID:   "node-old",
		State:          ClusterReachabilityUnreachable,
		LastObservedAt: time.Unix(1_700_000_000, 0).Add(-11 * time.Minute).Unix(),
	}); err != nil {
		t.Fatalf("seed removed peer reachability: %v", err)
	}
	if err := reachabilityStore.SaveReachability(&model.ClusterPeerReachability{
		DomainID:       1,
		TargetNodeID:   "node-b",
		State:          ClusterReachabilityReachable,
		LastObservedAt: time.Unix(1_700_000_000, 0).Unix(),
	}); err != nil {
		t.Fatalf("seed active peer reachability: %v", err)
	}

	prober := &ClusterPeerProbeService{
		store: &stubClusterProbeStore{
			localNodeID: "node-a",
			members: []model.ClusterMember{
				{NodeID: "node-a", DomainID: 1, Domain: &model.ClusterDomain{Id: 1, Domain: "edge.example.com"}},
				{NodeID: "node-b", DomainID: 1, Domain: &model.ClusterDomain{Id: 1, Domain: "edge.example.com"}},
			},
		},
		reachability: &ClusterReachabilityService{
			store:  reachabilityStore,
			policy: DefaultClusterReachabilityPolicy(),
			now: func() int64 {
				return time.Unix(1_700_000_000, 0).Unix()
			},
		},
	}

	if err := prober.ProbeIdlePeers(context.Background()); err != nil {
		t.Fatalf("probe multi-node domain: %v", err)
	}

	removed, err := reachabilityStore.GetReachability(1, "node-old")
	if err != nil {
		t.Fatalf("load removed peer reachability: %v", err)
	}
	if removed != nil {
		t.Fatalf("expected removed peer reachability to be cleared, got %#v", removed)
	}
}
