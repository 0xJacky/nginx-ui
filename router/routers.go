package router

import (
	"net/http"

	accesslist "github.com/0xJacky/Nginx-UI/api/access_list"
	"github.com/0xJacky/Nginx-UI/api/analytic"
	"github.com/0xJacky/Nginx-UI/api/audit"
	"github.com/0xJacky/Nginx-UI/api/backup"
	"github.com/0xJacky/Nginx-UI/api/blocklist"
	"github.com/0xJacky/Nginx-UI/api/cert_deploy"
	"github.com/0xJacky/Nginx-UI/api/certificate"
	"github.com/0xJacky/Nginx-UI/api/cluster"
	"github.com/0xJacky/Nginx-UI/api/config"
	"github.com/0xJacky/Nginx-UI/api/crypto"
	dnsapi "github.com/0xJacky/Nginx-UI/api/dns"
	"github.com/0xJacky/Nginx-UI/api/event"
	"github.com/0xJacky/Nginx-UI/api/external_notify"
	"github.com/0xJacky/Nginx-UI/api/geolite"
	"github.com/0xJacky/Nginx-UI/api/host"
	"github.com/0xJacky/Nginx-UI/api/ingress"
	"github.com/0xJacky/Nginx-UI/api/license"
	"github.com/0xJacky/Nginx-UI/api/llm"
	"github.com/0xJacky/Nginx-UI/api/nginx"
	nginxLog "github.com/0xJacky/Nginx-UI/api/nginx_log"
	"github.com/0xJacky/Nginx-UI/api/notification"
	"github.com/0xJacky/Nginx-UI/api/pages"
	pluginapi "github.com/0xJacky/Nginx-UI/api/plugin"
	"github.com/0xJacky/Nginx-UI/api/public"
	"github.com/0xJacky/Nginx-UI/api/settings"
	"github.com/0xJacky/Nginx-UI/api/sites"
	snippetapi "github.com/0xJacky/Nginx-UI/api/snippet"
	"github.com/0xJacky/Nginx-UI/api/streams"
	"github.com/0xJacky/Nginx-UI/api/system"
	"github.com/0xJacky/Nginx-UI/api/template"
	"github.com/0xJacky/Nginx-UI/api/terminal"
	"github.com/0xJacky/Nginx-UI/api/upstream"
	"github.com/0xJacky/Nginx-UI/api/upstream_discovery"
	"github.com/0xJacky/Nginx-UI/api/user"
	"github.com/0xJacky/Nginx-UI/internal/middleware"
	"github.com/0xJacky/Nginx-UI/mcp"
	"github.com/gin-gonic/gin"
	"github.com/uozi-tech/cosy"
	"github.com/uozi-tech/cosy/logger"
)

