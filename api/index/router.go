package index

import "github.com/gin-gonic/gin"

// InitRouter registers all the index related routes
func InitRouter(r *gin.RouterGroup) {
	r.GET("index/status", GetIndexStatus)
}
