package middleware_test

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	blocklistapi "github.com/0xJacky/Nginx-UI/api/blocklist"
	certdeployapi "github.com/0xJacky/Nginx-UI/api/cert_deploy"
	certificateapi "github.com/0xJacky/Nginx-UI/api/certificate"
	upstreamdiscoveryapi "github.com/0xJacky/Nginx-UI/api/upstream_discovery"
	"github.com/0xJacky/Nginx-UI/internal/cache"
	"github.com/0xJacky/Nginx-UI/internal/cert/dns"
	internalmcp "github.com/0xJacky/Nginx-UI/internal/mcp"
	"github.com/0xJacky/Nginx-UI/internal/middleware"
	"github.com/0xJacky/Nginx-UI/internal/nodeauth"
	"github.com/0xJacky/Nginx-UI/internal/testdb"
	internaluser "github.com/0xJacky/Nginx-UI/internal/user"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/0xJacky/Nginx-UI/router"
	appsettings "github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/uozi-tech/cosy"
	"github.com/uozi-tech/cosy/logger"
	cosysettings "github.com/uozi-tech/cosy/settings"
	"gorm.io/driver/sqlite"
)

// GHSA-23gr-943q-cjh9: a child node authenticates the controller's node
// principal, not the original token, so it returns stored plugin credentials
// in full. The controller must withhold them from service tokens, as the
// local handlers do, while an interactive administrator still sees them.
func TestProxyRedactsPluginResourceCredentialsForServiceTokens(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger.Init("fatal")
	cache.InitInMemoryCache()
	oldCrypto := appsettings.CryptoSettings.Secret
	oldInstance := appsettings.NodeSettings.InstanceID
	oldSecret := appsettings.NodeSettings.Secret
	oldJWT := cosysettings.AppSettings.JwtSecret
	t.Cleanup(func() {
		appsettings.CryptoSettings.Secret = oldCrypto
		appsettings.NodeSettings.InstanceID = oldInstance
		appsettings.NodeSettings.Secret = oldSecret
		cosysettings.AppSettings.JwtSecret = oldJWT
		model.Use(nil)
	})
	appsettings.CryptoSettings.Secret = "plugin-redaction-crypto-root"
	appsettings.NodeSettings.InstanceID = "dddddddd-dddd-4ddd-8ddd-dddddddddddd"
	appsettings.NodeSettings.Secret = "plugin-redaction-node-secret"
	cosysettings.AppSettings.JwtSecret = "plugin-redaction-jwt"

	cosy.RegisterModels(&model.User{}, &model.Passkey{}, &model.AuthToken{}, &model.MCPServiceToken{}, &model.Node{},
		&model.CertDeployTarget{}, &model.BlocklistSource{}, &model.UpstreamDiscovery{}, &model.DnsCredential{})
	db := cosy.InitDB(sqlite.Open(testdb.DSN(t)))
	model.Use(db)
	query.Use(db)
	query.SetDefault(db)

	admin := &model.User{Name: "plugin-redaction-admin", Status: true}
	require.NoError(t, db.Create(admin).Error)
	t.Cleanup(func() { internaluser.InvalidateUserCache(admin.ID) })
	payload, err := internaluser.GenerateJWT(admin)
	require.NoError(t, err)
	_, readToken, err := internalmcp.CreateServiceToken("reader", []string{model.APITokenScopeRead}, nil, admin.ID)
	require.NoError(t, err)
	_, writeToken, err := internalmcp.CreateServiceToken("writer", []string{model.APITokenScopeWrite}, nil, admin.ID)
	require.NoError(t, err)

	const deploySecret, blocklistSecret, discoverySecret, dnsSecret = "DEPLOY-SECRET-1", "BLOCKLIST-SECRET-2", "DISCOVERY-SECRET-3", "DNS-SECRET-4"
	target := &model.CertDeployTarget{Name: "cdn-target", Kind: "plugin:x", Config: map[string]string{"api_secret": deploySecret}}
	require.NoError(t, db.Create(target).Error)
	source := &model.BlocklistSource{Name: "feed-source", Kind: "plugin:y", Config: map[string]string{"api_key": blocklistSecret}}
	require.NoError(t, db.Create(source).Error)
	binding := &model.UpstreamDiscovery{UpstreamName: "backend-binding", Kind: "plugin:z", Service: "svc", Config: map[string]string{"token": discoverySecret}}
	require.NoError(t, db.Create(binding).Error)
	credential := &model.DnsCredential{Name: "cf-credential", Provider: "Cloudflare", ProviderCode: "cloudflare", Config: &dns.Config{
		Name: "Cloudflare", Code: "cloudflare",
		Configuration: &dns.Configuration{Credentials: map[string]string{"CF_DNS_API_TOKEN": dnsSecret}},
	}}
	require.NoError(t, db.Create(credential).Error)

	register := func(g *gin.RouterGroup) {
		certdeployapi.InitRouter(g)
		blocklistapi.InitRouter(g)
		upstreamdiscoveryapi.InitRouter(g)
		certificateapi.InitDNSCredentialRouter(g)
	}

	// The child is a separate engine on a loopback listener; both engines
	// share the test database, so the child serves the same rows.
	child := gin.New()
	register(child.Group("/api", middleware.AuthRequired()))
	childServer := httptest.NewServer(child)
	defer childServer.Close()

	node := &model.Node{Name: "child", URL: childServer.URL, Enabled: true, AuthMethod: model.NodeAuthMethodLegacy}
	require.NoError(t, db.Create(node).Error)
	encrypted, err := nodeauth.EncryptPrivateCredential(nodeauth.LegacyCredentialPurpose(node.ID), []byte(appsettings.NodeSettings.Secret))
	require.NoError(t, err)
	require.NoError(t, db.Model(node).Update("encrypted_legacy_secret", encrypted).Error)

	controller := gin.New()
	register(controller.Group("/api", middleware.AuthRequired(), middleware.Proxy()))

	id := func(value uint64) string { return strconv.FormatUint(value, 10) }
	cases := []struct{ path, secret, name string }{
		{"/api/cert_deploy_targets", deploySecret, target.Name},
		{"/api/cert_deploy_targets/" + id(target.ID), deploySecret, target.Name},
		{"/api/blocklist_sources", blocklistSecret, source.Name},
		{"/api/blocklist_sources/" + id(source.ID), blocklistSecret, source.Name},
		{"/api/upstream_discoveries", discoverySecret, binding.UpstreamName},
		{"/api/upstream_discoveries/" + id(binding.ID), discoverySecret, binding.UpstreamName},
		// Already covered before; kept as the control.
		{"/api/dns_credentials/" + id(credential.ID), dnsSecret, credential.Name},
	}
	callers := []struct {
		name, token  string
		seesSecrets  bool
		serviceToken bool
	}{
		{"api:read", readToken, false, true},
		{"api:write", writeToken, false, true},
		{"interactive administrator", payload.Token, true, false},
	}
	for _, tc := range cases {
		for _, caller := range callers {
			for _, remote := range []bool{false, true} {
				t.Run(caller.name+" "+tc.path+" remote="+strconv.FormatBool(remote), func(t *testing.T) {
					ctx, cancel := context.WithCancel(context.Background())
					defer cancel()
					req := httptest.NewRequest(http.MethodGet, tc.path, nil).WithContext(ctx)
					req.Header.Set("Authorization", "Bearer "+caller.token)
					if remote {
						req.Header.Set("X-Node-ID", id(node.ID))
					}
					res := httptest.NewRecorder()
					controller.ServeHTTP(res, req)
					require.Equal(t, http.StatusOK, res.Code, res.Body.String())
					body := res.Body.String()
					require.Contains(t, body, tc.name, "the rest of the record is still returned")
					require.Equal(t, caller.seesSecrets, strings.Contains(body, tc.secret), body)
				})
			}
		}
	}
}

