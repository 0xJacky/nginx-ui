package cert

import (
	"archive/zip"
	"bytes"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"testing"
	"time"
)

type archiveTestCert struct {
	cert   *x509.Certificate
	key    crypto.Signer
	pem    []byte
	keyPEM []byte
}

func newArchiveTestCert(t *testing.T, commonName string, isCA bool, parent *archiveTestCert) *archiveTestCert {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	serial, err := rand.Int(rand.Reader, big.NewInt(1<<62))
	if err != nil {
		t.Fatalf("serial: %v", err)
	}

	template := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: commonName},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		BasicConstraintsValid: true,
		IsCA:                  isCA,
	}
	if isCA {
		template.KeyUsage = x509.KeyUsageCertSign
	} else {
		template.DNSNames = []string{commonName}
		template.KeyUsage = x509.KeyUsageDigitalSignature
	}

	issuerCert, issuerKey := template, crypto.Signer(key)
	if parent != nil {
		issuerCert, issuerKey = parent.cert, parent.key
	}

	der, err := x509.CreateCertificate(rand.Reader, template, issuerCert, key.Public(), issuerKey)
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}
	parsed, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("parse certificate: %v", err)
	}

	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("marshal key: %v", err)
	}

	return &archiveTestCert{
		cert:   parsed,
		key:    key,
		pem:    pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}),
		keyPEM: pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}),
	}
}

