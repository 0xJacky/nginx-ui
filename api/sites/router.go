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
	r.PUT("sites", BatchUpdateSites)
	r.POST("sites/:name/advance", DomainEditByAdvancedMode)
	r.POST("auto_cert/:name", AddDomainToAutoCert)
	r.DELETE("auto_cert/:name", RemoveDomainFromAutoCert)

	// site navigation endpoints
	r.GET("site_navigation", GetSiteNavigation)
	r.GET("site_navigation/status", GetSiteNavigationStatus)
	r.POST("site_navigation/order", UpdateSiteOrder)
	r.GET("site_navigation/health_check/:id", GetHealthCheck)
	r.POST("site_navigation/health_check/:id", UpdateHealthCheck)
	r.POST("site_navigation/test_health_check/:id", TestHealthCheck)
	r.GET("site_navigation_ws", SiteNavigationWebSocket)

	o := r.Group("", middleware.RequireSecureSession())
	{
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
