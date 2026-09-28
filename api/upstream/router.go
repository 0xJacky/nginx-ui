package upstream

import (
	"github.com/0xJacky/Nginx-UI/internal/middleware"
	"github.com/gin-gonic/gin"
)

func InitHTTPRouter(r *gin.RouterGroup) {
	r.GET("/upstream/availability", GetAvailability)
	r.GET("/upstream/sockets", GetSocketList)
	r.GET("/upstream/health_check/status", GetHealthCheckStatus)
	r.PUT("/upstream/socket/:socket", UpdateSocketConfig)

	// Standalone upstream groups managed by Nginx UI.
	r.GET("/upstreams", ListManagedUpstreams)
	r.GET("/upstreams/:name", GetManagedUpstream)
	r.POST("/upstream/preview", PreviewManagedUpstream)

	o := r.Group("", middleware.RequireSecureSession())
	{
		o.POST("/upstreams", CreateManagedUpstream)
		o.POST("/upstreams/:name", UpdateManagedUpstream)
		o.DELETE("/upstreams/:name", DeleteManagedUpstream)
		o.POST("/upstream/server_state", SetUpstreamServerState)
	}
}

func InitWebSocketRouter(r *gin.RouterGroup) {
	r.GET("/upstream/availability_ws", AvailabilityWebSocket)
}
