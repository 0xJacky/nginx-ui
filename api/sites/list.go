package sites

import (
	"net/http"

	"github.com/0xJacky/Nginx-UI/internal/site"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/gin-gonic/gin"
	"github.com/spf13/cast"
	"github.com/uozi-tech/cosy"
)

func GetSiteList(c *gin.Context) {
	// Parse query parameters
	options := &site.ListOptions{
		Search:      c.Query("search"),
		Name:        c.Query("name"),
		Status:      c.Query("status"),
		OrderBy:     c.Query("sort_by"),
		Sort:        c.DefaultQuery("order", "desc"),
		NamespaceID: cast.ToUint64(c.Query("namespace_id")),
	}

	// Get sites from database
	s := query.Site
	db := cosy.UseDB(c)

	var sites []*model.Site
	var err error

	if options.NamespaceID == 0 {
		// Local tab: no namespace OR deploy_mode='local'
		err = db.Where("namespace_id IS NULL OR namespace_id IN (?)",
			db.Model(&model.Namespace{}).Where("deploy_mode = ?", "local").Select("id"),
		).Preload("Namespace").Find(&sites).Error
	} else {
		// Remote tab: specific namespace
		sites, err = s.Where(s.NamespaceID.Eq(options.NamespaceID)).Preload(s.Namespace).Find()
	}
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
