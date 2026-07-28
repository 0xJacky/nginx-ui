package middleware

import (
	"encoding/base64"
	"net/http"
	"testing"

	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetTokenWS_NoCookieFallback(t *testing.T) {
	t.Run("reads from Authorization header", func(t *testing.T) {
		c := newTestGinContext(t, "GET", "/ws", nil)
		c.Request.Header.Set("Authorization", "jwt-token-here")

		token := getTokenWS(c)
		assert.Equal(t, "jwt-token-here", token)
	})

	t.Run("reads short token from query", func(t *testing.T) {
		c := newTestGinContext(t, "GET", "/ws?token=abcdef1234567890", nil)

		token := getTokenWS(c)
		assert.Equal(t, "abcdef1234567890", token)
	})

	t.Run("decodes long base64 token from query", func(t *testing.T) {
		jwt := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.test"
		encoded := base64.StdEncoding.EncodeToString([]byte(jwt))

		c := newTestGinContext(t, "GET", "/ws?token="+encoded, nil)

		token := getTokenWS(c)
		assert.Equal(t, jwt, token)
	})

	t.Run("decodes URL-safe base64 token from query", func(t *testing.T) {
		// Pick a payload long enough that its encoded form is > 16 chars and
		// whose std-base64 encoding contains `+` / `/` so the URL-safe variant
		// differs (would have been corrupted by `c.Query` decoding `+` as space).
		jwt := "eyJhbGciOiJIUzI1NiJ9.test??>>>>>>>>"
		urlSafe := base64.RawURLEncoding.EncodeToString([]byte(jwt))
		std := base64.StdEncoding.EncodeToString([]byte(jwt))
		assert.NotEqual(t, urlSafe, std, "test payload must encode differently in std vs URL-safe")
		assert.Greater(t, len(urlSafe), 16, "encoded payload must be > 16 chars to hit the long-token branch")

		c := newTestGinContext(t, "GET", "/ws?token="+urlSafe, nil)

		token := getTokenWS(c)
		assert.Equal(t, jwt, token)
	})

	t.Run("does NOT read from cookie", func(t *testing.T) {
		c := newTestGinContext(t, "GET", "/ws", nil)
		c.Request.AddCookie(&http.Cookie{Name: "token", Value: "cookie-jwt-token"})

		token := getTokenWS(c)
		assert.Empty(t, token, "getTokenWS must not fall back to cookie")
	})
}

func TestGetToken_RequiresAuthorizationHeader(t *testing.T) {
	t.Run("reads from Authorization header", func(t *testing.T) {
		c := newTestGinContext(t, "GET", "/api/test", nil)
		c.Request.Header.Set("Authorization", "jwt-token-here")

		token := getToken(c)
		assert.Equal(t, "jwt-token-here", token)
	})

	t.Run("does not read from cookie", func(t *testing.T) {
		c := newTestGinContext(t, "GET", "/api/test", nil)
		c.Request.AddCookie(&http.Cookie{Name: "token", Value: "cookie-jwt-token"})

		token := getToken(c)
		assert.Empty(t, token)
	})

	t.Run("does not read from query", func(t *testing.T) {
		encoded := base64.StdEncoding.EncodeToString([]byte("jwt-token-here"))
		c := newTestGinContext(t, "GET", "/api/test?token="+encoded, nil)

		token := getToken(c)
		assert.Empty(t, token)
	})
}

func TestAuthenticateNodeRequestRejectsQuerySecretEvenWhenItMatches(t *testing.T) {
	originalSecret := settings.NodeSettings.Secret
	originalEnabled := settings.NodeSettings.LegacyAuthEnabled
	t.Cleanup(func() {
		settings.NodeSettings.Secret = originalSecret
		settings.NodeSettings.LegacyAuthEnabled = originalEnabled
	})
	settings.NodeSettings.Secret = "matching-legacy-secret"
	settings.NodeSettings.LegacyAuthEnabled = true

	c := newTestGinContext(t, "GET", "/api/node?node_secret=matching-legacy-secret", nil)
	handled, err := authenticateNodeRequest(c)

	require.Error(t, err)
	assert.True(t, handled)
	assert.ErrorContains(t, err, "query parameters")
}
