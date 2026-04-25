package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alireza0/s-ui/src/backend/internal/infra/db/model"
)

func TestClusterHubClientRejectsNon2xxResponses(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "bad gateway", http.StatusBadGateway)
	}))
	defer server.Close()

	client := &ClusterHubClient{HTTPClient: server.Client()}
	if _, err := client.GetLatestVersion(context.Background(), server.URL, "edge.example.com", "domain-token"); err == nil {
		t.Fatal("expected non-2xx hub response to fail")
	}
	if _, err := client.GetSnapshot(context.Background(), server.URL, "edge.example.com", "domain-token"); err == nil {
		t.Fatal("expected non-2xx hub snapshot response to fail")
	}
	if _, err := client.RegisterNode(context.Background(), server.URL, ClusterHubRegisterNodeRequest{}); err == nil {
		t.Fatal("expected non-2xx hub register response to fail")
	}
}

func TestClusterHubClientRejectsNonHttpsRemoteHubURL(t *testing.T) {
	client := &ClusterHubClient{}
	if _, err := client.GetLatestVersion(context.Background(), "http://example.com", "edge.example.com", "domain-token"); err == nil {
		t.Fatal("expected non-https remote hub URL to fail")
	}
}

func TestClusterHTTPBroadcasterUsesBasePathAndRejectsNon2xxResponses(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/panel/_cluster/v1/events" {
			t.Fatalf("expected path /panel/_cluster/v1/events, got %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer server.Close()

	broadcaster := &ClusterHTTPBroadcaster{
		secretProvider: stubClusterSecretProvider{secret: []byte("panel-secret-for-cluster-tests")},
		identity:       ClusterLocalIdentityService{store: &stubClusterLocalNodeStore{}},
		reachability:   newTestClusterReachabilityService(20),
		store: &stubClusterBroadcastStore{members: []model.ClusterMember{{
			NodeID:  "node-a",
			BaseURL: server.URL + "/panel/",
			Domain:  &model.ClusterDomain{Domain: "edge.example.com", TokenEncrypted: mustEncryptClusterToken(t, "panel-secret-for-cluster-tests", "cluster-token")},
		}}},
		HTTPClient: server.Client(),
	}

	if err := broadcaster.BroadcastNotifyVersion(context.Background(), 9, ""); err == nil {
		t.Fatal("expected non-2xx peer response to fail")
	}
}

func TestClusterHTTPBroadcasterRejectsNonHttpsPeerTargets(t *testing.T) {
	broadcaster := &ClusterHTTPBroadcaster{
		secretProvider: stubClusterSecretProvider{secret: []byte("panel-secret-for-cluster-tests")},
		identity:       ClusterLocalIdentityService{store: &stubClusterLocalNodeStore{}},
		reachability:   newTestClusterReachabilityService(20),
		store: &stubClusterBroadcastStore{members: []model.ClusterMember{{
			NodeID:             "node-a",
			BaseURL:            "http://example.com/panel/",
			PeerTokenEncrypted: mustEncryptClusterToken(t, "panel-secret-for-cluster-tests", "peer-token-a"),
			Domain:             &model.ClusterDomain{Domain: "edge.example.com", TokenEncrypted: mustEncryptClusterToken(t, "panel-secret-for-cluster-tests", "domain-token")},
		}}},
	}

	if err := broadcaster.BroadcastNotifyVersion(context.Background(), 9, ""); err == nil {
		t.Fatal("expected non-https peer target to be rejected")
	}
}

func TestClusterHTTPBroadcasterRejectsFailureJSONBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":false,"msg":"verification failed"}`))
	}))
	defer server.Close()

	broadcaster := &ClusterHTTPBroadcaster{
		secretProvider: stubClusterSecretProvider{secret: []byte("panel-secret-for-cluster-tests")},
		identity:       ClusterLocalIdentityService{store: &stubClusterLocalNodeStore{}},
		reachability:   newTestClusterReachabilityService(20),
		store: &stubClusterBroadcastStore{members: []model.ClusterMember{{
			NodeID:  "node-a",
			BaseURL: server.URL + "/panel/",
			Domain:  &model.ClusterDomain{Domain: "edge.example.com", TokenEncrypted: mustEncryptClusterToken(t, "panel-secret-for-cluster-tests", "cluster-token")},
		}}},
		HTTPClient: server.Client(),
	}

	if err := broadcaster.BroadcastNotifyVersion(context.Background(), 9, ""); err == nil {
		t.Fatal("expected failure JSON body to be treated as error")
	}
}

func TestClusterHTTPBroadcasterUsesTargetMemberPeerTokenInsteadOfDomainToken(t *testing.T) {
	var receivedToken string
	var receivedEnvelope ClusterEnvelope
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedToken = r.Header.Get("X-Cluster-Token")
		if err := json.NewDecoder(r.Body).Decode(&receivedEnvelope); err != nil {
			t.Fatalf("decode envelope: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true}`))
	}))
	defer server.Close()

	broadcaster := &ClusterHTTPBroadcaster{
		secretProvider: stubClusterSecretProvider{secret: []byte("panel-secret-for-cluster-tests")},
		identity:       ClusterLocalIdentityService{store: &stubClusterLocalNodeStore{}},
		reachability:   newTestClusterReachabilityService(20),
		store: &stubClusterBroadcastStore{members: []model.ClusterMember{{
			NodeID:             "node-a",
			BaseURL:            server.URL + "/panel/",
			PeerTokenEncrypted: mustEncryptClusterToken(t, "panel-secret-for-cluster-tests", "peer-token-a"),
			Domain:             &model.ClusterDomain{Domain: "edge.example.com", TokenEncrypted: mustEncryptClusterToken(t, "panel-secret-for-cluster-tests", "domain-token")},
		}}},
		HTTPClient: server.Client(),
	}

	if err := broadcaster.BroadcastNotifyVersion(context.Background(), 9, ""); err != nil {
		t.Fatalf("broadcast notify version: %v", err)
	}
	if receivedToken != "peer-token-a" {
		t.Fatalf("expected peer token header, got %q", receivedToken)
	}
	if receivedEnvelope.Domain != "edge.example.com" {
		t.Fatalf("expected domain context in envelope, got %q", receivedEnvelope.Domain)
	}
}

