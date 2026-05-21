package ssh

import (
	"crypto/rand"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/crypto/ed25519"
	gossh "golang.org/x/crypto/ssh"
)

func generateHostKey(t *testing.T) gossh.PublicKey {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := gossh.NewSignerFromKey(priv)
	if err != nil {
		t.Fatal(err)
	}
	return signer.PublicKey()
}

func TestKnownHosts_Roundtrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "known_hosts")
	kh, err := NewKnownHosts(path)
	if err != nil {
		t.Fatalf("NewKnownHosts: %v", err)
	}
	key := generateHostKey(t)

	if err := kh.Trust("example.com:22", key); err != nil {
		t.Fatalf("Trust: %v", err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("known_hosts file not created: %v", err)
	}

	kh2, err := NewKnownHosts(path)
	if err != nil {
		t.Fatalf("Reopen: %v", err)
	}
	if !kh2.IsTrusted("example.com:22", key) {
		t.Errorf("host should be trusted after reload")
	}
}

func TestKnownHosts_StrictRejectsUnknown(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "known_hosts")
	kh, _ := NewKnownHosts(path)
	key := generateHostKey(t)

	if kh.IsTrusted("never-seen.com:22", key) {
		t.Errorf("unknown host should not be trusted")
	}
}

func TestClientHostKeyCallbackRequiresKnownHosts(t *testing.T) {
	client := NewClient(ClientOptions{})

	if _, err := client.hostKeyCallback(); err == nil {
		t.Fatal("host key callback should require a known_hosts allow-list")
	}
}
