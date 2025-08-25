package utlis

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/0xJacky/Nginx-UI/internal/cache"
	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/uozi-tech/cosy/logger"
)

// IsValidLogPath checks if a log path is valid:
// 1. It must be a regular file or a symlink to a regular file
// 2. It must not point to a console or special device
// 3. It must be under the whitelist directories
func IsValidLogPath(logPath string) bool {
	// First check if the path is in the whitelist
	if !isLogPathUnderWhiteList(logPath) {
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

	// If it's a symlink, follow it safely
	if fileInfo.Mode()&os.ModeSymlink != 0 {
		// Use EvalSymlinks to safely resolve the entire symlink chain
		// This function detects circular symlinks and returns an error
		resolvedPath, err := filepath.EvalSymlinks(logPath)
		if err != nil {
			logger.Warn("Failed to resolve symlink (possible circular reference):", logPath, "error:", err)
			return false
		}

		// Check the resolved target file
		targetInfo, err := os.Stat(resolvedPath)
		if err != nil {
			return false
		}

		// Only accept regular files as targets
		return targetInfo.Mode().IsRegular()
	}

	// For non-symlinks, just check if it's a regular file
	return fileInfo.Mode().IsRegular()
}

// isLogPathUnderWhiteList checks if a log path is under one of the paths in LogDirWhiteList
func isLogPathUnderWhiteList(path string) bool {
	prefix := nginx.GetPrefix()
	cacheKey := fmt.Sprintf("isLogPathUnderWhiteList:%s", path)
	res, ok := cache.Get(cacheKey)

	// If cached, return the result directly
	if ok {
		return res.(bool)
	}

	// Only build the whitelist when cache miss occurs
	logDirWhiteList := append([]string{}, settings.NginxSettings.LogDirWhiteList...)

	accessLogPath := nginx.GetAccessLogPath()
	errorLogPath := nginx.GetErrorLogPath()

	if accessLogPath != "" {
		logDirWhiteList = append(logDirWhiteList, filepath.Dir(accessLogPath))
	}
	if errorLogPath != "" {
		logDirWhiteList = append(logDirWhiteList, filepath.Dir(errorLogPath))
	}
	if prefix != "" {
		logDirWhiteList = append(logDirWhiteList, prefix)
	}

	// Check if path is under any whitelist directory
	for _, whitePath := range logDirWhiteList {
		if helper.IsUnderDirectory(path, whitePath) {
			cache.Set(cacheKey, true, 0)
			return true
		}
	}

	// Cache negative result as well to avoid repeated checks
	cache.Set(cacheKey, false, 0)
	return false
}
