package sites

import "github.com/gin-gonic/gin"

func InitRouter(r *gin.RouterGroup) {
	r.GET("domains", GetSiteList)
	r.GET("domain/:name", GetSite)
	r.POST("domain/:name", SaveSite)
	r.POST("domain/:name/enable", EnableSite)
	r.POST("domain/:name/disable", DisableSite)
	r.POST("domain/:name/advance", DomainEditByAdvancedMode)
	r.DELETE("domain/:name", DeleteSite)
	r.POST("domain/:name/duplicate", DuplicateSite)
	r.POST("auto_cert/:name", AddDomainToAutoCert)
	r.DELETE("auto_cert/:name", RemoveDomainFromAutoCert)
}
