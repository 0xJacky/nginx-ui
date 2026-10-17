package cert

import (
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/0xJacky/Nginx-UI/settings"
)

func testValidatedContent(t *testing.T) *Content {
	t.Helper()
	root := t.TempDir()
	previous := *settings.NginxSettings
	settings.NginxSettings.ConfigDir = root
	settings.NginxSettings.ConfigPath = filepath.Join(root, "nginx.conf")
	t.Cleanup(func() { *settings.NginxSettings = previous })
	pair := newArchiveTestCert(t, "example.com", false, nil)
	return &Content{SSLCertificatePath: filepath.Join(root, "custom", "chain.pem"),
		SSLCertificateKeyPath: filepath.Join(root, "custom", "key.pem"),
		SSLCertificate:        string(pair.pem), SSLCertificateKey: string(pair.keyPEM)}
}

func TestCertificateWriteRejectsInvalidPairWithoutCreatingFiles(t *testing.T) {
	for _, which := range []string{"raw config", "trailing cert", "leading cert", "trailing key", "mismatch", "same path", "config path"} {
		t.Run(which, func(t *testing.T) {
			content := testValidatedContent(t)
			switch which {
			case "raw config":
				content.SSLCertificate = "user root;"
			case "trailing cert":
				content.SSLCertificate += "\nuser root;"
			case "leading cert":
				content.SSLCertificate = "user root;\n" + content.SSLCertificate
			case "trailing key":
				content.SSLCertificateKey += "\njs_import app.js;"
			case "mismatch":
				content.SSLCertificateKey = string(newArchiveTestCert(t, "other.example", false, nil).keyPEM)
			case "same path":
				content.SSLCertificateKeyPath = content.SSLCertificatePath
			case "config path":
				content.SSLCertificatePath = settings.NginxSettings.ConfigPath
			}
			if err := content.WriteFile(); err == nil {
				t.Fatal("unsafe write accepted")
			}
			if _, err := os.Stat(filepath.Join(settings.NginxSettings.ConfigDir, "custom")); !os.IsNotExist(err) {
				t.Fatalf("validation created directories: %v", err)
			}
		})
	}
}

func TestCertificateWritePreservesChainsCustomNamesAndPartialUpdates(t *testing.T) {
	content := testValidatedContent(t)
	pair := newArchiveTestCert(t, "example.com", false, nil)
	issuer := newArchiveTestCert(t, "issuer.example", true, nil)
	// OpenSSL's SEC1 EC private key plus explicit curve parameters remains valid.
	der, err := x509.MarshalECPrivateKey(pair.key.(*ecdsa.PrivateKey))
	if err != nil {
		t.Fatal(err)
	}
	content.SSLCertificate = "# certificate chain\n" + string(pair.pem) + string(issuer.pem)
	content.SSLCertificateKey = string(pem.EncodeToMemory(&pem.Block{Type: "EC PARAMETERS", Bytes: []byte{6, 8, 42, 134, 72, 206, 61, 3, 1, 7}})) + string(pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: der}))
	content.SSLCertificatePath = filepath.Join(settings.NginxSettings.ConfigDir, "custom", "fullchain")
	content.SSLCertificateKeyPath = filepath.Join(settings.NginxSettings.ConfigDir, "custom", "secret.custom")
	if err := content.ValidateSync(); err != nil {
		t.Fatal(err)
	}
	if err := content.WriteFile(); err != nil {
		t.Fatal(err)
	}
	if !content.MatchesFiles() {
		t.Fatal("sync pair no longer matches after write")
	}
	// An unchanged key may be omitted during an ordinary certificate edit.
	content.SSLCertificateKey = ""
	if err := content.WriteFile(); err != nil {
		t.Fatalf("partial certificate update rejected: %v", err)
	}
}

func TestCertificateWriteRejectsConfigAliasesAndUnrelatedTargets(t *testing.T) {
	content := testValidatedContent(t)
	root := settings.NginxSettings.ConfigDir
	configPath := filepath.Join(root, "nginx.conf")
	if err := os.WriteFile(configPath, []byte("events {}\nhttp {}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(root, "alias.pem")
	if err := os.Symlink(configPath, alias); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	content.SSLCertificatePath = alias
	if err := content.ValidateSync(); err == nil {
		t.Fatal("configuration symlink accepted")
	}
	content.SSLCertificatePath = filepath.Join(root, "unrelated")
	if err := os.WriteFile(content.SSLCertificatePath, []byte("keep this file"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := content.ValidateSync(); err == nil {
		t.Fatal("unrelated custom-name target accepted")
	}
}

func TestCertificateWriteAcceptsPKCS1RSAKey(t *testing.T) {
	content := testValidatedContent(t)
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{SerialNumber: big.NewInt(42), Subject: pkix.Name{CommonName: "rsa.example"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour)}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	content.SSLCertificate = string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
	content.SSLCertificateKey = string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}))
	if err := content.WriteFile(); err != nil {
		t.Fatalf("valid PKCS1 RSA pair rejected: %v", err)
	}
}
