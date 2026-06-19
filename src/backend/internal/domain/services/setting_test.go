package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	database "github.com/BeanYa/b-ui/src/backend/internal/infra/db"
	"github.com/BeanYa/b-ui/src/backend/internal/infra/db/model"
)

func TestSettingServiceExposesTerminalIdleTimeoutGetter(t *testing.T) {
	service := &SettingService{}
	getter := reflect.ValueOf(service).MethodByName("GetWebTerminalIdleTimeout")
	if !getter.IsValid() {
		t.Fatal("expected terminal idle timeout getter to exist")
	}
	if defaultValueMap["webTerminalIdleTimeout"] != "300" {
		t.Fatalf("expected terminal idle timeout default to be 300 seconds, got %q", defaultValueMap["webTerminalIdleTimeout"])
	}
	if _, exists := defaultValueMap["webSSHIdleTimeout"]; exists {
		t.Fatal("expected legacy webssh idle timeout key to be removed from defaults")
	}
}

func TestResolveSubTLSFilesUsesPanelTLSWhenSubscriptionTLSIsBlank(t *testing.T) {
	certFile, keyFile := resolveSubTLSFiles("", "", "/tmp/panel.crt", "/tmp/panel.key")

	if certFile != "/tmp/panel.crt" || keyFile != "/tmp/panel.key" {
		t.Fatalf("expected linked panel TLS files, got cert=%q key=%q", certFile, keyFile)
	}
}

func TestResolveSubTLSFilesUsesCustomTLSOnlyWhenBothSubscriptionPathsAreSet(t *testing.T) {
	certFile, keyFile := resolveSubTLSFiles("/tmp/sub.crt", "/tmp/sub.key", "/tmp/panel.crt", "/tmp/panel.key")

	if certFile != "/tmp/sub.crt" || keyFile != "/tmp/sub.key" {
		t.Fatalf("expected custom subscription TLS files, got cert=%q key=%q", certFile, keyFile)
	}

	certFile, keyFile = resolveSubTLSFiles("/tmp/sub.crt", "", "/tmp/panel.crt", "/tmp/panel.key")
	if certFile != "/tmp/panel.crt" || keyFile != "/tmp/panel.key" {
		t.Fatalf("expected incomplete custom TLS to keep linked panel files, got cert=%q key=%q", certFile, keyFile)
	}
}

func TestGetTimeLocationUsesPanelSetting(t *testing.T) {
	if err := database.InitDB(filepath.Join(t.TempDir(), "setting-time-location.db")); err != nil {
		if strings.Contains(err.Error(), "go-sqlite3 requires cgo") || strings.Contains(err.Error(), "C compiler") {
			t.Skipf("sqlite test database unavailable in this toolchain: %v", err)
		}
		t.Fatalf("init db: %v", err)
	}

	service := &SettingService{}
	if _, err := service.GetAllSetting(); err != nil {
		t.Fatalf("init default settings: %v", err)
	}
	if err := database.GetDB().Model(model.Setting{}).Where("key = ?", "timeLocation").Update("value", "Asia/Tokyo").Error; err != nil {
		t.Fatalf("save panel time location: %v", err)
	}

	loc, err := service.GetTimeLocation()
	if err != nil {
		t.Fatalf("get time location: %v", err)
	}
	if got := loc.String(); got != "Asia/Tokyo" {
		t.Fatalf("expected panel time location Asia/Tokyo, got %q", got)
	}
}

func TestFetchRegionPersistsResolvedRegion(t *testing.T) {
	if err := database.InitDB(filepath.Join(t.TempDir(), "setting-fetch-region.db")); err != nil {
		if strings.Contains(err.Error(), "go-sqlite3 requires cgo") || strings.Contains(err.Error(), "C compiler") {
			t.Skipf("sqlite test database unavailable in this toolchain: %v", err)
		}
		t.Fatalf("init db: %v", err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"success","countryCode":"JP","country":"Japan"}`))
	}))
	defer srv.Close()

	service := (&SettingService{}).WithGeoIP(NewGeoIPServiceWithURL(&http.Client{}, srv.URL))
	if _, err := service.GetAllSetting(); err != nil {
		t.Fatalf("init default settings: %v", err)
	}

	code, name, err := service.FetchRegion(context.Background())
	if err != nil {
		t.Fatalf("fetch region: %v", err)
	}
	if code != "JP" || name != "Japan" {
		t.Fatalf("expected JP/Japan, got code=%q name=%q", code, name)
	}

	savedCode, err := service.GetRegion()
	if err != nil {
		t.Fatalf("get region after fetch: %v", err)
	}
	savedName, err := service.GetRegionName()
	if err != nil {
		t.Fatalf("get region name after fetch: %v", err)
	}
	if savedCode != "JP" || savedName != "Japan" {
		t.Fatalf("expected persisted JP/Japan, got code=%q name=%q", savedCode, savedName)
	}
}
