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
	"gorm.io/gorm"
)

func initClusterInboundTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	if err := database.InitDB(filepath.Join(t.TempDir(), "cluster-domain-inbound.db")); err != nil {
		if strings.Contains(err.Error(), "go-sqlite3 requires cgo") || strings.Contains(err.Error(), "C compiler") {
			t.Skipf("sqlite test database unavailable in this toolchain: %v", err)
		}
		t.Fatalf("init test db: %v", err)
	}
	return database.GetDB()
}

func TestDomainInboundWrapperMigratesAndEnforcesRequestPerDomain(t *testing.T) {
	db := initClusterInboundTestDB(t)
	inbound := model.Inbound{Type: "vless", Tag: "cluster-vless-a"}
	if err := db.Create(&inbound).Error; err != nil {
		t.Fatalf("seed inbound: %v", err)
	}
	first := model.ClusterInbound{
		DomainID:  1,
		Domain:    "edge.example.com",
		NodeID:    "node-a",
		MemberID:  "node-a",
		InboundID: inbound.Id,
		RequestID: "req-1",
		Prefix:    "edge-",
		Suffix:    "-prod",
		Template:  "standard",
	}
	if err := db.Create(&first).Error; err != nil {
		t.Fatalf("create first wrapper: %v", err)
	}
	second := first
	second.Id = 0
	secondInbound := model.Inbound{Type: "vless", Tag: "cluster-vless-b"}
	if err := db.Create(&secondInbound).Error; err != nil {
		t.Fatalf("seed second inbound: %v", err)
	}
	second.InboundID = secondInbound.Id
	if err := db.Create(&second).Error; err == nil {
		t.Fatal("expected duplicate request id in same domain to fail")
	}
}

func TestDomainInboundWrapperEnforcesGroupPerDomainWhenGroupIDPresent(t *testing.T) {
	db := initClusterInboundTestDB(t)
	firstInbound := model.Inbound{Type: "vless", Tag: "cluster-vless-group-a"}
	if err := db.Create(&firstInbound).Error; err != nil {
		t.Fatalf("seed first inbound: %v", err)
	}
	first := model.ClusterInbound{
		DomainID:  1,
		Domain:    "edge.example.com",
		NodeID:    "node-a",
		MemberID:  "node-a",
		GroupID:   "group-1",
		InboundID: firstInbound.Id,
		RequestID: "req-1",
	}
	if err := db.Create(&first).Error; err != nil {
		t.Fatalf("create first wrapper: %v", err)
	}
	secondInbound := model.Inbound{Type: "vless", Tag: "cluster-vless-group-b"}
	if err := db.Create(&secondInbound).Error; err != nil {
		t.Fatalf("seed second inbound: %v", err)
	}
	second := model.ClusterInbound{
		DomainID:  1,
		Domain:    "edge.example.com",
		NodeID:    "node-a",
		MemberID:  "node-a",
		GroupID:   "group-1",
		InboundID: secondInbound.Id,
		RequestID: "req-2",
	}
	if err := db.Create(&second).Error; err == nil {
		t.Fatal("expected duplicate non-empty group id in same domain to fail")
	}
}

func TestDomainInboundWrapperAllowsEmptyGroupIDsPerDomain(t *testing.T) {
	db := initClusterInboundTestDB(t)
	firstInbound := model.Inbound{Type: "vless", Tag: "cluster-vless-empty-group-a"}
	if err := db.Create(&firstInbound).Error; err != nil {
		t.Fatalf("seed first inbound: %v", err)
	}
	first := model.ClusterInbound{
		DomainID:  1,
		Domain:    "edge.example.com",
		NodeID:    "node-a",
		MemberID:  "node-a",
		InboundID: firstInbound.Id,
		RequestID: "req-1",
	}
	if err := db.Create(&first).Error; err != nil {
		t.Fatalf("create first wrapper: %v", err)
	}
	secondInbound := model.Inbound{Type: "vless", Tag: "cluster-vless-empty-group-b"}
	if err := db.Create(&secondInbound).Error; err != nil {
		t.Fatalf("seed second inbound: %v", err)
	}
	second := model.ClusterInbound{
		DomainID:  1,
		Domain:    "edge.example.com",
		NodeID:    "node-a",
		MemberID:  "node-a",
		InboundID: secondInbound.Id,
		RequestID: "req-2",
	}
	if err := db.Create(&second).Error; err != nil {
		t.Fatalf("expected empty group ids to remain legacy-compatible: %v", err)
	}
}

func TestDomainInboundCreateRejectsMissingInbound(t *testing.T) {
	db := initClusterInboundTestDB(t)
	svc := NewClusterDomainInboundService(ClusterDomainInboundServiceOptions{
		DB: db,
		InboundSaver: domainInboundSaverFunc(func(tx *gorm.DB, act string, data json.RawMessage, initUserIds string, hostname string) error {
			t.Fatal("inbound saver should not be called for missing inbound")
			return nil
		}),
		Identity: &stubDomainInboundIdentity{node: &model.ClusterLocalNode{NodeID: "node-a"}},
	})
	_, err := svc.ApplyDomainInboundCreate(context.Background(), &model.ClusterDomain{Id: 1, Domain: "edge.example.com"}, clustertypes.DomainInboundCreatePayload{
		RequestID: "req-missing",
		DomainID:  "edge.example.com",
	}, "hub", false)
	if err == nil || !strings.Contains(err.Error(), "inbound is required") {
		t.Fatalf("expected inbound required error, got %v", err)
	}
}

