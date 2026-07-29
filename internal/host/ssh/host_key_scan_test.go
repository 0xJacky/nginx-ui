package ssh

import (
	"path/filepath"
	"strings"
	"testing"

	gossh "golang.org/x/crypto/ssh"
)

func TestClassifyHostKeys_UnknownNewChangedTrustedStale(t *testing.T) {
	dir := t.TempDir()
	kh, err := NewKnownHosts(filepath.Join(dir, "known_hosts"))
	if err != nil {
		t.Fatal(err)
	}

	oldEd := testPublicKeyFromSeed(t, "ssh-ed25519", 7)
	newEd := testPublicKeyFromSeed(t, "ssh-ed25519", 8)
	rsaKey := testPublicKeyFromSeed(t, "ssh-rsa", 9)
	if err := kh.Trust("host.docker.internal:22", oldEd); err != nil {
		t.Fatal(err)
	}
	if err := kh.Trust("host.docker.internal:22", rsaKey); err != nil {
		t.Fatal(err)
	}

	changed, err := ClassifyHostKeys("host.docker.internal:22", []gossh.PublicKey{newEd}, kh)
	if err != nil {
		t.Fatal(err)
	}
	if len(changed.Keys) != 1 {
		t.Fatalf("expected one scanned key, got %d", len(changed.Keys))
	}
	if changed.Keys[0].Status != HostKeyStatusChanged {
		t.Fatalf("expected changed, got %s", changed.Keys[0].Status)
	}
	if changed.Keys[0].ExistingFingerprint != gossh.FingerprintSHA256(oldEd) {
		t.Fatalf("expected old fingerprint, got %q", changed.Keys[0].ExistingFingerprint)
	}
	if len(changed.StaleKeys) != 1 || changed.StaleKeys[0].Algorithm != rsaKey.Type() {
		t.Fatalf("expected rsa stale key, got %+v", changed.StaleKeys)
	}

	trusted, err := ClassifyHostKeys("host.docker.internal:22", []gossh.PublicKey{oldEd}, kh)
	if err != nil {
		t.Fatal(err)
	}
	if trusted.Keys[0].Status != HostKeyStatusTrusted {
		t.Fatalf("expected trusted, got %s", trusted.Keys[0].Status)
	}

	unknown, err := ClassifyHostKeys("new-host:22", []gossh.PublicKey{newEd}, kh)
	if err != nil {
		t.Fatal(err)
	}
	if unknown.Keys[0].Status != HostKeyStatusUnknownHost {
		t.Fatalf("expected unknown host, got %s", unknown.Keys[0].Status)
	}
}

func TestClassifyHostKeys_NewAlgorithm(t *testing.T) {
	dir := t.TempDir()
	kh, err := NewKnownHosts(filepath.Join(dir, "known_hosts"))
	if err != nil {
		t.Fatal(err)
	}

	edKey := testPublicKeyFromSeed(t, "ssh-ed25519", 12)
	rsaKey := testPublicKeyFromSeed(t, "ssh-rsa", 13)
	if err := kh.Trust("host.docker.internal:22", edKey); err != nil {
		t.Fatal(err)
	}

	result, err := ClassifyHostKeys("host.docker.internal:22", []gossh.PublicKey{rsaKey}, kh)
	if err != nil {
		t.Fatal(err)
	}
	if result.Keys[0].Status != HostKeyStatusNewAlgorithm {
		t.Fatalf("expected new_algorithm, got %s", result.Keys[0].Status)
	}
}

func TestParseSSHKeyscanOutput(t *testing.T) {
	key := testPublicKeyFromSeed(t, "ssh-ed25519", 11)
	line := "host.docker.internal " + strings.TrimSpace(string(gossh.MarshalAuthorizedKey(key)))

	keys, err := ParseSSHKeyscanOutput(line)
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 1 {
		t.Fatalf("expected one key, got %d", len(keys))
	}
	if keys[0].Type() != "ssh-ed25519" {
		t.Fatalf("unexpected key type %s", keys[0].Type())
	}
}

func TestParseSSHKeyscanOutput_HashedHost(t *testing.T) {
	key := testPublicKeyFromSeed(t, "ssh-ed25519", 14)
	line := "|1|saltyvalue|hashedvalue " + strings.TrimSpace(string(gossh.MarshalAuthorizedKey(key)))

	keys, err := ParseSSHKeyscanOutput(line)
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 1 {
		t.Fatalf("expected one key, got %d", len(keys))
	}
	if keys[0].Type() != "ssh-ed25519" {
		t.Fatalf("unexpected key type %s", keys[0].Type())
	}
}
