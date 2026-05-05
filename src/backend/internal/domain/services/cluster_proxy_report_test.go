package service

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	database "github.com/BeanYa/b-ui/src/backend/internal/infra/db"
	"github.com/BeanYa/b-ui/src/backend/internal/infra/db/model"
)

func TestClusterProxyReportReportsNodeAndWrappedDomainInbounds(t *testing.T) {
	if err := database.InitDB(filepath.Join(t.TempDir(), "proxy-report-domain-inbounds.db")); err != nil {
		if strings.Contains(err.Error(), "go-sqlite3 requires cgo") || strings.Contains(err.Error(), "C compiler") {
			t.Skipf("sqlite test database unavailable in this toolchain: %v", err)
		}
		t.Fatalf("init test db: %v", err)
	}

	db := database.GetDB()
	localOnly := model.Inbound{Type: "vless", Tag: "local-only", Options: json.RawMessage(`{"listen_port":10001}`)}
	domainInbound := model.Inbound{Type: "vless", Tag: "domain-only", Options: json.RawMessage(`{"listen_port":10002,"listen":"::","users":[{"uuid":"x"}]}`)}
	if err := db.Create(&localOnly).Error; err != nil {
		t.Fatalf("seed local inbound: %v", err)
	}
	if err := db.Create(&domainInbound).Error; err != nil {
		t.Fatalf("seed domain inbound: %v", err)
	}
	if err := db.Create(&model.ClusterInbound{
		DomainID:  7,
		Domain:    "edge.example.com",
		NodeID:    "node-a",
		MemberID:  "node-a",
		InboundID: domainInbound.Id,
		RequestID: "req-1",
	}).Error; err != nil {
		t.Fatalf("seed wrapper: %v", err)
	}

	hub := &capturingClusterHubClient{}
	svc := NewClusterProxyReportService(hub, nil, stubClusterDomainProvider{domains: []clusterDomainInfo{{
		ID:          7,
		Name:        "edge.example.com",
		MemberID:    "node-a",
		BaseURL:     "https://node-a.example.com:9443",
		HubURL:      "https://hub.example.com",
		DomainToken: "domain-token",
	}}}, stubClusterNodeIdentity{nodeID: "node-a"})
	if err := svc.reportProxyConfigs(clusterDomainInfo{
		ID:          7,
		Name:        "edge.example.com",
		MemberID:    "node-a",
		BaseURL:     "https://node-a.example.com:9443",
		HubURL:      "https://hub.example.com",
		DomainToken: "domain-token",
	}); err != nil {
		t.Fatalf("report proxy configs: %v", err)
	}

	if len(hub.body.Configs) != 2 {
		t.Fatalf("expected node and domain reported configs, got %#v", hub.body.Configs)
	}
	if hub.body.Configs[0].Tag != "local-only" || hub.body.Configs[0].ListenPort != 10001 {
		t.Fatalf("expected local node config first, got %#v", hub.body.Configs[0])
	}
	if hub.body.Configs[0].Scope != "node" || hub.body.Configs[0].DomainInboundRequestID != "" {
		t.Fatalf("expected node scope metadata, got %#v", hub.body.Configs[0])
	}
	if hub.body.Configs[1].Tag != "domain-only" || hub.body.Configs[1].ListenPort != 10002 {
		t.Fatalf("unexpected reported domain config: %#v", hub.body.Configs[1])
	}
	if hub.body.Configs[1].Scope != "domain" || hub.body.Configs[1].DomainInboundRequestID != "req-1" {
		t.Fatalf("expected domain scope metadata, got %#v", hub.body.Configs[1])
	}
}

func TestClusterProxyReportBuildsNodeScopedConfig(t *testing.T) {
	inbound := model.Inbound{
		Id:      9,
		Type:    "trojan",
		Tag:     "panel-node",
		Options: json.RawMessage(`{"listen":"::","listen_port":10443,"password":"secret","sniff":true}`),
	}

	config := buildClusterProxyReportConfig(inbound, "node.example.com", "node", "")

	if config.InboundID != 9 || config.Type != "trojan" || config.Tag != "panel-node" {
		t.Fatalf("unexpected basic config fields: %#v", config)
	}
	if config.ListenPort != 10443 || config.Address != "node.example.com" {
		t.Fatalf("unexpected endpoint fields: %#v", config)
	}
	if config.Scope != "node" || config.DomainInboundRequestID != "" {
		t.Fatalf("expected node scope without domain request id: %#v", config)
	}
	var options map[string]any
	if err := json.Unmarshal(config.Options, &options); err != nil {
		t.Fatalf("unmarshal options: %v", err)
	}
	if _, ok := options["listen"]; ok {
		t.Fatalf("listen should not be included in report options: %#v", options)
	}
	if _, ok := options["sniff"]; ok {
		t.Fatalf("sniff should not be included in report options: %#v", options)
	}
	if options["password"] != "secret" {
		t.Fatalf("expected protocol options preserved, got %#v", options)
	}
}

type capturingClusterHubClient struct {
	body ClusterHubReportProxyConfigsRequest
}

func (c *capturingClusterHubClient) RegisterNode(context.Context, string, ClusterHubRegisterNodeRequest) (*ClusterHubOperationResponse, error) {
	return nil, nil
}

func (c *capturingClusterHubClient) GetLatestVersion(context.Context, string, string, string) (*ClusterHubVersionResponse, error) {
	return nil, nil
}

func (c *capturingClusterHubClient) GetSnapshot(context.Context, string, string, string) (*ClusterHubSnapshotResponse, error) {
	return nil, nil
}

func (c *capturingClusterHubClient) DeleteMember(context.Context, string, string, string, string) (*ClusterHubOperationResponse, error) {
	return nil, nil
}

func (c *capturingClusterHubClient) ClaimUpdate(context.Context, string, string, string, string, string) (*ClusterHubClaimUpdateResponse, error) {
	return nil, nil
}

func (c *capturingClusterHubClient) SetMemberStatus(context.Context, string, string, string, string, string, string, string) (*ClusterHubMemberStatusResponse, error) {
	return nil, nil
}

func (c *capturingClusterHubClient) ReportProxyConfigs(ctx context.Context, hubURL string, domainID string, body ClusterHubReportProxyConfigsRequest) error {
	c.body = body
	return nil
}

func (c *capturingClusterHubClient) ReportDomainReport(context.Context, string, string, ClusterHubReportRequest) error {
	return nil
}

type stubClusterDomainProvider struct {
	domains []clusterDomainInfo
}

func (s stubClusterDomainProvider) GetAllDomains() ([]clusterDomainInfo, error) {
	return s.domains, nil
}

type stubClusterNodeIdentity struct {
	nodeID string
}

func (s stubClusterNodeIdentity) GetLocalNodeID() string {
	return s.nodeID
}
