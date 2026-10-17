//go:build unembed

package router

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	nginxlogapi "github.com/0xJacky/Nginx-UI/api/nginx_log"
	"github.com/0xJacky/Nginx-UI/api/sites"
	"github.com/0xJacky/Nginx-UI/api/system"
	"github.com/0xJacky/Nginx-UI/api/upstream"
	"github.com/0xJacky/Nginx-UI/internal/cache"
	"github.com/0xJacky/Nginx-UI/internal/middleware"
	"github.com/0xJacky/Nginx-UI/internal/testdb"
	internaluser "github.com/0xJacky/Nginx-UI/internal/user"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	cSettings "github.com/uozi-tech/cosy/settings"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupMaintenanceSecurityRouter(t *testing.T) (*gin.Engine, string, uint64) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	cache.InitInMemoryCache()
	originalJWTSecret := cSettings.AppSettings.JwtSecret
	originalWhitelist := settings.AuthSettings.IPWhiteList
	originalDemo := settings.NodeSettings.Demo
	t.Cleanup(func() {
		cache.Shutdown()
		cSettings.AppSettings.JwtSecret = originalJWTSecret
		settings.AuthSettings.IPWhiteList = originalWhitelist
		settings.NodeSettings.Demo = originalDemo
		model.Use(nil)
	})
	cSettings.AppSettings.JwtSecret = "maintenance-route-test-secret"
	settings.AuthSettings.IPWhiteList = nil
	settings.NodeSettings.Demo = false

	database, err := gorm.Open(sqlite.Open(testdb.DSN(t)), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, database.AutoMigrate(&model.User{}, &model.AuthToken{}, &model.Passkey{}))
	model.Use(database)
	query.SetDefault(database)
	currentUser := &model.User{
		Name:      "maintenance-admin",
		Status:    true,
		OTPSecret: []byte("maintenance-otp-enabled"),
	}
	require.NoError(t, database.Create(currentUser).Error)
	payload, err := internaluser.GenerateJWT(currentUser)
	require.NoError(t, err)

	router := gin.New()
	// A future routing regression must fail this test without restarting the
	// test process. Other probes deliberately use harmless invalid payloads.
	router.Use(func(c *gin.Context) {
		if c.FullPath() == "/api/system/restart" {
			hasGate := false
			for _, name := range c.HandlerNames() {
				hasGate = hasGate || strings.Contains(name, "middleware.RequireSecureSession.")
			}
			if !hasGate {
				c.AbortWithStatus(http.StatusInternalServerError)
				return
			}
		}
		c.Next()
	})
	group := router.Group("/api", middleware.AuthRequired(), middleware.Proxy())
	system.InitPrivateRouter(group)
	upstream.InitHTTPRouter(group)
	sites.InitRouter(group)
	nginxlogapi.InitRouter(group)
	system.InitWebSocketRouter(router.Group("/api", middleware.AuthRequiredWS(), middleware.ProxyWs()))
	return router, payload.Token, currentUser.ID
}

func TestMaintenanceMutationsRejectMissingAndUnrelatedSecureSessions(t *testing.T) {
	router, token, userID := setupMaintenanceSecurityRouter(t)
	unrelatedSession := internaluser.SetSecureSessionID(userID + 1)
	requests := []struct{ method, path string }{
		{http.MethodPost, "/api/system/restart"},
		{http.MethodGet, "/api/system/restart"},
		{http.MethodGet, "/api/upgrade/perform"},
		{http.MethodPut, "/api/upstream/socket/127.0.0.1:8080"},
		{http.MethodPost, "/api/self_check/missing-task/fix"},
		{http.MethodPost, "/api/nginx_log/index/rebuild"},
		{http.MethodPost, "/api/nginx_log/settings/advanced_indexing/enable"},
		{http.MethodPost, "/api/nginx_log/settings/advanced_indexing/disable"},
		{http.MethodPost, "/api/site_navigation/test_health_check/1"},
	}
	for _, request := range requests {
		t.Run(request.method+" "+request.path, func(t *testing.T) {
			for _, session := range []string{"", unrelatedSession} {
				req := httptest.NewRequest(request.method, request.path, nil)
				req.Header.Set("Authorization", "Bearer "+token)
				if session != "" {
					req.Header.Set("X-Secure-Session-ID", session)
				}
				recorder := httptest.NewRecorder()
				router.ServeHTTP(recorder, req)
				require.Equal(t, http.StatusUnauthorized, recorder.Code, recorder.Body.String())
			}
		})
	}
}

// Invalid payloads and a plain HTTP upgrade probe prove that authorized callers
// still reach the real handlers without running an upgrade or restarting a process.
func TestMaintenanceMutationsAllowVerifiedSessionsToReachHandlers(t *testing.T) {
	router, token, userID := setupMaintenanceSecurityRouter(t)
	secureSession := internaluser.SetSecureSessionID(userID)
	requests := []struct{ method, path string }{
		{http.MethodGet, "/api/upgrade/perform"},
		{http.MethodPut, "/api/upstream/socket/127.0.0.1:8080"},
		{http.MethodPost, "/api/self_check/missing-task/fix"},
		{http.MethodPost, "/api/site_navigation/test_health_check/1"},
	}
	for _, request := range requests {
		t.Run(request.path, func(t *testing.T) {
			req := httptest.NewRequest(request.method, request.path, strings.NewReader("invalid-json"))
			req.Header.Set("Authorization", "Bearer "+token)
			req.Header.Set("X-Secure-Session-ID", secureSession)
			req.Header.Set("Content-Type", "application/json")
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, req)
			require.NotEqual(t, http.StatusUnauthorized, recorder.Code, recorder.Body.String())
			require.NotEqual(t, http.StatusForbidden, recorder.Code, recorder.Body.String())
			require.NotEqual(t, http.StatusNotFound, recorder.Code, recorder.Body.String())
			require.GreaterOrEqual(t, recorder.Code, http.StatusBadRequest, recorder.Body.String())
		})
	}

	// Existing read-only status routes must remain usable without step-up.
	for _, path := range []string{"/api/nginx_log/settings/advanced_indexing/status", "/api/upstream/health_check/status"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, req)
		require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	}
}