// serviceTokenAwareHandler maps a function under api/ that checks for a
// service token to the routes whose response that check shapes.
type serviceTokenAwareHandler struct {
	// routes are "METHOD /api/path" as registered by router.InitRouter.
	routes []string
	// exempt explains why the check needs no proxy counterpart.
	exempt string
}

// serviceTokenAwareHandlers lists every function under api/ that asks
// whether the caller is a service token. A local handler that withholds data
// from service tokens needs a matching entry in the node proxy's redaction
// tables, because a child node answers the controller's node principal in
// full. Add new checks here with the routes they serve; the test then demands
// the matching proxy redaction.
var serviceTokenAwareHandlers = map[string]serviceTokenAwareHandler{
	"audit.newRequestSnapshot":                      {exempt: "records the caller in the audit log and leaves the response alone"},
	"backup.GetAutoBackup":                          {routes: []string{"GET /api/auto_backup/:id"}},
	"backup.GetAutoBackupList":                      {routes: []string{"GET /api/auto_backup"}},
	"blocklist.setView":                             {routes: []string{"GET /api/blocklist_sources", "GET /api/blocklist_sources/:id"}},
	"cert_deploy.redactForServiceTokens":            {routes: []string{"GET /api/cert_deploy_targets", "GET /api/cert_deploy_targets/:id"}},
	"certificate.GetAcmeUser":                       {routes: []string{"GET /api/acme_users/:id"}},
	"certificate.GetAcmeUserList":                   {routes: []string{"GET /api/acme_users"}},
	"certificate.GetCert":                           {routes: []string{"GET /api/certs/:id"}},
	"certificate.GetDnsCredential":                  {routes: []string{"GET /api/dns_credentials/:id"}},
	"certificate.GetDnsCredentialList":              {routes: []string{"GET /api/dns_credentials"}},
	"config.GetConfig":                              {routes: []string{"GET /api/config"}},
	"config.GetConfigHistory":                       {routes: []string{"GET /api/config_histories"}},
	"dns.GetDomain":                                 {routes: []string{"GET /api/dns/domains/:id"}},
	"dns.ListDomains":                               {routes: []string{"GET /api/dns/domains"}},
	"external_notify.redactConfigForReadOnlyTokens": {routes: []string{"GET /api/external_notifies", "GET /api/external_notifies/:id"}},
	"sites.GetHealthCheck":                          {routes: []string{"GET /api/site_navigation/health_check/:id"}},
	"sites.GetSiteNavigation":                       {routes: []string{"GET /api/site_navigation"}},
	"sites.SiteNavigationWebSocket":                 {routes: []string{"GET /api/site_navigation_ws"}},
	"upstream_discovery.setView":                    {routes: []string{"GET /api/upstream_discoveries", "GET /api/upstream_discoveries/:id"}},
}

