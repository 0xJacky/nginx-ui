package sites

import "github.com/gin-gonic/gin"

func InitRouter(r *gin.RouterGroup) {
	r.GET("domains", GetSiteList)
	r.GET("domains/:name", GetSite)
	r.POST("domains/:name", SaveSite)
	r.PUT("domains", BatchUpdateSites)
	r.POST("domains/:name/enable", EnableSite)
	r.POST("domains/:name/disable", DisableSite)
	r.POST("domains/:name/advance", DomainEditByAdvancedMode)
	r.DELETE("domains/:name", DeleteSite)
	r.POST("domains/:name/duplicate", DuplicateSite)
	r.POST("auto_cert/:name", AddDomainToAutoCert)
	r.DELETE("auto_cert/:name", RemoveDomainFromAutoCert)
}

func InitCategoryRouter(r *gin.RouterGroup) {
	r.GET("site_categories", GetCategoryList)
	r.GET("site_categories/:id", GetCategory)
	r.POST("site_categories", AddCategory)
	r.POST("site_categories/:id", ModifyCategory)
	r.DELETE("site_categories/:id", DeleteCategory)
	r.POST("site_categories/:id/recover", RecoverCategory)
}
