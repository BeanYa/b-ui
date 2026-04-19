package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPrepareDBPathMigratesLegacyDatabase(t *testing.T) {
	dbDir := t.TempDir()
	t.Setenv("SUI_DB_FOLDER", dbDir)
	t.Setenv("SUI_DB_NAME", "")

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

	dbPath, err := PrepareDBPath()
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
	t.Setenv("SUI_DB_FOLDER", dbDir)
	t.Setenv("SUI_DB_NAME", "")

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
