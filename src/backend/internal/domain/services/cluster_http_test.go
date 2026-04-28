package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/BeanYa/b-ui/src/backend/internal/infra/db/model"
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

func TestClusterHubClientSendsLocalNodeIDOnReadRequests(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("X-Cluster-Node-Id"); got != "node-local" {
			t.Fatalf("expected X-Cluster-Node-Id node-local, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"version":7}`))
	}))
	defer server.Close()

	client := &ClusterHubClient{
		HTTPClient:    server.Client(),
		localIdentity: &ClusterLocalIdentityService{store: &stubClusterLocalNodeStore{node: &model.ClusterLocalNode{NodeID: "node-local"}}},
	}

	response, err := client.GetLatestVersion(context.Background(), server.URL, "edge.example.com", "domain-token")
	if err != nil {
		t.Fatalf("get latest version: %v", err)
	}
	if response.Version != 7 {
		t.Fatalf("expected version 7, got %d", response.Version)
	}
}

func TestClusterHubClientEscapesDomainAndMemberIDsInRequestPaths(t *testing.T) {
	requests := make([]string, 0, 3)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.URL.RequestURI())
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodGet:
			if strings.HasSuffix(r.URL.Path, "/snapshot") {
				_, _ = w.Write([]byte(`{"domain_id":"edge/id.example.com","version":7,"communication":{"endpoint_path":"/_cluster","protocol_version":"v1"},"members":[]}`))
				return
			}
			_, _ = w.Write([]byte(`{"version":7}`))
		case http.MethodDelete:
			_, _ = w.Write([]byte(`{"operation_id":"op-delete","request_id":"req-delete","status":"completed","domain_id":"edge/id.example.com","type":"delete"}`))
		default:
			t.Fatalf("unexpected method %s", r.Method)
		}
	}))
	defer server.Close()

	client := &ClusterHubClient{HTTPClient: server.Client()}
	if _, err := client.GetLatestVersion(context.Background(), server.URL, "edge/id.example.com", "domain-token"); err != nil {
		t.Fatalf("get latest version: %v", err)
	}
	if _, err := client.GetSnapshot(context.Background(), server.URL, "edge/id.example.com", "domain-token"); err != nil {
		t.Fatalf("get snapshot: %v", err)
	}
	if _, err := client.DeleteMember(context.Background(), server.URL, "edge/id.example.com", "domain-token", "member/one"); err != nil {
		t.Fatalf("delete member: %v", err)
	}

	expected := []string{
		"/v1/domains/edge%2Fid.example.com/version",
		"/v1/domains/edge%2Fid.example.com/snapshot",
		"/v1/domains/edge%2Fid.example.com/members/member%2Fone",
	}
	if !reflect.DeepEqual(requests, expected) {
		t.Fatalf("expected escaped request paths %#v, got %#v", expected, requests)
	}
}

