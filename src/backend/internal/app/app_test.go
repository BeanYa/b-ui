package app

import (
	"strings"
	"testing"

	service "github.com/BeanYa/b-ui/src/backend/internal/domain/services"
)

func TestInitAppliesStartupAdminCredentials(t *testing.T) {
	t.Setenv("BUI_DB_FOLDER", t.TempDir())
	t.Setenv("BUI_DB_NAME", "startup-admin")
	t.Setenv("BUI_DEFAULT_ADMIN_USERNAME", "dev-admin")
	t.Setenv("BUI_DEFAULT_ADMIN_PASSWORD", "dev-pass")

	app := NewApp()
	if err := app.Init(); err != nil {
		if strings.Contains(err.Error(), "go-sqlite3 requires cgo") || strings.Contains(err.Error(), "C compiler") {
			t.Skipf("sqlite test database unavailable in this toolchain: %v", err)
		}
		t.Fatalf("init app: %v", err)
	}

	user := (&service.UserService{}).CheckUser("dev-admin", "dev-pass", "127.0.0.1")
	if user == nil {
		t.Fatal("expected configured startup admin credentials to be usable")
	}
}

func TestInitRejectsPartialStartupAdminCredentials(t *testing.T) {
	t.Setenv("BUI_DB_FOLDER", t.TempDir())
	t.Setenv("BUI_DB_NAME", "partial-startup-admin")
	t.Setenv("BUI_DEFAULT_ADMIN_USERNAME", "dev-admin")
	t.Setenv("BUI_DEFAULT_ADMIN_PASSWORD", "")

	app := NewApp()
	err := app.Init()
	if err == nil {
		t.Fatal("expected partial startup admin credentials to fail")
	}
	if !strings.Contains(err.Error(), "BUI_DEFAULT_ADMIN_USERNAME") {
		t.Fatalf("unexpected error: %v", err)
	}
}
