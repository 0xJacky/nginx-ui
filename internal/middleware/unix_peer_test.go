package middleware

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func unixPeerRequest(t *testing.T, network, remoteAddr string, headers map[string]string) (string, int) {
	t.Helper()
	engine := gin.New()
	require.NoError(t, engine.SetTrustedProxies(nil))
	engine.Use(UnixPeerAddr(), func(c *gin.Context) {
		// Record the IP the allowlist will see, even when it then aborts.
		c.Header("X-Test-Client-IP", c.ClientIP())
		c.Next()
	}, IPWhiteList())
	engine.GET("/", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.RemoteAddr = remoteAddr
	for k, v := range headers {
		request.Header.Set(k, v)
	}
	var local net.Addr = &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 9000}
	if network == "unix" {
		local = &net.UnixAddr{Net: "unix", Name: "/run/nginx-ui.sock"}
	}
	request = request.WithContext(context.WithValue(request.Context(), http.LocalAddrContextKey, local))
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, request)
	return recorder.Header().Get("X-Test-Client-IP"), recorder.Code
}

func TestUnixPeerAddrDirectClientLooksLocal(t *testing.T) {
	originalWhitelist := settings.AuthSettings.IPWhiteList
	t.Cleanup(func() { settings.AuthSettings.IPWhiteList = originalWhitelist })
	settings.AuthSettings.IPWhiteList = []string{"203.0.113.20"}

	for _, remote := range []string{"", "@"} {
		ip, code := unixPeerRequest(t, "unix", remote, nil)
		require.Equal(t, "127.0.0.1", ip)
		require.Equal(t, http.StatusOK, code, "direct socket clients bypass the allowlist like loopback")
	}
}

func TestUnixPeerAddrKeepsForwardedClientIP(t *testing.T) {
	originalWhitelist := settings.AuthSettings.IPWhiteList
	t.Cleanup(func() { settings.AuthSettings.IPWhiteList = originalWhitelist })
	settings.AuthSettings.IPWhiteList = []string{"203.0.113.20"}

	ip, code := unixPeerRequest(t, "unix", "@", map[string]string{"X-Forwarded-For": "203.0.113.20"})
	require.Equal(t, "203.0.113.20", ip)
	require.Equal(t, http.StatusOK, code)

	ip, code = unixPeerRequest(t, "unix", "@", map[string]string{"X-Real-IP": "198.51.100.7"})
	require.Equal(t, "198.51.100.7", ip)
	require.Equal(t, http.StatusForbidden, code)
}

func TestUnixPeerAddrIgnoresTCPConnections(t *testing.T) {
	originalWhitelist := settings.AuthSettings.IPWhiteList
	t.Cleanup(func() { settings.AuthSettings.IPWhiteList = originalWhitelist })
	settings.AuthSettings.IPWhiteList = []string{"203.0.113.20"}

	// A TCP peer with an unparsable address stays invalid and fails closed.
	ip, code := unixPeerRequest(t, "tcp", "invalid-remote-address", nil)
	require.Equal(t, "", ip)
	require.Equal(t, http.StatusForbidden, code)

	// An untrusted TCP peer cannot spoof through the headers.
	ip, code = unixPeerRequest(t, "tcp", "198.51.100.20:1234", map[string]string{"X-Real-IP": "127.0.0.1"})
	require.Equal(t, "198.51.100.20", ip)
	require.Equal(t, http.StatusForbidden, code)
}
