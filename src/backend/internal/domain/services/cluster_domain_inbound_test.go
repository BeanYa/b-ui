package service

import (
	"path/filepath"
	"strings"
	"testing"

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
