package sites

import (
	"github.com/0xJacky/Nginx-UI/api"
	"github.com/0xJacky/Nginx-UI/internal/config"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/gin-gonic/gin"
	"net/http"
	"os"
	"strings"
)

func GetSiteList(c *gin.Context) {
	name := c.Query("name")
	enabled := c.Query("enabled")
	orderBy := c.Query("order_by")
	sort := c.DefaultQuery("sort", "desc")

	configFiles, err := os.ReadDir(nginx.GetConfPath("sites-available"))
	if err != nil {
		api.ErrHandler(c, err)
		return
	}

	enabledConfig, err := os.ReadDir(nginx.GetConfPath("sites-enabled"))
	if err != nil {
		api.ErrHandler(c, err)
		return
	}

	enabledConfigMap := make(map[string]bool)
	for i := range enabledConfig {
		enabledConfigMap[enabledConfig[i].Name()] = true
	}

	var configs []config.Config

	for i := range configFiles {
		file := configFiles[i]
		fileInfo, _ := file.Info()
		if !file.IsDir() {
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
			configs = append(configs, config.Config{
				Name:       file.Name(),
				ModifiedAt: fileInfo.ModTime(),
				Size:       fileInfo.Size(),
				IsDir:      fileInfo.IsDir(),
				Enabled:    enabledConfigMap[file.Name()],
			})
		}
	}

	configs = config.Sort(orderBy, sort, configs)

	c.JSON(http.StatusOK, gin.H{
		"data": configs,
	})
}
