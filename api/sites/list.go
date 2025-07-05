package sites

import (
	"net/http"

	"github.com/0xJacky/Nginx-UI/internal/site"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/gin-gonic/gin"
	"github.com/spf13/cast"
	"github.com/uozi-tech/cosy"
)

func GetSiteList(c *gin.Context) {
	// Parse query parameters
	options := &site.ListOptions{
		Search:     c.Query("search"),
		Status:     c.Query("status"),
		OrderBy:    c.Query("sort_by"),
		Sort:       c.DefaultQuery("order", "desc"),
		EnvGroupID: cast.ToUint64(c.Query("env_group_id")),
	}

	// Get sites from database
	s := query.Site
	sTx := s.Preload(s.EnvGroup)
	if options.EnvGroupID != 0 {
		sTx = sTx.Where(s.EnvGroupID.Eq(options.EnvGroupID))
	}

	sites, err := sTx.Find()
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	// Get site configurations using the internal logic
	configs, err := site.GetSiteConfigs(c, options, sites)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": configs,
	})
}
