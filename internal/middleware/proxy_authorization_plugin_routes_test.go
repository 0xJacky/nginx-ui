package middleware_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/0xJacky/Nginx-UI/api/blocklist"
	"github.com/0xJacky/Nginx-UI/api/cert_deploy"
	pluginapi "github.com/0xJacky/Nginx-UI/api/plugin"
	"github.com/0xJacky/Nginx-UI/api/upstream_discovery"
	"github.com/0xJacky/Nginx-UI/internal/cache"
	internalmcp "github.com/0xJacky/Nginx-UI/internal/mcp"
	"github.com/0xJacky/Nginx-UI/internal/middleware"
	"github.com/0xJacky/Nginx-UI/internal/nodeauth"
	"github.com/0xJacky/Nginx-UI/internal/testdb"
	internaluser "github.com/0xJacky/Nginx-UI/internal/user"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/uozi-tech/cosy/logger"
	cosysettings "github.com/uozi-tech/cosy/settings"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// pluginProxyFixture is a controller and a loopback child node that both
// mount the real plugin, cert deploy, blocklist and upstream discovery routers.
type pluginProxyFixture struct {
	controller *gin.Engine
	nodeID     uint64
	// Tokens of an administrator with an enrolled second factor, of one
	// without, and of api:write and api:read service tokens.
	mfaJWT, plainJWT, writeToken, readToken string
	secureSession                           string
	childReached, childSawNode              bool
}

func setupPluginProxyFixture(t *testing.T) *pluginProxyFixture {
	t.Helper()
	gin.SetMode(gin.TestMode)
	logger.Init("fatal")
	t.Setenv("TMPDIR", t.TempDir())
	cache.InitInMemoryCache()

	oldCrypto := settings.CryptoSettings.Secret
	oldInstance := settings.NodeSettings.InstanceID
	oldNodeSecret := settings.NodeSettings.Secret
	oldJWT := cosysettings.AppSettings.JwtSecret
	settings.CryptoSettings.Secret = "plugin-proxy-test-root"
	settings.NodeSettings.InstanceID = "cccccccc-cccc-4ccc-8ccc-cccccccccccc"
	settings.NodeSettings.Secret = "plugin-proxy-child-secret"
	cosysettings.AppSettings.JwtSecret = "plugin-proxy-jwt-secret"
	t.Cleanup(func() {
		settings.CryptoSettings.Secret = oldCrypto
		settings.NodeSettings.InstanceID = oldInstance
		settings.NodeSettings.Secret = oldNodeSecret
		cosysettings.AppSettings.JwtSecret = oldJWT
		model.Use(nil)
	})

	db, err := gorm.Open(sqlite.Open(testdb.DSN(t)), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Node{}, &model.User{}, &model.Passkey{}, &model.AuthToken{}, &model.MCPServiceToken{}))
	model.Use(db)
	query.SetDefault(db)

	f := &pluginProxyFixture{}
	admin := &model.User{Name: "plugin-mfa-admin", OTPSecret: []byte("enrolled-mfa"), Status: true}
	require.NoError(t, db.Create(admin).Error)
	plain := &model.User{Name: "plugin-plain-admin", Status: true}
	require.NoError(t, db.Create(plain).Error)
	t.Cleanup(func() {
		internaluser.InvalidateUserCache(admin.ID)
		internaluser.InvalidateUserCache(plain.ID)
	})
	payload, err := internaluser.GenerateJWT(admin)
	require.NoError(t, err)
	f.mfaJWT = payload.Token
	payload, err = internaluser.GenerateJWT(plain)
	require.NoError(t, err)
	f.plainJWT = payload.Token
	f.secureSession = internaluser.SetSecureSessionID(admin.ID)
	f.writeToken = createServiceToken(t, "plugin-proxy-write", model.APITokenScopeWrite, admin.ID)
	f.readToken = createServiceToken(t, "plugin-proxy-read", model.APITokenScopeRead, admin.ID)

	register := func(g *gin.RouterGroup) {
		pluginapi.InitRouter(g)
		pluginapi.InitSyncRouter(g)
		pluginapi.InitMarketplaceRouter(g)
		cert_deploy.InitRouter(g)
		blocklist.InitRouter(g)
		upstream_discovery.InitRouter(g)
	}

	child := gin.New()
	register(child.Group("/api", middleware.AuthRequired(), func(c *gin.Context) {
		f.childReached = true
		_, f.childSawNode = c.Get(nodeauth.GinPrincipalKey)
		c.Next()
	}))
	childServer := httptest.NewServer(child)
	t.Cleanup(childServer.Close)

	node := &model.Node{Name: "plugin-child", URL: childServer.URL, Enabled: true, AuthMethod: model.NodeAuthMethodLegacy}
	require.NoError(t, db.Create(node).Error)
	encrypted, err := nodeauth.EncryptPrivateCredential(nodeauth.LegacyCredentialPurpose(node.ID), []byte(settings.NodeSettings.Secret))
	require.NoError(t, err)
	require.NoError(t, db.Model(node).Update("encrypted_legacy_secret", encrypted).Error)
	f.nodeID = node.ID

	f.controller = gin.New()
	register(f.controller.Group("/api", middleware.AuthRequired(), middleware.Proxy()))
	return f
}

