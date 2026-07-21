package system

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/0xJacky/Nginx-UI/settings"
)

func TestValidateSSLCertificatesRejectsMismatchedKeyPair(t *testing.T) {
	confDir := t.TempDir()
	originalConfigDir := settings.NginxSettings.ConfigDir
	settings.NginxSettings.ConfigDir = confDir
	t.Cleanup(func() {
		settings.NginxSettings.ConfigDir = originalConfigDir
	})

	certPathA, keyPathA := writeTestCertificatePair(t, confDir, "a")
	_, keyPathB := writeTestCertificatePair(t, confDir, "b")

	if err := ValidateSSLCertificates(certPathA, keyPathA); err != nil {
		t.Fatalf("ValidateSSLCertificates() rejected a valid pair: %v", err)
	}

	err := ValidateSSLCertificates(certPathA, keyPathB)
	if err == nil {
		t.Fatal("ValidateSSLCertificates() accepted a mismatched certificate and key")
	}
	if !strings.Contains(err.Error(), "private key does not match public key") {
		t.Fatalf("ValidateSSLCertificates() error = %q, want key mismatch detail", err)
	}
}

func writeTestCertificatePair(t *testing.T, dir, name string) (string, string) {
	t.Helper()

	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate private key: %v", err)
	}

	now := time.Now()
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: name + ".example.test"},
		NotBefore:    now.Add(-time.Minute),
		NotAfter:     now.Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}

	certificateDER, err := x509.CreateCertificate(rand.Reader, template, template, &privateKey.PublicKey, privateKey)
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}
	privateKeyDER, err := x509.MarshalECPrivateKey(privateKey)
	if err != nil {
		t.Fatalf("marshal private key: %v", err)
	}

	certPath := filepath.Join(dir, name+".crt")
	keyPath := filepath.Join(dir, name+".key")
	if err := os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certificateDER}), 0o600); err != nil {
		t.Fatalf("write certificate: %v", err)
	}
	if err := os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: privateKeyDER}), 0o600); err != nil {
		t.Fatalf("write private key: %v", err)
	}

	return certPath, keyPath
}
