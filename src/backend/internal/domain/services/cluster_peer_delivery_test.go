package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	clustertypes "github.com/BeanYa/b-ui/src/backend/internal/domain/services/cluster/types"
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

func TestPeerDeliveryRejectsInvalidSuccessBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<html>ok</html>"))
	}))
	defer server.Close()

	delivery := &ClusterPeerDeliveryService{HTTPClient: server.Client()}
	err := delivery.Send(context.Background(), &PeerMessage{
		MessageID: "msg-invalid-body",
		Action:    "domain.inbound.delete",
	}, model.ClusterMember{NodeID: "node-b", BaseURL: server.URL}, "peer-token")
	if err == nil || !strings.Contains(err.Error(), "invalid cluster peer response") {
		t.Fatalf("expected invalid cluster peer response, got %v", err)
	}
}

func TestPeerDeliveryIncludesPeerErrorMessageFromNonSuccessBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"success":false,"msg":"cluster peer token not found"}`))
	}))
	defer server.Close()

	delivery := &ClusterPeerDeliveryService{HTTPClient: server.Client()}
	err := delivery.Send(context.Background(), &PeerMessage{
		MessageID: "msg-peer-error-body",
		Action:    PeerActionDomainInboundCreate,
	}, model.ClusterMember{NodeID: "node-b", BaseURL: server.URL}, "peer-token")
	if err == nil {
		t.Fatal("expected failed delivery")
	}
	if !strings.Contains(err.Error(), "cluster peer notify failed: 401 Unauthorized") {
		t.Fatalf("expected HTTP status in error, got %v", err)
	}
	if !strings.Contains(err.Error(), "cluster peer token not found") {
		t.Fatalf("expected peer response message in error, got %v", err)
	}
}

func TestClusterPeerDeliverySendWithResultParsesCommandResult(t *testing.T) {
	expected := clustertypes.DomainResourceCommandResult{
		Status:       "applied",
		OperationID:  "op-1",
		NodeID:       "node-b",
		ResourceKind: "domain_inbound",
		ResourceID:   "group-1",
		Revision:     7,
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"msg":"cluster message received","result":{"status":"applied","operation_id":"op-1","node_id":"node-b","resource_kind":"domain_inbound","resource_id":"group-1","revision":7}}`))
	}))
	defer server.Close()

	delivery := &ClusterPeerDeliveryService{HTTPClient: server.Client()}
	result, err := delivery.SendWithResult(context.Background(), &PeerMessage{
		MessageID: "msg-with-result",
		Action:    PeerActionDomainInboundCreate,
	}, model.ClusterMember{NodeID: "node-b", BaseURL: server.URL}, "peer-token")
	if err != nil {
		t.Fatalf("send with result: %v", err)
	}
	if result == nil || *result != expected {
		t.Fatalf("expected %#v, got %#v", expected, result)
	}
}

func TestClusterPeerDeliverySendWithResultSurfacesProtocolRejectedMessage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"rejected","code":"request_rejected","message":"cluster message: payload domain_id \"edge.example.com\" does not match local domain \"other.example.com\""}`))
	}))
	defer server.Close()

	delivery := &ClusterPeerDeliveryService{HTTPClient: server.Client()}
	_, err := delivery.SendWithResult(context.Background(), &PeerMessage{
		MessageID: "msg-rejected-protocol-response",
		Action:    PeerActionDomainInboundCreate,
	}, model.ClusterMember{NodeID: "node-b", BaseURL: server.URL}, "peer-token")
	if err == nil {
		t.Fatal("expected failed delivery")
	}
	if !strings.Contains(err.Error(), "payload domain_id") {
		t.Fatalf("expected peer rejection message, got %v", err)
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