func createServiceToken(t *testing.T, name, scope string, userID uint64) string {
	t.Helper()
	_, token, err := internalmcp.CreateServiceToken(name, []string{scope}, nil, userID)
	require.NoError(t, err)
	return token
}

// send issues a controller request selecting the child node, or the
// controller itself when remote is false.
func (f *pluginProxyFixture) send(method, path, token string, remote, steppedUp bool) *httptest.ResponseRecorder {
	f.childReached, f.childSawNode = false, false
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req := httptest.NewRequest(method, path, nil).WithContext(ctx)
	req.Header.Set("Authorization", "Bearer "+token)
	if remote {
		req.Header.Set("X-Node-ID", strconv.FormatUint(f.nodeID, 10))
	}
	if steppedUp {
		req.Header.Set("X-Secure-Session-ID", f.secureSession)
	}
	res := httptest.NewRecorder()
	f.controller.ServeHTTP(res, req)
	return res
}

// pluginManagementRoutes are the guarded plugin management routes. Each one
// requires a secure session and stays closed in demo mode.
var pluginManagementRoutes = []struct{ method, path string }{
	{http.MethodPost, "/api/plugins"},
	{http.MethodPost, "/api/plugins/inspect"},
	{http.MethodPost, "/api/plugins/demo/enable"},
	{http.MethodPost, "/api/plugins/demo/disable"},
	{http.MethodPost, "/api/plugins/demo/channel"},
	{http.MethodPost, "/api/plugins/demo/settings"},
	{http.MethodDelete, "/api/plugins/demo"},
	{http.MethodPost, "/api/plugins/demo/sync"},
	{http.MethodPost, "/api/plugins/demo/sync_policy"},
	{http.MethodPost, "/api/plugins/marketplace/install"},
	{http.MethodPost, "/api/plugins/marketplace/sources"},
	{http.MethodPost, "/api/plugins/marketplace/sources/probe"},
	{http.MethodPost, "/api/plugins/demo/update"},
	{http.MethodPost, "/api/plugins/demo/replace"},
}

// A caller that has not stepped up must not reach a child node's plugin
// management through the controller.
func TestProxyRefusesPluginManagementWithoutStepUp(t *testing.T) {
	f := setupPluginProxyFixture(t)
	for _, route := range pluginManagementRoutes {
		t.Run(route.method+" "+route.path, func(t *testing.T) {
			local := f.send(route.method, route.path, f.mfaJWT, false, false)
			require.Equal(t, http.StatusUnauthorized, local.Code, local.Body.String())

			remote := f.send(route.method, route.path, f.mfaJWT, true, false)
			require.Equal(t, http.StatusUnauthorized, remote.Code, remote.Body.String())
			require.False(t, f.childReached, "the request must not be forwarded")

			readOnly := f.send(route.method, route.path, f.readToken, true, false)
			require.Equal(t, http.StatusForbidden, readOnly.Code, readOnly.Body.String())
			require.False(t, f.childReached, "a read token must not be forwarded")
		})
	}
}