func buildTestZip(t *testing.T, files map[string][]byte) []byte {
	t.Helper()

	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for name, content := range files {
		f, err := w.Create(name)
		if err != nil {
			t.Fatalf("create zip entry: %v", err)
		}
		if _, err = f.Write(content); err != nil {
			t.Fatalf("write zip entry: %v", err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	return buf.Bytes()
}

func concatPEM(parts ...[]byte) []byte {
	return bytes.Join(parts, nil)
}

func TestParseCertificateArchivePrefersFullChainFile(t *testing.T) {
	root := newArchiveTestCert(t, "Test Root", true, nil)
	intermediate := newArchiveTestCert(t, "Test Intermediate", true, root)
	leaf := newArchiveTestCert(t, "*.example.com", false, intermediate)

	// Mirrors a cloud console download: an nginx bundle plus an Apache folder
	// that splits the leaf from its chain, and a CSR that must be ignored.
	data := buildTestZip(t, map[string][]byte{
		"example.com.csr":                  []byte("-----BEGIN CERTIFICATE REQUEST-----\nMIIB\n-----END CERTIFICATE REQUEST-----\n"),
		"Apache/2_example.com.crt":         leaf.pem,
		"Apache/1_root_bundle.crt":         intermediate.pem,
		"Apache/3_example.com.key":         leaf.keyPEM,
		"Nginx/example.com_bundle.crt":     concatPEM(leaf.pem, intermediate.pem),
		"Nginx/example.com.key":            leaf.keyPEM,
		"__MACOSX/Nginx/._example.com.key": []byte("junk"),
	})

	result, err := ParseCertificateArchive(data)
	if err != nil {
		t.Fatalf("ParseCertificateArchive() error = %v", err)
	}

	if result.CertificateFileName != "Nginx/example.com_bundle.crt" {
		t.Fatalf("certificate file = %q, want the nginx bundle", result.CertificateFileName)
	}
	if result.ChainCompletedFromZip {
		t.Fatal("chain should come from the bundle as-is")
	}
	if result.SSLCertificate != string(concatPEM(leaf.pem, intermediate.pem)) {
		t.Fatalf("certificate = %q, want leaf + intermediate", result.SSLCertificate)
	}
	if result.SSLCertificateKey != string(leaf.keyPEM) {
		t.Fatal("private key does not match the leaf key")
	}
	if result.Name != "*.example.com" {
		t.Fatalf("name = %q, want *.example.com", result.Name)
	}
}

func TestParseCertificateArchiveCompletesChainFromSeparateBundle(t *testing.T) {
	root := newArchiveTestCert(t, "Test Root", true, nil)
	intermediate := newArchiveTestCert(t, "Test Intermediate", true, root)
	leaf := newArchiveTestCert(t, "example.com", false, intermediate)

	data := buildTestZip(t, map[string][]byte{
		"example_com.crt":       leaf.pem,
		"example_com.ca-bundle": concatPEM(intermediate.pem, root.pem),
		"example_com.key":       leaf.keyPEM,
	})

	result, err := ParseCertificateArchive(data)
	if err != nil {
		t.Fatalf("ParseCertificateArchive() error = %v", err)
	}

	if !result.ChainCompletedFromZip {
		t.Fatal("expected the intermediate to be appended from the ca-bundle")
	}
	// The root is trusted by clients already and must not be served.
	if result.SSLCertificate != string(concatPEM(leaf.pem, intermediate.pem)) {
		t.Fatalf("certificate = %q, want leaf + intermediate without root", result.SSLCertificate)
	}
}

func TestParseCertificateArchiveAcceptsCombinedPEMAndDER(t *testing.T) {
	leaf := newArchiveTestCert(t, "combined.example.com", false, nil)

	combined, err := ParseCertificateArchive(buildTestZip(t, map[string][]byte{
		"combined.pem": concatPEM(leaf.keyPEM, leaf.pem),
	}))
	if err != nil {
		t.Fatalf("combined PEM: ParseCertificateArchive() error = %v", err)
	}
	if combined.SSLCertificate != string(leaf.pem) || combined.SSLCertificateKey != string(leaf.keyPEM) {
		t.Fatal("combined PEM was not split into certificate and key")
	}

	der, err := ParseCertificateArchive(buildTestZip(t, map[string][]byte{
		"server.cer": leaf.cert.Raw,
		"server.key": leaf.keyPEM,
	}))
	if err != nil {
		t.Fatalf("DER: ParseCertificateArchive() error = %v", err)
	}
	if der.SSLCertificate != string(leaf.pem) {
		t.Fatal("DER certificate was not converted to PEM")
	}
}

func TestParseCertificateArchiveErrors(t *testing.T) {
	leaf := newArchiveTestCert(t, "a.example.com", false, nil)
	other := newArchiveTestCert(t, "b.example.com", false, nil)

	tests := []struct {
		name string
		data []byte
		want int32
	}{
		{
			name: "not a zip",
			data: []byte("definitely not a zip"),
			want: 50060,
		},
		{
			name: "no certificate",
			data: buildTestZip(t, map[string][]byte{"a.key": leaf.keyPEM}),
			want: 50062,
		},
		{
			name: "no private key",
			data: buildTestZip(t, map[string][]byte{"a.crt": leaf.pem}),
			want: 50063,
		},
		{
			name: "key mismatch",
			data: buildTestZip(t, map[string][]byte{"a.crt": leaf.pem, "b.key": other.keyPEM}),
			want: 50064,
		},
		{
			name: "oversized upload",
			data: make([]byte, MaxCertificateArchiveSize+1),
			want: 50061,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseCertificateArchive(tt.data)
			requireCosyCode(t, err, tt.want)
		})
	}
}

func TestParseCertificateArchiveRejectsOversizedEntry(t *testing.T) {
	leaf := newArchiveTestCert(t, "big.example.com", false, nil)

	// Compresses to almost nothing but inflates past the per-entry cap; the
	// declared size is honest here, so it is skipped rather than read.
	padding := bytes.Repeat([]byte{'\n'}, maxCertificateArchiveEntrySize+1)
	data := buildTestZip(t, map[string][]byte{
		"padding.pem": padding,
		"a.crt":       leaf.pem,
		"a.key":       leaf.keyPEM,
	})

	result, err := ParseCertificateArchive(data)
	if err != nil {
		t.Fatalf("ParseCertificateArchive() error = %v", err)
	}
	if result.CertificateFileName != "a.crt" {
		t.Fatalf("certificate file = %q, want a.crt", result.CertificateFileName)
	}
}
