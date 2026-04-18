package middleware

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	internalSystem "github.com/0xJacky/Nginx-UI/internal/system"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	cSettings "github.com/uozi-tech/cosy/settings"
)

func setupInstallSecretMiddlewareTest(t *testing.T) string {
	t.Helper()

	confDir := t.TempDir()
	confPath := filepath.Join(confDir, "app.ini")
	require.NoError(t, os.WriteFile(confPath, []byte("[app]\n"), 0644))

	cSettings.ConfPath = confPath
	cSettings.AppSettings.JwtSecret = ""
	settings.NodeSettings.SkipInstallation = false
	internalSystem.SetInstallStartupTimeForTest(time.Now())
	internalSystem.CleanupInstallSecret()
	require.NoError(t, internalSystem.EnsureInstallSecret())

	t.Cleanup(func() {
		_ = internalSystem.CleanupInstallSecret()
		cSettings.AppSettings.JwtSecret = ""
		settings.NodeSettings.SkipInstallation = false
		internalSystem.SetInstallStartupTimeForTest(time.Now())
	})

	data, err := os.ReadFile(internalSystem.InstallSecretPath())
	require.NoError(t, err)
	return strings.TrimSpace(string(data))
}

func TestSetupAuthRequiredRejectsMissingSecret(t *testing.T) {
	setupInstallSecretMiddlewareTest(t)

	r := gin.New()
	r.POST("/setup/install", SetupAuthRequired(), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/setup/install", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusInternalServerError, w.Code)
	require.Contains(t, w.Body.String(), "\"code\":40309")
}

func TestSetupAuthRequiredAcceptsHeader(t *testing.T) {
	secret := setupInstallSecretMiddlewareTest(t)

	r := gin.New()
	r.POST("/setup/install", SetupAuthRequired(), func(c *gin.Context) {
		_, hasUser := c.Get("user")
		require.True(t, hasUser)
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/setup/install", nil)
	req.Header.Set(internalSystem.InstallSecretHeaderName, secret)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
}

func TestSetupAuthRequiredAcceptsQuerySecretDuringFirstRun(t *testing.T) {
	secret := setupInstallSecretMiddlewareTest(t)

	r := gin.New()
	r.GET("/setup/self_check/websocket", SetupAuthRequired(), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/setup/self_check/websocket?install_secret="+secret, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
}

func TestSetupAuthRequiredRejectsInstalledInstances(t *testing.T) {
	setupInstallSecretMiddlewareTest(t)

	r := gin.New()
	r.GET("/setup/self_check", SetupAuthRequired(), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/setup/self_check", nil)
	w := httptest.NewRecorder()

	cSettings.AppSettings.JwtSecret = "installed"
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusInternalServerError, w.Code)
	require.Contains(t, w.Body.String(), "\"code\":40301")
}

func TestSetupAuthRequiredRejectsTimedOutInstances(t *testing.T) {
	setupInstallSecretMiddlewareTest(t)

	r := gin.New()
	r.GET("/setup/self_check", SetupAuthRequired(), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/setup/self_check", nil)
	req.Header.Set(internalSystem.InstallSecretHeaderName, "test-secret")
	w := httptest.NewRecorder()

	internalSystem.SetInstallStartupTimeForTest(time.Now().Add(-internalSystem.InstallWindow - time.Second))
	t.Cleanup(func() {
		internalSystem.SetInstallStartupTimeForTest(time.Now())
	})

	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusInternalServerError, w.Code)
	require.Contains(t, w.Body.String(), "\"code\":40302")
}
