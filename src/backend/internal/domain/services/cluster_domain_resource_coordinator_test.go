package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	clustertypes "github.com/BeanYa/b-ui/src/backend/internal/domain/services/cluster/types"
	database "github.com/BeanYa/b-ui/src/backend/internal/infra/db"
	"github.com/BeanYa/b-ui/src/backend/internal/infra/db/model"
)

type clusterDomainPeerSenderFunc func(context.Context, *PeerMessage, model.ClusterMember, string) (*clustertypes.DomainResourceCommandResult, error)

func (f clusterDomainPeerSenderFunc) SendWithResult(ctx context.Context, message *PeerMessage, member model.ClusterMember, token string) (*clustertypes.DomainResourceCommandResult, error) {
	return f(ctx, message, member, token)
}

type countingDomainResourceProxyReporter struct {
	count    int
	domainID uint
}

func (r *countingDomainResourceProxyReporter) ReportProxyConfigs(domainID uint) {
	r.count++
	r.domainID = domainID
}

func TestDomainResourceCoordinatorRecordsPeerFailure(t *testing.T) {
	if err := database.InitDB(filepath.Join(t.TempDir(), "coordinator.db")); err != nil {
		if strings.Contains(err.Error(), "go-sqlite3 requires cgo") || strings.Contains(err.Error(), "CGO_ENABLED=0") || strings.Contains(err.Error(), "C compiler") {
			t.Skipf("sqlite test database unavailable in this toolchain: %v", err)
		}
		t.Fatalf("init db: %v", err)
	}
	db := database.GetDB()
	domain := &model.ClusterDomain{Id: 1, Domain: "edge.example.com", HubURL: "https://hub.example.com", TokenEncrypted: "token", LastVersion: 7}
	if err := db.Create(domain).Error; err != nil {
		t.Fatalf("seed domain: %v", err)
	}
	local := newTestClusterLocalNode(t, "node-a")
	if err := db.Create(local).Error; err != nil {
		t.Fatalf("seed local node: %v", err)
	}
	peerToken, err := EncryptClusterDomainToken([]byte("test-secret"), "peer-token")
	if err != nil {
		t.Fatalf("encrypt peer token: %v", err)
	}
	if err := db.Create(&model.ClusterMember{DomainID: domain.Id, NodeID: "node-b", DisplayName: "Node B", BaseURL: "https://node-b.example.com", PeerTokenEncrypted: peerToken, LastVersion: 7}).Error; err != nil {
		t.Fatalf("seed peer member: %v", err)
	}

	coordinator := &ClusterDomainResourceCoordinator{
		DB:             db,
		OperationStore: &ClusterDomainOperationStore{DB: db},
		PeerSender: clusterDomainPeerSenderFunc(func(context.Context, *PeerMessage, model.ClusterMember, string) (*clustertypes.DomainResourceCommandResult, error) {
			return nil, errors.New("node-b down")
		}),
		Identity:       &stubDomainInboundIdentity{node: local},
		SecretProvider: stubClusterSecretProvider{secret: []byte("test-secret")},
		PortAllocator:  func() (int, error) { return 32051, nil },
	}

	op, err := coordinator.CreateDomainInbound(context.Background(), domain.Id, ClusterDomainInboundCommandInput{
		GroupID: "group-1",
		Inbound: map[string]any{
			"type": "vless",
			"tag":  "main",
		},
	})
	if err != nil {
		t.Fatalf("create inbound: %v", err)
	}
	if op.Status != ClusterDomainOperationPartial {
		t.Fatalf("operation status = %q, want %q", op.Status, ClusterDomainOperationPartial)
	}
	if op.Summary.Applied != 1 || op.Summary.Failed != 1 || op.Summary.Total != 2 {
		t.Fatalf("summary = %+v, want applied=1 failed=1 total=2", op.Summary)
	}

	instances, err := coordinator.OperationStore.ListInstances(op.OperationID)
	if err != nil {
		t.Fatalf("list instances: %v", err)
	}
	if len(instances) != 2 {
		t.Fatalf("instances = %d, want 2", len(instances))
	}
	var failed *model.ClusterDomainOperationInstance
	for i := range instances {
		if instances[i].NodeID == "node-b" {
			failed = &instances[i]
			break
		}
	}
	if failed == nil {
		t.Fatalf("expected node-b operation instance")
	}
	if failed.Status != ClusterDomainOperationFailed {
		t.Fatalf("peer status = %q, want %q", failed.Status, ClusterDomainOperationFailed)
	}
	if failed.Error != "node-b down" {
		t.Fatalf("peer error = %q, want node-b down", failed.Error)
	}
}

