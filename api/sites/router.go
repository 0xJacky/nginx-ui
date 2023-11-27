package sites

import "github.com/gin-gonic/gin"

func InitRouter(r *gin.RouterGroup) {
	r.GET("domains", GetDomains)
	r.GET("domain/:name", GetDomain)
	r.POST("domain/:name", SaveDomain)
	r.POST("domain/:name/enable", EnableDomain)
	r.POST("domain/:name/disable", DisableDomain)
	r.POST("domain/:name/advance", DomainEditByAdvancedMode)
	r.DELETE("domain/:name", DeleteDomain)
	r.POST("domain/:name/duplicate", DuplicateSite)
	r.POST("auto_cert/:name", AddDomainToAutoCert)
	r.DELETE("auto_cert/:name", RemoveDomainFromAutoCert)
}
