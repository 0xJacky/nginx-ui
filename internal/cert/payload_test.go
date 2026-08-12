package cert

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/go-acme/lego/v5/certcrypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// useTempNginxConfDir points the nginx configuration directory at a temporary
// directory for the duration of a test.
func useTempNginxConfDir(t *testing.T) string {
	t.Helper()

	confDir := t.TempDir()
	originalConfigDir := settings.NginxSettings.ConfigDir
	settings.NginxSettings.ConfigDir = confDir
	t.Cleanup(func() {
		settings.NginxSettings.ConfigDir = originalConfigDir
	})

	return confDir
}

// usePayloadTestDB gives the test a private in-memory database holding the
// Cert table.
func usePayloadTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Cert{}))
	model.Use(db)
	t.Cleanup(func() { model.Use(nil) })

	return db
}

func TestUseExistingCertificatePathsPinsRenewalToTheConfiguredFiles(t *testing.T) {
	confDir := useTempNginxConfDir(t)

	// The record was issued by an older release, whose key type constants came
	// from lego v4 ("P256" instead of "EC256"), and its identifier list has
	// since been edited.
	legacyDir := filepath.Join(confDir, "ssl", "example.com_P256")
	certPath := filepath.Join(legacyDir, "fullchain.cer")
	keyPath := filepath.Join(legacyDir, "private.key")

	payload := &ConfigPayload{
		ServerName: []string{"example.com", "www.example.com"},
		KeyType:    certcrypto.EC256,
	}

	// Without the existing paths the renewal would target a brand-new directory.
	derived := payload.GetCertificatePath()
	assert.NotEqual(t, certPath, derived, "test premise: the derived path must differ")

	payload = &ConfigPayload{
		ServerName: []string{"example.com", "www.example.com"},
		KeyType:    certcrypto.EC256,
	}
	payload.UseExistingCertificatePaths(certPath, keyPath)

	assert.Equal(t, certPath, payload.GetCertificatePath())
	assert.Equal(t, keyPath, payload.GetCertificateKeyPath())
	assert.Equal(t, legacyDir, payload.getCertificateDirPath())
}

func TestUseExistingCertificatePathsIgnoresIncompleteOrEscapingPaths(t *testing.T) {
	confDir := useTempNginxConfDir(t)
	certPath := filepath.Join(confDir, "ssl", "example.com_EC256", "fullchain.cer")

	tests := []struct {
		name    string
		cert    string
		certKey string
	}{
		{name: "certificate path missing", cert: "", certKey: filepath.Join(confDir, "ssl", "private.key")},
		{name: "key path missing", cert: certPath, certKey: ""},
		{name: "certificate outside conf dir", cert: "/tmp/evil/fullchain.cer", certKey: filepath.Join(confDir, "ssl", "private.key")},
		{name: "key outside conf dir", cert: certPath, certKey: "/tmp/evil/private.key"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload := &ConfigPayload{
				ServerName: []string{"example.com"},
				KeyType:    certcrypto.EC256,
			}
			payload.UseExistingCertificatePaths(tt.cert, tt.certKey)

			assert.Empty(t, payload.SSLCertificatePath)
			assert.Empty(t, payload.SSLCertificateKeyPath)
			assert.Empty(t, payload.CertificateDir)
		})
	}
}

