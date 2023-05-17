package router

import (
	"github.com/0xJacky/Nginx-UI/server/api"
	"github.com/gin-contrib/static"
	"github.com/gin-gonic/gin"
	"net/http"
)

func InitRouter() *gin.Engine {
	r := gin.New()
	r.Use(gin.Logger())

	r.Use(recovery())

	r.Use(cacheJs())

	r.Use(static.Serve("/", mustFS("")))

	r.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, gin.H{
			"message": "not found",
		})
	})

	root := r.Group("/api")
	{
		root.GET("install", api.InstallLockCheck)
		root.POST("install", api.InstallNginxUI)

		root.POST("/login", api.Login)
		root.DELETE("/logout", api.Logout)

		w := root.Group("/", authRequired(), proxyWs())
		{
			// Analytic
			w.GET("analytic", api.Analytic)
			w.GET("analytic/intro", api.GetIntroAnalytic)
			w.GET("analytic/nodes", api.GetNodesAnalytic)
			// pty
			w.GET("pty", api.Pty)
			// Nginx log
			w.GET("nginx_log", api.NginxLog)
		}

		g := root.Group("/", authRequired(), proxy())
		{

			g.GET("analytic/init", api.GetAnalyticInit)

			g.GET("users", api.GetUsers)
			g.GET("user/:id", api.GetUser)
			g.POST("user", api.AddUser)
			g.POST("user/:id", api.EditUser)
			g.DELETE("user/:id", api.DeleteUser)

			g.GET("domains", api.GetDomains)
			g.GET("domain/:name", api.GetDomain)

			// Modify site configuration directly
			g.POST("domain/:name", api.SaveDomain)

			// Transform NgxConf to nginx configuration
			g.POST("ngx/build_config", api.BuildNginxConfig)
			// Tokenized nginx configuration to NgxConf
			g.POST("ngx/tokenize_config", api.TokenizeNginxConfig)
			// Format nginx configuration code
			g.POST("ngx/format_code", api.FormatNginxConfig)

			g.POST("nginx/reload", api.ReloadNginx)
			g.POST("nginx/restart", api.RestartNginx)
			g.POST("nginx/test", api.TestNginx)
			g.GET("nginx/status", api.NginxStatus)

			g.POST("domain/:name/enable", api.EnableDomain)
			g.POST("domain/:name/disable", api.DisableDomain)
			g.POST("domain/:name/advance", api.DomainEditByAdvancedMode)

			g.DELETE("domain/:name", api.DeleteDomain)

			g.POST("domain/:name/duplicate", api.DuplicateSite)
			g.GET("domain/:name/cert", api.IssueCert)

			g.GET("configs", api.GetConfigs)
			g.GET("config/*name", api.GetConfig)
			g.POST("config", api.AddConfig)
			g.POST("config/*name", api.EditConfig)

			//g.GET("backups", api.GetFileBackupList)
			//g.GET("backup/:id", api.GetFileBackup)

			g.GET("template", api.GetTemplate)
			g.GET("template/configs", api.GetTemplateConfList)
			g.GET("template/blocks", api.GetTemplateBlockList)
			g.GET("template/block/:name", api.GetTemplateBlock)
			g.POST("template/block/:name", api.GetTemplateBlock)

			g.GET("certs", api.GetCertList)
			g.GET("cert/:id", api.GetCert)
			g.POST("cert", api.AddCert)
			g.POST("cert/:id", api.ModifyCert)
			g.DELETE("cert/:id", api.RemoveCert)

			// Add domain to auto-renew cert list
			g.POST("auto_cert/:name", api.AddDomainToAutoCert)
			// Delete domain from auto-renew cert list
			g.DELETE("auto_cert/:name", api.RemoveDomainFromAutoCert)
			g.GET("auto_cert/dns/providers", api.GetDNSProvidersList)
			g.GET("auto_cert/dns/provider/:code", api.GetDNSProvider)

			// DNS Credential
			g.GET("dns_credentials", api.GetDnsCredentialList)
			g.GET("dns_credential/:id", api.GetDnsCredential)
			g.POST("dns_credential", api.AddDnsCredential)
			g.POST("dns_credential/:id", api.EditDnsCredential)
			g.DELETE("dns_credential/:id", api.DeleteDnsCredential)

			g.POST("nginx_log", api.GetNginxLogPage)

			// Settings
			g.GET("settings", api.GetSettings)
			g.POST("settings", api.SaveSettings)

			// Upgrade
			g.GET("upgrade/release", api.GetRelease)
			g.GET("upgrade/current", api.GetCurrentVersion)
			g.GET("upgrade/perform", api.PerformCoreUpgrade)

			// ChatGPT
			g.POST("chat_gpt", api.MakeChatCompletionRequest)
			g.POST("chat_gpt_record", api.StoreChatGPTRecord)

			// Environment
			g.GET("environments", api.GetEnvironmentList)
			envGroup := g.Group("environment")
			{
				envGroup.GET("/:id", api.GetEnvironment)
				envGroup.POST("", api.AddEnvironment)
				envGroup.POST("/:id", api.EditEnvironment)
				envGroup.DELETE("/:id", api.DeleteEnvironment)
			}

			// node
			g.GET("node", api.GetCurrentNode)
		}
	}

	return r
}
