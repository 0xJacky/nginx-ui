package dns

import (
	"github.com/0xJacky/Nginx-UI/internal/middleware"
	"github.com/gin-gonic/gin"

	_ "github.com/0xJacky/Nginx-UI/internal/dns/providers/alidns"
	_ "github.com/0xJacky/Nginx-UI/internal/dns/providers/azuredns"
	_ "github.com/0xJacky/Nginx-UI/internal/dns/providers/cloudflare"
	_ "github.com/0xJacky/Nginx-UI/internal/dns/providers/huaweicloud"
	_ "github.com/0xJacky/Nginx-UI/internal/dns/providers/tencentcloud"
)

func InitRouter(r *gin.RouterGroup) {
	group := r.Group("/dns")
	{
		group.GET("/domains", ListDomains)
		group.GET("/domains/:id", GetDomain)

		group.GET("/domains/:id/records", ListRecords)
		group.GET("/domains/:id/record-lines", ListRecordLines)

		group.GET("/domains/:id/ddns", GetDDNSConfig)

		group.GET("/ddns", ListDDNSConfig)

		// Every mutation here is relayed to a real DNS provider API with stored
		// credentials, so none of it belongs on a public demo.
		o := group.Group("", middleware.RequireSecureSession(), middleware.RejectInDemo())
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
