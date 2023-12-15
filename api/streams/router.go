package streams

import "github.com/gin-gonic/gin"

func InitRouter(r *gin.RouterGroup) {
	r.GET("streams", GetStreams)
	r.GET("stream/:name", GetStream)
	r.POST("stream/:name", SaveStream)
	r.POST("stream/:name/enable", EnableStream)
	r.POST("stream/:name/disable", DisableStream)
	r.POST("stream/:name/advance", AdvancedEdit)
	r.DELETE("stream/:name", DeleteStream)
	r.POST("stream/:name/duplicate", Duplicate)
}
