package dns

import (
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
		group.POST("/domains", CreateDomain)
		group.POST("/domains/:id", UpdateDomain)
		group.DELETE("/domains/:id", DeleteDomain)

		group.GET("/domains/:id/records", ListRecords)
		group.POST("/domains/:id/records", CreateRecord)
		group.PUT("/domains/:id/records/:record_id", UpdateRecord)
		group.DELETE("/domains/:id/records/:record_id", DeleteRecord)
	}
}

