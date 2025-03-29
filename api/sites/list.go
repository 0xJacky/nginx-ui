package sites

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/0xJacky/Nginx-UI/internal/config"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
	"github.com/spf13/cast"
	"github.com/uozi-tech/cosy"
)

func GetSiteList(c *gin.Context) {
	name := c.Query("name")
	enabled := c.Query("enabled")
	orderBy := c.Query("sort_by")
	sort := c.DefaultQuery("order", "desc")
	querySiteCategoryId := cast.ToUint64(c.Query("site_category_id"))

	configFiles, err := os.ReadDir(nginx.GetConfPath("sites-available"))
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	enabledConfig, err := os.ReadDir(nginx.GetConfPath("sites-enabled"))
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	s := query.Site
	sTx := s.Preload(s.SiteCategory)
	if querySiteCategoryId != 0 {
		sTx.Where(s.SiteCategoryID.Eq(querySiteCategoryId))
	}
	sites, err := sTx.Find()
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}
	sitesMap := lo.SliceToMap(sites, func(item *model.Site) (string, *model.Site) {
		return filepath.Base(item.Path), item
	})

	enabledConfigMap := make(map[string]bool)
	for i := range enabledConfig {
		enabledConfigMap[enabledConfig[i].Name()] = true
	}

	var configs []config.Config

	for i := range configFiles {
		file := configFiles[i]
		fileInfo, _ := file.Info()
		if file.IsDir() {
			continue
		}
		// name filter
		if name != "" && !strings.Contains(file.Name(), name) {
			continue
		}
		// status filter
		if enabled != "" {
			if enabled == "true" && !enabledConfigMap[file.Name()] {
				continue
			}
			if enabled == "false" && enabledConfigMap[file.Name()] {
				continue
			}
		}
		var (
			siteCategoryId uint64
			siteCategory   *model.SiteCategory
		)

		if site, ok := sitesMap[file.Name()]; ok {
			siteCategoryId = site.SiteCategoryID
			siteCategory = site.SiteCategory
		}

		// site category filter
		if querySiteCategoryId != 0 && siteCategoryId != querySiteCategoryId {
			continue
		}

		configs = append(configs, config.Config{
			Name:           file.Name(),
			ModifiedAt:     fileInfo.ModTime(),
			Size:           fileInfo.Size(),
			IsDir:          fileInfo.IsDir(),
			Enabled:        enabledConfigMap[file.Name()],
			SiteCategoryID: siteCategoryId,
			SiteCategory:   siteCategory,
		})
	}

	configs = config.Sort(orderBy, sort, configs)

	c.JSON(http.StatusOK, gin.H{
		"data": configs,
	})
}
