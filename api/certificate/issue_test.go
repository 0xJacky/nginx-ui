package certificate

import (
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/0xJacky/Nginx-UI/internal/cert"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/go-acme/lego/v5/certcrypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupCertTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	// Use a per-test private in-memory DB. The literal ":memory:" (no shared cache)
	// gives each gorm.Open a fresh isolated database, preventing cross-test pollution.
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Cert{}))
	model.Use(db)
	t.Cleanup(func() { model.Use(nil) })
	return db
}

func TestPersistCertDraftCreatesPendingRecord(t *testing.T) {
	db := setupCertTestDB(t)

	payload := &cert.ConfigPayload{
		ServerName:              []string{"example.com", "*.example.com"},
		ChallengeMethod:         "dns01",
		DNSCredentialID:         42,
		ACMEUserID:              7,
		KeyType:                 certcrypto.RSA2048,
		MustStaple:              true,
		LegoDisableCNAMESupport: true,
		RevokeOld:               true,
	}

	got, err := persistCertDraft("example.com", payload)
	require.NoError(t, err)
	assert.NotZero(t, got.ID)
	assert.Equal(t, model.CertStatusPending, got.Status)
	assert.Equal(t, "", got.LastError)
	assert.NotNil(t, got.LastAttemptAt)
	assert.WithinDuration(t, time.Now(), *got.LastAttemptAt, 5*time.Second)

	var fromDB model.Cert
	require.NoError(t, db.First(&fromDB, got.ID).Error)
	assert.Equal(t, []string{"example.com", "*.example.com"}, fromDB.Domains)
	assert.Equal(t, "dns01", fromDB.ChallengeMethod)
	assert.Equal(t, uint64(42), fromDB.DnsCredentialID)
	assert.Equal(t, uint64(7), fromDB.ACMEUserID)
	assert.True(t, fromDB.MustStaple)
	assert.True(t, fromDB.LegoDisableCNAMESupport)
	assert.True(t, fromDB.RevokeOld)
	assert.Equal(t, model.AutoCertEnabled, fromDB.AutoCert)
}

func TestPersistCertDraftReusesExistingRow(t *testing.T) {
	db := setupCertTestDB(t)
	existing := model.Cert{
		Name:      "example.com",
		Filename:  "example.com",
		KeyType:   certcrypto.RSA2048,
		Status:    model.CertStatusFailure,
		LastError: "prior failure",
	}
	require.NoError(t, db.Create(&existing).Error)

	payload := &cert.ConfigPayload{
		ServerName:      []string{"example.com"},
		ChallengeMethod: "http01",
		KeyType:         certcrypto.RSA2048,
	}

	got, err := persistCertDraft("example.com", payload)
	require.NoError(t, err)
	assert.Equal(t, existing.ID, got.ID)
	assert.Equal(t, model.CertStatusPending, got.Status)
	assert.Equal(t, "", got.LastError)

	var count int64
	require.NoError(t, db.Model(&model.Cert{}).Where("name = ?", "example.com").Count(&count).Error)
	assert.Equal(t, int64(1), count, "should reuse, not duplicate")
}

func TestMarkCertFailureSetsStatusAndError(t *testing.T) {
	db := setupCertTestDB(t)
	c := model.Cert{Name: "example.com", Filename: "example.com", Status: model.CertStatusPending}
	require.NoError(t, db.Create(&c).Error)

	markCertFailure(c.ID, "DNS challenge timed out after 60s")

	var got model.Cert
	require.NoError(t, db.First(&got, c.ID).Error)
	assert.Equal(t, model.CertStatusFailure, got.Status)
	assert.Equal(t, "DNS challenge timed out after 60s", got.LastError)
}

func TestMarkCertFailureDoesNotClobberResourceOrPaths(t *testing.T) {
	db := setupCertTestDB(t)
	c := model.Cert{
		Name:                  "example.com",
		Filename:              "example.com",
		Status:                model.CertStatusPending,
		SSLCertificatePath:    "/etc/nginx/ssl/example.com/fullchain.cer",
		SSLCertificateKeyPath: "/etc/nginx/ssl/example.com/private.key",
	}
	require.NoError(t, db.Create(&c).Error)

	markCertFailure(c.ID, "renewal failed")

	var got model.Cert
	require.NoError(t, db.First(&got, c.ID).Error)
	assert.Equal(t, "/etc/nginx/ssl/example.com/fullchain.cer", got.SSLCertificatePath, "must not erase paths")
	assert.Equal(t, "/etc/nginx/ssl/example.com/private.key", got.SSLCertificateKeyPath)
}

func TestMarkCertSuccessClearsLastError(t *testing.T) {
	db := setupCertTestDB(t)
	c := model.Cert{
		Name:      "example.com",
		Filename:  "example.com",
		Status:    model.CertStatusPending,
		LastError: "stale error",
	}
	require.NoError(t, db.Create(&c).Error)

	markCertSuccess(c.ID, "/etc/nginx/ssl/example.com/fullchain.cer", "/etc/nginx/ssl/example.com/private.key", nil)

	var got model.Cert
	require.NoError(t, db.First(&got, c.ID).Error)
	assert.Equal(t, model.CertStatusSuccess, got.Status)
	assert.Equal(t, "", got.LastError)
	assert.Equal(t, "/etc/nginx/ssl/example.com/fullchain.cer", got.SSLCertificatePath)
	assert.Equal(t, "/etc/nginx/ssl/example.com/private.key", got.SSLCertificateKeyPath)
}

func TestShortError(t *testing.T) {
	assert.Equal(t, "", shortError(nil))
	assert.Equal(t, "hello", shortError(errString("  hello  ")))

	long := make([]byte, 600)
	for i := range long {
		long[i] = 'a'
	}
	got := shortError(errString(string(long)))
	// 500 ASCII runes + the literal "…" suffix.
	assert.Equal(t, 500+len("…"), len(got))
	assert.Equal(t, "…", got[len(got)-len("…"):])

	// Multi-byte runes must not be split: each CJK char is 3 bytes,
	// 600 chars = 1800 bytes, which would corrupt the boundary if we
	// sliced by bytes. After rune-aware truncation we expect exactly
	// 500 CJK chars + the "…" suffix, and the result must be valid UTF-8.
	multi := strings.Repeat("中", 600)
	gotMulti := shortError(errString(multi))
	assert.True(t, utf8.ValidString(gotMulti), "truncated message must be valid UTF-8")
	assert.Equal(t, 500+1 /* ellipsis rune */, utf8.RuneCountInString(gotMulti))
	assert.Equal(t, "…", gotMulti[len(gotMulti)-len("…"):])
}

type errString string

func (e errString) Error() string { return string(e) }