func TestDomainResourceCoordinatorCreateInboundDispatchesInitialPeers(t *testing.T) {
	if err := database.InitDB(filepath.Join(t.TempDir(), "coordinator-initial-dispatch.db")); err != nil {
		if strings.Contains(err.Error(), "go-sqlite3 requires cgo") || strings.Contains(err.Error(), "CGO_ENABLED=0") || strings.Contains(err.Error(), "C compiler") {
			t.Skipf("sqlite test database unavailable in this toolchain: %v", err)
		}
		t.Fatalf("init db: %v", err)
	}
	db := database.GetDB()
	domain := &model.ClusterDomain{Id: 1, Domain: "edge.example.com", HubURL: "https://hub.example.com", TokenEncrypted: "token", LastVersion: 7}
	if err := db.Create(domain).Error; err != nil {
		t.Fatalf("seed domain: %v", err)
	}
	local := newTestClusterLocalNode(t, "node-a")
	if err := db.Create(local).Error; err != nil {
		t.Fatalf("seed local node: %v", err)
	}
	peerToken, err := EncryptClusterDomainToken([]byte("test-secret"), "peer-token")
	if err != nil {
		t.Fatalf("encrypt peer token: %v", err)
	}
	for _, member := range []model.ClusterMember{
		{DomainID: domain.Id, NodeID: "node-b", DisplayName: "Node B", BaseURL: "https://node-b.example.com", PeerTokenEncrypted: peerToken, LastVersion: 7},
		{DomainID: domain.Id, NodeID: "node-c", DisplayName: "Node C", BaseURL: "https://node-c.example.com", PeerTokenEncrypted: peerToken, LastVersion: 7},
	} {
		member := member
		if err := db.Create(&member).Error; err != nil {
			t.Fatalf("seed peer member %s: %v", member.NodeID, err)
		}
	}

	var sentNodes []string
	coordinator := &ClusterDomainResourceCoordinator{
		DB:             db,
		OperationStore: &ClusterDomainOperationStore{DB: db},
		PeerSender: clusterDomainPeerSenderFunc(func(_ context.Context, _ *PeerMessage, member model.ClusterMember, _ string) (*clustertypes.DomainResourceCommandResult, error) {
			sentNodes = append(sentNodes, member.NodeID)
			return &clustertypes.DomainResourceCommandResult{
				Status:       "applied",
				OperationID:  "unused-by-status",
				NodeID:       member.NodeID,
				MemberID:     member.NodeID,
				ResourceKind: ClusterDomainResourceInbound,
				ResourceID:   "group-1",
				Revision:     domain.LastVersion,
			}, nil
		}),
		Identity:       &stubDomainInboundIdentity{node: local},
		SecretProvider: stubClusterSecretProvider{secret: []byte("test-secret")},
		PortAllocator:  func() (int, error) { return 32051, nil },
	}

	op, err := coordinator.CreateDomainInbound(context.Background(), domain.Id, ClusterDomainInboundCommandInput{
		GroupID: "group-1",
		Inbound: map[string]any{
			"type": "vless",
			"tag":  "main",
		},
	})
	if err != nil {
		t.Fatalf("create inbound: %v", err)
	}
	if strings.Join(sentNodes, ",") != "node-b,node-c" {
		t.Fatalf("initial dispatch nodes = %#v, want node-b and node-c", sentNodes)
	}
	if op.Summary.Applied != 3 || op.Summary.Total != 3 {
		t.Fatalf("summary = %+v, want applied=3 total=3", op.Summary)
	}
}

func TestDomainResourceCoordinatorCreateInboundReportsProxyConfigsAfterLocalApply(t *testing.T) {
	if err := database.InitDB(filepath.Join(t.TempDir(), "coordinator-local-proxy-report.db")); err != nil {
		if strings.Contains(err.Error(), "go-sqlite3 requires cgo") || strings.Contains(err.Error(), "CGO_ENABLED=0") || strings.Contains(err.Error(), "C compiler") {
			t.Skipf("sqlite test database unavailable in this toolchain: %v", err)
		}
		t.Fatalf("init db: %v", err)
	}
	db := database.GetDB()
	domain := &model.ClusterDomain{Id: 1, Domain: "edge.example.com", HubURL: "https://hub.example.com", TokenEncrypted: "token", LastVersion: 7}
	if err := db.Create(domain).Error; err != nil {
		t.Fatalf("seed domain: %v", err)
	}
	local := newTestClusterLocalNode(t, "node-a")
	if err := db.Create(local).Error; err != nil {
		t.Fatalf("seed local node: %v", err)
	}

	reporter := &countingDomainResourceProxyReporter{}
	coordinator := &ClusterDomainResourceCoordinator{
		DB:             db,
		OperationStore: &ClusterDomainOperationStore{DB: db},
		PeerSender: clusterDomainPeerSenderFunc(func(context.Context, *PeerMessage, model.ClusterMember, string) (*clustertypes.DomainResourceCommandResult, error) {
			t.Fatal("no peer dispatch expected for local-only target")
			return nil, nil
		}),
		Identity:       &stubDomainInboundIdentity{node: local},
		SecretProvider: stubClusterSecretProvider{secret: []byte("test-secret")},
		PortAllocator:  func() (int, error) { return 32051, nil },
		ProxyReporter:  reporter,
	}

	_, err := coordinator.CreateDomainInbound(context.Background(), domain.Id, ClusterDomainInboundCommandInput{
		GroupID: "group-1",
		TargetMembers: []clustertypes.DomainInboundTarget{{
			NodeID:      local.NodeID,
			MemberID:    local.NodeID,
			DisplayName: "Node A",
		}},
		Inbound: map[string]any{
			"type": "vless",
			"tag":  "main",
		},
	})
	if err != nil {
		t.Fatalf("create inbound: %v", err)
	}
	if reporter.count != 1 || reporter.domainID != domain.Id {
		t.Fatalf("proxy report calls = %d for domain %d, want 1 for domain %d", reporter.count, reporter.domainID, domain.Id)
	}
}

func TestClusterServiceDomainResourceCoordinatorUsesProxyReporter(t *testing.T) {
	reporter := &ClusterProxyReportService{}
	service := &ClusterService{}
	service.SetProxyReportService(reporter)

	coordinator := service.newDomainResourceCoordinator()
	if coordinator.ProxyReporter != reporter {
		t.Fatalf("coordinator proxy reporter = %p, want %p", coordinator.ProxyReporter, reporter)
	}
}

