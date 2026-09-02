package sites

import (
	"github.com/0xJacky/Nginx-UI/internal/middleware"
	"github.com/gin-gonic/gin"
)

func InitRouter(r *gin.RouterGroup) {
	r.GET("sites", GetSiteList)
	r.GET("sites/:name", GetSite)
	r.GET("sites/:name/logs", GetSiteLogs)

	// site navigation endpoints
	r.GET("site_navigation", GetSiteNavigation)
	r.GET("site_navigation/status", GetSiteNavigationStatus)
	r.GET("site_navigation/health_check/:id", GetHealthCheck)
	// The request body controls the outbound destination, so this is an
	// arbitrary-fetch primitive. Demo visitors do not get one.
	r.POST("site_navigation/test_health_check/:id", middleware.RejectInDemo(), TestHealthCheck)

	o := r.Group("", middleware.RequireSecureSession())
	{
		o.PUT("sites", BatchUpdateSites)
		o.POST("sites/:name/advance", DomainEditByAdvancedMode)
		o.POST("auto_cert/:name", AddDomainToAutoCert)
		o.DELETE("auto_cert/:name", RemoveDomainFromAutoCert)
		o.POST("site_navigation/order", UpdateSiteOrder)
		o.POST("site_navigation/health_check/:id", UpdateHealthCheck)
		o.PUT("site_navigation/health_check/sync", SyncHealthCheck)

		// batch enable sites
		o.POST("sites/batch/enable", BatchEnableSites)
		// batch disable sites
		o.POST("sites/batch/disable", BatchDisableSites)
		// rename site
		o.POST("sites/:name/rename", RenameSite)
		// enable site
		o.POST("sites/:name/enable", EnableSite)
		// disable site
		o.POST("sites/:name/disable", DisableSite)
		// save site
		o.POST("sites/:name", SaveSite)
		// delete site
		o.DELETE("sites/:name", DeleteSite)
		// duplicate site
		o.POST("sites/:name/duplicate", DuplicateSite)
		// enable maintenance mode for site
		o.POST("sites/:name/maintenance", EnableMaintenanceSite)
	}
}

// InitWebSocketRouter registers the site navigation WebSocket endpoint.
//
// It must be mounted on the WebSocket router group (AuthRequiredWS + ProxyWs).
// Browsers cannot attach an Authorization header to a WebSocket handshake, so
// the token travels in the query string and only AuthRequiredWS accepts it.
// Mounting this route on the plain HTTP group made every handshake fail with
// "Authorization failed" before any token was read (issue #1793).
func InitWebSocketRouter(r *gin.RouterGroup) {
	// Initialize WebSocket notifications for site checking
	InitWebSocketNotifications()

	r.GET("site_navigation_ws", SiteNavigationWebSocket)
}
