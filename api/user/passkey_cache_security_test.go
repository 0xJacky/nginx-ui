package user

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/cache"
	internalcrypto "github.com/0xJacky/Nginx-UI/internal/crypto"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupPasskeyCacheTest(t *testing.T) {
	t.Helper()
	cache.InitInMemoryCache()
	t.Cleanup(cache.Shutdown)
}

func enablePasskeyForTest(t *testing.T) {
	t.Helper()
	original := *settings.WebAuthnSettings
	*settings.WebAuthnSettings = settings.WebAuthn{
		RPDisplayName: "NGINX UI",
		RPID:          "example.com",
		RPOrigins:     []string{"https://example.com"},
	}
	t.Cleanup(func() {
		*settings.WebAuthnSettings = original
	})
}

func TestPasskeySessionIDRequiresCanonicalUUID(t *testing.T) {
	valid := uuid.NewString()
	tests := []struct {
		name      string
		sessionID string
		want      bool
	}{
		{name: "canonical UUID", sessionID: valid, want: true},
		{name: "empty", sessionID: "", want: false},
		{name: "shared cache key", sessionID: internalcrypto.CacheKey, want: false},
		{name: "surrounding whitespace", sessionID: " " + valid + " ", want: false},
		{name: "uppercase", sessionID: strings.ToUpper(valid), want: false},
		{name: "UUID without hyphens", sessionID: strings.ReplaceAll(valid, "-", ""), want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, isCanonicalSessionID(tt.sessionID))
		})
	}
}

func TestPasskeyLoginCannotConsumeSharedCryptoCacheKey(t *testing.T) {
	setupPasskeyCacheTest(t)
	cryptoParams, err := internalcrypto.GetCryptoParams()
	require.NoError(t, err)
	publicKeyBlock, _ := pem.Decode([]byte(cryptoParams.PublicKey))
	require.NotNil(t, publicKeyBlock)
	publicKey, err := x509.ParsePKCS1PublicKey(publicKeyBlock.Bytes)
	require.NoError(t, err)
	ciphertext, err := rsa.EncryptPKCS1v15(rand.Reader, publicKey, []byte(`{"name":"admin","password":"secret"}`))
	require.NoError(t, err)

	var sessionData *webauthn.SessionData
	var ok bool
	require.NotPanics(t, func() {
		sessionData, ok = takeWebAuthnSession(internalcrypto.CacheKey, buildPasskeyLoginKey)
	})
	assert.Nil(t, sessionData)
	assert.False(t, ok)

	actual, found := cache.Get(internalcrypto.CacheKey)
	require.True(t, found)
	assert.Same(t, cryptoParams, actual)
	decrypted, err := internalcrypto.Decrypt(base64.StdEncoding.EncodeToString(ciphertext))
	require.NoError(t, err)
	assert.Equal(t, "admin", decrypted["name"])
	assert.Equal(t, "secret", decrypted["password"])
}

func TestPasskeySessionNamespacesAreIsolated(t *testing.T) {
	setupPasskeyCacheTest(t)
	sessionID := uuid.NewString()
	loginSession := &webauthn.SessionData{Challenge: "login"}
	secureSession := &webauthn.SessionData{Challenge: "secure-session"}
	cache.Set(buildPasskeyLoginKey(sessionID), loginSession, time.Minute)
	cache.Set(buildPasskeySecureSessionKey(sessionID), secureSession, time.Minute)

	actualLogin, ok := takeWebAuthnSession(sessionID, buildPasskeyLoginKey)
	require.True(t, ok)
	assert.Same(t, loginSession, actualLogin)
	_, loginStillExists := cache.Get(buildPasskeyLoginKey(sessionID))
	assert.False(t, loginStillExists)
	_, secureStillExists := cache.Get(buildPasskeySecureSessionKey(sessionID))
	assert.True(t, secureStillExists)

	actualSecure, ok := takeWebAuthnSession(sessionID, buildPasskeySecureSessionKey)
	require.True(t, ok)
	assert.Same(t, secureSession, actualSecure)
}

func TestPasskeySessionWrongTypesReturnControlledMiss(t *testing.T) {
	setupPasskeyCacheTest(t)

	tests := []struct {
		name     string
		buildKey func(string) string
	}{
		{name: "login", buildKey: buildPasskeyLoginKey},
		{name: "secure session", buildKey: buildPasskeySecureSessionKey},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sessionID := uuid.NewString()
			cache.Set(tt.buildKey(sessionID), "unexpected", time.Minute)

			assert.NotPanics(t, func() {
				sessionData, ok := takeWebAuthnSession(sessionID, tt.buildKey)
				assert.Nil(t, sessionData)
				assert.False(t, ok)
			})
		})
	}
}

func TestPasskeyRegistrationSessionWrongTypeReturnsControlledMiss(t *testing.T) {
	setupPasskeyCacheTest(t)
	cache.Set(buildCachePasskeyRegKey(42), "unexpected", time.Minute)

	assert.NotPanics(t, func() {
		sessionData, ok := takePasskeyRegistrationSession(42)
		assert.Nil(t, sessionData)
		assert.False(t, ok)
	})
}

func TestPasskeyFinishHandlersRejectMaliciousSessionHeaderWithoutEviction(t *testing.T) {
	gin.SetMode(gin.TestMode)
	setupPasskeyCacheTest(t)
	enablePasskeyForTest(t)
	cryptoParams := &internalcrypto.Params{PrivateKey: "private", PublicKey: "public"}

	tests := []struct {
		name    string
		handler gin.HandlerFunc
	}{
		{name: "login", handler: FinishPasskeyLogin},
		{name: "2FA secure session", handler: FinishStart2FASecureSessionByPasskey},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cache.Set(internalcrypto.CacheKey, cryptoParams, time.Minute)
			router := gin.New()
			router.POST("/finish", tt.handler)
			request := httptest.NewRequest(http.MethodPost, "/finish", nil)
			request.Header.Set("X-Passkey-Session-ID", internalcrypto.CacheKey)
			recorder := httptest.NewRecorder()

			require.NotPanics(t, func() {
				router.ServeHTTP(recorder, request)
			})
			assert.Contains(t, recorder.Body.String(), "session not found")
			actual, found := cache.Get(internalcrypto.CacheKey)
			require.True(t, found)
			assert.Same(t, cryptoParams, actual)
		})
	}
}
