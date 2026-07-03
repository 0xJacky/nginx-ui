package dns

import (
	"github.com/0xJacky/Nginx-UI/internal/middleware"
	"github.com/gin-gonic/gin"

	_ "github.com/0xJacky/Nginx-UI/internal/dns/providers/alidns"
	_ "github.com/0xJacky/Nginx-UI/internal/dns/providers/cloudflare"
	_ "github.com/0xJacky/Nginx-UI/internal/dns/providers/tencentcloud"
)

func InitRouter(r *gin.RouterGroup) {
	group := r.Group("/dns")
	{
		group.GET("/domains", ListDomains)
		group.GET("/domains/:id", GetDomain)

		group.GET("/domains/:id/records", ListRecords)

		group.GET("/domains/:id/ddns", GetDDNSConfig)

		group.GET("/ddns", ListDDNSConfig)

		o := group.Group("", middleware.RequireSecureSession())
		{
			o.POST("/domains", CreateDomain)
			o.POST("/domains/:id", UpdateDomain)
			o.DELETE("/domains/:id", DeleteDomain)
			o.POST("/domains/:id/records", CreateRecord)
			o.PUT("/domains/:id/records/:record_id", UpdateRecord)
			o.DELETE("/domains/:id/records/:record_id", DeleteRecord)
			o.PUT("/domains/:id/ddns", UpdateDDNSConfig)
			o.DELETE("/domains/:id/ddns", DeleteDDNSConfig)
		}
	}
}