func InitRouter() {
	r := cosy.GetEngine()

	r.GET("/healthz", public.Healthz)

	// Must run before anything that reads the client IP, including audit.
	r.Use(middleware.UnixPeerAddr())
	r.Use(audit.LoggingMiddleware())

	// Plugin packages are far larger than any other request body, see
	// middleware.LargeUploads.
	r.Use(middleware.ScopedBodyLimit())

	if err := configureTrustedProxies(r); err != nil {
		logger.Fatalf("Configure trusted proxies: %v", err)
	}

	// Add CORS middleware to allow all origins
	r.Use(middleware.CORS())

	// Registered before the embedded assets so the plugin routes skip the
	// asset cache middleware and revalidate against their own files.
	pluginapi.InitStaticRouter(r)

	initEmbedRoute(r)

	pages.InitRouter(r)

	r.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, gin.H{
			"message": "not found",
		})
	})

	mcp.InitRouter(r)

	root := r.Group("/api", middleware.IPWhiteList())
	{
		public.InitRouter(root)
		crypto.InitPublicRouter(root)
		user.InitAuthRouter(root)
		nginxLog.InitPublicRouter(root)

		system.InitPublicRouter(root)
		backup.InitRouter(root)

		setup := root.Group("/setup", middleware.SetupAuthRequired())
		{
			system.InitSetupRouter(setup)
			backup.InitSetupRouter(setup)
		}

		// Local-only routes (no proxy) - authorization required. Account and
		// secure-session state belong to the controller that authenticated the
		// browser, even while the UI is managing a selected remote node.
		local := root.Group("/", middleware.AuthRequired())
		{
			license.InitRouter(local)
			llm.InitLocalRouter(local)
			user.InitTokenRouter(local)
			user.InitUserRouter(local)
		}

		// Local-only WebSocket routes (no proxy). WebSocket handshakes cannot
		// carry an Authorization header, so they must use AuthRequiredWS, which
		// also accepts the token from the query string.
		localWs := root.Group("/", middleware.AuthRequiredWS())
		{
			llm.InitLocalWebSocketRouter(localWs)
		}

		// The plugin http route serves both plain requests and WebSocket
		// upgrades, so it picks its own authentication and proxy chain.
		pluginapi.InitHTTPRouter(root)

		// Authorization required and not websocket request.
		//
		// The cosy debug monitor (github.com/uozi-tech/cosy/debug) is
		// deliberately not imported or mounted: importing it registers a
		// reporter that keeps every audited request and response in memory and
		// serves them under /api/debug, which the frontend never uses.
		g := root.Group("/", middleware.AuthRequired(), middleware.Proxy())
		for _, register := range ProxiedRouters {
			register(g)
		}

		// The terminal applies its own gates before proxying to a child node.
		terminal.InitRouter(root.Group("/", middleware.AuthRequiredWS()))

		// Authorization required and websocket request (no cookie fallback to prevent CSWSH)
		w := root.Group("/", middleware.AuthRequiredWS(), middleware.ProxyWs())
		for _, register := range ProxiedWebSocketRouters {
			register(w)
		}
	}
}

// ProxiedRouters are mounted behind AuthRequired and Proxy, so a request that
// selects a remote node is forwarded before any route-level guard runs. Proxy
// enforces those guards against the original caller itself. The proxy
// authorization audit test registers exactly this list.
var ProxiedRouters = []func(*gin.RouterGroup){
	analytic.InitRouter,
	user.InitManageUserRouter,
	nginx.InitRouter,
	sites.InitRouter,
	accesslist.InitRouter,
	snippetapi.InitRouter,
	streams.InitRouter,
	config.InitRouter,
	template.InitRouter,
	certificate.InitCertificateRouter,
	certificate.InitDNSCredentialRouter,
	certificate.InitAcmeUserRouter,
	cert_deploy.InitRouter,
	blocklist.InitRouter,
	upstream_discovery.InitRouter,
	dnsapi.InitRouter,
	system.InitPrivateRouter,
	settings.InitRouter,
	llm.InitRouter,
	mcp.InitManagementRouter,
	cluster.InitRouter,
	host.InitRouter,
	notification.InitRouter,
	pluginapi.InitRouter,
	pluginapi.InitSyncRouter,
	pluginapi.InitMarketplaceRouter,
	external_notify.InitRouter,
	backup.InitAutoBackupRouter,
	nginxLog.InitRouter,
	upstream.InitHTTPRouter,
	ingress.InitRouter,
	func(g *gin.RouterGroup) { g.GET("/geolite/status", geolite.GetStatus) },
}

// ProxiedWebSocketRouters are mounted behind AuthRequiredWS and ProxyWs, with
// the same pre-proxy guard enforcement as ProxiedRouters.
var ProxiedWebSocketRouters = []func(*gin.RouterGroup){
	analytic.InitWebSocketRouter,
	certificate.InitCertificateWebSocketRouter,
	event.InitRouter,
	nginxLog.InitWebSocketRouter,
	sites.InitWebSocketRouter,
	upstream.InitWebSocketRouter,
	system.InitWebSocketRouter,
	nginx.InitWebSocketRouter,
	cluster.InitWebSocketRouter,
	func(w *gin.RouterGroup) { w.GET("/geolite/download", geolite.DownloadGeoLiteDB) },
}
