package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	clustertypes "github.com/BeanYa/b-ui/src/backend/internal/domain/services/cluster/types"
	"github.com/BeanYa/b-ui/src/backend/internal/infra/db/model"
)

func TestDomainUserUpsertCreatesManagedClientForSelectedDomainInbounds(t *testing.T) {
	db := initClusterInboundTestDB(t)

	inboundA := model.Inbound{Type: "vless", Tag: "edge-main-node-a", Options: json.RawMessage(`{"listen_port":32001}`)}
	if err := db.Create(&inboundA).Error; err != nil {
		t.Fatalf("seed inbound a: %v", err)
	}
	inboundB := model.Inbound{Type: "vmess", Tag: "edge-alt-node-a", Options: json.RawMessage(`{"listen_port":32002}`)}
	if err := db.Create(&inboundB).Error; err != nil {
		t.Fatalf("seed inbound b: %v", err)
	}
	if err := db.Create(&model.ClusterInbound{
		DomainID:  1,
		Domain:    "edge.example.com",
		NodeID:    "node-a",
		MemberID:  "hub",
		InboundID: inboundA.Id,
		RequestID: "req-inbound-a",
		Prefix:    "edge",
		Template:  "standard",
	}).Error; err != nil {
		t.Fatalf("seed wrapper a: %v", err)
	}
	if err := db.Create(&model.ClusterInbound{
		DomainID:  1,
		Domain:    "edge.example.com",
		NodeID:    "node-a",
		MemberID:  "hub",
		InboundID: inboundB.Id,
		RequestID: "req-inbound-b",
		Prefix:    "edge",
		Template:  "none",
	}).Error; err != nil {
		t.Fatalf("seed wrapper b: %v", err)
	}

	svc := NewClusterDomainUserService(ClusterDomainUserServiceOptions{
		DB:       db,
		Identity: &stubDomainInboundIdentity{node: &model.ClusterLocalNode{NodeID: "node-a"}},
		Now:      func() int64 { return 1700000000 },
	})
	result, err := svc.ApplyDomainUserUpsert(context.Background(), &model.ClusterDomain{Id: 1, Domain: "edge.example.com"}, clustertypes.DomainUserUpsertPayload{
		RequestID: "req-user-1",
		DomainID:  "edge.example.com",
		User: clustertypes.DomainUserPayload{
			UUID:   "user-uuid-1",
			Name:   "alice",
			Enable: true,
			Config: json.RawMessage(`{"vless":{"uuid":"11111111-1111-4111-8111-111111111111","flow":"xtls-rprx-vision"},"vmess":{"uuid":"22222222-2222-4222-8222-222222222222"}}`),
		},
		Inbounds: []string{"domain:req-inbound-a"},
	}, "hub", false)
	if err != nil {
		t.Fatalf("upsert domain user: %v", err)
	}
	if result.ClientID == 0 || !result.Created {
		t.Fatalf("unexpected result: %#v", result)
	}

	var client model.Client
	if err := db.First(&client, result.ClientID).Error; err != nil {
		t.Fatalf("load client: %v", err)
	}
	if client.Name != "alice" || !client.Enable {
		t.Fatalf("unexpected client: %#v", client)
	}
	var inboundIDs []uint
	if err := json.Unmarshal(client.Inbounds, &inboundIDs); err != nil {
		t.Fatalf("decode client inbounds: %v", err)
	}
	if len(inboundIDs) != 1 || inboundIDs[0] != inboundA.Id {
		t.Fatalf("expected selected local domain inbound, got %#v", inboundIDs)
	}

	var wrapper model.ClusterClient
	if err := db.Where("domain_id = ? AND hub_user_uuid = ?", 1, "user-uuid-1").First(&wrapper).Error; err != nil {
		t.Fatalf("load cluster client wrapper: %v", err)
	}
	if wrapper.ClientID != client.Id || wrapper.Domain != "edge.example.com" || wrapper.NodeID != "node-a" || wrapper.MemberID != "hub" {
		t.Fatalf("unexpected wrapper: %#v", wrapper)
	}
}

func TestDomainUserUpsertResolvesLocalProvidedConfigOnTargetNode(t *testing.T) {
	db := initClusterInboundTestDB(t)
	inbound := model.Inbound{Type: "vless", Tag: "edge-main-node-a", Options: json.RawMessage(`{"listen_port":32001}`)}
	if err := db.Create(&inbound).Error; err != nil {
		t.Fatalf("seed inbound: %v", err)
	}
	if err := db.Create(&model.ClusterInbound{
		DomainID:  1,
		Domain:    "edge.example.com",
		NodeID:    "node-a",
		MemberID:  "hub",
		InboundID: inbound.Id,
		RequestID: "req-inbound-a",
	}).Error; err != nil {
		t.Fatalf("seed wrapper: %v", err)
	}

	svc := NewClusterDomainUserService(ClusterDomainUserServiceOptions{
		DB:       db,
		Identity: &stubDomainInboundIdentity{node: &model.ClusterLocalNode{NodeID: "node-a"}},
		Now:      func() int64 { return 1700000000 },
	})
	result, err := svc.ApplyDomainUserUpsert(context.Background(), &model.ClusterDomain{Id: 1, Domain: "edge.example.com"}, clustertypes.DomainUserUpsertPayload{
		RequestID: "req-user-local-provided",
		DomainID:  "edge.example.com",
		User: clustertypes.DomainUserPayload{
			UUID:   "hub-user-uuid",
			Name:   "alice",
			Enable: true,
			Config: json.RawMessage(`{"vless":{"uuid":{"LocalProvided":"DomainUserUUID"},"flow":"xtls-rprx-vision"},"trojan":{"password":{"LocalProvided":"DomainUserPassword"}},"hysteria":{"auth_str":{"LocalProvided":"DomainUserAuth"}}}`),
		},
		Inbounds: []string{"domain:req-inbound-a"},
	}, "hub", false)
	if err != nil {
		t.Fatalf("upsert domain user: %v", err)
	}

	var client model.Client
	if err := db.First(&client, result.ClientID).Error; err != nil {
		t.Fatalf("load client: %v", err)
	}
	var config map[string]map[string]interface{}
	if err := json.Unmarshal(client.Config, &config); err != nil {
		t.Fatalf("decode client config: %v", err)
	}
	if _, ok := config["vless"]["uuid"].(map[string]interface{}); ok {
		t.Fatalf("expected vless uuid LocalProvided marker to be resolved, got %#v", config["vless"]["uuid"])
	}
	if strings.TrimSpace(config["vless"]["uuid"].(string)) == "" {
		t.Fatalf("expected generated vless uuid, got %#v", config["vless"]["uuid"])
	}
	if strings.TrimSpace(config["trojan"]["password"].(string)) == "" {
		t.Fatalf("expected generated trojan password, got %#v", config["trojan"]["password"])
	}
	if strings.TrimSpace(config["hysteria"]["auth_str"].(string)) == "" {
		t.Fatalf("expected generated hysteria auth, got %#v", config["hysteria"]["auth_str"])
	}

	var wrapper model.ClusterClient
	if err := db.Where("domain_id = ? AND hub_user_uuid = ?", 1, "hub-user-uuid").First(&wrapper).Error; err != nil {
		t.Fatalf("load cluster client wrapper: %v", err)
	}
	if wrapper.HubUserUUID != "hub-user-uuid" {
		t.Fatalf("expected stable hub user uuid to remain the resource id, got %#v", wrapper)
	}
}
