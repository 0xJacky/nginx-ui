package setup

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	gossh "golang.org/x/crypto/ssh"
)

func TestGenerateKeypair_WritesAndParses(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "host_key")

	pub, err := GenerateKeypair(keyPath)
	if err != nil {
		t.Fatalf("GenerateKeypair: %v", err)
	}
	if !strings.HasPrefix(pub, "ssh-ed25519 ") {
		t.Errorf("public key should be ssh-ed25519, got %q", pub)
	}

	info, err := os.Stat(keyPath)
	if err != nil {
		t.Fatalf("private key file: %v", err)
	}
	if runtime.GOOS != "windows" {
		if mode := info.Mode().Perm(); mode != 0o600 {
			t.Errorf("private key mode = %v, want 0600", mode)
		}
	}

	// Verify the private key parses with x/crypto/ssh.
	raw, _ := os.ReadFile(keyPath)
	if _, err := gossh.ParsePrivateKey(raw); err != nil {
		t.Errorf("private key not parseable: %v", err)
	}
}

func TestGenerateKeypair_OverwritesExisting(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "host_key")
	if _, err := GenerateKeypair(keyPath); err != nil {
		t.Fatal(err)
	}
	first, _ := os.ReadFile(keyPath)
	if _, err := GenerateKeypair(keyPath); err != nil {
		t.Fatal(err)
	}
	second, _ := os.ReadFile(keyPath)
	if string(first) == string(second) {
		t.Errorf("second generation should produce a different key")
	}
}
