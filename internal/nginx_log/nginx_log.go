package nginx_log

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/0xJacky/Nginx-UI/internal/cache"
	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/uozi-tech/cosy/logger"
)

// Regular expression for log directives - matches access_log or error_log
var logDirectiveRegex = regexp.MustCompile(`(?m)(access_log|error_log)\s+([^\s;]+)(?:\s+[^;]+)?;`)

// Use init function to automatically register callback
func init() {
	// Register the callback directly with the global registry
	cache.RegisterCallback(scanForLogDirectives)
}

// scanForLogDirectives scans and parses configuration files for log directives
func scanForLogDirectives(configPath string, content []byte) error {
	// Clear previous scan results when scanning the main config
	if configPath == nginx.GetConfPath("", "nginx.conf") {
		ClearLogCache()
	}

	// Find log directives using regex
	matches := logDirectiveRegex.FindAllSubmatch(content, -1)

	// Parse log paths
	for _, match := range matches {
		if len(match) >= 3 {
			directiveType := string(match[1]) // "access_log" or "error_log"
			logPath := string(match[2])       // Path to log file

			// Validate log path
			if IsLogPathUnderWhiteList(logPath) && isValidLogPath(logPath) {
				logType := "access"
				if directiveType == "error_log" {
					logType = "error"
				}

				// Add to cache
				AddLogPath(logPath, logType, filepath.Base(logPath))
			}
		}
	}

	return nil
}

// GetAllLogs returns all log paths
func GetAllLogs(filters ...func(*NginxLogCache) bool) []*NginxLogCache {
	return GetAllLogPaths(filters...)
}

// isValidLogPath checks if a log path is valid:
// 1. It must be a regular file or a symlink to a regular file
// 2. It must not point to a console or special device
// 3. It must be under the whitelist directories
func isValidLogPath(logPath string) bool {
	// First check if the path is in the whitelist
	if !IsLogPathUnderWhiteList(logPath) {
		logger.Warn("Log path is not under whitelist:", logPath)
		return false
	}

	// Check if the path exists
	fileInfo, err := os.Lstat(logPath)
	if err != nil {
		// If the file doesn't exist, it might be created later
		// We'll assume it's valid for now
		return true
	}

	// If it's a symlink, follow it
	if fileInfo.Mode()&os.ModeSymlink != 0 {
		linkTarget, err := os.Readlink(logPath)
		if err != nil {
			return false
		}

		// Make the link target path absolute if it's relative
		if !filepath.IsAbs(linkTarget) {
			linkTarget = filepath.Join(filepath.Dir(logPath), linkTarget)
		}

		// Check the target file
		targetInfo, err := os.Stat(linkTarget)
		if err != nil {
			return false
		}

		// Only accept regular files as targets
		return targetInfo.Mode().IsRegular()
	}

	// For non-symlinks, just check if it's a regular file
	return fileInfo.Mode().IsRegular()
}

// IsLogPathUnderWhiteList checks if a log path is under one of the paths in LogDirWhiteList
func IsLogPathUnderWhiteList(path string) bool {
	cacheKey := fmt.Sprintf("isLogPathUnderWhiteList:%s", path)
	res, ok := cache.Get(cacheKey)

	// Deep copy the whitelist
	logDirWhiteList := append([]string{}, settings.NginxSettings.LogDirWhiteList...)

	accessLogPath := nginx.GetAccessLogPath()
	errorLogPath := nginx.GetErrorLogPath()

	if accessLogPath != "" {
		logDirWhiteList = append(logDirWhiteList, filepath.Dir(accessLogPath))
	}
	if errorLogPath != "" {
		logDirWhiteList = append(logDirWhiteList, filepath.Dir(errorLogPath))
	}

	// No cache, check it
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
