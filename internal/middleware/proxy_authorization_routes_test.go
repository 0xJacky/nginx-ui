package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	internalmcp "github.com/0xJacky/Nginx-UI/internal/mcp"
	"github.com/0xJacky/Nginx-UI/internal/middleware"
	"github.com/0xJacky/Nginx-UI/internal/testdb"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/0xJacky/Nginx-UI/router"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uozi-tech/cosy/logger"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// TestProxyAuthorizationCoversRegisteredRouteGuards registers exactly the
// routers router.InitRouter mounts behind Proxy and ProxyWs, and checks that
// every route guard is enforced against the original caller before
// forwarding. It reports every miss rather than stopping at the first one.
func TestProxyAuthorizationCoversRegisteredRouteGuards(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger.Init("fatal")
	db, err := gorm.Open(sqlite.Open(testdb.DSN(t)), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Node{}))
	model.Use(db)
	query.SetDefault(db)
	t.Cleanup(func() { model.Use(nil) })
	for _, ws := range []bool{false, true} {
		engine := gin.New()
		var handlerNames []string
		probe := func(c *gin.Context) {
			handlerNames = c.HandlerNames()
			c.Set("ProxyNodeID", "1")
			if c.GetHeader("X-Test-Principal") == "service" {
				c.Set(internalmcp.ServiceTokenPrincipalKey, &internalmcp.ServiceTokenPrincipal{Scopes: []string{model.APITokenScopeWrite}})
			} else {
				c.Set("user", &model.User{EnabledTwoFA: true})
			}
			c.Next()
		}
		proxy := middleware.Proxy()
		if ws {
			proxy = middleware.ProxyWs()
		}
		group := engine.Group("/api", probe, proxy)
		registers := router.ProxiedRouters
		if ws {
			registers = router.ProxiedWebSocketRouters
		}
		for _, register := range registers {
			register(group)
		}
		count, interactive := 0, 0
		for _, route := range engine.Routes() {
			path := route.Path
			for _, segment := range strings.Split(path, "/") {
				if strings.HasPrefix(segment, ":") {
					path = strings.ReplaceAll(path, segment, "1")
				}
			}
			request := func(principal string) *httptest.ResponseRecorder {
				req := httptest.NewRequest(route.Method, path, nil)
				req.Header.Set("X-Test-Principal", principal)
				res := httptest.NewRecorder()
				engine.ServeHTTP(res, req)
				return res
			}
			// A missing target stops the proxy before a business handler can execute.
			userResponse := request("user")
			hasSecure, hasInteractive, hasVerified := false, false, false
			for _, name := range handlerNames {
				hasSecure = hasSecure || strings.Contains(name, "middleware.RequireSecureSession.")
				hasInteractive = hasInteractive || strings.Contains(name, "middleware.RequireInteractiveUser.")
				hasVerified = hasVerified || strings.Contains(name, "middleware.RequireVerifiedTwoFactorOrProxy.")
			}
			if userResponse.Code == http.StatusForbidden && strings.Contains(userResponse.Body.String(), "40302") {
				t.Errorf("%s %s carries a guard the node proxy does not enforce", route.Method, route.Path)
				continue
			}
			if !hasSecure && !hasVerified {
				assert.NotEqual(t, http.StatusUnauthorized, userResponse.Code, "unexpected caller step-up policy for unguarded %s %s", route.Method, route.Path)
			}
			if hasSecure || hasVerified {
				count++
				assert.Equal(t, http.StatusUnauthorized, userResponse.Code, "missing caller secure-session policy for %s %s", route.Method, route.Path)
			}
			if hasInteractive {
				interactive++
				assert.Equal(t, http.StatusForbidden, request("service").Code, "missing caller interactive policy for %s %s", route.Method, route.Path)
			} else if hasVerified {
				assert.Equal(t, http.StatusUnauthorized, request("service").Code, "missing caller verified-two-factor policy for %s %s", route.Method, route.Path)
			}
		}
		t.Logf("guarded proxied routes audited (websocket=%v): %d, interactive: %d", ws, count, interactive)
		require.Greater(t, count, 0, "no guarded routes audited (websocket=%v)", ws)
		if !ws {
			require.Greater(t, interactive, 0, "no interactive routes audited")
		}
	}
}
