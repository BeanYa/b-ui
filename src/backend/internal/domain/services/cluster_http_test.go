package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
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
	var receivedMessage PeerMessage
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedToken = r.Header.Get("X-Cluster-Token")
		if err := json.NewDecoder(r.Body).Decode(&receivedMessage); err != nil {
			t.Fatalf("decode peer message: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true}`))
	}))
	defer server.Close()

	broadcaster := &ClusterHTTPBroadcaster{
		secretProvider: stubClusterSecretProvider{secret: []byte("panel-secret-for-cluster-tests")},
		identity:       ClusterLocalIdentityService{store: &stubClusterLocalNodeStore{}},
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
	if receivedMessage.DomainID != "edge.example.com" {
		t.Fatalf("expected domain context in peer message, got %q", receivedMessage.DomainID)
	}
	if receivedMessage.Category != PeerCategoryEvent || receivedMessage.Action != PeerActionDomainClusterChanged {
		t.Fatalf("expected domain cluster changed event, got %s/%s", receivedMessage.Category, receivedMessage.Action)
	}
	if receivedMessage.Route.Mode != RouteModeBroadcast {
		t.Fatalf("expected broadcast route, got %q", receivedMessage.Route.Mode)
	}
	if receivedMessage.Route.Delivery == nil || receivedMessage.Route.Delivery.Ack != DeliveryAckNode {
		t.Fatalf("expected node ack delivery policy, got %#v", receivedMessage.Route.Delivery)
	}
}

func TestClusterHTTPBroadcasterFallsBackToLegacyEnvelopeWhenPeerMessageRejected(t *testing.T) {
	local := newTestClusterLocalNode(t, "node-local")
	var receivedTypes []string
	var legacyEnvelope ClusterEnvelope
	var receivedTokens []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedTokens = append(receivedTokens, r.Header.Get("X-Cluster-Token"))
		var raw map[string]any
		if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if _, ok := raw["messageId"]; ok {
			receivedTypes = append(receivedTypes, "peer")
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"success":false,"msg":"unsupported peer message"}`))
			return
		}
		receivedTypes = append(receivedTypes, "legacy")
		body, err := json.Marshal(raw)
		if err != nil {
			t.Fatalf("marshal legacy raw body: %v", err)
		}
		if err := json.Unmarshal(body, &legacyEnvelope); err != nil {
			t.Fatalf("decode legacy envelope: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true}`))
	}))
	defer server.Close()

	broadcaster := &ClusterHTTPBroadcaster{
		secretProvider: stubClusterSecretProvider{secret: []byte("panel-secret-for-cluster-tests")},
		identity:       ClusterLocalIdentityService{store: &stubClusterLocalNodeStore{node: local}},
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
	if len(receivedTypes) != 2 || receivedTypes[0] != "peer" || receivedTypes[1] != "legacy" {
		t.Fatalf("expected peer then legacy fallback, got %#v", receivedTypes)
	}
	if len(receivedTokens) != 2 || receivedTokens[0] != "peer-token-a" || receivedTokens[1] != "peer-token-a" {
		t.Fatalf("expected same peer token on fallback, got %#v", receivedTokens)
	}
	if legacyEnvelope.MessageType != "sync.notify_version" || legacyEnvelope.Version != 9 || legacyEnvelope.Domain != "edge.example.com" {
		t.Fatalf("unexpected legacy envelope: %#v", legacyEnvelope)
	}
	if _, err := VerifyClusterEnvelope(&legacyEnvelope, local.PublicKey); err != nil {
		t.Fatalf("expected signed legacy envelope: %v", err)
	}
}

func TestClusterHTTPBroadcasterDoesNotFallbackWhenPeerMessageSucceeds(t *testing.T) {
	var requests int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requests, 1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true}`))
	}))
	defer server.Close()

	broadcaster := &ClusterHTTPBroadcaster{
		secretProvider: stubClusterSecretProvider{secret: []byte("panel-secret-for-cluster-tests")},
		identity:       ClusterLocalIdentityService{store: &stubClusterLocalNodeStore{}},
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
	if got := atomic.LoadInt32(&requests); got != 1 {
		t.Fatalf("expected no legacy fallback after peer success, got %d requests", got)
	}
}

func TestClusterHTTPBroadcasterSkipsLocalIdentityNode(t *testing.T) {
	var localRequests int32
	localServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&localRequests, 1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true}`))
	}))
	defer localServer.Close()

	var remoteRequests int32
	remoteServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&remoteRequests, 1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true}`))
	}))
	defer remoteServer.Close()

	localStore := &stubClusterLocalNodeStore{}
	identity := ClusterLocalIdentityService{store: localStore}
	localNode, err := identity.GetOrCreate()
	if err != nil {
		t.Fatalf("create local identity: %v", err)
	}
	domain := &model.ClusterDomain{Domain: "edge.example.com", TokenEncrypted: mustEncryptClusterToken(t, "panel-secret-for-cluster-tests", "domain-token")}
	broadcaster := &ClusterHTTPBroadcaster{
		secretProvider: stubClusterSecretProvider{secret: []byte("panel-secret-for-cluster-tests")},
		identity:       identity,
		store: &stubClusterBroadcastStore{members: []model.ClusterMember{{
			NodeID:             localNode.NodeID,
			BaseURL:            localServer.URL + "/panel/",
			PeerTokenEncrypted: mustEncryptClusterToken(t, "panel-secret-for-cluster-tests", "peer-token-local"),
			Domain:             domain,
		}, {
			NodeID:             "node-remote",
			BaseURL:            remoteServer.URL + "/panel/",
			PeerTokenEncrypted: mustEncryptClusterToken(t, "panel-secret-for-cluster-tests", "peer-token-remote"),
			Domain:             domain,
		}}},
		HTTPClient: remoteServer.Client(),
	}

	if err := broadcaster.BroadcastNotifyVersion(context.Background(), 9, "node-unrelated"); err != nil {
		t.Fatalf("broadcast notify version: %v", err)
	}
	if got := atomic.LoadInt32(&localRequests); got != 0 {
		t.Fatalf("expected local node to receive no requests, got %d", got)
	}
	if got := atomic.LoadInt32(&remoteRequests); got != 1 {
		t.Fatalf("expected one remote request, got %d", got)
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
