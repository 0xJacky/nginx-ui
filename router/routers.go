package router

import (
	api2 "github.com/0xJacky/Nginx-UI/api"
	"github.com/gin-contrib/static"
	"github.com/gin-gonic/gin"
	"net/http"
)

func InitRouter() *gin.Engine {
	r := gin.New()
	r.Use(gin.Logger())

	r.Use(recovery())

	r.Use(cacheJs())

	//r.Use(OperationSync())

	r.Use(static.Serve("/", mustFS("")))

	r.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, gin.H{
			"message": "not found",
		})
	})

	root := r.Group("/api")
	{
		root.GET("install", api2.InstallLockCheck)
		root.POST("install", api2.InstallNginxUI)

		root.POST("/login", api2.Login)
		root.DELETE("/logout", api2.Logout)

		root.GET("/casdoor_uri", api2.GetCasdoorUri)
		root.POST("/casdoor_callback", api2.CasdoorCallback)

		// translation
		root.GET("translation/:code", api2.GetTranslation)

		w := root.Group("/", authRequired(), proxyWs())
		{
			// Analytic
			w.GET("analytic", api2.Analytic)
			w.GET("analytic/intro", api2.GetNodeStat)
			w.GET("analytic/nodes", api2.GetNodesAnalytic)
			// pty
			w.GET("pty", api2.Pty)
			// Nginx log
			w.GET("nginx_log", api2.NginxLog)
		}

		g := root.Group("/", authRequired(), proxy())
		{

			g.GET("analytic/init", api2.GetAnalyticInit)

			g.GET("users", api2.GetUsers)
			g.GET("user/:id", api2.GetUser)
			g.POST("user", api2.AddUser)
			g.POST("user/:id", api2.EditUser)
			g.DELETE("user/:id", api2.DeleteUser)

			g.GET("domains", api2.GetDomains)
			g.GET("domain/:name", api2.GetDomain)

			// Modify site configuration directly
			g.POST("domain/:name", api2.SaveDomain)

			// Transform NgxConf to nginx configuration
			g.POST("ngx/build_config", api2.BuildNginxConfig)
			// Tokenized nginx configuration to NgxConf
			g.POST("ngx/tokenize_config", api2.TokenizeNginxConfig)
			// Format nginx configuration code
			g.POST("ngx/format_code", api2.FormatNginxConfig)

			g.POST("nginx/reload", api2.ReloadNginx)
			g.POST("nginx/restart", api2.RestartNginx)
			g.POST("nginx/test", api2.TestNginx)
			g.GET("nginx/status", api2.NginxStatus)

			g.POST("domain/:name/enable", api2.EnableDomain)
			g.POST("domain/:name/disable", api2.DisableDomain)
			g.POST("domain/:name/advance", api2.DomainEditByAdvancedMode)

			g.DELETE("domain/:name", api2.DeleteDomain)

			g.POST("domain/:name/duplicate", api2.DuplicateSite)
			g.GET("domain/:name/cert", api2.IssueCert)

			g.GET("configs", api2.GetConfigs)
			g.GET("config/*name", api2.GetConfig)
			g.POST("config", api2.AddConfig)
			g.POST("config/*name", api2.EditConfig)

			//g.GET("backups", api.GetFileBackupList)
			//g.GET("backup/:id", api.GetFileBackup)

			g.GET("template", api2.GetTemplate)
			g.GET("template/configs", api2.GetTemplateConfList)
			g.GET("template/blocks", api2.GetTemplateBlockList)
			g.GET("template/block/:name", api2.GetTemplateBlock)
			g.POST("template/block/:name", api2.GetTemplateBlock)

			g.GET("certs", api2.GetCertList)
			g.GET("cert/:id", api2.GetCert)
			g.POST("cert", api2.AddCert)
			g.POST("cert/:id", api2.ModifyCert)
			g.DELETE("cert/:id", api2.RemoveCert)

			// Add domain to auto-renew cert list
			g.POST("auto_cert/:name", api2.AddDomainToAutoCert)
			// Delete domain from auto-renew cert list
			g.DELETE("auto_cert/:name", api2.RemoveDomainFromAutoCert)
			g.GET("auto_cert/dns/providers", api2.GetDNSProvidersList)
			g.GET("auto_cert/dns/provider/:code", api2.GetDNSProvider)

			// DNS Credential
			g.GET("dns_credentials", api2.GetDnsCredentialList)
			g.GET("dns_credential/:id", api2.GetDnsCredential)
			g.POST("dns_credential", api2.AddDnsCredential)
			g.POST("dns_credential/:id", api2.EditDnsCredential)
			g.DELETE("dns_credential/:id", api2.DeleteDnsCredential)

			g.POST("nginx_log", api2.GetNginxLogPage)

			// Settings
			g.GET("settings", api2.GetSettings)
			g.POST("settings", api2.SaveSettings)

			// Upgrade
			g.GET("upgrade/release", api2.GetRelease)
			g.GET("upgrade/current", api2.GetCurrentVersion)
			g.GET("upgrade/perform", api2.PerformCoreUpgrade)

			// ChatGPT
			g.POST("chat_gpt", api2.MakeChatCompletionRequest)
			g.POST("chat_gpt_record", api2.StoreChatGPTRecord)

			// Environment
			g.GET("environments", api2.GetEnvironmentList)
			envGroup := g.Group("environment")
			{
				envGroup.GET("/:id", api2.GetEnvironment)
				envGroup.POST("", api2.AddEnvironment)
				envGroup.POST("/:id", api2.EditEnvironment)
				envGroup.DELETE("/:id", api2.DeleteEnvironment)
			}

			// node
			g.GET("node", api2.GetCurrentNode)
		}
	}

	return r
}