func TestDomainResourceCoordinatorCreatesDistinctPeerMessagesForSameDomainVersion(t *testing.T) {
	if err := database.InitDB(filepath.Join(t.TempDir(), "coordinator-distinct-peer-messages.db")); err != nil {
		if strings.Contains(err.Error(), "go-sqlite3 requires cgo") || strings.Contains(err.Error(), "CGO_ENABLED=0") || strings.Contains(err.Error(), "C compiler") {
			t.Skipf("sqlite test database unavailable in this toolchain: %v", err)
		}
		t.Fatalf("init db: %v", err)
	}
	db := database.GetDB()
	domain := &model.ClusterDomain{Id: 1, Domain: "edge.example.com", HubURL: "https://hub.example.com", TokenEncrypted: "token", LastVersion: 7}
	if err := db.Create(domain).Error; err != nil {
		t.Fatalf("seed domain: %v", err)
	}
	local := newTestClusterLocalNode(t, "node-a")
	if err := db.Create(local).Error; err != nil {
		t.Fatalf("seed local node: %v", err)
	}
	peerToken, err := EncryptClusterDomainToken([]byte("test-secret"), "peer-token")
	if err != nil {
		t.Fatalf("encrypt peer token: %v", err)
	}
	if err := db.Create(&model.ClusterMember{DomainID: domain.Id, NodeID: "node-b", DisplayName: "Node B", BaseURL: "https://node-b.example.com", PeerTokenEncrypted: peerToken, LastVersion: 7}).Error; err != nil {
		t.Fatalf("seed peer member: %v", err)
	}

	peerStore := newMemoryPeerStore()
	coordinator := &ClusterDomainResourceCoordinator{
		DB:             db,
		OperationStore: &ClusterDomainOperationStore{DB: db},
		PeerSender: clusterDomainPeerSenderFunc(func(_ context.Context, message *PeerMessage, member model.ClusterMember, _ string) (*clustertypes.DomainResourceCommandResult, error) {
			if _, err := peerStore.RecordReceived(message); err != nil {
				return nil, err
			}
			return &clustertypes.DomainResourceCommandResult{
				Status:       "applied",
				OperationID:  message.Payload["operation_id"].(string),
				NodeID:       member.NodeID,
				MemberID:     member.NodeID,
				ResourceKind: ClusterDomainResourceInbound,
				ResourceID:   message.Payload["resource_id"].(string),
				Revision:     domain.LastVersion,
			}, nil
		}),
		Identity:       &stubDomainInboundIdentity{node: local},
		SecretProvider: stubClusterSecretProvider{secret: []byte("test-secret")},
		PortAllocator:  func() (int, error) { return 32051, nil },
	}

	for _, groupID := range []string{"group-1", "group-2"} {
		op, err := coordinator.CreateDomainInbound(context.Background(), domain.Id, ClusterDomainInboundCommandInput{
			GroupID: groupID,
			Inbound: map[string]any{
				"type": "vless",
				"tag":  groupID,
			},
		})
		if err != nil {
			t.Fatalf("create inbound %s: %v", groupID, err)
		}
		if op.Status != ClusterDomainOperationApplied {
			t.Fatalf("operation %s status = %q, want %q", groupID, op.Status, ClusterDomainOperationApplied)
		}
	}
}

func TestDomainResourcePeerMessagesDoNotShareDomainVersionAsSourceSequence(t *testing.T) {
	domain := &model.ClusterDomain{Domain: "edge.example.com", LastVersion: 7}
	peerStore := newMemoryPeerStore()

	for _, groupID := range []string{"group-1", "group-2"} {
		operationID := "domain-inbound-" + groupID
		payload := domainResourcePeerPayload(
			domain,
			"node-a",
			operationID,
			operationID,
			ClusterDomainResourceInbound,
			groupID,
			domain.LastVersion,
			nil,
			map[string]interface{}{
				"request_id": operationID,
				"domain_id":  domain.Domain,
				"group_id":   groupID,
			},
		)
		message, err := NewClusterPeerMessage(domain.Domain, domain.LastVersion, "node-a", domainResourcePeerSourceSeq(), PeerCategoryCommand, PeerActionDomainInboundCreate, payload)
		if err != nil {
			t.Fatalf("new peer message: %v", err)
		}
		message.PayloadHash, err = ClusterPeerPayloadHash(message.Payload)
		if err != nil {
			t.Fatalf("payload hash: %v", err)
		}
		if _, err := peerStore.RecordReceived(message); err != nil {
			t.Fatalf("record peer message for %s: %v", groupID, err)
		}
	}
}

func TestClusterHubClientReportDomainResourceStateUsesCallerContext(t *testing.T) {
	canceled, cancel := context.WithCancel(context.Background())
	cancel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("request should not be sent with a canceled context")
	}))
	defer server.Close()

	client := &ClusterHubClient{HTTPClient: server.Client()}
	err := client.ReportDomainResourceState(canceled, server.URL, "edge.example.com", "domain-token", ClusterHubResourceStateReportRequest{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("report error = %v, want context.Canceled", err)
	}
}