// A stepped-up administrator, an administrator without a second factor and
// an api:write token still manage plugins on a child node: the controller
// forwards the request, and the child runs the handler for its node principal.
func TestProxyForwardsPluginManagementForAuthorizedCallers(t *testing.T) {
	f := setupPluginProxyFixture(t)
	for _, tc := range []struct {
		name      string
		token     func() string
		steppedUp bool
	}{
		{"stepped-up administrator", func() string { return f.mfaJWT }, true},
		{"administrator without second factor", func() string { return f.plainJWT }, false},
		{"write service token", func() string { return f.writeToken }, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			res := f.send(http.MethodPost, "/api/plugins", tc.token(), true, tc.steppedUp)
			require.True(t, f.childReached, "the request is forwarded")
			require.True(t, f.childSawNode, "the child authenticates the controller as a node")
			// 55007 is plugin.ErrPackageInvalid: InstallPlugin ran on the child
			// past its own secure-session and demo guards and rejected the
			// empty upload.
			require.Contains(t, res.Body.String(), "55007")
		})
	}

	// Unguarded plugin routes still proxy for any authenticated caller.
	f.send(http.MethodGet, "/api/plugins/matrix", f.mfaJWT, true, false)
	require.True(t, f.childReached, "an unguarded route is forwarded without step-up")
}

// The controller pushes plugin changes to a child node signed as a node, with
// no node selection header. The child accepts that principal.
func TestChildAcceptsNodePrincipalForPluginManagement(t *testing.T) {
	f := setupPluginProxyFixture(t)
	f.childReached = false
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req := httptest.NewRequest(http.MethodPost, "/api/plugins", nil).WithContext(ctx)
	req.Header.Set("X-Node-Secret", settings.NodeSettings.Secret)
	res := httptest.NewRecorder()
	// The controller engine stands in for the child: a node-signed request
	// without X-Node-ID is handled locally.
	f.controller.ServeHTTP(res, req)
	require.Contains(t, res.Body.String(), "55007", "the node principal passes the plugin guards")
}

// Routes that need an interactive administrator and a secure session are
// refused on the controller before forwarding, for every caller that would
// be refused locally.
func TestProxyRefusesInteractiveRoutesForUnauthorizedCallers(t *testing.T) {
	f := setupPluginProxyFixture(t)
	routes := []struct{ method, path string }{
		{http.MethodPost, "/api/cert_deploy_targets"},
		{http.MethodPost, "/api/cert_deploy_targets/1"},
		{http.MethodDelete, "/api/cert_deploy_targets/1"},
		{http.MethodPatch, "/api/cert_deploy_targets/1"},
		{http.MethodPost, "/api/cert_deploy_targets/test"},
		{http.MethodPost, "/api/cert_deploy_targets/1/deploy"},
		{http.MethodPost, "/api/certs/1/deploy"},
		{http.MethodPost, "/api/blocklist_sources"},
		{http.MethodPost, "/api/blocklist_sources/1"},
		{http.MethodDelete, "/api/blocklist_sources/1"},
		{http.MethodPatch, "/api/blocklist_sources/1"},
		{http.MethodPost, "/api/blocklist_sources/1/refresh"},
		{http.MethodPost, "/api/upstream_discoveries"},
		{http.MethodPost, "/api/upstream_discoveries/1"},
		{http.MethodDelete, "/api/upstream_discoveries/1"},
		{http.MethodPatch, "/api/upstream_discoveries/1"},
		{http.MethodPost, "/api/upstream_discoveries/1/refresh"},
	}
	for _, route := range routes {
		t.Run(route.method+" "+route.path, func(t *testing.T) {
			res := f.send(route.method, route.path, f.mfaJWT, true, false)
			require.Equal(t, http.StatusUnauthorized, res.Code, res.Body.String())
			require.False(t, f.childReached)

			res = f.send(route.method, route.path, f.writeToken, true, false)
			require.Equal(t, http.StatusForbidden, res.Code, res.Body.String())
			require.False(t, f.childReached)

			res = f.send(route.method, route.path, f.readToken, true, false)
			require.Equal(t, http.StatusForbidden, res.Code, res.Body.String())
			require.False(t, f.childReached)

			// A stepped-up administrator passes the controller's checks. The
			// child keeps these routes to interactive administrators, so it
			// refuses the node principal, which the proxy reports as 503.
			res = f.send(route.method, route.path, f.mfaJWT, true, true)
			require.True(t, f.childReached)
			require.Equal(t, http.StatusServiceUnavailable, res.Code, res.Body.String())
			require.Equal(t, "403", res.Header().Get("X-Nginx-UI-Upstream-Status"))
		})
	}
}
