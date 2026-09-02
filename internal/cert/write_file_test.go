package cert

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0xJacky/Nginx-UI/settings"
)

func TestContentWriteFileUsesOwnerOnlyPrivateKeyMode(t *testing.T) {
	originalConfigDir := settings.NginxSettings.ConfigDir
	confDir := t.TempDir()
	settings.NginxSettings.ConfigDir = confDir
	t.Cleanup(func() {
		settings.NginxSettings.ConfigDir = originalConfigDir
	})

	dir := filepath.Join(confDir, "ssl", "example")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("create cert dir: %v", err)
	}

	// Simulate a pair left behind by an earlier version that wrote 0644 keys.
	certPath := filepath.Join(dir, "fullchain.cer")
	keyPath := filepath.Join(dir, "private.key")
	if err := os.WriteFile(certPath, []byte("old cert"), 0o644); err != nil {
		t.Fatalf("write old cert: %v", err)
	}
	if err := os.WriteFile(keyPath, []byte("old key"), 0o644); err != nil {
		t.Fatalf("write old key: %v", err)
	}

	content := &Content{
		SSLCertificatePath:    certPath,
		SSLCertificateKeyPath: keyPath,
		SSLCertificate:        "new cert",
		SSLCertificateKey:     "new key",
	}
	if err := content.WriteFile(); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	assertFileMode(t, certPath, 0644)
	assertFileMode(t, keyPath, 0600)
}

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

func TestContentWriteFileCreatesMissingDirectoriesAndLeavesNoTempFiles(t *testing.T) {
	originalConfigDir := settings.NginxSettings.ConfigDir
	confDir := t.TempDir()
	settings.NginxSettings.ConfigDir = confDir
	t.Cleanup(func() {
		settings.NginxSettings.ConfigDir = originalConfigDir
	})

	// Neither ssl/ nor the certificate directory exists yet; WriteFile has to
	// create them on the nginx target filesystem.
	dir := filepath.Join(confDir, "ssl", "fresh")
	certPath := filepath.Join(dir, "fullchain.cer")
	keyPath := filepath.Join(dir, "private.key")

	content := &Content{
		SSLCertificatePath:    certPath,
		SSLCertificateKeyPath: keyPath,
		SSLCertificate:        "cert",
		SSLCertificateKey:     "key",
	}
	if err := content.WriteFile(); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	assertFileContent(t, certPath, "cert")
	assertFileContent(t, keyPath, "key")
	assertFileMode(t, certPath, 0644)
	assertFileMode(t, keyPath, 0600)
	assertNoTempFiles(t, dir)
}

func TestWriteFileWithModeReplacesContentAndRepairsMode(t *testing.T) {
	originalConfigDir := settings.NginxSettings.ConfigDir
	confDir := t.TempDir()
	settings.NginxSettings.ConfigDir = confDir
	t.Cleanup(func() {
		settings.NginxSettings.ConfigDir = originalConfigDir
	})

	dir := filepath.Join(confDir, "ssl", "example")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("create cert dir: %v", err)
	}
	keyPath := filepath.Join(dir, "private.key")
	if err := os.WriteFile(keyPath, []byte("old key"), 0o644); err != nil {
		t.Fatalf("write old key: %v", err)
	}

	if err := writeFileWithMode(keyPath, []byte("new key"), 0600); err != nil {
		t.Fatalf("writeFileWithMode: %v", err)
	}

	assertFileContent(t, keyPath, "new key")
	assertFileMode(t, keyPath, 0600)
	assertNoTempFiles(t, dir)
}

func TestWriteFileWithModeRejectsDirectoryTarget(t *testing.T) {
	originalConfigDir := settings.NginxSettings.ConfigDir
	confDir := t.TempDir()
	settings.NginxSettings.ConfigDir = confDir
	t.Cleanup(func() {
		settings.NginxSettings.ConfigDir = originalConfigDir
	})

	target := filepath.Join(confDir, "ssl", "example", "private.key")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatalf("create directory at target path: %v", err)
	}

	if err := writeFileWithMode(target, []byte("key"), 0600); err == nil {
		t.Fatalf("expected an error when the target is a directory")
	}
	assertNoTempFiles(t, filepath.Dir(target))
}

func TestWriteTempFileNextToCreatesFileWithRequestedMode(t *testing.T) {
	originalConfigDir := settings.NginxSettings.ConfigDir
	confDir := t.TempDir()
	settings.NginxSettings.ConfigDir = confDir
	t.Cleanup(func() {
		settings.NginxSettings.ConfigDir = originalConfigDir
	})

	target := filepath.Join(confDir, "private.key")
	tmpPath, err := writeTempFileNextTo(target, []byte("staged"), 0600)
	if err != nil {
		t.Fatalf("writeTempFileNextTo: %v", err)
	}
	t.Cleanup(func() { _ = os.Remove(tmpPath) })

	if filepath.Dir(tmpPath) != confDir {
		t.Fatalf("temp file %s is not next to %s", tmpPath, target)
	}
	if base := filepath.Base(tmpPath); !strings.HasPrefix(base, ".private.key.") || !strings.HasSuffix(base, ".tmp") {
		t.Fatalf("unexpected temp file name %q", base)
	}
	assertFileContent(t, tmpPath, "staged")
	assertFileMode(t, tmpPath, 0600)

	// A second staging for the same target must not collide with the first.
	other, err := writeTempFileNextTo(target, []byte("again"), 0600)
	if err != nil {
		t.Fatalf("second writeTempFileNextTo: %v", err)
	}
	t.Cleanup(func() { _ = os.Remove(other) })
	if other == tmpPath {
		t.Fatalf("temp file names collided: %s", other)
	}
}

func assertFileContent(t *testing.T, path, want string) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if string(got) != want {
		t.Fatalf("%s content = %q, want %q", path, got, want)
	}
}

// assertNoTempFiles fails when a staging file was left behind in dir.
func assertNoTempFiles(t *testing.T, dir string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".tmp") {
			t.Fatalf("temp file left behind: %s", filepath.Join(dir, entry.Name()))
		}
	}
}
