package nginx

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/uozi-tech/cosy/logger"
)

// Regular expressions for parsing log directives from nginx -T output
const (
	// AccessLogRegexPattern matches access_log directive with unquoted path
	// Matches: access_log /path/to/file
	AccessLogRegexPattern = `(?m)^\s*access_log\s+([^\s;]+)`

	// ErrorLogRegexPattern matches error_log directive with unquoted path
	// Matches: error_log /path/to/file
	ErrorLogRegexPattern = `(?m)^\s*error_log\s+([^\s;]+)`
)

var (
	accessLogRegex *regexp.Regexp
	errorLogRegex  *regexp.Regexp
)

func init() {
	accessLogRegex = regexp.MustCompile(AccessLogRegexPattern)
	errorLogRegex = regexp.MustCompile(ErrorLogRegexPattern)
}

// isValidRegularFile checks if the given path is a valid regular file
// Returns true if the path exists and is a regular file (not a directory or special file)
func isValidRegularFile(path string) bool {
	if path == "" {
		return false
	}

	fileInfo, err := os.Stat(path)
	if err != nil {
		logger.Debug("nginx.isValidRegularFile: failed to stat file", "path", path, "error", err)
		return false
	}

	// Check if it's a regular file (not a directory or special file)
	if !fileInfo.Mode().IsRegular() {
		logger.Debug("nginx.isValidRegularFile: path is not a regular file", "path", path, "mode", fileInfo.Mode())
		return false
	}

	return true
}

// isCommentedLine checks if a line is commented (starts with #)
func isCommentedLine(line string) bool {
	trimmed := strings.TrimSpace(line)
	return strings.HasPrefix(trimmed, "#")
}

// getAccessLogPathFromNginxT extracts the first access_log path from nginx -T output
func getAccessLogPathFromNginxT() string {
	output := getNginxT()
	if output == "" {
		logger.Error("nginx.getAccessLogPathFromNginxT: nginx -T output is empty")
		return ""
	}

	lines := strings.Split(output, "\n")

	for _, line := range lines {
		// Skip commented lines
		if isCommentedLine(line) {
			continue
		}

		matches := accessLogRegex.FindStringSubmatch(line)
		if len(matches) >= 2 {
			logPath := matches[1]

			// Skip 'off' directive
			if logPath == "off" {
				continue
			}
			// Handle relative paths
			if !filepath.IsAbs(logPath) {
				logPath = filepath.Join(GetPrefix(), logPath)
			}
			resolvedPath := resolvePath(logPath)

			// Validate that the path is a regular file
			if !isValidRegularFile(resolvedPath) {
				logger.Warn("nginx.getAccessLogPathFromNginxT: path is not a valid regular file", "path", resolvedPath)
				continue
			}

			return resolvedPath
		}
	}

	logger.Error("nginx.getAccessLogPathFromNginxT: no valid access_log file found")
	return ""
}

// getErrorLogPathFromNginxT extracts the first error_log path from nginx -T output
func getErrorLogPathFromNginxT() string {
	output := getNginxT()
	if output == "" {
		logger.Error("nginx.getErrorLogPathFromNginxT: nginx -T output is empty")
		return ""
	}

	lines := strings.Split(output, "\n")

	for _, line := range lines {
		// Skip commented lines
		if isCommentedLine(line) {
			continue
		}

		matches := errorLogRegex.FindStringSubmatch(line)
		if len(matches) >= 2 {
			logPath := matches[1]

			// Handle relative paths
			if !filepath.IsAbs(logPath) {
				logPath = filepath.Join(GetPrefix(), logPath)
			}
			resolvedPath := resolvePath(logPath)

			// Validate that the path is a regular file
			if !isValidRegularFile(resolvedPath) {
				logger.Warn("nginx.getErrorLogPathFromNginxT: path is not a valid regular file", "path", resolvedPath)
				continue
			}

			return resolvedPath
		}
	}

	logger.Error("nginx.getErrorLogPathFromNginxT: no valid error_log file found")
	return ""
}
