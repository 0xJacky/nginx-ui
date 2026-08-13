package nodeauth

import (
	"crypto/ed25519"
	"crypto/rand"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupSignatureTest(t *testing.T) (*gorm.DB, ed25519.PrivateKey, time.Time) {
	t.Helper()
	originalInstanceID := settings.NodeSettings.InstanceID
	originalCryptoSecret := settings.CryptoSettings.Secret
	t.Cleanup(func() {
		settings.NodeSettings.InstanceID = originalInstanceID
		settings.CryptoSettings.Secret = originalCryptoSecret
		model.Use(nil)
	})
	settings.NodeSettings.InstanceID = "11111111-1111-4111-8111-111111111111"
	settings.CryptoSettings.Secret = "signature-test-root"

	database, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, database.AutoMigrate(&model.NodeControllerCredential{}))
	model.Use(database)

	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	require.NoError(t, database.Create(&model.NodeControllerCredential{
		CredentialID:         "22222222-2222-4222-8222-222222222222",
		ControllerInstanceID: "33333333-3333-4333-8333-333333333333",
		PublicKey:            publicKey,
		Status:               model.NodeCredentialStatusActive,
	}).Error)
	return database, privateKey, time.Unix(2_000_000_000, 0)
}

func newSignedTestRequest(t *testing.T, privateKey ed25519.PrivateKey, now time.Time) *http.Request {
	t.Helper()
	request, err := http.NewRequest(
		http.MethodPost,
		"https://node.example/api/sites/example?reload=true&source=cluster",
		io.NopCloser(strings.NewReader(`{"enabled":true}`)),
	)
	require.NoError(t, err)
	require.NoError(t, SignRequestWithKey(
		request,
		"22222222-2222-4222-8222-222222222222",
		settings.NodeSettings.InstanceID,
		privateKey,
		now,
	))
	return request
}

func TestSignAndVerifyRequest(t *testing.T) {
	database, privateKey, now := setupSignatureTest(t)
	request := newSignedTestRequest(t, privateKey, now)
	t.Cleanup(func() { CloseStagedBody(request) })

	principal, err := verifyRequest(request, database, now, NewReplayCache(100))
	require.NoError(t, err)
	assert.Equal(t, "22222222-2222-4222-8222-222222222222", principal.CredentialID)
	assert.Equal(t, model.NodeAuthMethodPaired, principal.AuthMethod)
	assert.Equal(t, "sha-256=:JrNCayWTdjyW0IkLSnegu/ZtE/xRKwxrE4ojwpDzCio=:", request.Header.Get(contentDigestHeader))
}

func TestVerifyRequestRejectsBodyTampering(t *testing.T) {
	database, privateKey, now := setupSignatureTest(t)
	request := newSignedTestRequest(t, privateKey, now)
	CloseStagedBody(request)
	request.Body = io.NopCloser(strings.NewReader(`{"enabled":false}`))

	_, err := verifyRequest(request, database, now, NewReplayCache(100))
	require.ErrorContains(t, err, "content digest mismatch")
	CloseStagedBody(request)
}

func TestVerifyRequestRejectsWrongTarget(t *testing.T) {
	database, privateKey, now := setupSignatureTest(t)
	request := newSignedTestRequest(t, privateKey, now)
	t.Cleanup(func() { CloseStagedBody(request) })
	request.Header.Set(TargetInstanceHeader, "44444444-4444-4444-8444-444444444444")

	_, err := verifyRequest(request, database, now, NewReplayCache(100))
	require.ErrorContains(t, err, "target")
}

func TestVerifyRequestEnforcesExpiryAndClockSkew(t *testing.T) {
	database, privateKey, now := setupSignatureTest(t)

	expired := newSignedTestRequest(t, privateKey, now.Add(-2*time.Minute-time.Second))
	_, err := verifyRequest(expired, database, now, NewReplayCache(100))
	require.ErrorContains(t, err, "expired")
	CloseStagedBody(expired)

	withinFutureSkew := newSignedTestRequest(t, privateKey, now.Add(59*time.Second))
	_, err = verifyRequest(withinFutureSkew, database, now, NewReplayCache(100))
	require.NoError(t, err)
	CloseStagedBody(withinFutureSkew)

	beyondFutureSkew := newSignedTestRequest(t, privateKey, now.Add(61*time.Second))
	_, err = verifyRequest(beyondFutureSkew, database, now, NewReplayCache(100))
	require.ErrorContains(t, err, "future")
	CloseStagedBody(beyondFutureSkew)
}

