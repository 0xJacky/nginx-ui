//go:build !windows

package process

import (
	"net"
	"os"
	"path/filepath"
	"testing"
)

func TestPrepareUnixSocket(t *testing.T) {
	dir, err := os.MkdirTemp("/tmp", "nui-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	// Missing path: nothing to do.
	missing := filepath.Join(dir, "missing.sock")
	if err := PrepareUnixSocket(missing); err != nil {
		t.Fatalf("missing path: %v", err)
	}

	// Regular file: refuse to touch it.
	regular := filepath.Join(dir, "regular.sock")
	if err := os.WriteFile(regular, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := PrepareUnixSocket(regular); err == nil {
		t.Fatal("regular file was accepted")
	}
	if _, err := os.Stat(regular); err != nil {
		t.Fatalf("regular file was removed: %v", err)
	}

	// Live socket: refuse, leave it alone.
	live := filepath.Join(dir, "live.sock")
	ln, err := net.Listen("unix", live)
	if err != nil {
		t.Fatal(err)
	}
	if err := PrepareUnixSocket(live); err == nil {
		t.Fatal("live socket was accepted")
	}
	if _, err := os.Stat(live); err != nil {
		t.Fatalf("live socket was removed: %v", err)
	}

	// Stale socket: simulate an unclean exit by keeping the file after close.
	stale := filepath.Join(dir, "stale.sock")
	staleLn, err := net.Listen("unix", stale)
	if err != nil {
		t.Fatal(err)
	}
	staleLn.(*net.UnixListener).SetUnlinkOnClose(false)
	_ = staleLn.Close()
	if _, err := os.Stat(stale); err != nil {
		t.Fatalf("stale socket precondition: %v", err)
	}
	if err := PrepareUnixSocket(stale); err != nil {
		t.Fatalf("stale socket: %v", err)
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Fatalf("stale socket still present: %v", err)
	}
	if _, err := net.Listen("unix", stale); err != nil {
		t.Fatalf("relisten after cleanup: %v", err)
	}
	_ = ln.Close()
}

func TestApplyUnixSocketMode(t *testing.T) {
	dir, err := os.MkdirTemp("/tmp", "nui-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, "mode.sock")
	ln, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	if err := ApplyUnixSocketMode(path, 0o666); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o666 {
		t.Fatalf("mode = %o, want 666", got)
	}
}
