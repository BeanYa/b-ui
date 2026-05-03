package service

import (
	"context"
	"encoding/json"
	"testing"

	clustertypes "github.com/BeanYa/b-ui/src/backend/internal/domain/services/cluster/types"
	"github.com/BeanYa/b-ui/src/backend/internal/infra/db/model"
)

func TestDomainCleanupRemovesManagedDomainResources(t *testing.T) {
	db := initClusterInboundTestDB(t)

	tlsConfig := model.Tls{
		Name:   "edge-main-tls",
		Server: json.RawMessage(`{"enabled":true}`),
		Client: json.RawMessage(`{"enabled":true}`),
	}
	if err := db.Create(&tlsConfig).Error; err != nil {
		t.Fatalf("seed tls: %v", err)
	}
	inbound := model.Inbound{Type: "vless", Tag: "edge-main-node-a", TlsId: tlsConfig.Id, Options: json.RawMessage(`{"listen_port":32001}`)}
	if err := db.Create(&inbound).Error; err != nil {
		t.Fatalf("seed inbound: %v", err)
	}
	clientInbounds, _ := json.Marshal([]uint{inbound.Id})
	client := model.Client{
		Enable:   true,
		Name:     "alice",
		Config:   json.RawMessage(`{"vless":{"uuid":"11111111-1111-4111-8111-111111111111"}}`),
		Inbounds: clientInbounds,
		Links:    json.RawMessage(`[]`),
	}
	if err := db.Create(&client).Error; err != nil {
		t.Fatalf("seed client: %v", err)
	}
	if err := db.Create(&model.ClusterInbound{
		DomainID:  1,
		Domain:    "edge.example.com",
		NodeID:    "node-a",
		MemberID:  "hub",
		InboundID: inbound.Id,
		RequestID: "req-inbound-a",
		Prefix:    "edge",
		Template:  "standard",
	}).Error; err != nil {
		t.Fatalf("seed inbound wrapper: %v", err)
	}
	if err := db.Create(&model.ClusterClient{
		DomainID:    1,
		Domain:      "edge.example.com",
		NodeID:      "node-a",
		MemberID:    "hub",
		ClientID:    client.Id,
		HubUserUUID: "user-uuid-1",
		RequestID:   "req-user-a",
	}).Error; err != nil {
		t.Fatalf("seed client wrapper: %v", err)
	}

	svc := NewClusterDomainCleanupService(ClusterDomainCleanupServiceOptions{DB: db})
	result, err := svc.ApplyDomainCleanup(context.Background(), &model.ClusterDomain{Id: 1, Domain: "edge.example.com"}, clustertypes.DomainCleanupPayload{
		RequestID: "req-cleanup",
		DomainID:  "edge.example.com",
	}, "hub", false)
	if err != nil {
		t.Fatalf("cleanup domain: %v", err)
	}
	if result.ClientsDeleted != 1 || result.InboundsDeleted != 1 || result.TLSDeleted != 1 {
		t.Fatalf("unexpected cleanup result: %#v", result)
	}

	for _, table := range []string{"cluster_clients", "cluster_inbounds", "clients", "inbounds", "tls"} {
		var count int64
		if err := db.Table(table).Count(&count).Error; err != nil {
			t.Fatalf("count %s: %v", table, err)
		}
		if count != 0 {
			t.Fatalf("expected %s to be empty, got %d", table, count)
		}
	}
}
