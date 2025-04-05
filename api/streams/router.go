package streams

import "github.com/gin-gonic/gin"

func InitRouter(r *gin.RouterGroup) {
	r.GET("streams", GetStreams)
	r.GET("streams/:name", GetStream)
	r.PUT("streams", BatchUpdateStreams)
	r.POST("streams/:name", SaveStream)
	r.POST("streams/:name/rename", RenameStream)
	r.POST("streams/:name/enable", EnableStream)
	r.POST("streams/:name/disable", DisableStream)
	r.POST("streams/:name/advance", AdvancedEdit)
	r.DELETE("streams/:name", DeleteStream)
	r.POST("streams/:name/duplicate", Duplicate)
}
