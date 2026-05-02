package database

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/BeanYa/b-ui/src/backend/internal/domain/config"
	"github.com/BeanYa/b-ui/src/backend/internal/shared/util/common"
)

type ResidualFile struct {
	Name string `json:"name"`
	Path string `json:"path"`
	Size int64  `json:"size"`
	Kind string `json:"kind"` // "backup", "temp", "migrating", "dated_backup", "sidecar"
}

// dateBackupPattern matches files like b-ui_20260101-120000.db or s-ui_20251231-235959.db
var dateBackupPattern = regexp.MustCompile(`_\d{8}-\d{6}\.db$`)

func ListResidualFiles() ([]ResidualFile, error) {
	folder := config.GetDBFolderPath()
	dbName := config.GetDBFileName()
	activeDB := dbName + ".db"

	entries, err := os.ReadDir(folder)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var files []ResidualFile
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		info, err := entry.Info()
		if err != nil {
			continue
		}

		kind := classifyResidualFile(name, dbName, activeDB)
		if kind == "" {
			continue
		}

		files = append(files, ResidualFile{
			Name: name,
			Path: filepath.Join(folder, name),
			Size: info.Size(),
			Kind: kind,
		})
	}
	return files, nil
}

func classifyResidualFile(name string, dbName string, activeDB string) string {
	if name == activeDB || name == activeDB+"-wal" || name == activeDB+"-shm" {
		return "" // active files, never cleanup candidates
	}

	lower := strings.ToLower(name)

	if strings.HasSuffix(lower, ".db.backup") || strings.HasSuffix(lower, ".backup") {
		return "backup"
	}
	if strings.HasSuffix(lower, ".db.temp") || strings.HasSuffix(lower, ".temp") {
		return "temp"
	}
	if strings.HasSuffix(lower, ".db.migrating") || strings.HasSuffix(lower, ".migrating") {
		return "migrating"
	}
	if dateBackupPattern.MatchString(name) && strings.HasSuffix(lower, ".db") {
		return "dated_backup"
	}
	// Legacy s-ui sidecar files (wal/shm for old db name)
	if strings.HasPrefix(name, "s-ui") && (strings.HasSuffix(lower, "-wal") || strings.HasSuffix(lower, "-shm")) {
		return "sidecar"
	}
	return ""
}

func DeleteResidualFile(path string) error {
	return os.Remove(path)
}

func DeleteResidualFiles(paths []string) error {
	var msgs []string
	for _, p := range paths {
		if err := os.Remove(p); err != nil {
			msgs = append(msgs, err.Error())
		}
	}
	if len(msgs) > 0 {
		return common.NewErrorf("failed to delete %d file(s): %s", len(msgs), strings.Join(msgs, "; "))
	}
	return nil
}