func TestDomainInboundCreateGeneratesPortAndCreatesWrapper(t *testing.T) {
	db := initClusterInboundTestDB(t)
	svc := NewClusterDomainInboundService(ClusterDomainInboundServiceOptions{
		DB: db,
		InboundSaver: domainInboundSaverFunc(func(tx *gorm.DB, act string, data json.RawMessage, initUserIds string, hostname string) error {
			var inbound model.Inbound
			if err := inbound.UnmarshalJSON(data); err != nil {
				return err
			}
			return tx.Create(&inbound).Error
		}),
		Identity:      &stubDomainInboundIdentity{node: &model.ClusterLocalNode{NodeID: "node-a"}},
		PortAllocator: func() (int, error) { return 32001, nil },
		Now:           func() int64 { return 1700000000 },
	})
	result, err := svc.ApplyDomainInboundCreate(context.Background(), &model.ClusterDomain{Id: 1, Domain: "edge.example.com"}, clustertypes.DomainInboundCreatePayload{
		RequestID: "req-1",
		DomainID:  "edge.example.com",
		Prefix:    "edge-",
		Suffix:    "-prod",
		Inbound:   json.RawMessage(`{"type":"vless","tag":"main","listen":"::","users":[]}`),
	}, "hub", false)
	if err != nil {
		t.Fatalf("apply create: %v", err)
	}
	if result.InboundID == 0 || result.Created != true {
		t.Fatalf("unexpected result: %#v", result)
	}
	var inbound model.Inbound
	if err := db.First(&inbound, result.InboundID).Error; err != nil {
		t.Fatalf("load inbound: %v", err)
	}
	if inbound.Tag != "edge-main-node-a-prod" {
		t.Fatalf("expected normalized tag, got %q", inbound.Tag)
	}
	var options map[string]interface{}
	if err := json.Unmarshal(inbound.Options, &options); err != nil {
		t.Fatalf("decode options: %v", err)
	}
	if options["listen_port"] != float64(32001) {
		t.Fatalf("expected listen_port 32001, got %#v", options["listen_port"])
	}
	var wrapper model.ClusterInbound
	if err := db.Where("domain_id = ? AND request_id = ?", 1, "req-1").First(&wrapper).Error; err != nil {
		t.Fatalf("load wrapper: %v", err)
	}
	if wrapper.InboundID != inbound.Id || wrapper.Prefix != "edge-" || wrapper.Suffix != "-prod" {
		t.Fatalf("unexpected wrapper: %#v", wrapper)
	}
}

func TestDomainInboundCreateUsesGroupSeedDisplayNameAndIsIdempotent(t *testing.T) {
	db := initClusterInboundTestDB(t)
	svc := NewClusterDomainInboundService(ClusterDomainInboundServiceOptions{
		DB: db,
		InboundSaver: domainInboundSaverFunc(func(tx *gorm.DB, act string, data json.RawMessage, initUserIds string, hostname string) error {
			var inbound model.Inbound
			if err := inbound.UnmarshalJSON(data); err != nil {
				return err
			}
			return tx.Create(&inbound).Error
		}),
		Identity:      &stubDomainInboundIdentity{node: &model.ClusterLocalNode{NodeID: "node-a"}},
		PortAllocator: func() (int, error) { return 32005, nil },
		Now:           func() int64 { return 1700000000 },
	})
	payload := clustertypes.DomainInboundCreatePayload{
		RequestID: "req-group-1",
		DomainID:  "edge.example.com",
		GroupID:   "group-1",
		TagSeed:   "main",
		Prefix:    "edge",
		Suffix:    "prod",
		Inbound:   json.RawMessage(`{"type":"vless","tag":"legacy"}`),
		TargetMembers: []clustertypes.DomainInboundTarget{
			{MemberID: "member-a", NodeID: "node-a", DisplayName: "de"},
		},
	}

	first, err := svc.ApplyDomainInboundCreate(context.Background(), &model.ClusterDomain{Id: 1, Domain: "edge.example.com"}, payload, "hub", false)
	if err != nil {
		t.Fatalf("apply first: %v", err)
	}
	if !first.Created || first.InboundID == 0 {
		t.Fatalf("unexpected first result: %#v", first)
	}
	var inbound model.Inbound
	if err := db.First(&inbound, first.InboundID).Error; err != nil {
		t.Fatalf("load inbound: %v", err)
	}
	if inbound.Tag != "main-edge-de-prod" {
		t.Fatalf("expected display-name tag, got %q", inbound.Tag)
	}
	var wrapper model.ClusterInbound
	if err := db.Where("domain_id = ? AND group_id = ?", 1, "group-1").First(&wrapper).Error; err != nil {
		t.Fatalf("load wrapper: %v", err)
	}
	if wrapper.GroupID != "group-1" || wrapper.InboundID != first.InboundID {
		t.Fatalf("unexpected wrapper: %#v", wrapper)
	}

	payload.RequestID = "req-group-2"
	second, err := svc.ApplyDomainInboundCreate(context.Background(), &model.ClusterDomain{Id: 1, Domain: "edge.example.com"}, payload, "hub", false)
	if err != nil {
		t.Fatalf("apply second: %v", err)
	}
	if second.InboundID != first.InboundID || second.RequestID != "req-group-1" || second.Created {
		t.Fatalf("expected existing grouped inbound, first=%#v second=%#v", first, second)
	}
}

