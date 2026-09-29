package certificate

import (
	"archive/zip"
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"io"
	"math/big"
	"sort"
	"testing"
	"time"
)

func newDownloadTestPair(t *testing.T) ([]byte, []byte) {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "*.example.com"},
		DNSNames:     []string{"*.example.com"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, key.Public(), key)
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("marshal key: %v", err)
	}

	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}),
		pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
}

func readTestZip(t *testing.T, data []byte) map[string][]byte {
	t.Helper()

	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}
	files := make(map[string][]byte, len(reader.File))
	for _, f := range reader.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("open entry %s: %v", f.Name, err)
		}
		content, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			t.Fatalf("read entry %s: %v", f.Name, err)
		}
		files[f.Name] = content
	}
	return files
}

func TestBuildCertificateZipPacksSelectedFormats(t *testing.T) {
	certPEM, keyPEM := newDownloadTestPair(t)
	filename := downloadCertFileName("*.example.com")

	data, err := buildCertificateZip(filename, []string{"crt", "key", "pfx", "crt"}, certPEM, keyPEM, "secret")
	if err != nil {
		t.Fatalf("buildCertificateZip() error = %v", err)
	}

	files := readTestZip(t, data)
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)

	want := []string{"_.example.com.crt", "_.example.com.key", "_.example.com.pfx"}
	if len(names) != len(want) {
		t.Fatalf("zip entries = %v, want %v", names, want)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Fatalf("zip entries = %v, want %v", names, want)
		}
	}
	if !bytes.Equal(files["_.example.com.crt"], certPEM) || !bytes.Equal(files["_.example.com.key"], keyPEM) {
		t.Fatal("zip entries do not hold the original certificate and key")
	}
	if len(files["_.example.com.pfx"]) == 0 {
		t.Fatal("pfx entry is empty")
	}
}

func TestBuildCertificateZipDefaultsToCertificateAndKey(t *testing.T) {
	certPEM, keyPEM := newDownloadTestPair(t)

	data, err := buildCertificateZip("site", nil, certPEM, keyPEM, "")
	if err != nil {
		t.Fatalf("buildCertificateZip() error = %v", err)
	}

	files := readTestZip(t, data)
	if len(files) != 2 || files["site.crt"] == nil || files["site.key"] == nil {
		t.Fatalf("zip entries = %v, want site.crt and site.key", files)
	}
}
