package notification

import "github.com/gin-gonic/gin"

func InitRouter(r *gin.RouterGroup) {
	r.GET("notifications", GetList)
	r.GET("notifications/:id", Get)
	r.DELETE("notifications/:id", Destroy)
	r.DELETE("notifications", DestroyAll)

	r.GET("notifications/live", Live)
}
