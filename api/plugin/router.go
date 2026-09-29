package plugin

import (
	"github.com/0xJacky/Nginx-UI/internal/middleware"
	"github.com/gin-gonic/gin"
)

// InitRouter registers the plugin management API. It belongs to the
// authenticated, proxied group so a cluster node can manage its own plugins.
func InitRouter(r *gin.RouterGroup) {
	r.GET("/plugins", GetPluginList)
	r.GET("/plugins/webapp", GetWebappEntries)
	r.GET("/plugins/spec", GetSpec)
	r.GET("/plugins/:id/settings", GetPluginSettings)
	r.GET("/plugins/:id/logs", GetPluginLogs)
	r.GET("/plugins/:id/usage", GetPluginUsage)

	// Installing code that runs on the host is the most sensitive operation
	// nginx-ui offers, so every mutation needs a fresh authentication and is
	// closed on the public demo.
	o := r.Group("", middleware.RequireSecureSession(), middleware.RejectInDemo())
	{
		o.POST("/plugins", InstallPlugin)
		o.POST("/plugins/inspect", InspectPlugin)
		o.DELETE("/plugins/:id", UninstallPlugin)
		o.POST("/plugins/:id/enable", EnablePlugin)
		o.POST("/plugins/:id/disable", DisablePlugin)
		o.POST("/plugins/:id/settings", SavePluginSettings)
	}
}

// InitStaticRouter serves the browser assets of the enabled plugins. The
// routes live outside /api because the bundles are loaded as plain scripts.
// Caching is left to serveStatic, which revalidates against the file mtime.
func InitStaticRouter(r *gin.Engine) {
	r.GET("/plugins/:id/webapp/*filepath", middleware.IPWhiteList(), ServeWebapp)
	r.GET("/plugins/:id/pages/*filepath", middleware.IPWhiteList(), ServePage)
}
