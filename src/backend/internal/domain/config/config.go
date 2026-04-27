package config

import (
	_ "embed"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

//go:embed version
var version string

//go:embed name
var name string

var buildVersion string

type LogLevel string

const (
	Debug LogLevel = "debug"
	Info  LogLevel = "info"
	Warn  LogLevel = "warn"
	Error LogLevel = "error"
)

const (
	defaultDBFileName = "b-ui"
	legacyDBFileName  = "s-ui"
)

func getenv(primary, legacy string) string {
	if value := os.Getenv(primary); value != "" {
		return value
	}
	return os.Getenv(legacy)
}

func GetVersion() string {
	if strings.TrimSpace(buildVersion) != "" {
		return strings.TrimSpace(buildVersion)
	}
	return strings.TrimSpace(version)
}

func GetName() string {
	return strings.TrimSpace(name)
}

func GetLogLevel() LogLevel {
	if IsDebug() {
		return Debug
	}
	logLevel := getenv("BUI_LOG_LEVEL", "SUI_LOG_LEVEL")
	if logLevel == "" {
		return Info
	}
	return LogLevel(logLevel)
}

func IsDebug() bool {
	return getenv("BUI_DEBUG", "SUI_DEBUG") == "true"
}

func GetDBFolderPath() string {
	dbFolderPath := getenv("BUI_DB_FOLDER", "SUI_DB_FOLDER")
	if dbFolderPath == "" {
		dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
		if err != nil {
			// Cross-platform fallback path
			if runtime.GOOS == "windows" {
				return "C:\\Program Files\\b-ui\\db"
			}
			return "/usr/local/b-ui/db"
		}
		dbFolderPath = filepath.Join(dir, "db")
	}
	return dbFolderPath
}

func GetDBFileName() string {
	dbFileName := normalizeDBFileName(getenv("BUI_DB_NAME", "SUI_DB_NAME"))
	if dbFileName != "" {
		return dbFileName
	}
	return defaultDBFileName
}

func GetDBPath() string {
	return filepath.Join(GetDBFolderPath(), fmt.Sprintf("%s.db", GetDBFileName()))
}

func GetLegacyDBPath() string {
	return filepath.Join(GetDBFolderPath(), fmt.Sprintf("%s.db", legacyDBFileName))
}

func PrepareDBPath() (string, error) {
	return GetDBPath(), nil
}

func PrepareDBPathForMigration() (string, error) {
	return prepareDBPath(true)
}

func prepareDBPath(forceLegacyMigration bool) (string, error) {
	targetPath := GetDBPath()
	legacyPath := GetLegacyDBPath()
	if filepath.Clean(targetPath) == filepath.Clean(legacyPath) {
		return targetPath, nil
	}

	targetExists, err := pathExists(targetPath)
	if err != nil {
		return "", err
	}

	legacyExists, err := pathExists(legacyPath)
	if err != nil {
		return "", err
	}
	if !legacyExists {
		return targetPath, nil
	}

	if targetExists {
		if !forceLegacyMigration {
			return targetPath, nil
		}
	} else {
		for _, conflictPath := range []string{targetPath + "-wal", targetPath + "-shm"} {
			conflictExists, err := pathExists(conflictPath)
			if err != nil {
				return "", err
			}
			if conflictExists {
				return "", fmt.Errorf("target database sidecar already exists: %s", conflictPath)
			}
		}
	}

	err = migrateLegacyDBFiles(legacyPath, targetPath, forceLegacyMigration && targetExists)
	if err != nil {
		return "", err
	}

	return targetPath, nil
}

func normalizeDBFileName(dbFileName string) string {
	dbFileName = strings.TrimSpace(dbFileName)
	dbFileName = strings.TrimSuffix(dbFileName, ".db")
	return dbFileName
}

func pathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, err
}

func migrateLegacyDBFiles(legacyPath string, targetPath string, overwriteTarget bool) error {
	type fileMove struct {
		sourcePath string
		targetPath string
		tempPath   string
	}

	var files []fileMove
	for _, suffix := range []string{"", "-wal", "-shm"} {
		sourcePath := legacyPath + suffix
		exists, err := pathExists(sourcePath)
		if err != nil {
			return err
		}
		if !exists {
			continue
		}

		targetPathWithSuffix := targetPath + suffix
		targetExists, err := pathExists(targetPathWithSuffix)
		if err != nil {
			return err
		}
		if targetExists && !overwriteTarget {
			return fmt.Errorf("target database file already exists: %s", targetPathWithSuffix)
		}

		files = append(files, fileMove{
			sourcePath: sourcePath,
			targetPath: targetPathWithSuffix,
			tempPath:   targetPathWithSuffix + ".migrating",
		})
	}

	if len(files) == 0 {
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(targetPath), 01740); err != nil {
		return err
	}

	if overwriteTarget {
		for _, suffix := range []string{"", "-wal", "-shm"} {
			targetPathWithSuffix := targetPath + suffix
			if err := os.Remove(targetPathWithSuffix); err != nil && !errors.Is(err, os.ErrNotExist) {
				return err
			}
		}
	}

	var copiedFiles []string
	for _, file := range files {
		if err := copyFile(file.sourcePath, file.tempPath); err != nil {
			cleanupCopiedFiles(copiedFiles)
			return fmt.Errorf("copy legacy database file %s: %w", file.sourcePath, err)
		}
		copiedFiles = append(copiedFiles, file.tempPath)
	}

	var finalizedFiles []string
	for _, file := range files {
		if err := os.Rename(file.tempPath, file.targetPath); err != nil {
			cleanupCopiedFiles(copiedFiles)
			cleanupCopiedFiles(finalizedFiles)
			return fmt.Errorf("finalize migrated database file %s: %w", file.targetPath, err)
		}
		finalizedFiles = append(finalizedFiles, file.targetPath)
	}

	return nil
}

func copyFile(sourcePath string, targetPath string) error {
	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	if err := os.Remove(targetPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}

	targetFile, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}

	_, err = io.Copy(targetFile, sourceFile)
	closeErr := targetFile.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}

	info, err := os.Stat(sourcePath)
	if err != nil {
		return err
	}

	return os.Chmod(targetPath, info.Mode())
}

func cleanupCopiedFiles(paths []string) {
	for _, path := range paths {
		_ = os.Remove(path)
	}
}