func TestClusterHubClientTreatsProtocolRejectedReadAsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"rejected","code":"member_not_found","message":"node-local is not active"}`))
	}))
	defer server.Close()

	client := &ClusterHubClient{HTTPClient: server.Client()}
	_, err := client.GetSnapshot(context.Background(), server.URL, "edge.example.com", "domain-token")
	if err == nil {
		t.Fatal("expected rejected read response to fail")
	}
	var rejectedErr *clusterHubReadRejectedError
	if !errors.As(err, &rejectedErr) {
		t.Fatalf("expected clusterHubReadRejectedError, got %v", err)
	}
	if rejectedErr.Code != "member_not_found" {
		t.Fatalf("expected reject code member_not_found, got %q", rejectedErr.Code)
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
			NodeID:             "node-a",
			BaseURL:            server.URL + "/panel/",
			PeerTokenEncrypted: mustEncryptClusterToken(t, "panel-secret-for-cluster-tests", "peer-token-a"),
			Domain:             &model.ClusterDomain{Domain: "edge.example.com", TokenEncrypted: mustEncryptClusterToken(t, "panel-secret-for-cluster-tests", "cluster-token")},
		}}},
		HTTPClient:     server.Client(),
		saveAckAttempt: noopPeerAckAttempt,
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
		saveAckAttempt: noopPeerAckAttempt,
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
			NodeID:             "node-a",
			BaseURL:            server.URL + "/panel/",
			PeerTokenEncrypted: mustEncryptClusterToken(t, "panel-secret-for-cluster-tests", "peer-token-a"),
			Domain:             &model.ClusterDomain{Domain: "edge.example.com", TokenEncrypted: mustEncryptClusterToken(t, "panel-secret-for-cluster-tests", "cluster-token")},
		}}},
		HTTPClient:     server.Client(),
		saveAckAttempt: noopPeerAckAttempt,
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
	var peerMessage PeerMessage
	var receivedTokens []string
	var ackAttempts []model.ClusterPeerAckState
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedTokens = append(receivedTokens, r.Header.Get("X-Cluster-Token"))
		var raw map[string]any
		if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if _, ok := raw["messageId"]; ok {
			receivedTypes = append(receivedTypes, "peer")
			body, err := json.Marshal(raw)
			if err != nil {
				t.Fatalf("marshal peer raw body: %v", err)
			}
			if err := json.Unmarshal(body, &peerMessage); err != nil {
				t.Fatalf("decode peer message: %v", err)
			}
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
		reachability:   newTestClusterReachabilityService(20),
		store: &stubClusterBroadcastStore{members: []model.ClusterMember{{
			NodeID:             "node-a",
			BaseURL:            server.URL + "/panel/",
			PeerTokenEncrypted: mustEncryptClusterToken(t, "panel-secret-for-cluster-tests", "peer-token-a"),
			Domain:             &model.ClusterDomain{Domain: "edge.example.com", TokenEncrypted: mustEncryptClusterToken(t, "panel-secret-for-cluster-tests", "domain-token")},
		}}},
		HTTPClient: server.Client(),
		saveAckAttempt: func(messageID string, targetNode string, status string, errorMessage string) error {
			ackAttempts = append(ackAttempts, model.ClusterPeerAckState{MessageID: messageID, TargetNode: targetNode, Status: status, Error: errorMessage})
			return nil
		},
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
	if len(ackAttempts) != 2 {
		t.Fatalf("expected failed peer ack then successful fallback ack, got %#v", ackAttempts)
	}
	if ackAttempts[0].MessageID != peerMessage.MessageID || ackAttempts[0].TargetNode != "node-a" || ackAttempts[0].Status != PeerAckStatusFailed || ackAttempts[0].Error == "" {
		t.Fatalf("unexpected failed peer ack attempt: %#v", ackAttempts[0])
	}
	if ackAttempts[1].MessageID != peerMessage.MessageID || ackAttempts[1].TargetNode != "node-a" || ackAttempts[1].Status != PeerAckStatusSucceeded || ackAttempts[1].Error != "" {
		t.Fatalf("unexpected successful fallback ack attempt: %#v", ackAttempts[1])
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
		reachability:   newTestClusterReachabilityService(20),
		store: &stubClusterBroadcastStore{members: []model.ClusterMember{{
			NodeID:             "node-a",
			BaseURL:            server.URL + "/panel/",
			PeerTokenEncrypted: mustEncryptClusterToken(t, "panel-secret-for-cluster-tests", "peer-token-a"),
			Domain:             &model.ClusterDomain{Domain: "edge.example.com", TokenEncrypted: mustEncryptClusterToken(t, "panel-secret-for-cluster-tests", "domain-token")},
		}}},
		HTTPClient:     server.Client(),
		saveAckAttempt: noopPeerAckAttempt,
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
		reachability:   newTestClusterReachabilityService(20),
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
		HTTPClient:     remoteServer.Client(),
		saveAckAttempt: noopPeerAckAttempt,
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

func noopPeerAckAttempt(string, string, string, string) error {
	return nil
}

func mustEncryptClusterToken(t *testing.T, secret string, token string) string {
	t.Helper()
	encrypted, err := EncryptClusterDomainToken([]byte(secret), token)
	if err != nil {
		t.Fatalf("encrypt token: %v", err)
	}
	return encrypted
}
