package router

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func setupClientIPTest(t *testing.T, isOfficialDocker bool, trustedProxies []string) *gin.Engine {
	t.Helper()

	originalTrustedProxies := settings.AuthSettings.TrustedProxies
	t.Cleanup(func() {
		settings.AuthSettings.TrustedProxies = originalTrustedProxies
	})
	settings.AuthSettings.TrustedProxies = trustedProxies

	if isOfficialDocker {
		t.Setenv("NGINX_UI_OFFICIAL_DOCKER", "true")
	} else {
		t.Setenv("NGINX_UI_OFFICIAL_DOCKER", "false")
	}
	t.Setenv("NGINX_UI_DISABLE_BUNDLED_NGINX", "false")

	engine := gin.New()
	require.NoError(t, configureTrustedProxies(engine))
	engine.GET("/client-ip", func(c *gin.Context) {
		c.String(http.StatusOK, c.ClientIP())
	})
	return engine
}

func requestClientIP(t *testing.T, engine *gin.Engine, remoteAddr, forwardedFor, realIP string) string {
	t.Helper()

	request := httptest.NewRequest(http.MethodGet, "/client-ip", nil)
	request.RemoteAddr = remoteAddr
	if forwardedFor != "" {
		request.Header.Set("X-Forwarded-For", forwardedFor)
	}
	if realIP != "" {
		request.Header.Set("X-Real-IP", realIP)
	}
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusOK, recorder.Code)
	return recorder.Body.String()
}

func TestClientIPIgnoresForwardedHeadersWithoutTrustedProxy(t *testing.T) {
	engine := setupClientIPTest(t, false, nil)

	clientIP := requestClientIP(t, engine, "192.0.2.10:1234", "203.0.113.20", "203.0.113.21")

	require.Equal(t, "192.0.2.10", clientIP)
}

func TestOfficialDockerTrustsBundledLoopbackProxy(t *testing.T) {
	engine := setupClientIPTest(t, true, nil)

	require.Equal(t, "198.51.100.20", requestClientIP(t, engine,
		"127.0.0.1:1234", "198.51.100.20", ""))
	require.Equal(t, "198.51.100.21", requestClientIP(t, engine,
		"127.0.0.1:1234", "", "198.51.100.21"))
	require.Equal(t, "2001:db8::20", requestClientIP(t, engine,
		"[::1]:1234", "2001:db8::20", ""))
}

func TestOfficialDockerDoesNotTrustLoopbackWhenBundledNginxIsDisabled(t *testing.T) {
	originalTrustedProxies := settings.AuthSettings.TrustedProxies
	t.Cleanup(func() {
		settings.AuthSettings.TrustedProxies = originalTrustedProxies
	})
	settings.AuthSettings.TrustedProxies = nil
	t.Setenv("NGINX_UI_OFFICIAL_DOCKER", "true")
	t.Setenv("NGINX_UI_DISABLE_BUNDLED_NGINX", "true")

	engine := gin.New()
	require.NoError(t, configureTrustedProxies(engine))
	engine.GET("/client-ip", func(c *gin.Context) {
		c.String(http.StatusOK, c.ClientIP())
	})

	require.Equal(t, "127.0.0.1", requestClientIP(t, engine,
		"127.0.0.1:1234", "198.51.100.20", ""))
}

func TestClientIPStopsAtFirstUntrustedProxy(t *testing.T) {
	engine := setupClientIPTest(t, false, []string{"127.0.0.1"})

	clientIP := requestClientIP(t, engine, "127.0.0.1:1234",
		"203.0.113.99, 198.51.100.10", "")

	require.Equal(t, "198.51.100.10", clientIP)
}

func TestClientIPUsesConfiguredMultiHopProxyChain(t *testing.T) {
	engine := setupClientIPTest(t, false, []string{"127.0.0.1", "198.51.100.0/24"})

	clientIP := requestClientIP(t, engine, "127.0.0.1:1234",
		"203.0.113.20, 198.51.100.10", "")

	require.Equal(t, "203.0.113.20", clientIP)
}

func TestConfigureTrustedProxiesRejectsInvalidNetwork(t *testing.T) {
	t.Setenv("NGINX_UI_OFFICIAL_DOCKER", "false")
	originalTrustedProxies := settings.AuthSettings.TrustedProxies
	t.Cleanup(func() {
		settings.AuthSettings.TrustedProxies = originalTrustedProxies
	})
	settings.AuthSettings.TrustedProxies = []string{"not-a-network"}

	require.Error(t, configureTrustedProxies(gin.New()))
}
