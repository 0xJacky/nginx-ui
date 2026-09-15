//go:build unix

package cert

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestWritePrivateKeyPreservesGroupOwnership(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("changing a test file to a distinct group requires root")
	}

	keyPath := filepath.Join(t.TempDir(), "private.key")
	if err := os.WriteFile(keyPath, []byte("old key"), 0640); err != nil {
		t.Fatalf("write old private key: %v", err)
	}
	const groupID = 12345
	if err := os.Chown(keyPath, 0, groupID); err != nil {
		t.Fatalf("set private key ownership: %v", err)
	}

	if err := writePrivateKey(keyPath, []byte("new key")); err != nil {
		t.Fatalf("writePrivateKey: %v", err)
	}

	info, err := os.Stat(keyPath)
	if err != nil {
		t.Fatalf("stat private key: %v", err)
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		t.Fatalf("private key stat does not expose Unix ownership")
	}
	if stat.Uid != 0 || stat.Gid != groupID {
		t.Fatalf("private key ownership = %d:%d, want 0:%d", stat.Uid, stat.Gid, groupID)
	}
	assertFileMode(t, keyPath, 0640)
	assertFileContent(t, keyPath, "new key")
}