func TestDomainInboundCreateMaterializesStandardTLSWithGeneratedCertificate(t *testing.T) {
	db := initClusterInboundTestDB(t)
	svc := NewClusterDomainInboundService(ClusterDomainInboundServiceOptions{
		DB: db,
		InboundSaver: domainInboundSaverFunc(func(tx *gorm.DB, act string, data json.RawMessage, initUserIds string, hostname string) error {
			var inbound model.Inbound
			if err := inbound.UnmarshalJSON(data); err != nil {
				return err
			}
			return tx.Create(&inbound).Error
		}),
		Identity:      &stubDomainInboundIdentity{node: &model.ClusterLocalNode{NodeID: "node-a"}},
		PortAllocator: func() (int, error) { return 32003, nil },
		Now:           func() int64 { return 1700000000 },
		TLSKeypairGenerator: func(serverName string) []string {
			if serverName != "edge.example.com" {
				t.Fatalf("expected server name edge.example.com, got %q", serverName)
			}
			return []string{
				"-----BEGIN PRIVATE KEY-----",
				"private-line",
				"-----END PRIVATE KEY-----",
				"-----BEGIN CERTIFICATE-----",
				"cert-line",
				"-----END CERTIFICATE-----",
			}
		},
	})

	result, err := svc.ApplyDomainInboundCreate(context.Background(), &model.ClusterDomain{Id: 1, Domain: "edge.example.com"}, clustertypes.DomainInboundCreatePayload{
		RequestID:   "req-tls",
		DomainID:    "edge.example.com",
		Inbound:     json.RawMessage(`{"type":"vless","tag":"main"}`),
		TLSTemplate: "standard",
		TLS: &clustertypes.DomainInboundTLS{
			Name:   "edge-main-tls",
			Server: json.RawMessage(`{"enabled":true,"server_name":"edge.example.com","alpn":["h2","http/1.1"],"certificate_path":"","key_path":""}`),
			Client: json.RawMessage(`{"insecure":true}`),
		},
	}, "hub", false)
	if err != nil {
		t.Fatalf("apply create: %v", err)
	}

	var inbound model.Inbound
	if err := db.First(&inbound, result.InboundID).Error; err != nil {
		t.Fatalf("load inbound: %v", err)
	}
	if inbound.TlsId == 0 {
		t.Fatal("expected generated tls row to be bound to inbound")
	}

	var tlsConfig model.Tls
	if err := db.First(&tlsConfig, inbound.TlsId).Error; err != nil {
		t.Fatalf("load tls: %v", err)
	}
	var server map[string]interface{}
	if err := json.Unmarshal(tlsConfig.Server, &server); err != nil {
		t.Fatalf("decode tls server: %v", err)
	}
	if _, ok := server["certificate_path"]; ok {
		t.Fatalf("expected generated certificate text instead of certificate_path, got %#v", server)
	}
	if _, ok := server["key_path"]; ok {
		t.Fatalf("expected generated key text instead of key_path, got %#v", server)
	}
	if cert, ok := server["certificate"].([]interface{}); !ok || len(cert) == 0 {
		t.Fatalf("expected generated certificate lines, got %#v", server["certificate"])
	}
	if key, ok := server["key"].([]interface{}); !ok || len(key) == 0 {
		t.Fatalf("expected generated key lines, got %#v", server["key"])
	}
}

func TestDomainInboundGeneratedTLSKeypairUsesInlineCertificateMaterial(t *testing.T) {
	svc := NewClusterDomainInboundService(ClusterDomainInboundServiceOptions{
		TLSKeypairGenerator: func(serverName string) []string {
			if serverName != "''" {
				t.Fatalf("expected blank server name to be normalized, got %q", serverName)
			}
			return []string{
				"-----BEGIN PRIVATE KEY-----",
				"private-line",
				"-----END PRIVATE KEY-----",
				"-----BEGIN CERTIFICATE-----",
				"cert-line",
				"-----END CERTIFICATE-----",
			}
		},
	})
	server := map[string]interface{}{
		"enabled":          true,
		"server_name":      "",
		"certificate_path": "",
		"key_path":         "",
	}

	if err := svc.ensureGeneratedTLSKeypair(server); err != nil {
		t.Fatalf("generate tls keypair: %v", err)
	}
	if _, ok := server["certificate_path"]; ok {
		t.Fatalf("expected certificate_path to be removed, got %#v", server)
	}
	if _, ok := server["key_path"]; ok {
		t.Fatalf("expected key_path to be removed, got %#v", server)
	}
	if cert, ok := server["certificate"].([]string); !ok || len(cert) == 0 {
		t.Fatalf("expected certificate lines, got %#v", server["certificate"])
	}
	if key, ok := server["key"].([]string); !ok || len(key) == 0 {
		t.Fatalf("expected key lines, got %#v", server["key"])
	}
}

