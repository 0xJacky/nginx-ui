package cert

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/uozi-tech/cosy/logger"
)

// assertFileMode fails the test when the file at path does not carry want.
// Permission bits are not meaningful on Windows, so the check is skipped there.
func assertFileMode(t *testing.T, path string, want os.FileMode) {
	t.Helper()
	if runtime.GOOS == "windows" {
		return
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	if got := info.Mode().Perm(); got != want {
		t.Fatalf("%s mode = %04o, want %04o", path, got, want)
	}
}

// newTestPayload builds a ConfigPayload writing into dir. CertID stays zero so
// WriteFile returns before touching the database.
func newTestPayload(dir string, certificate, privateKey []byte) *ConfigPayload {
	return &ConfigPayload{
		ServerName:     []string{"example.com"},
		CertificateDir: dir,
		Resource: &model.CertificateResource{
			Certificate: certificate,
			PrivateKey:  privateKey,
		},
	}
}

func TestConfigPayloadWriteFileUsesOwnerOnlyPrivateKeyMode(t *testing.T) {
	logger.Init("debug")

	dir := filepath.Join(t.TempDir(), "example.com_2048")
	payload := newTestPayload(dir, []byte("cert pem"), []byte("key pem"))

	l := NewLogger()
	defer l.Close()

	if err := payload.WriteFile(l); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	assertFileMode(t, payload.GetCertificatePath(), 0644)
	assertFileMode(t, payload.GetCertificateKeyPath(), 0600)

	key, err := os.ReadFile(payload.GetCertificateKeyPath())
	if err != nil {
		t.Fatalf("read private key: %v", err)
	}
	if string(key) != "key pem" {
		t.Fatalf("private key = %q, want %q", key, "key pem")
	}
}

func TestConfigPayloadWriteFileTightensExistingPrivateKeyMode(t *testing.T) {
	logger.Init("debug")

	dir := filepath.Join(t.TempDir(), "example.com_2048")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("create certificate dir: %v", err)
	}

	// Simulate a key left behind by an earlier version that wrote 0644.
	keyPath := filepath.Join(dir, "private.key")
	if err := os.WriteFile(keyPath, []byte("old key"), 0644); err != nil {
		t.Fatalf("write old private key: %v", err)
	}
	certPath := filepath.Join(dir, "fullchain.cer")
	if err := os.WriteFile(certPath, []byte("old cert"), 0644); err != nil {
		t.Fatalf("write old certificate: %v", err)
	}

	payload := newTestPayload(dir, []byte("new cert"), []byte("new key"))

	l := NewLogger()
	defer l.Close()

	if err := payload.WriteFile(l); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	assertFileMode(t, certPath, 0644)
	assertFileMode(t, keyPath, 0600)

	key, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatalf("read private key: %v", err)
	}
	if string(key) != "new key" {
		t.Fatalf("private key = %q, want %q", key, "new key")
	}
}