// serviceTokenChecks are the identifiers a handler uses to tell a service
// token apart, and whether the check covers every service token or only the
// read-only ones.
var serviceTokenChecks = map[string]bool{
	"IsServiceTokenRequest":         true,
	"ServiceTokenPrincipalKey":      true,
	"IsReadOnlyServiceTokenRequest": false,
}

// findServiceTokenChecks returns, for each function under apiDir that uses a
// service token check, whether any of its checks covers every service token.
func findServiceTokenChecks(t *testing.T, apiDir string) map[string]bool {
	t.Helper()
	found := map[string]bool{}
	fileSet := token.NewFileSet()
	err := filepath.WalkDir(apiDir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return err
		}
		file, err := parser.ParseFile(fileSet, path, nil, 0)
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(apiDir, filepath.Dir(path))
		if err != nil {
			return err
		}
		pkg := filepath.ToSlash(relative)
		for _, decl := range file.Decls {
			name := "<package scope>"
			if function, ok := decl.(*ast.FuncDecl); ok {
				name = function.Name.Name
			}
			ast.Inspect(decl, func(n ast.Node) bool {
				selector, ok := n.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				if allTokens, isCheck := serviceTokenChecks[selector.Sel.Name]; isCheck {
					key := pkg + "." + name
					found[key] = found[key] || allTokens
				}
				return true
			})
		}
		return nil
	})
	require.NoError(t, err)
	return found
}

func proxiedRoutes(registers []func(*gin.RouterGroup)) map[string]bool {
	engine := gin.New()
	group := engine.Group("/api")
	for _, register := range registers {
		register(group)
	}
	routes := map[string]bool{}
	for _, route := range engine.Routes() {
		routes[route.Method+" "+route.Path] = true
	}
	return routes
}

// A local handler that withholds stored secrets from service tokens must have
// a counterpart in the node proxy, or a service token reads the secret from a
// child node by selecting it.
func TestServiceTokenAwareHandlersHaveProxyRedaction(t *testing.T) {
	gin.SetMode(gin.TestMode)
	found := findServiceTokenChecks(t, filepath.Join("..", "..", "api"))
	require.NotEmpty(t, found)

	httpRoutes := proxiedRoutes(router.ProxiedRouters)
	webSocketRoutes := proxiedRoutes(router.ProxiedWebSocketRouters)
	allTokens, readOnly, refusedFeeds := middleware.ProxyServiceTokenRedactedRoutes()

	names := make([]string, 0, len(found))
	for name := range found {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		coversAllTokens := found[name]
		handler, mapped := serviceTokenAwareHandlers[name]
		if !mapped {
			t.Errorf("%s checks for a service token but is not listed in serviceTokenAwareHandlers; list the routes it serves", name)
			continue
		}
		if handler.exempt != "" {
			continue
		}
		require.NotEmpty(t, handler.routes, name)
		for _, route := range handler.routes {
			_, path, _ := strings.Cut(route, " ")
			switch {
			case webSocketRoutes[route]:
				if !refusedFeeds[path] {
					t.Errorf("%s: the node WebSocket proxy relays %s to service tokens without refusing it", name, route)
				}
			case httpRoutes[route]:
				if coversAllTokens && !allTokens[path] {
					t.Errorf("%s redacts %s for every service token, but the node proxy has no serviceTokenRedactedResponseFields entry", name, route)
				}
				if !coversAllTokens && !allTokens[path] && !readOnly[path] {
					t.Errorf("%s redacts %s for read-only service tokens, but the node proxy has no readOnlyServiceTokenProxyRedactions entry", name, route)
				}
			default:
				t.Errorf("%s: %s is not a route router.InitRouter mounts behind the node proxy", name, route)
			}
		}
	}
	for name := range serviceTokenAwareHandlers {
		if _, ok := found[name]; !ok {
			t.Errorf("serviceTokenAwareHandlers lists %s, which no longer checks for a service token", name)
		}
	}
}
