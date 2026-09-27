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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writePair(t *testing.T, dir string, certPEM, keyPEM []byte) (certPath, keyPath string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(dir, 0o755))
	certPath = filepath.Join(dir, "fullchain.cer")
	keyPath = filepath.Join(dir, "private.key")
	require.NoError(t, os.WriteFile(certPath, certPEM, 0o644))
	require.NoError(t, os.WriteFile(keyPath, keyPEM, 0o600))
	return certPath, keyPath
}

func TestLoadCertificatePair(t *testing.T) {
	confDir := useTempNginxConfDir(t)
	certPEM, keyPEM, err := GenerateSelfSigned(SelfSignedOptions{
		CommonName:  "example.com",
		DNSNames:    []string{"example.com", "*.example.com"},
		IPAddresses: []string{"192.0.2.10"},
		KeyType:     "P256",
	})
	require.NoError(t, err)
	certPath, keyPath := writePair(t, filepath.Join(confDir, "ssl", "example"), certPEM, keyPEM)

	pair, err := LoadCertificatePair(certPath, keyPath)
	require.NoError(t, err)
	assert.Equal(t, "P256", pair.KeyType)
	assert.Equal(t, "example.com", pair.Info.SubjectName)
	assert.True(t, pair.Info.NotAfter.After(time.Now()))
	assert.ElementsMatch(t, []string{"example.com", "*.example.com"}, pair.Leaf.DNSNames)

	t.Run("missing key file", func(t *testing.T) {
		_, err := LoadCertificatePair(certPath, filepath.Join(confDir, "ssl", "missing.key"))
		assert.ErrorIs(t, err, os.ErrNotExist)
	})

	t.Run("key of another certificate", func(t *testing.T) {
		_, otherKey, err := GenerateSelfSigned(SelfSignedOptions{DNSNames: []string{"example.com"}, KeyType: "P256"})
		require.NoError(t, err)
		_, otherKeyPath := writePair(t, filepath.Join(confDir, "ssl", "other"), certPEM, otherKey)
		_, err = LoadCertificatePair(certPath, otherKeyPath)
		assert.Error(t, err)
	})

	t.Run("not a certificate", func(t *testing.T) {
		badCert, badKey := writePair(t, filepath.Join(confDir, "ssl", "bad"), []byte("garbage"), keyPEM)
		_, err := LoadCertificatePair(badCert, badKey)
		assert.Error(t, err)
	})

	t.Run("empty path", func(t *testing.T) {
		_, err := LoadCertificatePair("", keyPath)
		assert.ErrorIs(t, err, ErrCertPathIsEmpty)
	})

	t.Run("outside the nginx configuration directory", func(t *testing.T) {
		outside := t.TempDir()
		outCert, outKey := writePair(t, outside, certPEM, keyPEM)
		_, err := LoadCertificatePair(outCert, outKey)
		assert.ErrorIs(t, err, ErrCertPathIsNotUnderTheNginxConfDir)
	})
}

func TestLoadCertificatePairReportsExpiredCertificate(t *testing.T) {
	confDir := useTempNginxConfDir(t)
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "old.example.com"},
		DNSNames:     []string{"old.example.com"},
		NotBefore:    time.Now().Add(-48 * time.Hour),
		NotAfter:     time.Now().Add(-24 * time.Hour),
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, key.Public(), key)
	require.NoError(t, err)
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	require.NoError(t, err)
	certPath, keyPath := writePair(t, filepath.Join(confDir, "ssl", "old"),
		pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}),
		pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}))

	// Loading does not judge the validity period; callers do.
	pair, err := LoadCertificatePair(certPath, keyPath)
	require.NoError(t, err)
	assert.True(t, pair.Info.NotAfter.Before(time.Now()))
}

func TestCertificateCoversIdentifier(t *testing.T) {
	leaf := &x509.Certificate{
		DNSNames:    []string{"example.com", "*.example.com", "Other.Example.NET"},
		IPAddresses: []net.IP{net.ParseIP("192.0.2.10"), net.ParseIP("2001:db8::1")},
	}

	for identifier, want := range map[string]bool{
		"example.com":         true,
		"EXAMPLE.com.":        true,
		"www.example.com":     true,
		"a.b.example.com":     false,
		"*.example.com":       true,
		"*.other.example.net": false,
		"other.example.net":   true,
		"example.org":         false,
		"192.0.2.10":          true,
		"192.0.2.11":          false,
		"2001:db8::1":         true,
		"[2001:db8::1]":       true,
		"2001:db8::2":         false,
		"":                    false,
	} {
		assert.Equal(t, want, CertificateCoversIdentifier(leaf, identifier), identifier)
	}

	assert.False(t, CertificateCoversIdentifier(nil, "example.com"))
	// The common name alone does not cover a host.
	assert.False(t, CertificateCoversIdentifier(&x509.Certificate{Subject: pkix.Name{CommonName: "cn.example.com"}}, "cn.example.com"))
}
