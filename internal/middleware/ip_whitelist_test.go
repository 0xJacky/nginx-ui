package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func whitelistStatus(t *testing.T, trustedProxies, whitelist []string,
	remoteAddr, forwardedFor string,
) int {
	t.Helper()

	originalWhitelist := settings.AuthSettings.IPWhiteList
	t.Cleanup(func() {
		settings.AuthSettings.IPWhiteList = originalWhitelist
	})
	settings.AuthSettings.IPWhiteList = whitelist

	engine := gin.New()
	require.NoError(t, engine.SetTrustedProxies(trustedProxies))
	engine.Use(IPWhiteList())
	engine.GET("/", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.RemoteAddr = remoteAddr
	if forwardedFor != "" {
		request.Header.Set("X-Forwarded-For", forwardedFor)
	}
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, request)
	return recorder.Code
}

func TestIPWhiteListUsesValidatedForwardedClientIP(t *testing.T) {
	require.Equal(t, http.StatusForbidden, whitelistStatus(t,
		[]string{"127.0.0.1"}, []string{"203.0.113.20"},
		"127.0.0.1:1234", "198.51.100.20"))

	require.Equal(t, http.StatusNoContent, whitelistStatus(t,
		[]string{"127.0.0.1"}, []string{"198.51.100.20"},
		"127.0.0.1:1234", "198.51.100.20"))
}

func TestIPWhiteListRejectsSpoofedHeaderFromUntrustedPeer(t *testing.T) {
	require.Equal(t, http.StatusForbidden, whitelistStatus(t,
		nil, []string{"203.0.113.20"},
		"198.51.100.20:1234", "203.0.113.20"))
}

func TestIPWhiteListFailsClosedWhenClientIPIsInvalid(t *testing.T) {
	require.Equal(t, http.StatusForbidden, whitelistStatus(t,
		nil, []string{"203.0.113.20"},
		"invalid-remote-address", ""))
}

func TestIPWhiteListKeepsDirectLoopbackCompatibility(t *testing.T) {
	require.Equal(t, http.StatusNoContent, whitelistStatus(t,
		nil, []string{"203.0.113.20"},
		"127.0.0.1:1234", ""))
	require.Equal(t, http.StatusNoContent, whitelistStatus(t,
		nil, []string{"2001:db8::20"},
		"[::1]:1234", ""))
}