func TestDomainInboundGeneratedTLSKeypairReplacesLocalProvidedMarkers(t *testing.T) {
	svc := NewClusterDomainInboundService(ClusterDomainInboundServiceOptions{
		TLSKeypairGenerator: func(serverName string) []string {
			if serverName != "edge.example.com" {
				t.Fatalf("expected server name edge.example.com, got %q", serverName)
			}
			return []string{
				"-----BEGIN PRIVATE KEY-----",
				"private-line",
				"-----END PRIVATE KEY-----",
				"-----BEGIN CERTIFICATE-----",
				"cert-line",
				"-----END CERTIFICATE-----",
			}
		},
	})
	server := map[string]interface{}{
		"enabled":     true,
		"server_name": "edge.example.com",
		"certificate": map[string]interface{}{"LocalProvided": "GeneratedTLSCertificate"},
		"key":         map[string]interface{}{"LocalProvided": "GeneratedTLSKey"},
	}

	if err := svc.ensureGeneratedTLSKeypair(server); err != nil {
		t.Fatalf("generate tls keypair: %v", err)
	}
	if cert, ok := server["certificate"].([]string); !ok || len(cert) == 0 {
		t.Fatalf("expected certificate lines, got %#v", server["certificate"])
	}
	if key, ok := server["key"].([]string); !ok || len(key) == 0 {
		t.Fatalf("expected key lines, got %#v", server["key"])
	}
}

func TestDomainInboundTLSLocalProvidedPanelCertificateUsesTargetSettings(t *testing.T) {
	svc := NewClusterDomainInboundService(ClusterDomainInboundServiceOptions{
		PanelCertificateProvider: func() (DomainInboundPanelCertificateSettings, error) {
			return DomainInboundPanelCertificateSettings{
				WebDomain:   "target-panel.example.com",
				WebCertFile: "/target/fullchain.pem",
				WebKeyFile:  "/target/privkey.pem",
			}, nil
		},
	})
	server := map[string]interface{}{
		"enabled":          true,
		"server_name":      map[string]interface{}{"LocalProvided": "PanelWebDomain"},
		"certificate_path": map[string]interface{}{"LocalProvided": "PanelWebCertFile"},
		"key_path":         map[string]interface{}{"LocalProvided": "PanelWebKeyFile"},
	}
	client := map[string]interface{}{}

	if err := svc.resolveDomainInboundTLSLocalProvided(&model.ClusterDomain{Domain: "edge.example.com"}, server, client); err != nil {
		t.Fatalf("resolve local provided tls: %v", err)
	}
	if server["server_name"] != "target-panel.example.com" {
		t.Fatalf("expected target panel domain, got %#v", server["server_name"])
	}
	if server["certificate_path"] != "/target/fullchain.pem" {
		t.Fatalf("expected target cert path, got %#v", server["certificate_path"])
	}
	if server["key_path"] != "/target/privkey.pem" {
		t.Fatalf("expected target key path, got %#v", server["key_path"])
	}
}

func TestDomainInboundTLSLocalProvidedRealityKeypairGeneratesClientPublicKey(t *testing.T) {
	svc := NewClusterDomainInboundService(ClusterDomainInboundServiceOptions{})
	server := map[string]interface{}{
		"enabled":     true,
		"server_name": "www.youtube.com",
		"reality": map[string]interface{}{
			"enabled":     true,
			"private_key": map[string]interface{}{"LocalProvided": "RealityPrivateKey"},
			"short_id":    "0123abcd",
		},
	}
	client := map[string]interface{}{
		"reality": map[string]interface{}{
			"enabled":    true,
			"public_key": map[string]interface{}{"LocalProvided": "RealityPublicKey"},
			"short_id":   "0123abcd",
		},
	}

	if err := svc.resolveDomainInboundTLSLocalProvided(&model.ClusterDomain{Domain: "edge.example.com"}, server, client); err != nil {
		t.Fatalf("resolve local provided reality: %v", err)
	}
	if err := ensureRealityKeypair(server, client, "edge-main-node-a", 1700000000); err != nil {
		t.Fatalf("generate reality keypair: %v", err)
	}
	serverReality := server["reality"].(map[string]interface{})
	clientReality := client["reality"].(map[string]interface{})
	if strings.TrimSpace(stringValue(serverReality["private_key"])) == "" {
		t.Fatalf("expected generated private key, got %#v", serverReality["private_key"])
	}
	if strings.TrimSpace(stringValue(clientReality["public_key"])) == "" {
		t.Fatalf("expected generated public key, got %#v", clientReality["public_key"])
	}
	if clientReality["short_id"] != "0123abcd" {
		t.Fatalf("expected short id to be preserved, got %#v", clientReality["short_id"])
	}
}

