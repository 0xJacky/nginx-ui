package sites

import (
	"github.com/0xJacky/Nginx-UI/internal/middleware"
	"github.com/gin-gonic/gin"
)

func InitRouter(r *gin.RouterGroup) {
	// Initialize WebSocket notifications for site checking
	InitWebSocketNotifications()

	r.GET("sites", GetSiteList)
	r.GET("sites/:name", GetSite)

	// site navigation endpoints
	r.GET("site_navigation", GetSiteNavigation)
	r.GET("site_navigation/status", GetSiteNavigationStatus)
	r.GET("site_navigation/health_check/:id", GetHealthCheck)
	r.POST("site_navigation/test_health_check/:id", TestHealthCheck)
	r.GET("site_navigation_ws", SiteNavigationWebSocket)

	o := r.Group("", middleware.RequireSecureSession())
	{
		o.PUT("sites", BatchUpdateSites)
		o.POST("sites/:name/advance", DomainEditByAdvancedMode)
		o.POST("auto_cert/:name", AddDomainToAutoCert)
		o.DELETE("auto_cert/:name", RemoveDomainFromAutoCert)
		o.POST("site_navigation/order", UpdateSiteOrder)
		o.POST("site_navigation/health_check/:id", UpdateHealthCheck)

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
