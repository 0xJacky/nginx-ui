package notification

import "github.com/gin-gonic/gin"

func InitRouter(r *gin.RouterGroup) {
	r.GET("notifications", GetList)
	r.GET("notification/:id", Get)
	r.DELETE("notification/:id", Destroy)
	r.DELETE("notifications", DestroyAll)
}
