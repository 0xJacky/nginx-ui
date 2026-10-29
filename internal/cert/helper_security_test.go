package cert

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/0xJacky/Nginx-UI/settings"
)

// useCertPathTestConfDir points the nginx configuration directory at a fresh
// temporary directory for the duration of the test and returns it.
func useCertPathTestConfDir(t *testing.T) string {
	t.Helper()
	confDir := t.TempDir()
	original := settings.NginxSettings.ConfigDir
	settings.NginxSettings.ConfigDir = confDir
	t.Cleanup(func() { settings.NginxSettings.ConfigDir = original })
	return confDir
}

func writeCertPathTestFile(t *testing.T, path string, content []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestCertificatePathValidatorsAcceptPEMUnderConfDir(t *testing.T) {
	confDir := useCertPathTestConfDir(t)
	pair := newArchiveTestCert(t, "example.com", false, nil)
	certPath := filepath.Join(confDir, "ssl", "example.com", "fullchain.cer")
	keyPath := filepath.Join(confDir, "ssl", "example.com", "private.key")
	writeCertPathTestFile(t, certPath, pair.pem)
	writeCertPathTestFile(t, keyPath, pair.keyPEM)

	if !IsCertificatePath(certPath) {
		t.Fatal("PEM certificate under the conf dir was rejected")
	}
	if !IsPrivateKeyPath(keyPath) {
		t.Fatal("PEM private key under the conf dir was rejected")
	}
	missing := filepath.Join(confDir, "ssl", "new.example.com", "fullchain.cer")
	if !IsCertificatePath(missing) || !IsPrivateKeyPath(missing) {
		t.Fatal("a not yet written path under the conf dir was rejected")
	}
}

func TestCertificatePathValidatorsRejectInvalidContentUnderConfDir(t *testing.T) {
	confDir := useCertPathTestConfDir(t)
	pair := newArchiveTestCert(t, "example.com", false, nil)
	certPath := filepath.Join(confDir, "ssl", "fullchain.cer")
	keyPath := filepath.Join(confDir, "ssl", "private.key")
	plainPath := filepath.Join(confDir, "nginx.conf")
	writeCertPathTestFile(t, certPath, pair.pem)
	writeCertPathTestFile(t, keyPath, pair.keyPEM)
	writeCertPathTestFile(t, plainPath, []byte("user nginx;\n"))

	cases := map[string]bool{
		"key at certificate path": IsCertificatePath(keyPath),
		"certificate at key path": IsPrivateKeyPath(certPath),
		"non-PEM certificate":     IsCertificatePath(plainPath),
		"non-PEM key":             IsPrivateKeyPath(plainPath),
		"directory certificate":   IsCertificatePath(filepath.Join(confDir, "ssl")),
		"directory key":           IsPrivateKeyPath(filepath.Join(confDir, "ssl")),
		"empty certificate path":  IsCertificatePath(""),
		"empty key path":          IsPrivateKeyPath(""),
	}
	for name, passed := range cases {
		if passed {
			t.Errorf("%s: validator passed, want rejection", name)
		}
	}
}

// A valid PEM block followed by padding still parses, so only the size cap
// keeps the validator from buffering the whole file.
func TestCertificatePathValidatorsRejectOversizedFileUnderConfDir(t *testing.T) {
	confDir := useCertPathTestConfDir(t)
	pair := newArchiveTestCert(t, "example.com", false, nil)
	// 1 MiB of padding on top of the PEM block exceeds the 1 MiB cap.
	padding := bytes.Repeat([]byte("\n"), 1<<20)
	certPath := filepath.Join(confDir, "ssl", "fullchain.cer")
	keyPath := filepath.Join(confDir, "ssl", "private.key")
	writeCertPathTestFile(t, certPath, append(append([]byte{}, pair.pem...), padding...))
	writeCertPathTestFile(t, keyPath, append(append([]byte{}, pair.keyPEM...), padding...))

	if IsCertificatePath(certPath) {
		t.Error("oversized certificate file under the conf dir was accepted")
	}
	if IsPrivateKeyPath(keyPath) {
		t.Error("oversized private key file under the conf dir was accepted")
	}
}

// Paths outside the conf dir are externally managed. They are accepted
// without being inspected, so the result is the same whether the path is
// missing, a certificate, or something else entirely, and reveals nothing.
func TestCertificatePathValidatorsDoNotInspectPathsOutsideConfDir(t *testing.T) {
	confDir := useCertPathTestConfDir(t)
	outside := t.TempDir()
	pair := newArchiveTestCert(t, "example.com", false, nil)

	plain := filepath.Join(outside, "passwd")
	writeCertPathTestFile(t, plain, []byte("root:x:0:0:root:/root:/bin/sh\n"))
	pemCert := filepath.Join(outside, "live", "fullchain.pem")
	writeCertPathTestFile(t, pemCert, pair.pem)
	locked := filepath.Join(outside, "locked")
	if err := os.Mkdir(locked, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o700) })
	link := filepath.Join(confDir, "ssl", "linked.pem")
	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(plain, link); err != nil {
		t.Skip("symlink:", err)
	}
	// An outside path is not even resolved, so a link from outside into the
	// conf dir is not followed to the non-PEM file it points at.
	inwardTarget := filepath.Join(confDir, "nginx.conf")
	writeCertPathTestFile(t, inwardTarget, []byte("user nginx;\n"))
	inward := filepath.Join(outside, "inward.pem")
	if err := os.Symlink(inwardTarget, inward); err != nil {
		t.Fatal(err)
	}

	paths := map[string]string{
		"outside symlink into conf dir": inward,
		"missing file":                  filepath.Join(outside, "does-not-exist"),
		"existing non-PEM file":         plain,
		"existing PEM certificate":      pemCert,
		"directory":                     outside,
		"child of unreadable dir":       filepath.Join(locked, "x"),
		"conf dir symlink to outside":   link,
		"certbot style external path":   "/etc/letsencrypt/live/example.com/fullchain.pem",
	}
	for name, path := range paths {
		if !IsCertificatePath(path) {
			t.Errorf("%s: certificate path validator rejected an external path", name)
		}
		if !IsPrivateKeyPath(path) {
			t.Errorf("%s: private key path validator rejected an external path", name)
		}
	}
}
