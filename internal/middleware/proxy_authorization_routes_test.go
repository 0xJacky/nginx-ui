package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	accesslist "github.com/0xJacky/Nginx-UI/api/access_list"
	"github.com/0xJacky/Nginx-UI/api/analytic"
	"github.com/0xJacky/Nginx-UI/api/backup"
	"github.com/0xJacky/Nginx-UI/api/certificate"
	"github.com/0xJacky/Nginx-UI/api/cluster"
	"github.com/0xJacky/Nginx-UI/api/config"
	"github.com/0xJacky/Nginx-UI/api/dns"
	"github.com/0xJacky/Nginx-UI/api/event"
	"github.com/0xJacky/Nginx-UI/api/external_notify"
	"github.com/0xJacky/Nginx-UI/api/host"
	"github.com/0xJacky/Nginx-UI/api/ingress"
	"github.com/0xJacky/Nginx-UI/api/llm"
	"github.com/0xJacky/Nginx-UI/api/nginx"
	"github.com/0xJacky/Nginx-UI/api/nginx_log"
	"github.com/0xJacky/Nginx-UI/api/notification"
	settingsapi "github.com/0xJacky/Nginx-UI/api/settings"
	"github.com/0xJacky/Nginx-UI/api/sites"
	"github.com/0xJacky/Nginx-UI/api/snippet"
	"github.com/0xJacky/Nginx-UI/api/streams"
	"github.com/0xJacky/Nginx-UI/api/system"
	"github.com/0xJacky/Nginx-UI/api/template"
	"github.com/0xJacky/Nginx-UI/api/upstream"
	userapi "github.com/0xJacky/Nginx-UI/api/user"
	internalmcp "github.com/0xJacky/Nginx-UI/internal/mcp"
	"github.com/0xJacky/Nginx-UI/internal/middleware"
	"github.com/0xJacky/Nginx-UI/internal/testdb"
	"github.com/0xJacky/Nginx-UI/mcp"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/uozi-tech/cosy/logger"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// Handler names are inspected only in this registration audit. Runtime proxy
// authorization uses explicit method/full-path policy, never function names.
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
		router := gin.New()
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
		group := router.Group("/api", probe, proxy)
		if ws {
			for _, register := range []func(*gin.RouterGroup){analytic.InitWebSocketRouter, certificate.InitCertificateWebSocketRouter, event.InitRouter, nginx_log.InitWebSocketRouter, sites.InitWebSocketRouter, upstream.InitWebSocketRouter, system.InitWebSocketRouter, nginx.InitWebSocketRouter, cluster.InitWebSocketRouter} {
				register(group)
			}
		} else {
			for _, register := range []func(*gin.RouterGroup){analytic.InitRouter, userapi.InitManageUserRouter, nginx.InitRouter, sites.InitRouter, accesslist.InitRouter, snippet.InitRouter, streams.InitRouter, config.InitRouter, template.InitRouter, certificate.InitCertificateRouter, certificate.InitDNSCredentialRouter, certificate.InitAcmeUserRouter, dns.InitRouter, system.InitPrivateRouter, settingsapi.InitRouter, llm.InitRouter, mcp.InitManagementRouter, cluster.InitRouter, host.InitRouter, notification.InitRouter, external_notify.InitRouter, backup.InitAutoBackupRouter, nginx_log.InitRouter, upstream.InitHTTPRouter, ingress.InitRouter} {
				register(group)
			}
		}
		count := 0
		for _, route := range router.Routes() {
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
				router.ServeHTTP(res, req)
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
			if !hasSecure && !hasVerified {
				require.NotEqual(t, http.StatusUnauthorized, userResponse.Code, "unexpected caller step-up policy for unguarded %s %s", route.Method, route.Path)
			}
			if hasSecure || hasVerified {
				count++
				require.Equal(t, http.StatusUnauthorized, userResponse.Code, "missing caller secure-session policy for %s %s", route.Method, route.Path)
			}
			if hasInteractive {
				require.Equal(t, http.StatusForbidden, request("service").Code, "missing caller interactive policy for %s %s", route.Method, route.Path)
			} else if hasVerified {
				require.Equal(t, http.StatusUnauthorized, request("service").Code, "missing caller verified-two-factor policy for %s %s", route.Method, route.Path)
			}
		}
		require.Greater(t, count, 0, "no guarded routes audited (websocket=%v)", ws)
	}
}
