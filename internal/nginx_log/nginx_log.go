package nginx_log

import (
	"fmt"
	"github.com/0xJacky/Nginx-UI/internal/cache"
	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/settings"
	"path/filepath"
)

// IsLogPathUnderWhiteList checks if the log path is under one of the paths in LogDirWhiteList
func IsLogPathUnderWhiteList(path string) bool {
	cacheKey := fmt.Sprintf("isLogPathUnderWhiteList:%s", path)
	res, ok := cache.Get(cacheKey)

	// deep copy
	logDirWhiteList := append([]string{}, settings.NginxSettings.LogDirWhiteList...)

	accessLogPath := nginx.GetAccessLogPath()
	errorLogPath := nginx.GetErrorLogPath()

	if accessLogPath != "" {
		logDirWhiteList = append(logDirWhiteList, filepath.Dir(accessLogPath))
	}
	if errorLogPath != "" {
		logDirWhiteList = append(logDirWhiteList, filepath.Dir(errorLogPath))
	}

	// no cache, check it
	if !ok {
		for _, whitePath := range logDirWhiteList {
			if helper.IsUnderDirectory(path, whitePath) {
				cache.Set(cacheKey, true, 0)
				return true
			}
		}
		return false
	}
	return res.(bool)
}
