package public

import "github.com/gin-gonic/gin"

func InitRouter(r *gin.RouterGroup) {
	r.GET("/icp_settings", GetICPSettings)
}