func TestDomainResourceReportContextSurvivesCallerCancellation(t *testing.T) {
	canceled, cancel := context.WithCancel(context.Background())
	cancel()

	reportCtx, reportCancel := domainResourceReportContext(canceled)
	defer reportCancel()

	if err := reportCtx.Err(); err != nil {
		t.Fatalf("report context should remain usable after caller cancellation, got %v", err)
	}
}

func TestDomainResourceCoordinatorPickListTargetsOnlySelectedNodes(t *testing.T) {
	members := []model.ClusterMember{
		{NodeID: "node-a", DisplayName: "Node A", BaseURL: "https://node-a.example.com"},
		{NodeID: "node-b", DisplayName: "Node B", BaseURL: "https://node-b.example.com"},
		{NodeID: "node-c", DisplayName: "Node C", BaseURL: "https://node-c.example.com"},
		{NodeID: "node-d", DisplayName: "Node D"},
	}
	targets := []clustertypes.DomainInboundTarget{
		{NodeID: "node-c", DisplayName: "Node C"},
	}

	if shouldApplyLocalDomainResourceTarget(targets, "node-a") {
		t.Fatal("local node should not apply when it is absent from the pick list")
	}

	selected := selectedDomainResourceTargets(members, "node-a", targets)
	if len(selected) != 1 || selected[0].NodeID != "node-c" {
		t.Fatalf("selected targets = %#v, want only node-c", selected)
	}
}

func TestDomainResourceCoordinatorPickListCanIncludeLocalNode(t *testing.T) {
	targets := []clustertypes.DomainInboundTarget{
		{NodeID: "node-a", DisplayName: "Node A"},
		{NodeID: "node-c", DisplayName: "Node C"},
	}

	if !shouldApplyLocalDomainResourceTarget(targets, "node-a") {
		t.Fatal("local node should apply when it is included in the pick list")
	}
}

