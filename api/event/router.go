package event

import "github.com/gin-gonic/gin"

// InitRouter registers the WebSocket event bus route
func InitRouter(r *gin.RouterGroup) {
	r.GET("events", Bus)
}
