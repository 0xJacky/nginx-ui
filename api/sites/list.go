package sites

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/0xJacky/Nginx-UI/internal/config"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/internal/site"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
	"github.com/spf13/cast"
	"github.com/uozi-tech/cosy"
)

func GetSiteList(c *gin.Context) {
	name := c.Query("name")
	status := c.Query("status")
	orderBy := c.Query("sort_by")
	sort := c.DefaultQuery("order", "desc")
	queryEnvGroupId := cast.ToUint64(c.Query("env_group_id"))

	configFiles, err := os.ReadDir(nginx.GetConfPath("sites-available"))
	if err != nil {
		cosy.ErrHandler(c, cosy.WrapErrorWithParams(site.ErrReadDirFailed, err.Error()))
		return
	}

	enabledConfig, err := os.ReadDir(nginx.GetConfPath("sites-enabled"))
	if err != nil {
		cosy.ErrHandler(c, cosy.WrapErrorWithParams(site.ErrReadDirFailed, err.Error()))
		return
	}

	s := query.Site
	sTx := s.Preload(s.EnvGroup)
	if queryEnvGroupId != 0 {
		sTx.Where(s.EnvGroupID.Eq(queryEnvGroupId))
	}
	sites, err := sTx.Find()
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}
	sitesMap := lo.SliceToMap(sites, func(item *model.Site) (string, *model.Site) {
		return filepath.Base(item.Path), item
	})

	configStatusMap := make(map[string]config.ConfigStatus)
	for _, site := range configFiles {
		configStatusMap[site.Name()] = config.StatusDisabled
	}

	// Check for enabled sites and maintenance mode sites
	for _, enabledSite := range enabledConfig {
		name := enabledSite.Name()

		// Check if this is a maintenance mode configuration
		if strings.HasSuffix(name, site.MaintenanceSuffix) {
			// Extract the original site name by removing maintenance suffix
			originalName := strings.TrimSuffix(name, site.MaintenanceSuffix)
			configStatusMap[originalName] = config.StatusMaintenance
		} else {
			configStatusMap[nginx.GetConfNameBySymlinkName(name)] = config.StatusEnabled
		}
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
		if status != "" && configStatusMap[file.Name()] != config.ConfigStatus(status) {
			continue
		}

		var (
			envGroupId uint64
			envGroup   *model.EnvGroup
		)

		if site, ok := sitesMap[file.Name()]; ok {
			envGroupId = site.EnvGroupID
			envGroup = site.EnvGroup
		}

		// env group filter
		if queryEnvGroupId != 0 && envGroupId != queryEnvGroupId {
			continue
		}

		indexedSite := site.GetIndexedSite(file.Name())

		configs = append(configs, config.Config{
			Name:       file.Name(),
			ModifiedAt: fileInfo.ModTime(),
			Size:       fileInfo.Size(),
			IsDir:      fileInfo.IsDir(),
			Status:     configStatusMap[file.Name()],
			EnvGroupID: envGroupId,
			EnvGroup:   envGroup,
			Urls:       indexedSite.Urls,
		})
	}

	configs = config.Sort(orderBy, sort, configs)

	c.JSON(http.StatusOK, gin.H{
		"data": configs,
	})
}
