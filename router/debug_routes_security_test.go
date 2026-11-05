//go:build unembed

package router

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/0xJacky/Nginx-UI/internal/cache"
	internalmcp "github.com/0xJacky/Nginx-UI/internal/mcp"
	"github.com/0xJacky/Nginx-UI/internal/testdb"
	internaluser "github.com/0xJacky/Nginx-UI/internal/user"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uozi-tech/cosy/logger"
	cosyRouter "github.com/uozi-tech/cosy/router"
	cSettings "github.com/uozi-tech/cosy/settings"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// debugRouteCredentials returns an interactive JWT plus api:read and api:write
// service tokens issued by the same administrator, wired into the real router.
func debugRouteCredentials(t *testing.T) map[string]string {
	t.Helper()

	gin.SetMode(gin.TestMode)
	cache.InitInMemoryCache()
	originalJWTSecret := cSettings.AppSettings.JwtSecret
	originalCryptoSecret := settings.CryptoSettings.Secret
	originalInstanceID := settings.NodeSettings.InstanceID
	t.Cleanup(func() {
		cache.Shutdown()
		cSettings.AppSettings.JwtSecret = originalJWTSecret
		settings.CryptoSettings.Secret = originalCryptoSecret
		settings.NodeSettings.InstanceID = originalInstanceID
		model.Use(nil)
	})
	cSettings.AppSettings.JwtSecret = "debug-routes-test-jwt-secret"
	settings.CryptoSettings.Secret = "debug-routes-test-crypto-secret"
	settings.NodeSettings.InstanceID = "dddddddd-dddd-4ddd-8ddd-dddddddddddd"

	database, err := gorm.Open(sqlite.Open(testdb.DSN(t)), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, database.AutoMigrate(&model.User{}, &model.AuthToken{}, &model.Passkey{}, &model.Node{}, &model.MCPServiceToken{}))
	model.Use(database)
	query.SetDefault(database)

	admin := &model.User{Model: model.Model{ID: 1}, Name: "admin", Status: true}
	require.NoError(t, database.Create(admin).Error)
	adminJWT, err := internaluser.GenerateJWT(admin)
	require.NoError(t, err)
	_, readToken, err := internalmcp.CreateServiceToken("reader", []string{model.APITokenScopeRead}, nil, admin.ID)
	require.NoError(t, err)
	_, writeToken, err := internalmcp.CreateServiceToken("writer", []string{model.APITokenScopeWrite}, nil, admin.ID)
	require.NoError(t, err)

	cosyRouter.Init()
	InitRouter()

	return map[string]string{
		"interactive": adminJWT.Token,
		"api:read":    readToken,
		"api:write":   writeToken,
	}
}

// serveAuthenticated sends one request through the production router built
// by debugRouteCredentials. Requests reuse the same engine, as in production;
// the audit middleware keeps each gin context alive until cosy's audit
// goroutine is done with it.
func serveAuthenticated(method, target, credential string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, target, nil)
	request.Header.Set("Authorization", "Bearer "+credential)
	recorder := httptest.NewRecorder()
	cosyRouter.GetEngine().ServeHTTP(recorder, request)
	return recorder
}

// TestDebugRequestTraceRoutesAreNotServed ensures the cosy request-trace
// monitor is not part of the production router: its history and live stream
// carry every audited request body, so no credential may reach it.
func TestDebugRequestTraceRoutesAreNotServed(t *testing.T) {
	credentials := debugRouteCredentials(t)

	for _, route := range cosyRouter.GetEngine().Routes() {
		assert.False(t, strings.HasPrefix(route.Path, "/api/debug"), "unexpected debug route %s %s", route.Method, route.Path)
	}

	targets := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/debug/request/00000000-0000-0000-0000-000000000000"},
		{http.MethodGet, "/api/debug/requests"},
		{http.MethodPost, "/api/debug/requests/search"},
		{http.MethodGet, "/api/debug/monitor?include_requests=true&limit=100"},
		{http.MethodGet, "/api/debug/ws"},
		{http.MethodGet, "/api/debug/pprof/"},
	}
	for name, credential := range credentials {
		for _, target := range targets {
			recorder := serveAuthenticated(target.method, target.path, credential)
			assert.Equal(t, http.StatusNotFound, recorder.Code, "%s %s %s: %s", name, target.method, target.path, recorder.Body.String())
		}
	}
}

// TestDebugMonitorReporterIsNotRegistered guards against the cosy debug
// package being linked back in through any import: its init function installs
// a reporter that copies every audit log entry into an in-memory trace store.
func TestDebugMonitorReporterIsNotRegistered(t *testing.T) {
	assert.Nil(t, logger.GetMonitorReporter())
}

// TestNormalRoutesStillServeServiceTokens confirms the removal did not change
// how ordinary authenticated routes treat interactive users and tokens.
func TestNormalRoutesStillServeServiceTokens(t *testing.T) {
	credentials := debugRouteCredentials(t)

	for name, credential := range credentials {
		recorder := serveAuthenticated(http.MethodGet, "/api/settings/server/name", credential)
		assert.Equal(t, http.StatusOK, recorder.Code, "%s: %s", name, recorder.Body.String())
	}
}
