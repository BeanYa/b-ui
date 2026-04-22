package service

import (
	"context"
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
	if _, err := client.GetLatestVersion(context.Background(), server.URL, "edge.example.com"); err == nil {
		t.Fatal("expected non-2xx hub response to fail")
	}
	if _, err := client.GetSnapshot(context.Background(), server.URL, "edge.example.com"); err == nil {
		t.Fatal("expected non-2xx hub snapshot response to fail")
	}
	if _, err := client.RegisterNode(context.Background(), server.URL, ClusterHubRegisterNodeRequest{}); err == nil {
		t.Fatal("expected non-2xx hub register response to fail")
	}
}

func TestClusterHTTPBroadcasterUsesBasePathAndRejectsNon2xxResponses(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/panel/cluster/message" {
			t.Fatalf("expected path /panel/cluster/message, got %s", r.URL.Path)
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
