package indexer

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

const (
	indexStorageVersionFile = ".nginx-ui-index-version"
	indexStorageVersion     = "3"
)

// PrepareIndexStorage removes rebuildable shard data when the on-disk format
// predates the current Zap format, mapping, sharding, or Scorch resource
// settings. Metadata must be reset before CommitIndexStorageVersion is called.
func PrepareIndexStorage(indexPath string) (bool, error) {
	if err := os.MkdirAll(indexPath, 0o755); err != nil {
		return false, fmt.Errorf("create index directory: %w", err)
	}

	versionPath := filepath.Join(indexPath, indexStorageVersionFile)
	version, err := os.ReadFile(versionPath)
	if err == nil && strings.TrimSpace(string(version)) == indexStorageVersion {
		return false, nil
	}
	if err != nil && !os.IsNotExist(err) {
		return false, fmt.Errorf("read index storage version: %w", err)
	}

	entries, err := os.ReadDir(indexPath)
	if err != nil {
		return false, fmt.Errorf("read index directory: %w", err)
	}
	for _, entry := range entries {
		if !isManagedIndexDirectory(entry) {
			continue
		}
		entryPath := filepath.Join(indexPath, entry.Name())
		if err := os.RemoveAll(entryPath); err != nil {
			return false, fmt.Errorf("remove incompatible index directory %s: %w", entryPath, err)
		}
	}

	return true, nil
}

// CommitIndexStorageVersion marks a completed disk and metadata migration.
func CommitIndexStorageVersion(indexPath string) error {
	versionPath := filepath.Join(indexPath, indexStorageVersionFile)
	if err := os.WriteFile(versionPath, []byte(indexStorageVersion+"\n"), 0o600); err != nil {
		return fmt.Errorf("write index storage version: %w", err)
	}
	return nil
}

func isManagedIndexDirectory(entry os.DirEntry) bool {
	if !entry.IsDir() {
		return false
	}
	if strings.HasPrefix(entry.Name(), "shard_") {
		return true
	}
	_, err := uuid.Parse(entry.Name())
	return err == nil
}
