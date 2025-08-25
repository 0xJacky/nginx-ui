package nginx

import (
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/uozi-tech/cosy/logger"
)

var (
	nginxPrefix string
)

// GetNginxExeDir Returns the directory containing the nginx executable
func GetNginxExeDir() string {
	return filepath.Dir(getNginxSbinPath())
}

// Resolves relative paths by joining them with the nginx executable directory on Windows
func resolvePath(path string) string {
	if path == "" {
		return ""
	}

	// Handle relative paths on Windows
	if runtime.GOOS == "windows" && !filepath.IsAbs(path) {
		return filepath.Join(GetNginxExeDir(), path)
	}

	return path
}

// GetPrefix returns the prefix of the nginx executable
func GetPrefix() string {
	if nginxPrefix != "" {
		return nginxPrefix
	}

	out := getNginxV()
	r, _ := regexp.Compile(`--prefix=(\S+)`)
	match := r.FindStringSubmatch(out)
	if len(match) < 1 {
		logger.Debug("nginx.GetPrefix len(match) < 1")
		if runtime.GOOS == "windows" {
			nginxPrefix = GetNginxExeDir()
		} else {
			nginxPrefix = "/usr/local/nginx"
		}
		return nginxPrefix
	}

	nginxPrefix = resolvePath(match[1])
	return nginxPrefix
}

// GetConfPath returns the path of the nginx configuration file
func GetConfPath(dir ...string) (confPath string) {
	if settings.NginxSettings.ConfigDir == "" {
		out := getNginxV()
		r, _ := regexp.Compile("--conf-path=(.*)/(.*.conf)")
		match := r.FindStringSubmatch(out)
		if len(match) < 1 {
			logger.Error("nginx.GetConfPath len(match) < 1")
			return ""
		}
		confPath = match[1]
	} else {
		confPath = settings.NginxSettings.ConfigDir
	}

	confPath = resolvePath(confPath)

	joined := filepath.Clean(filepath.Join(confPath, filepath.Join(dir...)))
	if !helper.IsUnderDirectory(joined, confPath) {
		return confPath
	}
	return joined
}

// GetConfEntryPath returns the path of the nginx configuration file
func GetConfEntryPath() (path string) {
	if settings.NginxSettings.ConfigPath == "" {
		out := getNginxV()
		r, _ := regexp.Compile("--conf-path=(.*.conf)")
		match := r.FindStringSubmatch(out)
		if len(match) < 1 {
			logger.Error("nginx.GetConfEntryPath len(match) < 1")
			return ""
		}
		path = match[1]
	} else {
		path = settings.NginxSettings.ConfigPath
	}

	return resolvePath(path)
}

// GetPIDPath returns the path of the nginx PID file
func GetPIDPath() (path string) {
	if settings.NginxSettings.PIDPath == "" {
		out := getNginxV()
		r, _ := regexp.Compile("--pid-path=(.*.pid)")
		match := r.FindStringSubmatch(out)
		if len(match) < 1 {
			logger.Error("pid path not found in nginx -V output")
			return ""
		}
		path = match[1]
	} else {
		path = settings.NginxSettings.PIDPath
	}

	return resolvePath(path)
}

// GetSbinPath returns the path of the nginx executable
func GetSbinPath() (path string) {
	return getNginxSbinPath()
}

// GetAccessLogPath returns the path of the nginx access log file
func GetAccessLogPath() (path string) {
	path = settings.NginxSettings.AccessLogPath

	if path == "" {
		out := getNginxV()
		r, _ := regexp.Compile(`--http-log-path=(\S+)`)
		match := r.FindStringSubmatch(out)
		if len(match) > 1 {
			path = match[1]
			resolvedPath := resolvePath(path)

			// Check if the matched path exists but is not a regular file
			if !isValidRegularFile(resolvedPath) {
				logger.Debug("access log path from nginx -V exists but is not a regular file, try to get from nginx -T output", "path", resolvedPath)
				fallbackPath := getAccessLogPathFromNginxT()
				if fallbackPath != "" {
					path = fallbackPath
					return path // Already resolved in getAccessLogPathFromNginxT
				}
			}
		}
		if path == "" {
			logger.Debug("access log path not found in nginx -V output, try to get from nginx -T output")
			path = getAccessLogPathFromNginxT()
		}
	}

	return resolvePath(path)
}

// GetErrorLogPath returns the path of the nginx error log file
func GetErrorLogPath() string {
	path := settings.NginxSettings.ErrorLogPath

	if path == "" {
		out := getNginxV()
		r, _ := regexp.Compile(`--error-log-path=(\S+)`)
		match := r.FindStringSubmatch(out)
		if len(match) > 1 {
			path = match[1]
			resolvedPath := resolvePath(path)

			// Check if the matched path exists but is not a regular file
			if !isValidRegularFile(resolvedPath) {
				logger.Debug("error log path from nginx -V exists but is not a regular file, try to get from nginx -T output", "path", resolvedPath)
				fallbackPath := getErrorLogPathFromNginxT()
				if fallbackPath != "" {
					path = fallbackPath
					return path // Already resolved in getErrorLogPathFromNginxT
				}
			}
		}
		if path == "" {
			logger.Debug("error log path not found in nginx -V output, try to get from nginx -T output")
			path = getErrorLogPathFromNginxT()
		}
	}

	return resolvePath(path)
}

// GetModulesPath returns the path of the nginx modules
func GetModulesPath() string {
	// First try to get from nginx -V output
	out := getNginxV()
	if out != "" {
		// Look for --modules-path in the output
		if strings.Contains(out, "--modules-path=") {
			parts := strings.Split(out, "--modules-path=")
			if len(parts) > 1 {
				// Extract the path
				path := strings.Split(parts[1], " ")[0]
				// Remove quotes if present
				path = strings.Trim(path, "\"")
				return resolvePath(path)
			}
		}
	}

	// Default path if not found
	if runtime.GOOS == "windows" {
		return resolvePath("modules")
	}
	return resolvePath("/usr/lib/nginx/modules")
}
