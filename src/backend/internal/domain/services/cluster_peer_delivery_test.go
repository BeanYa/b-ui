package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	database "github.com/BeanYa/b-ui/src/backend/internal/infra/db"
	"github.com/BeanYa/b-ui/src/backend/internal/infra/db/model"
)

func TestExpandPeerRouteBroadcastSkipsSourceAndExcludedNodes(t *testing.T) {
	members := []model.ClusterMember{
		{NodeID: "node-a", BaseURL: "https://node-a.example.com"},
		{NodeID: "node-b", BaseURL: "https://node-b.example.com"},
		{NodeID: "node-c", BaseURL: "https://node-c.example.com"},
	}
	targets := ExpandClusterPeerRoute(RoutePlan{
		Mode:     RouteModeBroadcast,
		Selector: &TargetSelector{Exclude: []string{"node-c"}},
	}, members, "node-a")
	if len(targets) != 1 || targets[0].NodeID != "node-b" {
		t.Fatalf("expected only node-b, got %#v", targets)
	}
}

func TestExpandPeerRouteMulticastUsesFixedTargets(t *testing.T) {
	members := []model.ClusterMember{
		{NodeID: "node-a", BaseURL: "https://node-a.example.com"},
		{NodeID: "node-b", BaseURL: "https://node-b.example.com"},
		{NodeID: "node-c", BaseURL: "https://node-c.example.com"},
	}
	targets := ExpandClusterPeerRoute(RoutePlan{Mode: RouteModeMulticast, Targets: []string{"node-c", "node-b"}}, members, "node-a")
	if len(targets) != 2 || targets[0].NodeID != "node-c" || targets[1].NodeID != "node-b" {
		t.Fatalf("expected fixed multicast order c,b, got %#v", targets)
	}
}

func TestExpandPeerRouteDirectRejectsMultipleTargets(t *testing.T) {
	members := []model.ClusterMember{
		{NodeID: "node-a", BaseURL: "https://node-a.example.com"},
		{NodeID: "node-b", BaseURL: "https://node-b.example.com"},
		{NodeID: "node-c", BaseURL: "https://node-c.example.com"},
	}
	targets := ExpandClusterPeerRoute(RoutePlan{Mode: RouteModeDirect, Targets: []string{"node-b", "node-c"}}, members, "node-a")
	if len(targets) != 0 {
		t.Fatalf("expected malformed direct route to fail closed, got %#v", targets)
	}
}

func TestExpandPeerRouteDirectRejectsZeroTargets(t *testing.T) {
	members := []model.ClusterMember{
		{NodeID: "node-a", BaseURL: "https://node-a.example.com"},
		{NodeID: "node-b", BaseURL: "https://node-b.example.com"},
	}
	targets := ExpandClusterPeerRoute(RoutePlan{Mode: RouteModeDirect}, members, "node-a")
	if len(targets) != 0 {
		t.Fatalf("expected empty direct route to fail closed, got %#v", targets)
	}
}

func TestExpandPeerRouteDirectUsesSingleEligibleTarget(t *testing.T) {
	members := []model.ClusterMember{
		{NodeID: "node-a", BaseURL: "https://node-a.example.com"},
		{NodeID: "node-b", BaseURL: "https://node-b.example.com"},
		{NodeID: "node-c", BaseURL: "https://node-c.example.com"},
	}
	targets := ExpandClusterPeerRoute(RoutePlan{Mode: RouteModeDirect, Targets: []string{"node-c"}}, members, "node-a")
	if len(targets) != 1 || targets[0].NodeID != "node-c" {
		t.Fatalf("expected direct route to node-c, got %#v", targets)
	}
}

func TestExpandPeerRouteBroadcastWithCapabilityRequiredFailsClosed(t *testing.T) {
	members := []model.ClusterMember{
		{NodeID: "node-a", BaseURL: "https://node-a.example.com"},
		{NodeID: "node-b", BaseURL: "https://node-b.example.com"},
	}
	targets := ExpandClusterPeerRoute(RoutePlan{
		Mode:     RouteModeBroadcast,
		Selector: &TargetSelector{CapabilityRequired: []string{"sync-v2"}},
	}, members, "node-a")
	if len(targets) != 0 {
		t.Fatalf("expected capability selector to fail closed, got %#v", targets)
	}
}

func TestPeerDeliveryRecordsAckAttempts(t *testing.T) {
	if err := database.InitDB(filepath.Join(t.TempDir(), "peer-delivery-ack.db")); err != nil {
		if strings.Contains(err.Error(), "go-sqlite3 requires cgo") || strings.Contains(err.Error(), "C compiler") {
			t.Skipf("sqlite test database unavailable in this toolchain: %v", err)
		}
		t.Fatalf("init test db: %v", err)
	}

	failingServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "temporary failure", http.StatusBadGateway)
	}))
	defer failingServer.Close()

	message := &PeerMessage{
		MessageID: "msg-ack",
		Route: RoutePlan{Delivery: &DeliveryPolicy{
			Ack: DeliveryAckNode,
		}},
	}
	member := model.ClusterMember{NodeID: "node-b", BaseURL: failingServer.URL}
	delivery := &ClusterPeerDeliveryService{HTTPClient: failingServer.Client()}

	if err := delivery.Send(context.Background(), message, member, "peer-token"); err == nil {
		t.Fatal("expected failed delivery")
	}
	ack := loadPeerAckState(t, message.MessageID, member.NodeID)
	if ack.Status != PeerAckStatusFailed || ack.Attempts != 1 || ack.Error == "" {
		t.Fatalf("expected failed first ack attempt, got %#v", ack)
	}

	successServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"success":true}`))
	}))
	defer successServer.Close()

	member.BaseURL = successServer.URL
	delivery.HTTPClient = successServer.Client()
	if err := delivery.Send(context.Background(), message, member, "peer-token"); err != nil {
		t.Fatalf("send success: %v", err)
	}
	ack = loadPeerAckState(t, message.MessageID, member.NodeID)
	if ack.Status != PeerAckStatusSucceeded || ack.Attempts != 2 || ack.Error != "" {
		t.Fatalf("expected succeeded second ack attempt, got %#v", ack)
	}
}

func loadPeerAckState(t *testing.T, messageID string, targetNode string) model.ClusterPeerAckState {
	t.Helper()
	var ack model.ClusterPeerAckState
	if err := database.GetDB().Where("message_id = ? AND target_node = ?", messageID, targetNode).First(&ack).Error; err != nil {
		t.Fatalf("load ack state: %v", err)
	}
	return ack
}
