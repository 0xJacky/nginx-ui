package streams

import (
	"github.com/0xJacky/Nginx-UI/internal/middleware"
	"github.com/gin-gonic/gin"
)

func InitRouter(r *gin.RouterGroup) {
	r.GET("streams", GetStreams)
	r.GET("streams/:name", GetStream)
	r.PUT("streams", BatchUpdateStreams)
	o := r.Group("", middleware.RequireSecureSession())
	{
		o.POST("streams/:name", SaveStream)
		o.POST("streams/:name/rename", RenameStream)
		o.POST("streams/:name/enable", EnableStream)
		o.POST("streams/:name/disable", DisableStream)
		o.DELETE("streams/:name", DeleteStream)
		o.POST("streams/:name/duplicate", Duplicate)
	}
	r.POST("streams/:name/advance", AdvancedEdit)
}
