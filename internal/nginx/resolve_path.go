package nginx

import (
	"os"
	"path/filepath"
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

func extractConfigureArg(out, flag string) string {
	if out == "" || flag == "" {
		return ""
	}

	if !strings.HasPrefix(flag, "--") {
		flag = "--" + flag
	}

	needle := flag + "="
	idx := strings.Index(out, needle)
	if idx == -1 {
		return ""
	}

	start := idx + len(needle)
	if start >= len(out) {
		return ""
	}

	value := out[start:]
	value = strings.TrimLeft(value, " \t")
	if value == "" {
		return ""
	}

	if value[0] == '"' || value[0] == '\'' {
		quoteChar := value[0]
		rest := value[1:]
		closingIdx := strings.IndexByte(rest, quoteChar)
		if closingIdx == -1 {
			return strings.TrimSpace(rest)
		}
		return strings.TrimSpace(rest[:closingIdx])
	}

	cut := len(value)
	if idx := strings.Index(value, " --"); idx != -1 && idx < cut {
		cut = idx
	}
	if idx := strings.IndexAny(value, "\r\n"); idx != -1 && idx < cut {
		cut = idx
	}

	return strings.TrimSpace(value[:cut])
}

// GetPrefix returns the prefix of the nginx executable
func GetPrefix() string {
	if nginxPrefix != "" {
		return nginxPrefix
	}

	out := getNginxV()
	prefix := extractConfigureArg(out, "--prefix")
	if prefix == "" {
		logger.Debug("nginx.GetPrefix len(match) < 1")
		if runtime.GOOS == "windows" {
			nginxPrefix = GetNginxExeDir()
		} else {
			nginxPrefix = "/usr/local/nginx"
		}
		return nginxPrefix
	}

	nginxPrefix = resolvePath(prefix)
	return nginxPrefix
}

// GetConfPath returns the nginx configuration directory (e.g. "/etc/nginx").
// It tries to derive it from `nginx -V --conf-path=...`.
// If parsing fails, it falls back to a reasonable default instead of returning "".
func GetConfPath(dir ...string) (confPath string) {
	if settings.NginxSettings.ConfigDir == "" {
		out := getNginxV()
		fullConf := extractConfigureArg(out, "--conf-path")

		if fullConf != "" {
			confPath = filepath.Dir(fullConf)
		} else {
			if runtime.GOOS == "windows" {
				confPath = GetPrefix()
			} else {
				confPath = "/etc/nginx"
			}

			logger.Debug("nginx.GetConfPath fallback used", "base", confPath)
		}
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

// GetConfEntryPath returns the absolute path to the main nginx.conf.
// It prefers the value from `nginx -V --conf-path=...`.
// If that can't be parsed, it falls back to "<confDir>/nginx.conf".
func GetConfEntryPath() (path string) {
	if settings.NginxSettings.ConfigPath == "" {
		out := getNginxV()
		path = extractConfigureArg(out, "--conf-path")

		if path == "" {
			baseDir := GetConfPath()

			if baseDir != "" {
				path = filepath.Join(baseDir, "nginx.conf")
			} else {
				logger.Error("nginx.GetConfEntryPath: cannot determine nginx.conf path")
				path = ""
			}
		}
	} else {
		path = settings.NginxSettings.ConfigPath
	}

	return resolvePath(path)
}

// GetPIDPath returns the nginx master process PID file path.
// We try to read it from `nginx -V --pid-path=...`.
// If that fails (which often happens in container images), we probe common
// locations like /run/nginx.pid and /var/run/nginx.pid instead of just failing.
func GetPIDPath() (path string) {
	if settings.NginxSettings.PIDPath == "" {
		out := getNginxV()
		path = extractConfigureArg(out, "--pid-path")

		if path == "" {
			candidates := []string{
				"/var/run/nginx.pid",
				"/run/nginx.pid",
			}

			for _, c := range candidates {
				if _, err := os.Stat(c); err == nil {
					logger.Debug("GetPIDPath fallback hit", "path", c)
					path = c
					break
				}
			}

			if path == "" {
				logger.Error("GetPIDPath: could not determine PID path")
				return ""
			}
		}
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
		path = extractConfigureArg(out, "--http-log-path")
		if path != "" {
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
		path = extractConfigureArg(out, "--error-log-path")
		if path != "" {
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
		if path := extractConfigureArg(out, "--modules-path"); path != "" {
			return resolvePath(path)
		}
	}

	// Default path if not found
	if runtime.GOOS == "windows" {
		return resolvePath("modules")
	}
	return resolvePath("/usr/lib/nginx/modules")
}
