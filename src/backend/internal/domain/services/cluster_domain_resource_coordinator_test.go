package service

import (
	"context"
	"encoding/json"
	"errors"
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
		DomainID:    domain.Id,
		Domain:      domain.Domain,
		NodeID:      local.NodeID,
		MemberID:    local.NodeID,
		ClientID:    client.Id,
		HubUserUUID: "user-a",
		RequestID:   "req-user-1",
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
	if len(body.Resources.Users) != 1 {
		t.Fatalf("users = %d, want 1", len(body.Resources.Users))
	}
	if body.Resources.Users[0].UUID != "user-a" || body.Resources.Users[0].Name != "Alice" {
		t.Fatalf("unexpected user payload: %#v", body.Resources.Users[0])
	}
	if body.Resources.Users[0].SubToken == "" {
		t.Fatalf("expected resource user sub_token for Hub read model")
	}
	if len(body.Resources.Users[0].Config) == 0 || string(body.Resources.Users[0].Config) != `{"level":1}` {
		t.Fatalf("user config = %s, want {\"level\":1}", body.Resources.Users[0].Config)
	}
	if len(body.Resources.Users[0].Inbounds) != 1 || string(body.Resources.Users[0].Inbounds) != `[11]` {
		t.Fatalf("user inbounds = %#v, want [11]", body.Resources.Users[0].Inbounds)
	}
	if body.OperationSummary.ResourceKind != ClusterDomainResourceInbound || body.OperationSummary.ResourceID != "group-1" || body.OperationSummary.Action != ClusterDomainOperationCreate {
		t.Fatalf("unexpected operation payload: %#v", body.OperationSummary)
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
