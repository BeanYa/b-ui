package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPrepareDBPathIgnoresLegacyDatabaseOutsideMigration(t *testing.T) {
	dbDir := t.TempDir()
	t.Setenv("BUI_DB_FOLDER", dbDir)
	t.Setenv("BUI_DB_NAME", "")

	legacyPath := filepath.Join(dbDir, "s-ui.db")
	if err := os.WriteFile(legacyPath, []byte("legacy"), 0o600); err != nil {
		t.Fatalf("write legacy db: %v", err)
	}

	dbPath, err := PrepareDBPath()
	if err != nil {
		t.Fatalf("prepare db path: %v", err)
	}
	targetPath := filepath.Join(dbDir, "b-ui.db")
	if dbPath != targetPath {
		t.Fatalf("db path mismatch: got %s want %s", dbPath, targetPath)
	}
	if _, err := os.Stat(targetPath); !os.IsNotExist(err) {
		t.Fatalf("target database should not be created during normal startup: %v", err)
	}
	content, err := os.ReadFile(legacyPath)
	if err != nil {
		t.Fatalf("legacy file should be preserved: %v", err)
	}
	if string(content) != "legacy" {
		t.Fatalf("legacy file changed unexpectedly: got %q", string(content))
	}
}

func TestPrepareDBPathForMigrationMigratesLegacyDatabase(t *testing.T) {
	dbDir := t.TempDir()
	t.Setenv("BUI_DB_FOLDER", dbDir)
	t.Setenv("BUI_DB_NAME", "")

	legacyMain := filepath.Join(dbDir, "s-ui.db")
	legacyWal := legacyMain + "-wal"
	legacyShm := legacyMain + "-shm"

	if err := os.WriteFile(legacyMain, []byte("legacy-main"), 0o600); err != nil {
		t.Fatalf("write legacy main: %v", err)
	}
	if err := os.WriteFile(legacyWal, []byte("legacy-wal"), 0o600); err != nil {
		t.Fatalf("write legacy wal: %v", err)
	}
	if err := os.WriteFile(legacyShm, []byte("legacy-shm"), 0o600); err != nil {
		t.Fatalf("write legacy shm: %v", err)
	}

	dbPath, err := PrepareDBPathForMigration()
	if err != nil {
		t.Fatalf("prepare db path: %v", err)
	}

	if want := filepath.Join(dbDir, "b-ui.db"); dbPath != want {
		t.Fatalf("db path mismatch: got %s want %s", dbPath, want)
	}

	for _, file := range []struct {
		path    string
		content string
	}{
		{path: dbPath, content: "legacy-main"},
		{path: dbPath + "-wal", content: "legacy-wal"},
		{path: dbPath + "-shm", content: "legacy-shm"},
	} {
		content, err := os.ReadFile(file.path)
		if err != nil {
			t.Fatalf("read migrated file %s: %v", file.path, err)
		}
		if string(content) != file.content {
			t.Fatalf("migrated file %s mismatch: got %q want %q", file.path, string(content), file.content)
		}
	}

	for _, legacyPath := range []string{legacyMain, legacyWal, legacyShm} {
		if _, err := os.Stat(legacyPath); err != nil {
			t.Fatalf("legacy file should be preserved: %s (%v)", legacyPath, err)
		}
	}
}

func TestPrepareDBPathKeepsExistingTargetDatabase(t *testing.T) {
	dbDir := t.TempDir()
	t.Setenv("BUI_DB_FOLDER", dbDir)
	t.Setenv("BUI_DB_NAME", "")

	targetPath := filepath.Join(dbDir, "b-ui.db")
	legacyPath := filepath.Join(dbDir, "s-ui.db")

	if err := os.WriteFile(targetPath, []byte("current"), 0o600); err != nil {
		t.Fatalf("write target db: %v", err)
	}
	if err := os.WriteFile(legacyPath, []byte("legacy"), 0o600); err != nil {
		t.Fatalf("write legacy db: %v", err)
	}

	dbPath, err := PrepareDBPath()
	if err != nil {
		t.Fatalf("prepare db path: %v", err)
	}

	if dbPath != targetPath {
		t.Fatalf("db path mismatch: got %s want %s", dbPath, targetPath)
	}

	content, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("read target db: %v", err)
	}
	if string(content) != "current" {
		t.Fatalf("target db changed unexpectedly: got %q", string(content))
	}
}

func TestPrepareDBPathForMigrationReplacesExistingTargetWithLegacyData(t *testing.T) {
	dbDir := t.TempDir()
	t.Setenv("BUI_DB_FOLDER", dbDir)
	t.Setenv("BUI_DB_NAME", "")

	legacyPath := filepath.Join(dbDir, "s-ui.db")
	targetPath := filepath.Join(dbDir, "b-ui.db")

	if err := os.WriteFile(legacyPath, []byte("legacy-main"), 0o600); err != nil {
		t.Fatalf("write legacy db: %v", err)
	}
	if err := os.WriteFile(legacyPath+"-wal", []byte("legacy-wal"), 0o600); err != nil {
		t.Fatalf("write legacy wal: %v", err)
	}
	if err := os.WriteFile(targetPath, []byte("placeholder-main"), 0o600); err != nil {
		t.Fatalf("write target db: %v", err)
	}
	if err := os.WriteFile(targetPath+"-wal", []byte("placeholder-wal"), 0o600); err != nil {
		t.Fatalf("write target wal: %v", err)
	}

	dbPath, err := PrepareDBPathForMigration()
	if err != nil {
		t.Fatalf("prepare db path: %v", err)
	}
	if dbPath != targetPath {
		t.Fatalf("db path mismatch: got %s want %s", dbPath, targetPath)
	}

	for _, file := range []struct {
		path    string
		content string
	}{
		{path: targetPath, content: "legacy-main"},
		{path: targetPath + "-wal", content: "legacy-wal"},
	} {
		content, err := os.ReadFile(file.path)
		if err != nil {
			t.Fatalf("read migrated file %s: %v", file.path, err)
		}
		if string(content) != file.content {
			t.Fatalf("migrated file %s mismatch: got %q want %q", file.path, string(content), file.content)
		}
	}
}
