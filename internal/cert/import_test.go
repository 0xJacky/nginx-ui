package cert

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/go-acme/lego/v5/certcrypto"
	"github.com/uozi-tech/cosy"
)

func withImportTestNginxConfigDir(t *testing.T) string {
	t.Helper()

	originalConfigDir := settings.NginxSettings.ConfigDir
	confDir := t.TempDir()
	settings.NginxSettings.ConfigDir = confDir
	t.Cleanup(func() {
		settings.NginxSettings.ConfigDir = originalConfigDir
	})

	return confDir
}

func writeImportTestPair(t *testing.T, dir string, certNames, keyNames []string) {
	t.Helper()

	certPEM, keyPEM, err := GenerateSelfSigned(SelfSignedOptions{
		CommonName:   filepath.Base(dir),
		DNSNames:     []string{filepath.Base(dir)},
		KeyType:      certcrypto.EC256,
		ValidityDays: 365,
	})
	if err != nil {
		t.Fatalf("generate test certificate: %v", err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("create certificate directory: %v", err)
	}

	for _, name := range certNames {
		if err := os.WriteFile(filepath.Join(dir, name), certPEM, 0o644); err != nil {
			t.Fatalf("write certificate %s: %v", name, err)
		}
	}
	for _, name := range keyNames {
		if err := os.WriteFile(filepath.Join(dir, name), keyPEM, 0o600); err != nil {
			t.Fatalf("write private key %s: %v", name, err)
		}
	}
}

func TestDiscoverCertificatePairPrefersKnownCandidateNames(t *testing.T) {
	confDir := withImportTestNginxConfigDir(t)

	tests := []struct {
		name     string
		dirName  string
		certs    []string
		keys     []string
		wantCert string
		wantKey  string
	}{
		{
			name:     "certbot layout",
			dirName:  "example.local",
			certs:    []string{"cert.pem", "fullchain.pem", "example.local.crt"},
			keys:     []string{"key.pem", "privkey.pem", "example.local.key"},
			wantCert: "fullchain.pem",
			wantKey:  "privkey.pem",
		},
		{
			name:     "tls layout",
			dirName:  "tls.example.local",
			certs:    []string{"tls.example.local.crt", "tls.crt"},
			keys:     []string{"tls.example.local.key", "tls.key"},
			wantCert: "tls.crt",
			wantKey:  "tls.key",
		},
		{
			name:     "basename fallback",
			dirName:  "basename.example.local",
			certs:    []string{"fallback.crt", "basename.example.local.pem"},
			keys:     []string{"fallback.key", "basename.example.local.key"},
			wantCert: "basename.example.local.pem",
			wantKey:  "basename.example.local.key",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := filepath.Join(confDir, "ssl", tt.dirName)
			writeImportTestPair(t, dir, tt.certs, tt.keys)

			pair, err := DiscoverCertificatePair(dir)
			if err != nil {
				t.Fatalf("DiscoverCertificatePair returned error: %v", err)
			}

			if got := filepath.Base(pair.SSLCertificatePath); got != tt.wantCert {
				t.Fatalf("certificate candidate = %q, want %q", got, tt.wantCert)
			}
			if got := filepath.Base(pair.SSLCertificateKeyPath); got != tt.wantKey {
				t.Fatalf("private key candidate = %q, want %q", got, tt.wantKey)
			}
			if pair.Fingerprint == "" {
				t.Fatalf("expected fingerprint to be populated")
			}
		})
	}
}

