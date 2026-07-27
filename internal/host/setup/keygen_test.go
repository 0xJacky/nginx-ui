package setup

import (
	"errors"
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

func TestSavePrivateKeyValidatesAndWritesAtomically(t *testing.T) {
	dir := t.TempDir()
	sourcePath := filepath.Join(dir, "source_key")
	targetPath := filepath.Join(dir, "target_key")
	if _, err := GenerateKeypair(sourcePath); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatal(err)
	}

	publicKey, err := SavePrivateKey(targetPath, raw)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(publicKey, "ssh-ed25519 ") || !strings.HasSuffix(publicKey, " nginx-ui@provided") {
		t.Fatalf("unexpected public key: %q", publicKey)
	}
	written, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(written) != string(raw) {
		t.Fatal("stored private key differs from submitted key")
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(targetPath)
		if err != nil {
			t.Fatal(err)
		}
		if mode := info.Mode().Perm(); mode != 0o600 {
			t.Fatalf("private key mode = %v, want 0600", mode)
		}
	}
}

func TestSavePrivateKeyDoesNotOverwriteOnValidationFailure(t *testing.T) {
	targetPath := filepath.Join(t.TempDir(), "host_key")
	if err := os.WriteFile(targetPath, []byte("existing"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := SavePrivateKey(targetPath, []byte("not a private key"))
	if !errors.Is(err, ErrInvalidPrivateKey) {
		t.Fatalf("error = %v, want ErrInvalidPrivateKey", err)
	}
	written, readErr := os.ReadFile(targetPath)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(written) != "existing" {
		t.Fatalf("invalid import overwrote existing key: %q", written)
	}
}

func TestReadPrivateKeyFileRejectsIrregularFile(t *testing.T) {
	if _, err := ReadPrivateKeyFile(t.TempDir()); err == nil {
		t.Fatal("expected a directory to be rejected")
	}
}

func TestReadPrivateKeyFileRejectsOversizedFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "huge_key")
	if err := os.WriteFile(path, make([]byte, MaxPrivateKeyFileSize+1), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadPrivateKeyFile(path); err == nil {
		t.Fatal("expected an oversized private key to be rejected")
	}
}