func TestVerifyRequestAcceptsFourSecondClockSkew(t *testing.T) {
	database, privateKey, now := setupSignatureTest(t)

	for _, offset := range []time.Duration{-4 * time.Second, 4 * time.Second} {
		t.Run(offset.String(), func(t *testing.T) {
			request := newSignedTestRequest(t, privateKey, now.Add(offset))
			t.Cleanup(func() { CloseStagedBody(request) })

			_, err := verifyRequest(request, database, now, NewReplayCache(100))
			require.NoError(t, err)
		})
	}
}

func TestVerifyRequestRejectsReplay(t *testing.T) {
	database, privateKey, now := setupSignatureTest(t)
	request := newSignedTestRequest(t, privateKey, now)
	t.Cleanup(func() { CloseStagedBody(request) })
	cache := NewReplayCache(100)

	_, err := verifyRequest(request, database, now, cache)
	require.NoError(t, err)
	_, err = verifyRequest(request, database, now, cache)
	require.ErrorContains(t, err, "already used")
}

func TestVerifyRequestRejectsRevokedCredential(t *testing.T) {
	database, privateKey, now := setupSignatureTest(t)
	require.NoError(t, database.Model(&model.NodeControllerCredential{}).
		Where("credential_id = ?", "22222222-2222-4222-8222-222222222222").
		Updates(map[string]any{
			"revoked_at": now,
			"status":     model.NodeCredentialStatusRevoked,
		}).Error)
	request := newSignedTestRequest(t, privateKey, now)
	t.Cleanup(func() { CloseStagedBody(request) })

	_, err := verifyRequest(request, database, now, NewReplayCache(100))
	require.ErrorContains(t, err, "unknown or revoked")
}

func TestVerifyRequestAcceptsPreviousKeyOnlyDuringRotationGrace(t *testing.T) {
	database, oldPrivateKey, now := setupSignatureTest(t)
	newPublicKey, _, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	grace := now.Add(10 * time.Minute)
	require.NoError(t, database.Model(&model.NodeControllerCredential{}).
		Where("credential_id = ?", "22222222-2222-4222-8222-222222222222").
		Updates(map[string]any{
			"previous_public_key":  ed25519.PrivateKey(oldPrivateKey).Public().(ed25519.PublicKey),
			"previous_valid_until": grace,
			"public_key":           newPublicKey,
		}).Error)

	withinGrace := newSignedTestRequest(t, oldPrivateKey, now)
	_, err = verifyRequest(withinGrace, database, now, NewReplayCache(100))
	require.NoError(t, err)
	CloseStagedBody(withinGrace)

	afterGrace := newSignedTestRequest(t, oldPrivateKey, grace.Add(time.Second))
	_, err = verifyRequest(afterGrace, database, grace.Add(time.Second), NewReplayCache(100))
	require.ErrorContains(t, err, "invalid")
	CloseStagedBody(afterGrace)
}

func TestVerifyRequestRejectsQueryCredentials(t *testing.T) {
	database, privateKey, now := setupSignatureTest(t)
	request := newSignedTestRequest(t, privateKey, now)
	t.Cleanup(func() { CloseStagedBody(request) })
	query := request.URL.Query()
	query.Set("node_secret", "leaked")
	request.URL.RawQuery = query.Encode()

	_, err := verifyRequest(request, database, now, NewReplayCache(100))
	require.ErrorContains(t, err, "query parameters")
}

func TestEmptyContentDigestMatchesRFC9530Encoding(t *testing.T) {
	request, err := http.NewRequest(http.MethodGet, "https://example.test/", nil)
	require.NoError(t, err)
	digest, err := stageRequestBody(request)
	require.NoError(t, err)
	assert.Equal(t, "sha-256=:47DEQpj8HBSa+/TImW+5JCeuQeRkm5NMpJWZG3hSuFU=:", digest)
}