func TestCertificatePairIsImportedDeduplicatesByNamePathsAndFingerprint(t *testing.T) {
	baseCertPath := filepath.Join(t.TempDir(), "ssl", "existing", "fullchain.pem")
	baseKeyPath := filepath.Join(filepath.Dir(baseCertPath), "privkey.pem")
	existing := []model.Cert{
		{
			Name:                  "existing-name",
			SSLCertificatePath:    baseCertPath,
			SSLCertificateKeyPath: baseKeyPath,
			Fingerprint:           "known-fingerprint",
		},
	}

	tests := []struct {
		name string
		pair DiscoveredCertificatePair
		want bool
	}{
		{
			name: "name",
			pair: DiscoveredCertificatePair{Name: "EXISTING-NAME"},
			want: true,
		},
		{
			name: "certificate path",
			pair: DiscoveredCertificatePair{SSLCertificatePath: baseCertPath},
			want: true,
		},
		{
			name: "private key path",
			pair: DiscoveredCertificatePair{SSLCertificateKeyPath: baseKeyPath},
			want: true,
		},
		{
			name: "fingerprint",
			pair: DiscoveredCertificatePair{Fingerprint: "known-fingerprint"},
			want: true,
		},
		{
			name: "new certificate",
			pair: DiscoveredCertificatePair{
				Name:                  "new-name",
				SSLCertificatePath:    filepath.Join(t.TempDir(), "ssl", "new", "fullchain.pem"),
				SSLCertificateKeyPath: filepath.Join(t.TempDir(), "ssl", "new", "privkey.pem"),
				Fingerprint:           "new-fingerprint",
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := certificatePairIsImported(tt.pair, existing); got != tt.want {
				t.Fatalf("certificatePairIsImported() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestScanCertificateSSLDirectoryUsesConfiguredSSLRoot(t *testing.T) {
	confDir := withImportTestNginxConfigDir(t)

	includedDir := filepath.Join(confDir, "ssl", "included.example.local")
	writeImportTestPair(t, includedDir, []string{"fullchain.pem"}, []string{"privkey.pem"})

	outsideDir := filepath.Join(confDir, "not-ssl", "ignored.example.local")
	writeImportTestPair(t, outsideDir, []string{"fullchain.pem"}, []string{"privkey.pem"})

	pairs, err := ScanCertificateSSLDirectory(false)
	if err != nil {
		t.Fatalf("ScanCertificateSSLDirectory returned error: %v", err)
	}

	if len(pairs) != 1 {
		t.Fatalf("expected exactly one discovered pair, got %d: %#v", len(pairs), pairs)
	}
	if pairs[0].Dir != includedDir {
		t.Fatalf("discovered dir = %q, want %q", pairs[0].Dir, includedDir)
	}
}

func TestScanCertificateSSLDirectoryHandlesMissingSSLRoot(t *testing.T) {
	withImportTestNginxConfigDir(t)

	pairs, err := ScanCertificateSSLDirectory(false)
	if err != nil {
		t.Fatalf("ScanCertificateSSLDirectory returned error: %v", err)
	}
	if len(pairs) != 0 {
		t.Fatalf("expected no pairs for missing ssl root, got %d", len(pairs))
	}
}

func TestDiscoverCertificatePairDoesNotAutoDetectCombinedSinglePEM(t *testing.T) {
	confDir := withImportTestNginxConfigDir(t)
	dir := filepath.Join(confDir, "ssl", "combined.example.local")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("create certificate directory: %v", err)
	}

	certPEM, keyPEM, err := GenerateSelfSigned(SelfSignedOptions{
		CommonName:   filepath.Base(dir),
		DNSNames:     []string{filepath.Base(dir)},
		KeyType:      certcrypto.EC256,
		ValidityDays: 365,
	})
	if err != nil {
		t.Fatalf("generate test certificate: %v", err)
	}

	combined := append(append([]byte{}, certPEM...), keyPEM...)
	if err := os.WriteFile(filepath.Join(dir, "fullchain.pem"), combined, 0o600); err != nil {
		t.Fatalf("write combined pem: %v", err)
	}

	_, err = DiscoverCertificatePair(dir)
	if err == nil {
		t.Fatalf("expected combined single-PEM file not to be auto-detected")
	}
	var cosyErr *cosy.Error
	if !errors.As(err, &cosyErr) {
		t.Fatalf("expected cosy error, got %T: %v", err, err)
	}
	if cosyErr.Scope != "cert" || cosyErr.Code != 50044 {
		t.Fatalf("unexpected discovery error scope/code: %s/%d", cosyErr.Scope, cosyErr.Code)
	}
}
