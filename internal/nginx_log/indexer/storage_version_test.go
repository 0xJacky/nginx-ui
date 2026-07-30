package indexer

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPrepareIndexStorageRemovesOnlyManagedIndexes(t *testing.T) {
	indexPath := t.TempDir()
	managedDirectories := []string{
		"4a589aa8-79b9-4c45-b98f-2807af3b13f8",
		"shard_0",
	}
	for _, name := range managedDirectories {
		path := filepath.Join(indexPath, name)
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatalf("create managed directory: %v", err)
		}
	}
	unmanagedPath := filepath.Join(indexPath, "keep-me")
	if err := os.MkdirAll(unmanagedPath, 0o755); err != nil {
		t.Fatalf("create unmanaged directory: %v", err)
	}

	needsReset, err := PrepareIndexStorage(indexPath)
	if err != nil {
		t.Fatalf("PrepareIndexStorage() error = %v", err)
	}
	if !needsReset {
		t.Fatal("PrepareIndexStorage() needsReset = false, want true")
	}
	for _, name := range managedDirectories {
		if _, err := os.Stat(filepath.Join(indexPath, name)); !os.IsNotExist(err) {
			t.Fatalf("managed directory %q still exists", name)
		}
	}
	if _, err := os.Stat(unmanagedPath); err != nil {
		t.Fatalf("unmanaged directory was removed: %v", err)
	}

	if err := CommitIndexStorageVersion(indexPath); err != nil {
		t.Fatalf("CommitIndexStorageVersion() error = %v", err)
	}
	needsReset, err = PrepareIndexStorage(indexPath)
	if err != nil {
		t.Fatalf("PrepareIndexStorage() after commit error = %v", err)
	}
	if needsReset {
		t.Fatal("PrepareIndexStorage() after commit needsReset = true, want false")
	}
}
