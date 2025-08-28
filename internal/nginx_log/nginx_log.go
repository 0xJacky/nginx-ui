package nginx_log

import (
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/cache"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/internal/nginx_log/utlis"
	"github.com/uozi-tech/cosy/logger"
)

// Regular expression for log directives - matches access_log or error_log
var (
	logDirectiveRegex = regexp.MustCompile(`(?m)(access_log|error_log)\s+([^\s;]+)(?:\s+[^;]+)?;`)
)

// Use init function to automatically register callback
func init() {
	// Register the callback directly with the global registry
	cache.RegisterCallback("nginx_log.scanForLogDirectives", scanForLogDirectives)
}

// scanForLogDirectives scans and parses configuration files for log directives
func scanForLogDirectives(configPath string, content []byte) error {
	// Step 1: Get nginx prefix
	prefix := nginx.GetPrefix()
	
	// Step 2: Remove existing log paths - with timeout protection
	removeSuccess := make(chan bool, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.Warnf("[scanForLogDirectives] RemoveLogPathsFromConfig panicked for %s: %v", configPath, r)
			}
		}()
		RemoveLogPathsFromConfig(configPath)
		removeSuccess <- true
	}()
	
	select {
	case <-removeSuccess:
		// Success - no logging needed
	case <-time.After(2 * time.Second):
		logger.Warnf("[scanForLogDirectives] RemoveLogPathsFromConfig timed out after 2 seconds for config: %s", configPath)
	}

	// Step 3: Find log directives using regex
	matches := logDirectiveRegex.FindAllSubmatch(content, -1)

	// Step 4: Parse log paths
	for i, match := range matches {
		if len(match) >= 3 {
			// Check if this match is from a commented line
			if isCommentedMatch(content, match) {
				continue // Skip commented directives
			}

			directiveType := string(match[1]) // "access_log" or "error_log"
			logPath := string(match[2])       // Path to log file

			// Skip if log is disabled with "off"
			if logPath == "off" {
				continue
			}

			// Handle relative paths by joining with nginx prefix
			if !filepath.IsAbs(logPath) {
				logPath = filepath.Join(prefix, logPath)
			}

			// Validate log path
			if utlis.IsValidLogPath(logPath) {
				logType := "access"
				if directiveType == "error_log" {
					logType = "error"
				}

				// Add to cache with config file path - with timeout protection
				addSuccess := make(chan bool, 1)
				go func() {
					defer func() {
						if r := recover(); r != nil {
							logger.Warnf("[scanForLogDirectives] AddLogPath panicked for match %d, path %s: %v", i, logPath, r)
						}
					}()
					AddLogPath(logPath, logType, filepath.Base(logPath), configPath)
					addSuccess <- true
				}()
				
				select {
				case <-addSuccess:
					// Success - no logging needed
				case <-time.After(1 * time.Second):
					logger.Warnf("[scanForLogDirectives] AddLogPath timed out after 1 second for match %d, path %s", i, logPath)
				}
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
