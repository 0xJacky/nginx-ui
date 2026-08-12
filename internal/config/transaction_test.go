package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFileTransactionRollbackRestoresEveryWrittenFile(t *testing.T) {
	dir := t.TempDir()
	existingPath := filepath.Join(dir, "existing.conf")
	createdPath := filepath.Join(dir, "created.conf")
	previousContent := "server {\n    listen 80;\n}\n"
	if err := os.WriteFile(existingPath, []byte(previousContent), 0o640); err != nil {
		t.Fatalf("failed to seed config file: %v", err)
	}

	tx := &FileTransaction{}
	if err := tx.Write(existingPath, []byte("broken\n"), 0o644); err != nil {
		t.Fatalf("failed to write existing file: %v", err)
	}
	if err := tx.Write(createdPath, []byte("broken\n"), 0o644); err != nil {
		t.Fatalf("failed to write created file: %v", err)
	}
	if tx.Len() != 2 {
		t.Fatalf("expected 2 written files, got %d", tx.Len())
	}

	if err := tx.Rollback(); err != nil {
		t.Fatalf("Rollback returned error: %v", err)
	}

	content, err := os.ReadFile(existingPath)
	if err != nil {
		t.Fatalf("failed to read config file: %v", err)
	}
	if string(content) != previousContent {
		t.Fatalf("expected previous content, got %q", string(content))
	}

	info, err := os.Stat(existingPath)
	if err != nil {
		t.Fatalf("failed to stat config file: %v", err)
	}
	if info.Mode().Perm() != 0o640 {
		t.Fatalf("expected restored permissions 0640, got %v", info.Mode().Perm())
	}

	if _, err := os.Stat(createdPath); !os.IsNotExist(err) {
		t.Fatalf("expected the created file to be removed, got %v", err)
	}
}

func TestLockApplySerializesConfigurationMutations(t *testing.T) {
	release := LockApply()

	acquired := make(chan struct{})
	go func() {
		secondRelease := LockApply()
		close(acquired)
		secondRelease()
	}()

	select {
	case <-acquired:
		release()
		t.Fatal("LockApply must not hand out the apply lock while it is held")
	case <-time.After(50 * time.Millisecond):
	}

	release()

	select {
	case <-acquired:
	case <-time.After(5 * time.Second):
		t.Fatal("LockApply must hand over the apply lock after it is released")
	}
}
