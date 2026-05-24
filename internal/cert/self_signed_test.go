package cert

import (
	"crypto/x509"
	"encoding/pem"
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/go-acme/lego/v5/certcrypto"
)

func parseTestCert(t *testing.T, certPEM []byte) *x509.Certificate {
	t.Helper()
	block, _ := pem.Decode(certPEM)
	if block == nil {
		t.Fatalf("failed to decode certificate PEM")
	}
	parsed, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("failed to parse certificate: %v", err)
	}
	return parsed
}

func TestGenerateSelfSignedProducesValidCertificate(t *testing.T) {
	certPEM, keyPEM, err := GenerateSelfSigned(SelfSignedOptions{
		CommonName:   "example.com",
		DNSNames:     []string{"example.com", "www.example.com"},
		IPAddresses:  []string{"192.168.1.10"},
		KeyType:      certcrypto.EC256,
		ValidityDays: 30,
	})
	if err != nil {
		t.Fatalf("GenerateSelfSigned returned error: %v", err)
	}

	parsed := parseTestCert(t, certPEM)

	if parsed.Issuer.CommonName != parsed.Subject.CommonName {
		t.Fatalf("expected self-signed cert, issuer %q != subject %q",
			parsed.Issuer.CommonName, parsed.Subject.CommonName)
	}
	if parsed.Subject.CommonName != "example.com" {
		t.Fatalf("unexpected common name: %s", parsed.Subject.CommonName)
	}
	if len(parsed.DNSNames) != 2 || parsed.DNSNames[0] != "example.com" {
		t.Fatalf("unexpected DNS names: %v", parsed.DNSNames)
	}
	if len(parsed.IPAddresses) != 1 || !parsed.IPAddresses[0].Equal(net.ParseIP("192.168.1.10")) {
		t.Fatalf("unexpected IP addresses: %v", parsed.IPAddresses)
	}
	gotDays := int(parsed.NotAfter.Sub(parsed.NotBefore).Hours() / 24)
	if gotDays < 29 || gotDays > 31 {
		t.Fatalf("unexpected validity window: %d days", gotDays)
	}
	if parsed.IsCA {
		t.Fatalf("leaf certificate must not be a CA")
	}
	if err := parsed.CheckSignature(parsed.SignatureAlgorithm,
		parsed.RawTBSCertificate, parsed.Signature); err != nil {
		t.Fatalf("self-signature verification failed: %v", err)
	}
	if !IsPrivateKey(string(keyPEM)) {
		t.Fatalf("generated key is not a valid private key")
	}
}

func TestGenerateSelfSignedSupportsKeyTypes(t *testing.T) {
	// RSA8192 and RSA3072 are omitted: key generation is too slow under -race.
	keyTypes := []certcrypto.KeyType{
		certcrypto.RSA2048, certcrypto.RSA4096, certcrypto.EC256, certcrypto.EC384,
	}
	for _, kt := range keyTypes {
		t.Run(string(kt), func(t *testing.T) {
			certPEM, _, err := GenerateSelfSigned(SelfSignedOptions{
				CommonName:   "test.local",
				DNSNames:     []string{"test.local"},
				KeyType:      kt,
				ValidityDays: 365,
			})
			if err != nil {
				t.Fatalf("GenerateSelfSigned(%s) error: %v", kt, err)
			}
			parseTestCert(t, certPEM)
		})
	}
}

func TestGenerateSelfSignedRejectsEmptySAN(t *testing.T) {
	_, _, err := GenerateSelfSigned(SelfSignedOptions{
		CommonName:   "example.com",
		KeyType:      certcrypto.EC256,
		ValidityDays: 365,
	})
	if err == nil {
		t.Fatalf("expected an error when no DNS names or IP addresses are given")
	}
}

func TestGenerateSelfSignedRejectsInvalidIP(t *testing.T) {
	_, _, err := GenerateSelfSigned(SelfSignedOptions{
		CommonName:   "example.com",
		IPAddresses:  []string{"not-an-ip"},
		KeyType:      certcrypto.EC256,
		ValidityDays: 365,
	})
	if err == nil {
		t.Fatalf("expected an error for an invalid IP address")
	}
}

