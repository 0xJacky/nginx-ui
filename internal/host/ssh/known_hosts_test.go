package ssh

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	gossh "golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
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

func TestKnownHosts_HostKeyAlgorithms(t *testing.T) {
	dir := t.TempDir()
	kh, err := NewKnownHosts(filepath.Join(dir, "known_hosts"))
	if err != nil {
		t.Fatal(err)
	}

	edKey := testPublicKey(t, gossh.KeyAlgoED25519)
	rsaKey := testPublicKey(t, gossh.KeyAlgoRSA)
	if err := kh.Trust("example.com:22", edKey); err != nil {
		t.Fatal(err)
	}
	if err := kh.Trust("other.example.com:22", rsaKey); err != nil {
		t.Fatal(err)
	}

	algorithms, err := kh.HostKeyAlgorithms("example.com:22")
	if err != nil {
		t.Fatal(err)
	}
	if len(algorithms) != 1 || algorithms[0] != gossh.KeyAlgoED25519 {
		t.Fatalf("unexpected algorithms: %v", algorithms)
	}

	algorithms, err = kh.HostKeyAlgorithms("other.example.com:22")
	if err != nil {
		t.Fatal(err)
	}
	wantRSA := []string{gossh.KeyAlgoRSASHA512, gossh.KeyAlgoRSASHA256, gossh.KeyAlgoRSA}
	if !slices.Equal(algorithms, wantRSA) {
		t.Fatalf("unexpected RSA algorithms: got %v want %v", algorithms, wantRSA)
	}
}

func TestKnownHosts_HostKeyAlgorithmsMatchesHashedHost(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "known_hosts")
	key := testPublicKey(t, gossh.KeyAlgoED25519)
	hashedHost := knownhosts.HashHostname("[example.com]:2222")
	if err := os.WriteFile(path, []byte(knownhosts.Line([]string{hashedHost}, key)+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	kh, err := NewKnownHosts(path)
	if err != nil {
		t.Fatal(err)
	}

	algorithms, err := kh.HostKeyAlgorithms("example.com:2222")
	if err != nil {
		t.Fatal(err)
	}
	if len(algorithms) != 1 || algorithms[0] != gossh.KeyAlgoED25519 {
		t.Fatalf("unexpected algorithms for hashed host: %v", algorithms)
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

// OpenSSH writes hashed entries by default on Debian and Ubuntu. If they are
// invisible to List, ClassifyHostKeys reports a CHANGED key as an unknown host
// and the wizard offers the benign trust flow instead of the alarm.
func TestKnownHostsListSeesHashedEntry(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "known_hosts")

	kh, err := NewKnownHosts(path)
	if err != nil {
		t.Fatal(err)
	}
	key := generateHostKey(t)
	if err := kh.Trust("host.docker.internal:22", key); err != nil {
		t.Fatal(err)
	}

	// Rewrite the plain entry in the hashed form ssh-keygen -H produces.
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	fields := strings.Fields(string(raw))
	if len(fields) < 3 {
		t.Fatalf("unexpected known_hosts line: %q", raw)
	}
	hashed := knownhosts.HashHostname("host.docker.internal") + " " + fields[1] + " " + fields[2] + "\n"
	if err := os.WriteFile(path, []byte(hashed), 0o600); err != nil {
		t.Fatal(err)
	}

	reopened, err := NewKnownHosts(path)
	if err != nil {
		t.Fatal(err)
	}
	entries, err := reopened.List("host.docker.internal:22")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("List() = %d entries, want 1; a hashed entry is invisible", len(entries))
	}
	if entries[0].Fingerprint != gossh.FingerprintSHA256(key) {
		t.Fatalf("fingerprint = %q, want %q", entries[0].Fingerprint, gossh.FingerprintSHA256(key))
	}
}

// The dial callback refuses a revoked key, so classification must not call it
// trusted, and Delete must not erase the revocation record.
func TestKnownHostsRevokedKeyIsNotTrusted(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "known_hosts")
	key := generateHostKey(t)
	line := "@revoked " + knownhosts.Line([]string{"host.docker.internal:22"}, key) + "\n"
	if err := os.WriteFile(path, []byte(line), 0o600); err != nil {
		t.Fatal(err)
	}

	kh, err := NewKnownHosts(path)
	if err != nil {
		t.Fatal(err)
	}
	if kh.IsTrusted("host.docker.internal:22", key) {
		t.Fatal("a revoked key must not be trusted at dial time")
	}

	result, err := ClassifyHostKeys("host.docker.internal:22", []gossh.PublicKey{key}, kh)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Keys) != 1 || result.Keys[0].Status != HostKeyStatusRevoked {
		t.Fatalf("status = %q, want %q", result.Keys[0].Status, HostKeyStatusRevoked)
	}

	err = kh.Delete("host.docker.internal:22", key.Type(), gossh.FingerprintSHA256(key))
	if err == nil {
		t.Fatal("Delete removed the revocation record")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "@revoked") {
		t.Fatalf("revocation record was lost:\n%s", raw)
	}
}

// x/crypto only falls back to its default host key algorithms when the slice
// is nil. An empty non-nil slice is offered verbatim and the handshake fails
// with "no common algorithm" before HostKeyCallback can report the unknown host.
func TestKnownHosts_HostKeyAlgorithmsReturnsNilWithoutRestriction(t *testing.T) {
	edKey := testPublicKey(t, gossh.KeyAlgoED25519)
	caLine := "@cert-authority *.example.com " + strings.TrimSpace(string(gossh.MarshalAuthorizedKey(edKey)))

	tests := []struct {
		name     string
		contents string
		hostPort string
		wantNil  bool
	}{
		{name: "empty file", contents: "", hostPort: "example.com:22", wantNil: true},
		{name: "only other hosts", contents: knownhosts.Line([]string{"other.example.com:22"}, edKey) + "\n", hostPort: "example.com:22", wantNil: true},
		{name: "only a cert authority", contents: caLine + "\n", hostPort: "example.com:22", wantNil: true},
		{name: "cert authority next to a matching key", contents: caLine + "\n" + knownhosts.Line([]string{"example.com:22"}, edKey) + "\n", hostPort: "example.com:22", wantNil: true},
		{name: "matching key", contents: knownhosts.Line([]string{"example.com:22"}, edKey) + "\n", hostPort: "example.com:22", wantNil: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "known_hosts")
			if err := os.WriteFile(path, []byte(tt.contents), 0o600); err != nil {
				t.Fatal(err)
			}
			kh, err := NewKnownHosts(path)
			if err != nil {
				t.Fatal(err)
			}
			algorithms, err := kh.HostKeyAlgorithms(tt.hostPort)
			if err != nil {
				t.Fatal(err)
			}
			if tt.wantNil {
				if algorithms != nil {
					t.Fatalf("HostKeyAlgorithms() = %v, want nil so x/crypto uses its defaults", algorithms)
				}
				return
			}
			if len(algorithms) != 1 || algorithms[0] != gossh.KeyAlgoED25519 {
				t.Fatalf("HostKeyAlgorithms() = %v, want [%s]", algorithms, gossh.KeyAlgoED25519)
			}
		})
	}
}
