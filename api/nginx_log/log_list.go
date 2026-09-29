package nginx_log

import (
	"net/http"
	"strings"

	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/internal/nginx_log"
	"github.com/gin-gonic/gin"
)

// GetLogList returns the known Nginx log files
func GetLogList(c *gin.Context) {
	var filters []func(*nginx_log.NginxLogCache) bool

	if logType := c.Query("type"); logType != "" {
		filters = append(filters, func(entry *nginx_log.NginxLogCache) bool {
			return entry.Type == logType
		})
	}

	if name := c.Query("name"); name != "" {
		filters = append(filters, func(entry *nginx_log.NginxLogCache) bool {
			return strings.Contains(entry.Name, name)
		})
	}

	if path, _ := helper.DecodePathParam(c.Query("path")); path != "" {
		filters = append(filters, func(entry *nginx_log.NginxLogCache) bool {
			return strings.Contains(entry.Path, path)
		})
	}

	data := nginx_log.GetAllLogPathsGrouped(filters...)

	orderBy := c.DefaultQuery("sort_by", "name")
	sort := c.DefaultQuery("order", "desc")

	c.JSON(http.StatusOK, gin.H{
		"data": nginx_log.Sort(orderBy, sort, data),
	})
}