// TestWriteFileReplacesTheCertificateReferencedByNginx is the regression test
// for issue #1794: a renewal has to overwrite the files the nginx configuration
// points at and leave the database in sync with what is on disk, even when the
// identifier list or the key type no longer produce the original directory name.
func TestWriteFileReplacesTheCertificateReferencedByNginx(t *testing.T) {
	confDir := useTempNginxConfDir(t)
	db := usePayloadTestDB(t)

	// Files as they exist on disk today, in the directory an older release
	// derived from the lego v4 key type name and the original domain list.
	legacyDir := filepath.Join(confDir, "ssl", "example.com_P256")
	require.NoError(t, os.MkdirAll(legacyDir, 0o755))
	certPath := filepath.Join(legacyDir, "fullchain.cer")
	keyPath := filepath.Join(legacyDir, "private.key")

	oldCertPEM, oldKeyPEM, err := GenerateSelfSigned(SelfSignedOptions{
		CommonName: "example.com",
		DNSNames:   []string{"example.com"},
		KeyType:    certcrypto.EC256,
	})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(certPath, oldCertPEM, 0o644))
	require.NoError(t, os.WriteFile(keyPath, oldKeyPEM, 0o600))

	staleRenewAt := time.Now().Add(24 * time.Hour)
	certModel := model.Cert{
		Name:                         "example.com",
		Filename:                     "example.com",
		Domains:                      []string{"example.com", "www.example.com"},
		KeyType:                      certcrypto.EC256,
		SSLCertificatePath:           certPath,
		SSLCertificateKeyPath:        keyPath,
		AutoCert:                     model.AutoCertEnabled,
		NextAutoRenewAt:              &staleRenewAt,
		LastRenewalInfoCheckAt:       &staleRenewAt,
		AutoRenewScheduleFingerprint: "stale-ari-fingerprint",
	}
	require.NoError(t, db.Create(&certModel).Error)

	// The freshly renewed material.
	newCertPEM, newKeyPEM, err := GenerateSelfSigned(SelfSignedOptions{
		CommonName: "example.com",
		DNSNames:   []string{"example.com", "www.example.com"},
		KeyType:    certcrypto.EC256,
	})
	require.NoError(t, err)

	payload := &ConfigPayload{
		CertID:     certModel.ID,
		ServerName: certModel.Domains,
		KeyType:    certModel.GetKeyType(),
		Resource: &model.CertificateResource{
			Certificate: newCertPEM,
			PrivateKey:  newKeyPEM,
		},
	}
	payload.UseExistingCertificatePaths(certModel.SSLCertificatePath, certModel.SSLCertificateKeyPath)

	log := NewLogger()
	require.NoError(t, payload.WriteFile(log))
	log.Close()

	// The file nginx loads now holds the renewed certificate.
	gotCert, err := os.ReadFile(certPath)
	require.NoError(t, err)
	assert.Equal(t, string(newCertPEM), string(gotCert), "certificate on disk was not replaced")

	gotKey, err := os.ReadFile(keyPath)
	require.NoError(t, err)
	assert.Equal(t, string(newKeyPEM), string(gotKey), "private key on disk was not replaced")

	// No second copy was created next to it.
	entries, err := os.ReadDir(filepath.Join(confDir, "ssl"))
	require.NoError(t, err)
	require.Len(t, entries, 1, "renewal must not create a second certificate directory")
	assert.Equal(t, "example.com_P256", entries[0].Name())

	// The database still points at the same files and describes what is on disk.
	var stored model.Cert
	require.NoError(t, db.First(&stored, certModel.ID).Error)
	assert.Equal(t, certPath, stored.SSLCertificatePath)
	assert.Equal(t, keyPath, stored.SSLCertificateKeyPath)

	diskFingerprint, err := CertificateFingerprintFromPath(certPath)
	require.NoError(t, err)
	assert.Equal(t, diskFingerprint, stored.Fingerprint,
		"database fingerprint must match the certificate on disk")
	require.NotNil(t, stored.Resource)
	assert.Equal(t, string(newCertPEM), string(stored.Resource.Certificate),
		"database resource must match the certificate on disk")

	// The ARI renewal schedule describes the certificate that was just replaced.
	assert.Nil(t, stored.NextAutoRenewAt)
	assert.Nil(t, stored.LastRenewalInfoCheckAt)
	assert.Empty(t, stored.AutoRenewScheduleFingerprint)
}

// TestWriteFileWithoutExistingPathsUsesTheDerivedDirectory guards the initial
// issuance path, where no file has been written yet.
func TestWriteFileWithoutExistingPathsUsesTheDerivedDirectory(t *testing.T) {
	confDir := useTempNginxConfDir(t)
	usePayloadTestDB(t)

	certPEM, keyPEM, err := GenerateSelfSigned(SelfSignedOptions{
		CommonName: "fresh.example.com",
		DNSNames:   []string{"fresh.example.com"},
		KeyType:    certcrypto.EC256,
	})
	require.NoError(t, err)

	payload := &ConfigPayload{
		ServerName: []string{"fresh.example.com"},
		KeyType:    certcrypto.EC256,
		Resource: &model.CertificateResource{
			Certificate: certPEM,
			PrivateKey:  keyPEM,
		},
	}
	payload.UseExistingCertificatePaths("", "")

	log := NewLogger()
	require.NoError(t, payload.WriteFile(log))
	log.Close()

	want := filepath.Join(confDir, "ssl", "fresh.example.com_"+string(certcrypto.EC256), "fullchain.cer")
	assert.Equal(t, want, payload.GetCertificatePath())

	gotCert, err := os.ReadFile(want)
	require.NoError(t, err)
	assert.Equal(t, string(certPEM), string(gotCert))
}