func TestClusterHTTPBroadcasterSkipsRoutineFanoutToKnownUnreachablePeersAndContinuesOthers(t *testing.T) {
	var hits int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true}`))
	}))
	defer server.Close()

	reachabilityStore := newStubClusterReachabilityStore()
	if err := reachabilityStore.SaveReachability(&model.ClusterPeerReachability{
		DomainID:       1,
		TargetNodeID:   "node-skip",
		State:          ClusterReachabilityUnreachable,
		LastObservedAt: 20,
		NextProbeAt:    40,
	}); err != nil {
		t.Fatalf("seed unreachable reachability: %v", err)
	}

	broadcaster := &ClusterHTTPBroadcaster{
		secretProvider: stubClusterSecretProvider{secret: []byte("panel-secret-for-cluster-tests")},
		identity:       ClusterLocalIdentityService{store: &stubClusterLocalNodeStore{}},
		reachability: &ClusterReachabilityService{
			store:  reachabilityStore,
			policy: DefaultClusterReachabilityPolicy(),
			now: func() int64 {
				return 20
			},
		},
		store: &stubClusterBroadcastStore{members: []model.ClusterMember{
			{
				NodeID:             "node-skip",
				BaseURL:            "http://example.com/panel/",
				DomainID:           1,
				PeerTokenEncrypted: mustEncryptClusterToken(t, "panel-secret-for-cluster-tests", "peer-token-skip"),
				Domain:             &model.ClusterDomain{Id: 1, Domain: "edge.example.com", CommunicationEndpointPath: "/_cluster", CommunicationProtocolVersion: "v1"},
			},
			{
				NodeID:             "node-hit",
				BaseURL:            server.URL + "/panel/",
				DomainID:           1,
				PeerTokenEncrypted: mustEncryptClusterToken(t, "panel-secret-for-cluster-tests", "peer-token-hit"),
				Domain:             &model.ClusterDomain{Id: 1, Domain: "edge.example.com", CommunicationEndpointPath: "/_cluster", CommunicationProtocolVersion: "v1"},
			},
		}},
		HTTPClient: server.Client(),
	}

	if err := broadcaster.BroadcastNotifyVersion(context.Background(), 9, ""); err != nil {
		t.Fatalf("broadcast notify version: %v", err)
	}
	if hits != 1 {
		t.Fatalf("expected exactly one outbound hit, got %d", hits)
	}
}

func TestClusterHTTPBroadcasterRetriesStaleUnreachablePeerWhenProbeWindowReopens(t *testing.T) {
	var hits int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true}`))
	}))
	defer server.Close()

	reachabilityStore := newStubClusterReachabilityStore()
	if err := reachabilityStore.SaveReachability(&model.ClusterPeerReachability{
		DomainID:       1,
		TargetNodeID:   "node-retry",
		State:          ClusterReachabilityUnreachable,
		LastObservedAt: 1,
		NextProbeAt:    0,
	}); err != nil {
		t.Fatalf("seed unreachable reachability: %v", err)
	}

	broadcaster := &ClusterHTTPBroadcaster{
		secretProvider: stubClusterSecretProvider{secret: []byte("panel-secret-for-cluster-tests")},
		identity:       ClusterLocalIdentityService{store: &stubClusterLocalNodeStore{}},
		reachability: &ClusterReachabilityService{
			store:  reachabilityStore,
			policy: DefaultClusterReachabilityPolicy(),
			now: func() int64 {
				return 100
			},
		},
		store: &stubClusterBroadcastStore{members: []model.ClusterMember{{
			NodeID:             "node-retry",
			BaseURL:            server.URL + "/panel/",
			DomainID:           1,
			PeerTokenEncrypted: mustEncryptClusterToken(t, "panel-secret-for-cluster-tests", "peer-token-retry"),
			Domain:             &model.ClusterDomain{Id: 1, Domain: "edge.example.com", CommunicationEndpointPath: "/_cluster", CommunicationProtocolVersion: "v1"},
		}}},
		HTTPClient: server.Client(),
	}

	if err := broadcaster.BroadcastNotifyVersion(context.Background(), 9, ""); err != nil {
		t.Fatalf("broadcast notify version after stale unreachable state: %v", err)
	}
	if hits != 1 {
		t.Fatalf("expected stale unreachable peer to be retried once, got %d hits", hits)
	}
}

func newTestClusterReachabilityService(now int64) *ClusterReachabilityService {
	return &ClusterReachabilityService{
		store:  newStubClusterReachabilityStore(),
		policy: DefaultClusterReachabilityPolicy(),
		now: func() int64 {
			return now
		},
	}
}

type stubClusterBroadcastStore struct{ members []model.ClusterMember }

func (s *stubClusterBroadcastStore) ListMembersWithDomain() ([]model.ClusterMember, error) {
	return s.members, nil
}

func mustEncryptClusterToken(t *testing.T, secret string, token string) string {
	t.Helper()
	encrypted, err := EncryptClusterDomainToken([]byte(secret), token)
	if err != nil {
		t.Fatalf("encrypt token: %v", err)
	}
	return encrypted
}