func TestDomainInboundCreateDuplicateRequestReturnsExistingInbound(t *testing.T) {
	db := initClusterInboundTestDB(t)
	saveCalls := 0
	svc := NewClusterDomainInboundService(ClusterDomainInboundServiceOptions{
		DB: db,
		InboundSaver: domainInboundSaverFunc(func(tx *gorm.DB, act string, data json.RawMessage, initUserIds string, hostname string) error {
			saveCalls++
			var inbound model.Inbound
			if err := inbound.UnmarshalJSON(data); err != nil {
				return err
			}
			return tx.Create(&inbound).Error
		}),
		Identity:      &stubDomainInboundIdentity{node: &model.ClusterLocalNode{NodeID: "node-a"}},
		PortAllocator: func() (int, error) { return 32002, nil },
		Now:           func() int64 { return 1700000000 },
	})
	payload := clustertypes.DomainInboundCreatePayload{
		RequestID: "req-dupe",
		DomainID:  "edge.example.com",
		Inbound:   json.RawMessage(`{"type":"vless","tag":"main"}`),
	}
	first, err := svc.ApplyDomainInboundCreate(context.Background(), &model.ClusterDomain{Id: 1, Domain: "edge.example.com"}, payload, "hub", false)
	if err != nil {
		t.Fatalf("apply first: %v", err)
	}
	second, err := svc.ApplyDomainInboundCreate(context.Background(), &model.ClusterDomain{Id: 1, Domain: "edge.example.com"}, payload, "hub", false)
	if err != nil {
		t.Fatalf("apply second: %v", err)
	}
	if first.InboundID != second.InboundID || second.Created {
		t.Fatalf("expected existing inbound on duplicate, first=%#v second=%#v", first, second)
	}
	if saveCalls != 1 {
		t.Fatalf("expected one save call, got %d", saveCalls)
	}
}

func TestDomainInboundCreateKeepsLocalResultWhenPeerBroadcastFails(t *testing.T) {
	db := initClusterInboundTestDB(t)
	svc := NewClusterDomainInboundService(ClusterDomainInboundServiceOptions{
		DB: db,
		InboundSaver: domainInboundSaverFunc(func(tx *gorm.DB, act string, data json.RawMessage, initUserIds string, hostname string) error {
			var inbound model.Inbound
			if err := inbound.UnmarshalJSON(data); err != nil {
				return err
			}
			return tx.Create(&inbound).Error
		}),
		Identity: &stubDomainInboundIdentity{node: &model.ClusterLocalNode{NodeID: "node-a"}},
		Broadcaster: domainInboundBroadcasterFunc(func(context.Context, *model.ClusterDomain, clustertypes.DomainInboundCreatePayload) error {
			return errors.New("cluster peer notify failed: 401 Unauthorized")
		}),
		PortAllocator: func() (int, error) { return 32004, nil },
		Now:           func() int64 { return 1700000000 },
	})

	result, err := svc.ApplyDomainInboundCreate(context.Background(), &model.ClusterDomain{Id: 1, Domain: "edge.example.com"}, clustertypes.DomainInboundCreatePayload{
		RequestID: "req-broadcast-fail",
		DomainID:  "edge.example.com",
		Inbound:   json.RawMessage(`{"type":"vless","tag":"main"}`),
	}, "hub", true)
	if err != nil {
		t.Fatalf("expected local create result despite peer broadcast failure, got %v", err)
	}
	if result.InboundID == 0 || !result.Created {
		t.Fatalf("unexpected result: %#v", result)
	}
	var wrapper model.ClusterInbound
	if err := db.Where("domain_id = ? AND request_id = ?", 1, "req-broadcast-fail").First(&wrapper).Error; err != nil {
		t.Fatalf("load wrapper: %v", err)
	}
}

func TestDomainInboundUpdateMissingGroupReturnsClearError(t *testing.T) {
	db := initClusterInboundTestDB(t)
	svc := NewClusterDomainInboundService(ClusterDomainInboundServiceOptions{
		DB:       db,
		Identity: &stubDomainInboundIdentity{node: &model.ClusterLocalNode{NodeID: "node-a"}},
	})

	_, err := svc.ApplyDomainInboundUpdate(context.Background(), &model.ClusterDomain{Id: 1, Domain: "edge.example.com"}, clustertypes.DomainInboundUpdatePayload{
		RequestID: "req-update-missing",
		DomainID:  "edge.example.com",
		GroupID:   "group-missing",
		Inbound:   json.RawMessage(`{"type":"vless","tag":"main"}`),
	}, "hub", false)
	if err == nil || !strings.Contains(err.Error(), "domain inbound group") || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected clear missing group error, got %v", err)
	}
}

