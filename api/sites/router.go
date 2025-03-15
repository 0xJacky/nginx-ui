package sites

import "github.com/gin-gonic/gin"

func InitRouter(r *gin.RouterGroup) {
	r.GET("sites", GetSiteList)
	r.GET("sites/:name", GetSite)
	r.PUT("sites", BatchUpdateSites)
	r.POST("sites/:name/advance", DomainEditByAdvancedMode)
	r.POST("auto_cert/:name", AddDomainToAutoCert)
	r.DELETE("auto_cert/:name", RemoveDomainFromAutoCert)

	// rename site
	r.POST("sites/:name/rename", RenameSite)
	// enable site
	r.POST("sites/:name/enable", EnableSite)
	// disable site
	r.POST("sites/:name/disable", DisableSite)
	// save site
	r.POST("sites/:name", SaveSite)
	// delete site
	r.DELETE("sites/:name", DeleteSite)
	// duplicate site
	r.POST("sites/:name/duplicate", DuplicateSite)
}

func InitCategoryRouter(r *gin.RouterGroup) {
	r.GET("site_categories", GetCategoryList)
	r.GET("site_categories/:id", GetCategory)
	r.POST("site_categories", AddCategory)
	r.POST("site_categories/:id", ModifyCategory)
	r.DELETE("site_categories/:id", DeleteCategory)
	r.POST("site_categories/:id/recover", RecoverCategory)
	r.POST("site_categories/order", UpdateCategoriesOrder)
}