func TestRegenerateSelfSignedReusesKey(t *testing.T) {
	_, keyPEM, err := GenerateSelfSigned(SelfSignedOptions{
		CommonName:   "reuse.local",
		DNSNames:     []string{"reuse.local"},
		KeyType:      certcrypto.EC256,
		ValidityDays: 365,
	})
	if err != nil {
		t.Fatalf("GenerateSelfSigned error: %v", err)
	}

	keyPath := filepath.Join(t.TempDir(), "private.key")
	if err := os.WriteFile(keyPath, keyPEM, 0600); err != nil {
		t.Fatalf("write key: %v", err)
	}

	certModel := &model.Cert{
		Domains:               []string{"reuse.local"},
		KeyType:               certcrypto.EC256,
		SSLCertificateKeyPath: keyPath,
		SelfSignedConfig:      &model.SelfSignedCertConfig{ValidityDays: 365},
	}

	newCertPEM, newKeyPEM, err := RegenerateSelfSigned(certModel)
	if err != nil {
		t.Fatalf("RegenerateSelfSigned error: %v", err)
	}
	if string(newKeyPEM) != string(keyPEM) {
		t.Fatalf("expected the private key to be reused unchanged")
	}
	parseTestCert(t, newCertPEM)
}

func TestRegenerateSelfSignedWithOptionsReusesKey(t *testing.T) {
	_, keyPEM, err := GenerateSelfSigned(SelfSignedOptions{
		CommonName:   "old.local",
		DNSNames:     []string{"old.local"},
		KeyType:      certcrypto.EC256,
		ValidityDays: 365,
	})
	if err != nil {
		t.Fatalf("GenerateSelfSigned error: %v", err)
	}

	keyPath := filepath.Join(t.TempDir(), "private.key")
	if err := os.WriteFile(keyPath, keyPEM, 0600); err != nil {
		t.Fatalf("write key: %v", err)
	}

	certModel := &model.Cert{SSLCertificateKeyPath: keyPath}
	newCertPEM, newKeyPEM, err := RegenerateSelfSignedWithOptions(certModel, SelfSignedOptions{
		CommonName:   "new.local",
		DNSNames:     []string{"new.local"},
		KeyType:      certcrypto.EC256,
		ValidityDays: 90,
	})
	if err != nil {
		t.Fatalf("RegenerateSelfSignedWithOptions error: %v", err)
	}
	if string(newKeyPEM) != string(keyPEM) {
		t.Fatalf("expected the private key to be reused unchanged")
	}
	parsed := parseTestCert(t, newCertPEM)
	if parsed.Subject.CommonName != "new.local" {
		t.Fatalf("CommonName = %q, want %q", parsed.Subject.CommonName, "new.local")
	}
}

func TestRegenerateSelfSignedFallsBackToFreshKey(t *testing.T) {
	certModel := &model.Cert{
		Domains:               []string{"fresh.local"},
		KeyType:               certcrypto.EC256,
		SSLCertificateKeyPath: filepath.Join(t.TempDir(), "missing.key"),
		SelfSignedConfig:      &model.SelfSignedCertConfig{ValidityDays: 365},
	}

	certPEM, keyPEM, err := RegenerateSelfSigned(certModel)
	if err != nil {
		t.Fatalf("RegenerateSelfSigned error: %v", err)
	}
	parseTestCert(t, certPEM)
	if !IsPrivateKey(string(keyPEM)) {
		t.Fatalf("fallback key is not a valid private key")
	}
}

func TestDeriveSelfSignedCommonName(t *testing.T) {
	if got := deriveSelfSignedCommonName([]string{"a.com", "b.com"}, nil); got != "a.com" {
		t.Fatalf("expected first DNS name, got %q", got)
	}
	if got := deriveSelfSignedCommonName(nil, []string{"10.0.0.1"}); got != "10.0.0.1" {
		t.Fatalf("expected first IP, got %q", got)
	}
}
