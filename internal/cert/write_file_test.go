package cert

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/0xJacky/Nginx-UI/settings"
)

func TestContentWriteFileKeepsExistingPairWhenKeyWriteFails(t *testing.T) {
	originalConfigDir := settings.NginxSettings.ConfigDir
	confDir := t.TempDir()
	settings.NginxSettings.ConfigDir = confDir
	t.Cleanup(func() {
		settings.NginxSettings.ConfigDir = originalConfigDir
	})

	certPath := filepath.Join(confDir, "ssl", "example", "fullchain.cer")
	if err := os.MkdirAll(filepath.Dir(certPath), 0o755); err != nil {
		t.Fatalf("create cert dir: %v", err)
	}
	if err := os.WriteFile(certPath, []byte("old cert"), 0o644); err != nil {
		t.Fatalf("write old cert: %v", err)
	}

	keyPath := filepath.Join(confDir, "ssl", "example", "private.key")
	if err := os.MkdirAll(keyPath, 0o755); err != nil {
		t.Fatalf("create key path directory: %v", err)
	}

	content := &Content{
		SSLCertificatePath:    certPath,
		SSLCertificateKeyPath: keyPath,
		SSLCertificate:        "new cert",
		SSLCertificateKey:     "new key",
	}
	if err := content.WriteFile(); err == nil {
		t.Fatalf("expected key write failure")
	}

	got, err := os.ReadFile(certPath)
	if err != nil {
		t.Fatalf("read cert after failed write: %v", err)
	}
	if string(got) != "old cert" {
		t.Fatalf("certificate changed after failed key write: %q", got)
	}
}