func TestDomainResourceCoordinatorReportsHubResourceStateCurrentShape(t *testing.T) {
	if err := database.InitDB(filepath.Join(t.TempDir(), "coordinator-report.db")); err != nil {
		if strings.Contains(err.Error(), "go-sqlite3 requires cgo") || strings.Contains(err.Error(), "CGO_ENABLED=0") || strings.Contains(err.Error(), "C compiler") {
			t.Skipf("sqlite test database unavailable in this toolchain: %v", err)
		}
		t.Fatalf("init db: %v", err)
	}
	db := database.GetDB()
	domainToken := "domain-token"
	domain := &model.ClusterDomain{
		Id:          1,
		Domain:      "edge.example.com",
		HubURL:      "https://hub.example.com",
		LastVersion: 7,
	}
	var err error
	domain.TokenEncrypted, err = EncryptClusterDomainToken([]byte("test-secret"), domainToken)
	if err != nil {
		t.Fatalf("encrypt domain token: %v", err)
	}
	if err := db.Create(domain).Error; err != nil {
		t.Fatalf("seed domain: %v", err)
	}
	local := newTestClusterLocalNode(t, "node-a")
	if err := db.Create(local).Error; err != nil {
		t.Fatalf("seed local node: %v", err)
	}
	inbound := &model.Inbound{
		Id:      11,
		Type:    "vless",
		Tag:     "main",
		Options: json.RawMessage(`{"listen_port":443}`),
	}
	if err := db.Create(inbound).Error; err != nil {
		t.Fatalf("seed inbound: %v", err)
	}
	if err := db.Create(&model.ClusterInbound{
		DomainID:  domain.Id,
		Domain:    domain.Domain,
		NodeID:    local.NodeID,
		MemberID:  local.NodeID,
		GroupID:   "group-1",
		InboundID: inbound.Id,
		RequestID: "req-inbound-1",
	}).Error; err != nil {
		t.Fatalf("seed cluster inbound: %v", err)
	}
	client := &model.Client{
		Id:         21,
		Enable:     true,
		Name:       "Alice",
		Config:     json.RawMessage(`{"level":1}`),
		Inbounds:   json.RawMessage(`[11]`),
		Links:      json.RawMessage(`[]`),
		Volume:     0,
		Down:       0,
		Up:         0,
		Expiry:     0,
		DelayStart: false,
		AutoReset:  false,
		ResetDays:  0,
		TotalUp:    0,
		TotalDown:  0,
	}
	if err := db.Create(client).Error; err != nil {
		t.Fatalf("seed client: %v", err)
	}
	if err := db.Create(&model.ClusterClient{
		DomainID:             domain.Id,
		Domain:               domain.Domain,
		NodeID:               local.NodeID,
		MemberID:             local.NodeID,
		ClientID:             client.Id,
		HubUserUUID:          "user-a",
		RequestID:            "req-user-1",
		SubToken:             "stable-token",
		BoundInboundGroupIDs: json.RawMessage(`["group-1"]`),
	}).Error; err != nil {
		t.Fatalf("seed cluster client: %v", err)
	}

	hub := &stubClusterHubClient{}
	coordinator := &ClusterDomainResourceCoordinator{
		DB:             db,
		OperationStore: &ClusterDomainOperationStore{DB: db},
		HubClient:      hub,
		Identity:       &stubDomainInboundIdentity{node: local},
		SecretProvider: stubClusterSecretProvider{secret: []byte("test-secret")},
	}
	op := &ClusterDomainOperationView{
		OperationID:  "op-1",
		DomainID:     domain.Id,
		Domain:       domain.Domain,
		ResourceKind: ClusterDomainResourceInbound,
		ResourceID:   "group-1",
		Action:       ClusterDomainOperationCreate,
		Revision:     7,
		Status:       ClusterDomainOperationPartial,
		Summary:      ClusterDomainOperationSummary{},
	}

	if err := coordinator.ReportDomainResourceState(context.Background(), domain, op); err != nil {
		t.Fatalf("report domain resource state: %v", err)
	}
	if hub.lastResourceStateHubURL != domain.HubURL || hub.lastResourceStateDomain != domain.Domain {
		t.Fatalf("unexpected report target: hub=%q domain=%q", hub.lastResourceStateHubURL, hub.lastResourceStateDomain)
	}
	if hub.lastResourceStateToken != domainToken {
		t.Fatalf("resource state token = %q, want %q", hub.lastResourceStateToken, domainToken)
	}
	body := hub.lastResourceStateBody
	if body.ReportedByNodeID != local.NodeID {
		t.Fatalf("reported_by_node_id = %q, want %q", body.ReportedByNodeID, local.NodeID)
	}
	if body.OperationID != "op-1" {
		t.Fatalf("operation_id = %q, want op-1", body.OperationID)
	}
	if len(body.Resources.Inbounds) != 1 {
		t.Fatalf("inbounds = %d, want 1", len(body.Resources.Inbounds))
	}
	if body.Resources.Inbounds[0].GroupID != "group-1" || body.Resources.Inbounds[0].Type != "vless" {
		t.Fatalf("unexpected inbound payload: %#v", body.Resources.Inbounds[0])
	}
	if body.Resources.Inbounds[0].OptionsJSON != `{"listen_port":443}` {
		t.Fatalf("options_json = %q", body.Resources.Inbounds[0].OptionsJSON)
	}
	if len(body.Resources.Inbounds[0].Instances) != 1 {
		t.Fatalf("inbound instances = %d, want 1", len(body.Resources.Inbounds[0].Instances))
	}
	if body.Resources.Inbounds[0].Instances[0].NodeID != local.NodeID || body.Resources.Inbounds[0].Instances[0].Status != ClusterDomainOperationApplied {
		t.Fatalf("unexpected inbound instance: %#v", body.Resources.Inbounds[0].Instances[0])
	}
	rawReport, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal resource state body: %v", err)
	}
	if strings.Contains(string(rawReport), "nodeId") || !strings.Contains(string(rawReport), `"node_id":"`+"node-a"+`"`) {
		t.Fatalf("resource state instances must use snake_case fields: %s", rawReport)
	}
	if len(body.Resources.Users) != 1 {
		t.Fatalf("users = %d, want 1", len(body.Resources.Users))
	}
	if body.Resources.Users[0].UUID != "user-a" || body.Resources.Users[0].Name != "Alice" {
		t.Fatalf("unexpected user payload: %#v", body.Resources.Users[0])
	}
	if body.Resources.Users[0].SubToken == "" {
		t.Fatalf("expected resource user sub_token for Hub read model")
	}
	if body.Resources.Users[0].SubToken != "stable-token" {
		t.Fatalf("expected stable resource user sub_token, got %q", body.Resources.Users[0].SubToken)
	}
	if len(body.Resources.Users[0].Config) == 0 || string(body.Resources.Users[0].Config) != `{"level":1}` {
		t.Fatalf("user config = %s, want {\"level\":1}", body.Resources.Users[0].Config)
	}
	if len(body.Resources.Users[0].BoundInboundGroupIDs) != 1 || body.Resources.Users[0].BoundInboundGroupIDs[0] != "group-1" {
		t.Fatalf("user bound groups = %#v, want [group-1]", body.Resources.Users[0].BoundInboundGroupIDs)
	}
	if len(body.Resources.Users[0].Inbounds) == 0 || string(body.Resources.Users[0].Inbounds) != `["domain:group-1"]` {
		t.Fatalf("user inbounds = %#v, want domain group selector", body.Resources.Users[0].Inbounds)
	}
	if body.OperationSummary.ResourceKind != ClusterDomainResourceInbound || body.OperationSummary.ResourceID != "group-1" || body.OperationSummary.Action != ClusterDomainOperationCreate {
		t.Fatalf("unexpected operation payload: %#v", body.OperationSummary)
	}
}

