package router

import (
	"net/http"

	"github.com/gin-contrib/pprof"

	"github.com/0xJacky/Nginx-UI/api/analytic"
	"github.com/0xJacky/Nginx-UI/api/certificate"
	"github.com/0xJacky/Nginx-UI/api/cluster"
	"github.com/0xJacky/Nginx-UI/api/config"
	"github.com/0xJacky/Nginx-UI/api/crypto"
	"github.com/0xJacky/Nginx-UI/api/external_notify"
	"github.com/0xJacky/Nginx-UI/api/nginx"
	nginxLog "github.com/0xJacky/Nginx-UI/api/nginx_log"
	"github.com/0xJacky/Nginx-UI/api/notification"
	"github.com/0xJacky/Nginx-UI/api/openai"
	"github.com/0xJacky/Nginx-UI/api/pages"
	"github.com/0xJacky/Nginx-UI/api/public"
	"github.com/0xJacky/Nginx-UI/api/settings"
	"github.com/0xJacky/Nginx-UI/api/sites"
	"github.com/0xJacky/Nginx-UI/api/streams"
	"github.com/0xJacky/Nginx-UI/api/system"
	"github.com/0xJacky/Nginx-UI/api/template"
	"github.com/0xJacky/Nginx-UI/api/terminal"
	"github.com/0xJacky/Nginx-UI/api/upstream"
	"github.com/0xJacky/Nginx-UI/api/user"
	"github.com/0xJacky/Nginx-UI/internal/middleware"
	"github.com/0xJacky/Nginx-UI/mcp"
	"github.com/gin-gonic/gin"
	"github.com/uozi-tech/cosy"
	cSettings "github.com/uozi-tech/cosy/settings"
)

func InitRouter() {
	r := cosy.GetEngine()

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
		system.InitPublicRouter(root)
		system.InitBackupRestoreRouter(root)
		system.InitSelfCheckRouter(root)
		user.InitAuthRouter(root)

		// Authorization required and not websocket request
		g := root.Group("/", middleware.AuthRequired(), middleware.Proxy())
		{
			if cSettings.ServerSettings.RunMode == gin.DebugMode {
				pprof.Register(g)
			}
			user.InitUserRouter(g)
			analytic.InitRouter(g)
			user.InitManageUserRouter(g)
			nginx.InitRouter(g)
			sites.InitRouter(g)
			streams.InitRouter(g)
			config.InitRouter(g)
			template.InitRouter(g)
			certificate.InitCertificateRouter(g)
			certificate.InitDNSCredentialRouter(g)
			certificate.InitAcmeUserRouter(g)
			system.InitPrivateRouter(g)
			settings.InitRouter(g)
			openai.InitRouter(g)
			cluster.InitRouter(g)
			notification.InitRouter(g)
			external_notify.InitRouter(g)
		}

		// Authorization required and websocket request
		w := root.Group("/", middleware.AuthRequired(), middleware.ProxyWs())
		{
			analytic.InitWebSocketRouter(w)
			certificate.InitCertificateWebSocketRouter(w)
			o := w.Group("", middleware.RequireSecureSession())
			{
				terminal.InitRouter(o)
			}
			nginxLog.InitRouter(w)
			upstream.InitRouter(w)
			system.InitWebSocketRouter(w)
		}
	}
}
