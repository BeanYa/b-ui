package service

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/BeanYa/b-ui/src/backend/internal/infra/db/model"
)

func TestManagedDomainResourcesAreAnnotatedAndReadOnly(t *testing.T) {
	db := initClusterInboundTestDB(t)

	tlsConfig := model.Tls{Name: "edge-main-tls", Server: json.RawMessage(`{}`), Client: json.RawMessage(`{}`)}
	if err := db.Create(&tlsConfig).Error; err != nil {
		t.Fatalf("seed tls: %v", err)
	}
	inbound := model.Inbound{Type: "vless", Tag: "edge-main-node-a", TlsId: tlsConfig.Id, Options: json.RawMessage(`{"listen_port":32001}`)}
	if err := db.Create(&inbound).Error; err != nil {
		t.Fatalf("seed inbound: %v", err)
	}
	client := model.Client{
		Enable:   true,
		Name:     "alice",
		Config:   json.RawMessage(`{"vless":{"uuid":"11111111-1111-4111-8111-111111111111"}}`),
		Inbounds: json.RawMessage(`[1]`),
		Links:    json.RawMessage(`[]`),
	}
	if err := db.Create(&client).Error; err != nil {
		t.Fatalf("seed client: %v", err)
	}
	if err := db.Create(&model.ClusterInbound{DomainID: 1, Domain: "edge.example.com", InboundID: inbound.Id, RequestID: "req-inbound-a"}).Error; err != nil {
		t.Fatalf("seed inbound wrapper: %v", err)
	}
	if err := db.Create(&model.ClusterClient{DomainID: 1, Domain: "edge.example.com", ClientID: client.Id, HubUserUUID: "user-uuid-1", RequestID: "req-user-a"}).Error; err != nil {
		t.Fatalf("seed client wrapper: %v", err)
	}

	clients, err := (&ClientService{}).GetAll()
	if err != nil {
		t.Fatalf("get clients: %v", err)
	}
	if len(*clients) != 1 || !(*clients)[0].ClusterManaged || !(*clients)[0].ClusterReadOnly || (*clients)[0].ClusterDomain != "edge.example.com" {
		t.Fatalf("expected managed client annotation, got %#v", *clients)
	}
	inbounds, err := (&InboundService{}).GetAll()
	if err != nil {
		t.Fatalf("get inbounds: %v", err)
	}
	if len(*inbounds) != 1 || (*inbounds)[0]["cluster_managed"] != true || (*inbounds)[0]["cluster_read_only"] != true {
		t.Fatalf("expected managed inbound annotation, got %#v", *inbounds)
	}
	tlsConfigs, err := (&TlsService{}).GetAll()
	if err != nil {
		t.Fatalf("get tls: %v", err)
	}
	if len(tlsConfigs) != 1 || !tlsConfigs[0].ClusterManaged || !tlsConfigs[0].ClusterReadOnly {
		t.Fatalf("expected managed tls annotation, got %#v", tlsConfigs)
	}

	clientDelete, _ := json.Marshal(client.Id)
	if _, err := (&ClientService{}).Save(db, "del", clientDelete, "edge.example.com"); err == nil || !strings.Contains(err.Error(), "read-only") {
		t.Fatalf("expected managed client delete rejection, got %v", err)
	}
	inboundEdit, _ := inbound.MarshalJSON()
	if err := (&InboundService{}).Save(db, "edit", inboundEdit, "", "edge.example.com"); err == nil || !strings.Contains(err.Error(), "read-only") {
		t.Fatalf("expected managed inbound edit rejection, got %v", err)
	}
	tlsDelete, _ := json.Marshal(tlsConfig.Id)
	if err := (&TlsService{}).Save(db, "del", tlsDelete, "edge.example.com"); err == nil || !strings.Contains(err.Error(), "read-only") {
		t.Fatalf("expected managed tls delete rejection, got %v", err)
	}
	inboundDelete, _ := json.Marshal(inbound.Tag)
	if err := (&InboundService{}).Save(db, "del", inboundDelete, "", "edge.example.com"); err != nil {
		t.Fatalf("expected managed inbound delete to be allowed, got %v", err)
	}
	var wrapperCount int64
	if err := db.Model(&model.ClusterInbound{}).Where("inbound_id = ?", inbound.Id).Count(&wrapperCount).Error; err != nil {
		t.Fatalf("count managed inbound wrapper: %v", err)
	}
	if wrapperCount != 0 {
		t.Fatalf("expected managed inbound wrapper to be removed, got %d", wrapperCount)
	}
	var tlsCount int64
	if err := db.Model(&model.Tls{}).Where("id = ?", tlsConfig.Id).Count(&tlsCount).Error; err != nil {
		t.Fatalf("count managed inbound tls: %v", err)
	}
	if tlsCount != 0 {
		t.Fatalf("expected unused managed inbound tls to be removed, got %d", tlsCount)
	}
}