func TestDomainInboundUpdateReplacesExistingInboundAndTLS(t *testing.T) {
	db := initClusterInboundTestDB(t)
	originalTLS := model.Tls{Name: "old-tls", Server: json.RawMessage(`{"enabled":true}`), Client: json.RawMessage(`{}`)}
	if err := db.Create(&originalTLS).Error; err != nil {
		t.Fatalf("seed original tls: %v", err)
	}
	inbound := model.Inbound{Type: "vless", Tag: "old-tag", TlsId: originalTLS.Id, Options: json.RawMessage(`{"listen":"::","listen_port":32000}`)}
	if err := db.Create(&inbound).Error; err != nil {
		t.Fatalf("seed inbound: %v", err)
	}
	wrapper := model.ClusterInbound{
		DomainID:  1,
		Domain:    "edge.example.com",
		NodeID:    "node-a",
		MemberID:  "node-a",
		GroupID:   "group-1",
		InboundID: inbound.Id,
		RequestID: "req-create",
		Prefix:    "old",
		Suffix:    "old",
		Template:  "standard",
	}
	if err := db.Create(&wrapper).Error; err != nil {
		t.Fatalf("seed wrapper: %v", err)
	}
	svc := NewClusterDomainInboundService(ClusterDomainInboundServiceOptions{
		DB:            db,
		Identity:      &stubDomainInboundIdentity{node: &model.ClusterLocalNode{NodeID: "node-a"}},
		PortAllocator: func() (int, error) { return 32010, nil },
		Now:           func() int64 { return 1700000001 },
	})

	result, err := svc.ApplyDomainInboundUpdate(context.Background(), &model.ClusterDomain{Id: 1, Domain: "edge.example.com"}, clustertypes.DomainInboundUpdatePayload{
		RequestID:   "req-update",
		DomainID:    "edge.example.com",
		GroupID:     "group-1",
		TagSeed:     "main",
		Prefix:      "edge",
		Suffix:      "prod",
		Inbound:     json.RawMessage(`{"type":"trojan","tag":"legacy","listen":"127.0.0.1","tcp_fast_open":true}`),
		TLSTemplate: "standard",
		TLS: &clustertypes.DomainInboundTLS{
			Name:   "new-tls",
			Server: json.RawMessage(`{"enabled":true,"server_name":"edge.example.com","certificate":["cert"],"key":["key"]}`),
			Client: json.RawMessage(`{"insecure":false}`),
		},
		TargetMembers: []clustertypes.DomainInboundTarget{{NodeID: "node-a", DisplayName: "de"}},
	}, "hub", false)
	if err != nil {
		t.Fatalf("apply update: %v", err)
	}
	if result.InboundID != inbound.Id || result.RequestID != "req-update" || result.Created {
		t.Fatalf("unexpected result: %#v", result)
	}
	var updated model.Inbound
	if err := db.First(&updated, inbound.Id).Error; err != nil {
		t.Fatalf("load updated inbound: %v", err)
	}
	if updated.Type != "trojan" || updated.Tag != "main-edge-de-prod" || updated.TlsId == 0 || updated.TlsId == originalTLS.Id {
		t.Fatalf("expected updated inbound type/tag/tls, got %#v", updated)
	}
	var options map[string]interface{}
	if err := json.Unmarshal(updated.Options, &options); err != nil {
		t.Fatalf("decode updated options: %v", err)
	}
	if options["listen"] != "127.0.0.1" || options["listen_port"] != float64(32010) || options["tcp_fast_open"] != true {
		t.Fatalf("expected editable inbound options to be replaced, got %#v", options)
	}
	var updatedWrapper model.ClusterInbound
	if err := db.First(&updatedWrapper, wrapper.Id).Error; err != nil {
		t.Fatalf("load wrapper: %v", err)
	}
	if updatedWrapper.RequestID != "req-update" || updatedWrapper.Prefix != "edge" || updatedWrapper.Suffix != "prod" || updatedWrapper.Template != "standard" || updatedWrapper.MemberID != "hub" {
		t.Fatalf("expected wrapper metadata update, got %#v", updatedWrapper)
	}
	var oldTLSCount int64
	if err := db.Model(model.Tls{}).Where("id = ?", originalTLS.Id).Count(&oldTLSCount).Error; err != nil {
		t.Fatalf("count old tls: %v", err)
	}
	if oldTLSCount != 0 {
		t.Fatalf("expected old tls to be deleted, count=%d", oldTLSCount)
	}
}