func TestDomainResourcesIncludeRemoteOnlyDomainInboundOperations(t *testing.T) {
	if err := database.InitDB(filepath.Join(t.TempDir(), "coordinator-remote-only-read-model.db")); err != nil {
		if strings.Contains(err.Error(), "go-sqlite3 requires cgo") || strings.Contains(err.Error(), "CGO_ENABLED=0") || strings.Contains(err.Error(), "C compiler") {
			t.Skipf("sqlite test database unavailable in this toolchain: %v", err)
		}
		t.Fatalf("init db: %v", err)
	}
	db := database.GetDB()
	domain := &model.ClusterDomain{Id: 1, Domain: "edge.example.com", HubURL: "https://hub.example.com", TokenEncrypted: "token", LastVersion: 8}
	if err := db.Create(domain).Error; err != nil {
		t.Fatalf("seed domain: %v", err)
	}
	local := newTestClusterLocalNode(t, "node-a")
	if err := db.Create(local).Error; err != nil {
		t.Fatalf("seed local node: %v", err)
	}

	store := &ClusterDomainOperationStore{DB: db}
	payload := clustertypes.DomainInboundCreatePayload{
		RequestID:   "op-remote-only",
		DomainID:    domain.Domain,
		GroupID:     "remote-edge",
		TagSeed:     "edge",
		Prefix:      "domain",
		Suffix:      "remote",
		Inbound:     json.RawMessage(`{"type":"vless","tag":"remote","options":{"listen_port":443}}`),
		TLSTemplate: "standard",
		TargetMembers: []clustertypes.DomainInboundTarget{{
			NodeID:      "node-b",
			MemberID:    "node-b",
			DisplayName: "Node B",
		}},
	}
	desiredPayload, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	if err := store.SaveOperation(&model.ClusterDomainOperation{
		OperationID:       payload.RequestID,
		DomainID:          domain.Id,
		Domain:            domain.Domain,
		ResourceKind:      ClusterDomainResourceInbound,
		ResourceID:        payload.GroupID,
		Action:            ClusterDomainOperationCreate,
		Revision:          domain.LastVersion,
		CoordinatorNodeID: local.NodeID,
		Status:            ClusterDomainOperationApplied,
		DesiredPayload:    desiredPayload,
	}); err != nil {
		t.Fatalf("save operation: %v", err)
	}
	if err := store.SaveInstance(&model.ClusterDomainOperationInstance{
		OperationID:     payload.RequestID,
		NodeID:          "node-b",
		MemberID:        "node-b",
		DisplayName:     "Node B",
		TargetTag:       "remote",
		Status:          ClusterDomainOperationApplied,
		AttemptCount:    1,
		LocalResourceID: 77,
	}); err != nil {
		t.Fatalf("save instance: %v", err)
	}

	coordinator := &ClusterDomainResourceCoordinator{DB: db, OperationStore: store}
	resources, err := coordinator.buildDomainResources(domain.Id)
	if err != nil {
		t.Fatalf("build domain resources: %v", err)
	}
	if len(resources.Inbounds) != 1 {
		t.Fatalf("inbounds = %d, want 1", len(resources.Inbounds))
	}
	inbound := resources.Inbounds[0]
	if inbound.GroupID != "remote-edge" || inbound.Type != "vless" {
		t.Fatalf("unexpected inbound read model: %#v", inbound)
	}
	if inbound.LastOperationID != "op-remote-only" || inbound.LastOperationStatus != ClusterDomainOperationApplied {
		t.Fatalf("unexpected operation metadata: %#v", inbound)
	}
	if len(inbound.Instances) != 1 || inbound.Instances[0].NodeID != "node-b" || inbound.Instances[0].Status != ClusterDomainOperationApplied {
		t.Fatalf("unexpected remote inbound instances: %#v", inbound.Instances)
	}
}

func TestDomainInboundResourceFromOperationPreservesTopLevelListenPortSource(t *testing.T) {
	payload := clustertypes.DomainInboundCreatePayload{
		RequestID:   "domain-inbound-auto",
		DomainID:    "edge.example.com",
		GroupID:     "group-1",
		TagSeed:     "domain",
		Prefix:      "domain",
		Inbound:     json.RawMessage(`{"type":"vless","tag":"domain","listen":"::","listen_port":{"LocalProvided":"DomainInboundListenPort"}}`),
		TLSTemplate: "standard",
	}
	desiredPayload, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	resource, ok := domainInboundResourceFromOperation(model.ClusterDomainOperation{
		OperationID:       payload.RequestID,
		DomainID:          1,
		Domain:            payload.DomainID,
		ResourceKind:      ClusterDomainResourceInbound,
		ResourceID:        payload.GroupID,
		Action:            ClusterDomainOperationCreate,
		Revision:          8,
		CoordinatorNodeID: "node-a",
		Status:            ClusterDomainOperationApplied,
		DesiredPayload:    desiredPayload,
	}, nil)

	if !ok {
		t.Fatalf("expected domain inbound resource")
	}
	if resource.OptionsJSON != `{"listen":"::","listen_port":{"LocalProvided":"DomainInboundListenPort"}}` {
		t.Fatalf("options_json = %q", resource.OptionsJSON)
	}
}

func TestApplyDomainInboundOperationResourcePreservesDesiredOptions(t *testing.T) {
	existing := ClusterHubDomainResourceInbound{
		GroupID:     "group-1",
		TagSeed:     "group-1",
		Type:        "vless",
		TLSTemplate: "standard",
		OptionsJSON: `{"listen":"::","listen_port":443}`,
		Status:      "active",
		Instances: []ClusterHubDomainResourceInstance{{
			NodeID: "node-a",
			Status: ClusterDomainOperationApplied,
		}},
	}
	desired := ClusterHubDomainResourceInbound{
		GroupID:             "group-1",
		TagSeed:             "domain",
		Prefix:              "domain",
		Type:                "vless",
		TLSTemplate:         "standard",
		OptionsJSON:         `{"listen":"::","listen_port":{"LocalProvided":"DomainInboundListenPort"}}`,
		LastOperationID:     "domain-inbound-auto",
		LastOperationStatus: ClusterDomainOperationApplied,
		Instances: []ClusterHubDomainResourceInstance{{
			NodeID:    "node-a",
			TargetTag: "domain-node-a",
			Status:    ClusterDomainOperationApplied,
		}},
	}

	applyDomainInboundOperationResource(&existing, desired)

	if existing.OptionsJSON != `{"listen":"::","listen_port":{"LocalProvided":"DomainInboundListenPort"}}` {
		t.Fatalf("options_json = %q", existing.OptionsJSON)
	}
	if existing.LastOperationID != "domain-inbound-auto" || existing.TagSeed != "domain" || existing.Prefix != "domain" {
		t.Fatalf("operation metadata was not applied: %#v", existing)
	}
	if len(existing.Instances) != 1 || existing.Instances[0].TargetTag != "domain-node-a" {
		t.Fatalf("instances were not merged: %#v", existing.Instances)
	}
}

