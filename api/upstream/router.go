package upstream

import "github.com/gin-gonic/gin"

func InitHTTPRouter(r *gin.RouterGroup) {
	r.GET("/upstream/availability", GetAvailability)
	r.GET("/upstream/sockets", GetSocketList)
	r.GET("/upstream/health_check/status", GetHealthCheckStatus)
	r.PUT("/upstream/socket/:socket", UpdateSocketConfig)
}

func InitWebSocketRouter(r *gin.RouterGroup) {
	r.GET("/upstream/availability_ws", AvailabilityWebSocket)
}
