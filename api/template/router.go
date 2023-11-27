package template

import "github.com/gin-gonic/gin"

func InitRouter(r *gin.RouterGroup) {
	r.GET("template", GetTemplate)
	r.GET("template/configs", GetTemplateConfList)
	r.GET("template/blocks", GetTemplateBlockList)
	r.GET("template/block/:name", GetTemplateBlock)
	r.POST("template/block/:name", GetTemplateBlock)
}