func TestApplyDomainInboundOperationResourceDoesNotReplaceOptionsForFailedOperation(t *testing.T) {
	existing := ClusterHubDomainResourceInbound{
		GroupID:     "group-1",
		TagSeed:     "group-1",
		Type:        "vless",
		OptionsJSON: `{"listen":"::","listen_port":443}`,
		Status:      "active",
	}
	desired := ClusterHubDomainResourceInbound{
		GroupID:             "group-1",
		TagSeed:             "domain",
		Type:                "vless",
		OptionsJSON:         `{"listen":"::","listen_port":{"LocalProvided":"DomainInboundListenPort"}}`,
		LastOperationID:     "domain-inbound-failed",
		LastOperationStatus: ClusterDomainOperationFailed,
	}

	applyDomainInboundOperationResource(&existing, desired)

	if existing.OptionsJSON != `{"listen":"::","listen_port":443}` {
		t.Fatalf("failed operation should not replace options_json, got %q", existing.OptionsJSON)
	}
	if existing.LastOperationID != "domain-inbound-failed" || existing.LastOperationStatus != ClusterDomainOperationFailed {
		t.Fatalf("operation metadata was not applied: %#v", existing)
	}
}

func TestDomainResourcesPreserveDesiredDomainInboundListenPortSource(t *testing.T) {
	if err := database.InitDB(filepath.Join(t.TempDir(), "coordinator-inbound-listen-source.db")); err != nil {
		if strings.Contains(err.Error(), "go-sqlite3 requires cgo") || strings.Contains(err.Error(), "CGO_ENABLED=0") || strings.Contains(err.Error(), "C compiler") {
			t.Skipf("sqlite test database unavailable in this toolchain: %v", err)
		}
		t.Fatalf("init db: %v", err)
	}
	db := database.GetDB()
	domain := &model.ClusterDomain{Id: 1, Domain: "edge.example.com", HubURL: "https://hub.example.com", TokenEncrypted: "token", LastVersion: 8}
	if err := db.Create(domain).Error; err != nil {
		t.Fatalf("seed domain: %v", err)
	}
	local := newTestClusterLocalNode(t, "node-a")
	if err := db.Create(local).Error; err != nil {
		t.Fatalf("seed local node: %v", err)
	}
	inbound := &model.Inbound{
		Id:      11,
		Type:    "vless",
		Tag:     "domain-node-a",
		Options: json.RawMessage(`{"listen":"::","listen_port":443}`),
	}
	if err := db.Create(inbound).Error; err != nil {
		t.Fatalf("seed inbound: %v", err)
	}
	if err := db.Create(&model.ClusterInbound{
		DomainID:  domain.Id,
		Domain:    domain.Domain,
		NodeID:    local.NodeID,
		MemberID:  local.NodeID,
		GroupID:   "group-1",
		InboundID: inbound.Id,
		RequestID: "domain-inbound-auto",
	}).Error; err != nil {
		t.Fatalf("seed cluster inbound: %v", err)
	}

	payload := clustertypes.DomainInboundCreatePayload{
		RequestID:   "domain-inbound-auto",
		DomainID:    domain.Domain,
		GroupID:     "group-1",
		TagSeed:     "domain",
		Prefix:      "domain",
		Inbound:     json.RawMessage(`{"type":"vless","tag":"domain","listen":"::","listen_port":{"LocalProvided":"DomainInboundListenPort"}}`),
		TLSTemplate: "standard",
	}
	desiredPayload, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	store := &ClusterDomainOperationStore{DB: db}
	if err := store.SaveOperation(&model.ClusterDomainOperation{
		OperationID:       payload.RequestID,
		DomainID:          domain.Id,
		Domain:            domain.Domain,
		ResourceKind:      ClusterDomainResourceInbound,
		ResourceID:        payload.GroupID,
		Action:            ClusterDomainOperationCreate,
		Revision:          domain.LastVersion,
		CoordinatorNodeID: local.NodeID,
		Status:            ClusterDomainOperationApplied,
		DesiredPayload:    desiredPayload,
	}); err != nil {
		t.Fatalf("save operation: %v", err)
	}

	coordinator := &ClusterDomainResourceCoordinator{DB: db, OperationStore: store}
	resources, err := coordinator.buildDomainResources(domain.Id)
	if err != nil {
		t.Fatalf("build domain resources: %v", err)
	}
	if len(resources.Inbounds) != 1 {
		t.Fatalf("inbounds = %d, want 1", len(resources.Inbounds))
	}
	if resources.Inbounds[0].OptionsJSON != `{"listen":"::","listen_port":{"LocalProvided":"DomainInboundListenPort"}}` {
		t.Fatalf("options_json = %q", resources.Inbounds[0].OptionsJSON)
	}
}

