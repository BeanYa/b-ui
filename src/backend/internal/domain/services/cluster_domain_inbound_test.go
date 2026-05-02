package service

import (
	"context"
	"encoding/json"
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

func TestDomainInboundBuildTagSanitizesBaseAndNode(t *testing.T) {
	tag := buildClusterInboundTag("edge-", "bad tag!", "node/a", "-prod")
	if tag != "edge-bad-tag-node-a-prod" {
		t.Fatalf("expected sanitized tag, got %q", tag)
	}
}

type domainInboundSaverFunc func(*gorm.DB, string, json.RawMessage, string, string) error

func (fn domainInboundSaverFunc) Save(tx *gorm.DB, act string, data json.RawMessage, initUserIds string, hostname string) error {
	return fn(tx, act, data, initUserIds, hostname)
}

type stubDomainInboundIdentity struct {
	node *model.ClusterLocalNode
}

func (s *stubDomainInboundIdentity) GetOrCreate() (*model.ClusterLocalNode, error) {
	return s.node, nil
}
