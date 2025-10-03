package upstream

import "github.com/gin-gonic/gin"

func InitRouter(r *gin.RouterGroup) {
	r.GET("/upstream/availability", GetAvailability)
	r.GET("/upstream/availability_ws", AvailabilityWebSocket)
	r.GET("/upstream/sockets", GetSocketList)
	r.PUT("/upstream/socket/:socket", UpdateSocketConfig)
}
