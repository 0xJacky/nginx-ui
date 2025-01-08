package template

import "github.com/gin-gonic/gin"

func InitRouter(r *gin.RouterGroup) {
	r.GET("default_site_template", GetDefaultSiteTemplate)
	r.GET("templates/configs", GetTemplateConfList)
	r.GET("templates/blocks", GetTemplateBlockList)
	r.GET("templates/block/:name", GetTemplateBlock)
	r.POST("templates/block/:name", GetTemplateBlock)
}
