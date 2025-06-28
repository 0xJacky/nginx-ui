package nginx_log

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/0xJacky/Nginx-UI/internal/cache"
	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/uozi-tech/cosy/logger"
)

// Regular expression for log directives - matches access_log or error_log
var (
	logDirectiveRegex = regexp.MustCompile(`(?m)(access_log|error_log)\s+([^\s;]+)(?:\s+[^;]+)?;`)
)

// Use init function to automatically register callback
func init() {
	// Register the callback directly with the global registry
	cache.RegisterCallback(scanForLogDirectives)
}

// scanForLogDirectives scans and parses configuration files for log directives
func scanForLogDirectives(configPath string, content []byte) error {
	prefix := nginx.GetPrefix()
	// First, remove all log paths that came from this config file
	// This ensures that removed log directives are properly cleaned up
	RemoveLogPathsFromConfig(configPath)

	// Find log directives using regex
	matches := logDirectiveRegex.FindAllSubmatch(content, -1)

	// Parse log paths
	for _, match := range matches {
		if len(match) >= 3 {
			// Check if this match is from a commented line
			if isCommentedMatch(content, match) {
				continue // Skip commented directives
			}

			directiveType := string(match[1]) // "access_log" or "error_log"
			logPath := string(match[2])       // Path to log file

			// Handle relative paths by joining with nginx prefix
			if !filepath.IsAbs(logPath) {
				logPath = filepath.Join(prefix, logPath)
			}

			// Validate log path
			if isValidLogPath(logPath) {
				logType := "access"
				if directiveType == "error_log" {
					logType = "error"
				}

				// Add to cache with config file path
				AddLogPath(logPath, logType, filepath.Base(logPath), configPath)
			}
		}
	}

	return nil
}

// isCommentedMatch checks if a regex match is from a commented line
func isCommentedMatch(content []byte, match [][]byte) bool {
	// Find the position of the match in the content
	matchStr := string(match[0])
	matchIndex := strings.Index(string(content), matchStr)
	if matchIndex == -1 {
		return false
	}

	// Find the start of the line containing this match
	lineStart := matchIndex
	for lineStart > 0 && content[lineStart-1] != '\n' {
		lineStart--
	}

	// Check if the line starts with # (possibly with leading whitespace)
	for i := lineStart; i < matchIndex; i++ {
		char := content[i]
		if char == '#' {
			return true // This is a commented line
		}
		if char != ' ' && char != '\t' {
			return false // Found non-whitespace before the directive, not a comment
		}
	}

	return false
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

// IsLogPathUnderWhiteList checks if a log path is under one of the paths in LogDirWhiteList
func IsLogPathUnderWhiteList(path string) bool {
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