func TestDomainInboundDeleteRemovesExistingWrapperAndInbound(t *testing.T) {
	db := initClusterInboundTestDB(t)
	tlsConfig := model.Tls{Name: "delete-tls", Server: json.RawMessage(`{}`), Client: json.RawMessage(`{}`)}
	if err := db.Create(&tlsConfig).Error; err != nil {
		t.Fatalf("seed tls: %v", err)
	}
	inbound := model.Inbound{Type: "vless", Tag: "delete-tag", TlsId: tlsConfig.Id}
	if err := db.Create(&inbound).Error; err != nil {
		t.Fatalf("seed inbound: %v", err)
	}
	wrapper := model.ClusterInbound{DomainID: 1, Domain: "edge.example.com", NodeID: "node-a", MemberID: "node-a", GroupID: "group-delete", InboundID: inbound.Id, RequestID: "req-create"}
	if err := db.Create(&wrapper).Error; err != nil {
		t.Fatalf("seed wrapper: %v", err)
	}
	svc := NewClusterDomainInboundService(ClusterDomainInboundServiceOptions{DB: db})

	if err := svc.ApplyDomainInboundDelete(context.Background(), &model.ClusterDomain{Id: 1, Domain: "edge.example.com"}, clustertypes.DomainInboundDeletePayload{
		RequestID: "req-delete",
		DomainID:  "edge.example.com",
		GroupID:   "group-delete",
	}, "hub", false); err != nil {
		t.Fatalf("apply delete: %v", err)
	}
	var wrapperCount int64
	if err := db.Model(model.ClusterInbound{}).Where("id = ?", wrapper.Id).Count(&wrapperCount).Error; err != nil {
		t.Fatalf("count wrapper: %v", err)
	}
	if wrapperCount != 0 {
		t.Fatalf("expected wrapper deleted, count=%d", wrapperCount)
	}
	var inboundCount int64
	if err := db.Model(model.Inbound{}).Where("id = ?", inbound.Id).Count(&inboundCount).Error; err != nil {
		t.Fatalf("count inbound: %v", err)
	}
	if inboundCount != 0 {
		t.Fatalf("expected inbound deleted, count=%d", inboundCount)
	}
	var tlsCount int64
	if err := db.Model(model.Tls{}).Where("id = ?", tlsConfig.Id).Count(&tlsCount).Error; err != nil {
		t.Fatalf("count tls: %v", err)
	}
	if tlsCount != 0 {
		t.Fatalf("expected unused tls deleted, count=%d", tlsCount)
	}
}

func TestDomainInboundDeleteKeepsLocalDeleteWhenPeerBroadcastFails(t *testing.T) {
	db := initClusterInboundTestDB(t)
	inbound := model.Inbound{Type: "vless", Tag: "delete-broadcast-tag"}
	if err := db.Create(&inbound).Error; err != nil {
		t.Fatalf("seed inbound: %v", err)
	}
	wrapper := model.ClusterInbound{DomainID: 1, Domain: "edge.example.com", NodeID: "node-a", MemberID: "node-a", GroupID: "group-broadcast", InboundID: inbound.Id, RequestID: "req-create"}
	if err := db.Create(&wrapper).Error; err != nil {
		t.Fatalf("seed wrapper: %v", err)
	}
	svc := NewClusterDomainInboundService(ClusterDomainInboundServiceOptions{
		DB:          db,
		Broadcaster: stubDomainInboundBroadcaster{deleteErr: errors.New("cluster peer notify failed: 401 Unauthorized")},
	})

	if err := svc.ApplyDomainInboundDelete(context.Background(), &model.ClusterDomain{Id: 1, Domain: "edge.example.com"}, clustertypes.DomainInboundDeletePayload{
		RequestID: "req-delete-broadcast",
		DomainID:  "edge.example.com",
		GroupID:   "group-broadcast",
	}, "hub", true); err != nil {
		t.Fatalf("expected local delete despite broadcast failure, got %v", err)
	}
	var wrapperCount int64
	if err := db.Model(model.ClusterInbound{}).Where("id = ?", wrapper.Id).Count(&wrapperCount).Error; err != nil {
		t.Fatalf("count wrapper: %v", err)
	}
	if wrapperCount != 0 {
		t.Fatalf("expected wrapper deleted, count=%d", wrapperCount)
	}
	var inboundCount int64
	if err := db.Model(model.Inbound{}).Where("id = ?", inbound.Id).Count(&inboundCount).Error; err != nil {
		t.Fatalf("count inbound: %v", err)
	}
	if inboundCount != 0 {
		t.Fatalf("expected inbound deleted, count=%d", inboundCount)
	}
}

func TestDomainInboundDeleteMissingGroupIsIdempotent(t *testing.T) {
	db := initClusterInboundTestDB(t)
	svc := NewClusterDomainInboundService(ClusterDomainInboundServiceOptions{
		DB:       db,
		Identity: &stubDomainInboundIdentity{node: &model.ClusterLocalNode{NodeID: "node-a"}},
	})

	if err := svc.ApplyDomainInboundDelete(context.Background(), &model.ClusterDomain{Id: 1, Domain: "edge.example.com"}, clustertypes.DomainInboundDeletePayload{
		RequestID: "req-delete-missing",
		DomainID:  "edge.example.com",
		GroupID:   "group-missing",
	}, "hub", true); err != nil {
		t.Fatalf("expected missing delete to be idempotent, got %v", err)
	}
}

