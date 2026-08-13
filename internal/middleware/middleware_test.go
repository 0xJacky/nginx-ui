package middleware

import (
	"encoding/base64"
	"errors"
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
	t.Cleanup(func() {
		settings.NodeSettings.Secret = originalSecret
	})
	settings.NodeSettings.Secret = "matching-legacy-secret"

	c := newTestGinContext(t, "GET", "/api/node?node_secret=matching-legacy-secret", nil)
	handled, err := authenticateNodeRequest(c)

	require.Error(t, err)
	assert.True(t, handled)
	assert.ErrorContains(t, err, "query parameters")
}

func TestNodeAuthenticationFailureLogExcludesCredentials(t *testing.T) {
	const secret = "must-not-appear-in-node-authentication-log"
	c := newTestGinContext(t, http.MethodGet, "/api/node?node_secret="+secret, nil)
	c.Request.RemoteAddr = "192.0.2.10:54321"
	c.Request.Header.Set("Signature", secret)
	c.Request.Header.Set("Signature-Input", secret)
	c.Request.Header.Set("X-Node-Secret", secret)

	failure := newNodeAuthenticationFailureLog(c, "signature", errors.New("node signature is expired"))

	assert.Equal(t, "signature", failure.CredentialType)
	assert.Equal(t, http.MethodGet, failure.Method)
	assert.Equal(t, "/api/node", failure.Path)
	assert.Equal(t, "192.0.2.10", failure.RemoteIP)
	assert.Equal(t, "node signature is expired", failure.Reason)
	assert.NotContains(t, failure.Path, secret)
	assert.NotContains(t, failure.Reason, secret)
}