func TestDomainResourceCoordinatorRetryOnlyFailedTargets(t *testing.T) {
	if err := database.InitDB(filepath.Join(t.TempDir(), "coordinator-retry.db")); err != nil {
		if strings.Contains(err.Error(), "go-sqlite3 requires cgo") || strings.Contains(err.Error(), "CGO_ENABLED=0") || strings.Contains(err.Error(), "C compiler") {
			t.Skipf("sqlite test database unavailable in this toolchain: %v", err)
		}
		t.Fatalf("init db: %v", err)
	}
	db := database.GetDB()
	domain := &model.ClusterDomain{Id: 1, Domain: "edge.example.com", HubURL: "https://hub.example.com", TokenEncrypted: "token", LastVersion: 7}
	if err := db.Create(domain).Error; err != nil {
		t.Fatalf("seed domain: %v", err)
	}
	local := newTestClusterLocalNode(t, "node-a")
	if err := db.Create(local).Error; err != nil {
		t.Fatalf("seed local node: %v", err)
	}
	peerToken, err := EncryptClusterDomainToken([]byte("test-secret"), "peer-token")
	if err != nil {
		t.Fatalf("encrypt peer token: %v", err)
	}
	peerMembers := []model.ClusterMember{
		{DomainID: domain.Id, NodeID: "node-b", DisplayName: "Node B", BaseURL: "https://node-b.example.com", PeerTokenEncrypted: peerToken, LastVersion: 7},
		{DomainID: domain.Id, NodeID: "node-c", DisplayName: "Node C", BaseURL: "https://node-c.example.com", PeerTokenEncrypted: peerToken, LastVersion: 7},
	}
	for _, member := range peerMembers {
		member := member
		if err := db.Create(&member).Error; err != nil {
			t.Fatalf("seed peer member %s: %v", member.NodeID, err)
		}
	}
	store := &ClusterDomainOperationStore{DB: db}
	payload := clustertypes.DomainInboundCreatePayload{
		RequestID: "op-retry",
		DomainID:  domain.Domain,
		GroupID:   "group-1",
		Inbound:   json.RawMessage(`{"type":"vless","tag":"main"}`),
	}
	desiredPayload, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	if err := store.SaveOperation(&model.ClusterDomainOperation{
		OperationID:       payload.RequestID,
		DomainID:          domain.Id,
		Domain:            domain.Domain,
		ResourceKind:      ClusterDomainResourceInbound,
		ResourceID:        payload.GroupID,
		Action:            ClusterDomainOperationCreate,
		Revision:          domain.LastVersion,
		CoordinatorNodeID: local.NodeID,
		Status:            ClusterDomainOperationPartial,
		DesiredPayload:    desiredPayload,
	}); err != nil {
		t.Fatalf("save operation: %v", err)
	}
	for _, instance := range []model.ClusterDomainOperationInstance{
		{OperationID: payload.RequestID, NodeID: local.NodeID, MemberID: local.NodeID, DisplayName: "Node A", Status: ClusterDomainOperationApplied, AttemptCount: 1},
		{OperationID: payload.RequestID, NodeID: "node-b", MemberID: "node-b", DisplayName: "Node B", Status: ClusterDomainOperationFailed, AttemptCount: 1, Error: "dial tcp timeout"},
		{OperationID: payload.RequestID, NodeID: "node-c", MemberID: "node-c", DisplayName: "Node C", Status: ClusterDomainOperationApplied, AttemptCount: 1},
	} {
		instance := instance
		if err := store.SaveInstance(&instance); err != nil {
			t.Fatalf("save instance %s: %v", instance.NodeID, err)
		}
	}

	var sentNodes []string
	coordinator := &ClusterDomainResourceCoordinator{
		DB:             db,
		OperationStore: store,
		PeerSender: clusterDomainPeerSenderFunc(func(_ context.Context, _ *PeerMessage, member model.ClusterMember, _ string) (*clustertypes.DomainResourceCommandResult, error) {
			sentNodes = append(sentNodes, member.NodeID)
			return &clustertypes.DomainResourceCommandResult{
				Status:       "applied",
				OperationID:  payload.RequestID,
				NodeID:       member.NodeID,
				MemberID:     member.NodeID,
				ResourceKind: ClusterDomainResourceInbound,
				ResourceID:   payload.GroupID,
				Revision:     domain.LastVersion,
			}, nil
		}),
		Identity:       &stubDomainInboundIdentity{node: local},
		SecretProvider: stubClusterSecretProvider{secret: []byte("test-secret")},
		PortAllocator: func() (int, error) {
			t.Fatal("local apply should not run on retry for already-applied local instance")
			return 0, nil
		},
	}

	op, err := coordinator.RetryDomainOperation(context.Background(), payload.RequestID)
	if err != nil {
		t.Fatalf("retry operation: %v", err)
	}
	if len(sentNodes) != 1 || sentNodes[0] != "node-b" {
		t.Fatalf("sent nodes = %#v, want [node-b]", sentNodes)
	}
	if op.Status != ClusterDomainOperationApplied {
		t.Fatalf("operation status = %q, want %q", op.Status, ClusterDomainOperationApplied)
	}
	instances, err := store.ListInstances(payload.RequestID)
	if err != nil {
		t.Fatalf("list instances: %v", err)
	}
	for _, instance := range instances {
		switch instance.NodeID {
		case "node-a":
			if instance.AttemptCount != 1 {
				t.Fatalf("local attempt count = %d, want 1", instance.AttemptCount)
			}
		case "node-b":
			if instance.AttemptCount != 2 {
				t.Fatalf("node-b attempt count = %d, want 2", instance.AttemptCount)
			}
		case "node-c":
			if instance.AttemptCount != 1 {
				t.Fatalf("node-c attempt count = %d, want 1", instance.AttemptCount)
			}
		}
	}
}