func TestDomainInboundBuildTagSanitizesBaseAndNode(t *testing.T) {
	tag := buildLegacyClusterInboundTag("edge-", "bad tag!", "node/a", "-prod")
	if tag != "edge-bad-tag-node-a-prod" {
		t.Fatalf("expected sanitized tag, got %q", tag)
	}
}

func TestDomainInboundBuildLegacyTagPreservesPrefixBaseNodeSuffix(t *testing.T) {
	tag := buildLegacyClusterInboundTag("edge-", "main", "node-a", "-prod")
	if tag != "edge-main-node-a-prod" {
		t.Fatalf("expected edge-main-node-a-prod, got %q", tag)
	}
}

func TestDomainInboundBuildTagUsesSeedPrefixDisplayNameSuffix(t *testing.T) {
	tag := buildClusterInboundTag("main", "edge", "de", "prod")
	if tag != "main-edge-de-prod" {
		t.Fatalf("expected main-edge-de-prod, got %q", tag)
	}
	tag = buildClusterInboundTag("main", "", "de", "")
	if tag != "main-de" {
		t.Fatalf("expected main-de, got %q", tag)
	}
}

func TestDomainInboundBuildTagOmitsEmptySanitizedOptionalParts(t *testing.T) {
	tag := buildClusterInboundTag("main", "!!!", "de", "---")
	if tag != "main-de" {
		t.Fatalf("expected main-de, got %q", tag)
	}
}

func TestDomainInboundLocalDisplayNameFallsBackToNodeIDWhenDisplayNameSanitizesEmpty(t *testing.T) {
	displayName := domainInboundLocalDisplayName([]clustertypes.DomainInboundTarget{
		{MemberID: "member-a", NodeID: "node-a", DisplayName: "!!!"},
	}, "node-a")
	if displayName != "node-a" {
		t.Fatalf("expected node-a fallback, got %q", displayName)
	}
	tag := buildClusterInboundTag("main", "edge", displayName, "prod")
	if tag != "main-edge-node-a-prod" {
		t.Fatalf("expected node fallback tag, got %q", tag)
	}
}

func TestDomainInboundLocalDisplayNameDoesNotMatchEmptyNodeID(t *testing.T) {
	displayName := domainInboundLocalDisplayName([]clustertypes.DomainInboundTarget{
		{MemberID: "member-empty", NodeID: "", DisplayName: "de"},
	}, "")
	if displayName != "" {
		t.Fatalf("expected empty display name for empty local node id, got %q", displayName)
	}
}

func TestDomainInboundLocalDisplayNameUsesNodeIDForMissingTarget(t *testing.T) {
	displayName := domainInboundLocalDisplayName([]clustertypes.DomainInboundTarget{
		{MemberID: "member-b", NodeID: "node-b", DisplayName: "fr"},
	}, "node-a")
	if displayName != "node-a" {
		t.Fatalf("expected node-a fallback, got %q", displayName)
	}
}

type domainInboundSaverFunc func(*gorm.DB, string, json.RawMessage, string, string) error

func (fn domainInboundSaverFunc) Save(tx *gorm.DB, act string, data json.RawMessage, initUserIds string, hostname string) error {
	return fn(tx, act, data, initUserIds, hostname)
}

type domainInboundBroadcasterFunc func(context.Context, *model.ClusterDomain, clustertypes.DomainInboundCreatePayload) error

func (fn domainInboundBroadcasterFunc) BroadcastDomainInboundCreate(ctx context.Context, domain *model.ClusterDomain, payload clustertypes.DomainInboundCreatePayload) error {
	return fn(ctx, domain, payload)
}

func (fn domainInboundBroadcasterFunc) BroadcastDomainInboundUpdate(ctx context.Context, domain *model.ClusterDomain, payload clustertypes.DomainInboundUpdatePayload) error {
	return nil
}

func (fn domainInboundBroadcasterFunc) BroadcastDomainInboundDelete(ctx context.Context, domain *model.ClusterDomain, payload clustertypes.DomainInboundDeletePayload) error {
	return nil
}

type stubDomainInboundBroadcaster struct {
	createErr error
	updateErr error
	deleteErr error
}

func (s stubDomainInboundBroadcaster) BroadcastDomainInboundCreate(context.Context, *model.ClusterDomain, clustertypes.DomainInboundCreatePayload) error {
	return s.createErr
}

func (s stubDomainInboundBroadcaster) BroadcastDomainInboundUpdate(context.Context, *model.ClusterDomain, clustertypes.DomainInboundUpdatePayload) error {
	return s.updateErr
}

func (s stubDomainInboundBroadcaster) BroadcastDomainInboundDelete(context.Context, *model.ClusterDomain, clustertypes.DomainInboundDeletePayload) error {
	return s.deleteErr
}

type stubDomainInboundIdentity struct {
	node *model.ClusterLocalNode
}

func (s *stubDomainInboundIdentity) GetOrCreate() (*model.ClusterLocalNode, error) {
	return s.node, nil
}
