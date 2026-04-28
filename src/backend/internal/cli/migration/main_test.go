package migration

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPrepareDBPathForCommandPreservesCurrentDatabaseByDefault(t *testing.T) {
	dbDir := t.TempDir()
	t.Setenv("BUI_DB_FOLDER", dbDir)
	t.Setenv("BUI_DB_NAME", "")
	t.Setenv("BUI_LEGACY_DB_MIGRATION", "")

	currentPath := filepath.Join(dbDir, "b-ui.db")
	legacyPath := filepath.Join(dbDir, "s-ui.db")
	writeTestFile(t, currentPath, "current")
	writeTestFile(t, legacyPath, "legacy")

	dbPath, err := prepareDBPathForCommand()
	if err != nil {
		t.Fatalf("prepare db path for command: %v", err)
	}

	if dbPath != currentPath {
		t.Fatalf("db path mismatch: got %s want %s", dbPath, currentPath)
	}
	if got := readTestFile(t, currentPath); got != "current" {
		t.Fatalf("current database was replaced during normal migration: got %q", got)
	}
}

func TestPrepareDBPathForCommandMigratesLegacyDatabaseWhenEnabled(t *testing.T) {
	dbDir := t.TempDir()
	t.Setenv("BUI_DB_FOLDER", dbDir)
	t.Setenv("BUI_DB_NAME", "")
	t.Setenv("BUI_LEGACY_DB_MIGRATION", "1")

	currentPath := filepath.Join(dbDir, "b-ui.db")
	legacyPath := filepath.Join(dbDir, "s-ui.db")
	writeTestFile(t, currentPath, "placeholder")
	writeTestFile(t, legacyPath, "legacy")

	dbPath, err := prepareDBPathForCommand()
	if err != nil {
		t.Fatalf("prepare db path for command: %v", err)
	}

	if dbPath != currentPath {
		t.Fatalf("db path mismatch: got %s want %s", dbPath, currentPath)
	}
	if got := readTestFile(t, currentPath); got != "legacy" {
		t.Fatalf("legacy database was not staged for explicit migration: got %q", got)
	}
}

func writeTestFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write test file %s: %v", path, err)
	}
}

func readTestFile(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read test file %s: %v", path, err)
	}
	return string(content)
}
