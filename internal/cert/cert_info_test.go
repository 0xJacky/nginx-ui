package cert

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/0xJacky/Nginx-UI/settings"
)

func TestGetCertInfoFallsBackToIPAddressSAN(t *testing.T) {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	notBefore := time.Date(2026, time.July, 27, 12, 0, 0, 0, time.UTC)
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{},
		Issuer:       pkix.Name{CommonName: "Test Issuer"},
		NotBefore:    notBefore,
		NotAfter:     notBefore.Add(160 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses:  []net.IP{net.ParseIP("203.0.113.8")},
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &privateKey.PublicKey, privateKey)
	if err != nil {
		t.Fatal(err)
	}

	confDir := t.TempDir()
	originalConfigDir := settings.NginxSettings.ConfigDir
	settings.NginxSettings.ConfigDir = confDir
	t.Cleanup(func() {
		settings.NginxSettings.ConfigDir = originalConfigDir
	})

	certPath := filepath.Join(confDir, "ip-cert.pem")
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	if err := os.WriteFile(certPath, certPEM, 0o644); err != nil {
		t.Fatal(err)
	}

	info, err := GetCertInfo(certPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.SubjectName != "203.0.113.8" {
		t.Fatalf("SubjectName = %q, want IP SAN", info.SubjectName)
	}

	parsed, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	if imported := infoFromCertificate(parsed); imported.SubjectName != "203.0.113.8" {
		t.Fatalf("imported SubjectName = %q, want IP SAN", imported.SubjectName)
	}
}

func TestCertificateSubjectNamePrecedence(t *testing.T) {
	cert := &x509.Certificate{
		Subject:     pkix.Name{CommonName: "common.example.com"},
		DNSNames:    []string{"san.example.com"},
		IPAddresses: []net.IP{net.ParseIP("203.0.113.8")},
	}
	if got := certificateSubjectName(cert); got != "common.example.com" {
		t.Fatalf("certificateSubjectName() = %q, want common name", got)
	}

	cert.Subject.CommonName = ""
	if got := certificateSubjectName(cert); got != "san.example.com" {
		t.Fatalf("certificateSubjectName() = %q, want DNS SAN", got)
	}
}
