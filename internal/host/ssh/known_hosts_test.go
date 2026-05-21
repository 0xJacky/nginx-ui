package ssh

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"os"
	"path/filepath"
	"testing"

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

func TestKnownHosts_ListMultipleAlgorithms(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "known_hosts")
	kh, err := NewKnownHosts(path)
	if err != nil {
		t.Fatal(err)
	}

	edKey := testPublicKey(t, "ssh-ed25519")
	rsaKey := testPublicKey(t, "ssh-rsa")

	if err := kh.Trust("host.docker.internal:22", edKey); err != nil {
		t.Fatal(err)
	}
	if err := kh.Trust("host.docker.internal:22", rsaKey); err != nil {
		t.Fatal(err)
	}

	entries, err := kh.List("host.docker.internal:22")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].Fingerprint == "" || entries[1].Fingerprint == "" {
		t.Fatalf("fingerprints should be populated: %+v", entries)
	}
}

func TestKnownHosts_ReplaceOnlySameAlgorithm(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "known_hosts")
	kh, err := NewKnownHosts(path)
	if err != nil {
		t.Fatal(err)
	}

	oldEd := testPublicKey(t, "ssh-ed25519")
	newEd := testPublicKeyFromSeed(t, "ssh-ed25519", 99)
	rsaKey := testPublicKey(t, "ssh-rsa")
	if err := kh.Trust("host.docker.internal:22", oldEd); err != nil {
		t.Fatal(err)
	}
	if err := kh.Trust("host.docker.internal:22", rsaKey); err != nil {
		t.Fatal(err)
	}

	oldFP := gossh.FingerprintSHA256(oldEd)
	if err := kh.Replace("host.docker.internal:22", oldFP, newEd); err != nil {
		t.Fatal(err)
	}

	entries, err := kh.List("host.docker.internal:22")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries after replace, got %d", len(entries))
	}
	if !kh.IsTrusted("host.docker.internal:22", newEd) {
		t.Fatal("new ed25519 key should be trusted")
	}
	if !kh.IsTrusted("host.docker.internal:22", rsaKey) {
		t.Fatal("rsa key should remain trusted")
	}
	if kh.IsTrusted("host.docker.internal:22", oldEd) {
		t.Fatal("old ed25519 key should no longer be trusted")
	}
}

func TestKnownHosts_DeleteExactEntry(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "known_hosts")
	kh, err := NewKnownHosts(path)
	if err != nil {
		t.Fatal(err)
	}

	edKey := testPublicKey(t, "ssh-ed25519")
	rsaKey := testPublicKey(t, "ssh-rsa")
	if err := kh.Trust("host.docker.internal:22", edKey); err != nil {
		t.Fatal(err)
	}
	if err := kh.Trust("host.docker.internal:22", rsaKey); err != nil {
		t.Fatal(err)
	}

	if err := kh.Delete("host.docker.internal:22", rsaKey.Type(), gossh.FingerprintSHA256(rsaKey)); err != nil {
		t.Fatal(err)
	}

	if !kh.IsTrusted("host.docker.internal:22", edKey) {
		t.Fatal("ed25519 key should remain trusted")
	}
	if kh.IsTrusted("host.docker.internal:22", rsaKey) {
		t.Fatal("rsa key should be deleted")
	}
	entries, err := kh.List("host.docker.internal:22")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Algorithm != edKey.Type() {
		t.Fatalf("unexpected entries after delete: %+v", entries)
	}
}

func TestClientHostKeyCallbackRequiresKnownHosts(t *testing.T) {
	client := NewClient(ClientOptions{})

	if _, err := client.hostKeyCallback(); err == nil {
		t.Fatal("host key callback should require a known_hosts allow-list")
	}
}

func testPublicKey(t *testing.T, algorithm string) gossh.PublicKey {
	t.Helper()
	return testPublicKeyFromSeed(t, algorithm, 1)
}

func testPublicKeyFromSeed(t *testing.T, algorithm string, seed byte) gossh.PublicKey {
	t.Helper()
	switch algorithm {
	case "ssh-ed25519":
		private := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{seed}, ed25519.SeedSize))
		public, err := gossh.NewPublicKey(private.Public())
		if err != nil {
			t.Fatal(err)
		}
		return public
	case "ssh-rsa":
		private, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			t.Fatal(err)
		}
		public, err := gossh.NewPublicKey(&private.PublicKey)
		if err != nil {
			t.Fatal(err)
		}
		return public
	default:
		t.Fatalf("unsupported test algorithm %q", algorithm)
		return nil
	}
}
