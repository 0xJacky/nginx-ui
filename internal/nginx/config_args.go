package nginx

import (
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/uozi-tech/cosy/logger"
)

var nginxExePath string

// Returns the path to the nginx executable
func getNginxExePath() string {
	if nginxExePath != "" {
		return nginxExePath
	}

	var path string
	var err error
	if runtime.GOOS == "windows" {
		path, err = exec.LookPath("nginx.exe")
	} else {
		path, err = exec.LookPath("nginx")
	}
	if err == nil {
		nginxExePath = path
		return nginxExePath
	}
	return nginxExePath
}

// Returns the directory containing the nginx executable
func GetNginxExeDir() string {
	return filepath.Dir(getNginxExePath())
}

func getNginxV() string {
	exePath := getNginxExePath()
	out, err := execCommand(exePath, "-V")
	if err != nil {
		logger.Error(err)
		return ""
	}
	return string(out)
}

// getNginxT executes nginx -T and returns the output
func getNginxT() string {
	exePath := getNginxExePath()
	out, err := execCommand(exePath, "-T")
	if err != nil {
		logger.Error(err)
		return ""
	}
	return out
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
	out := getNginxV()
	r, _ := regexp.Compile(`--prefix=(\S+)`)
	match := r.FindStringSubmatch(out)
	if len(match) < 1 {
		logger.Error("nginx.GetPrefix len(match) < 1")
		return "/usr/local/nginx"
	}
	return resolvePath(match[1])
}

// GetConfPath returns the path to the nginx configuration file
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

// GetConfEntryPath returns the path to the nginx configuration file
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

// GetPIDPath returns the path to the nginx PID file
func GetPIDPath() (path string) {
	if settings.NginxSettings.PIDPath == "" {
		out := getNginxV()
		r, _ := regexp.Compile("--pid-path=(.*.pid)")
		match := r.FindStringSubmatch(out)
		if len(match) < 1 {
			logger.Error("nginx.GetPIDPath len(match) < 1")
			return ""
		}
		path = match[1]
	} else {
		path = settings.NginxSettings.PIDPath
	}

	return resolvePath(path)
}

// GetSbinPath returns the path to the nginx executable
func GetSbinPath() (path string) {
	out := getNginxV()
	r, _ := regexp.Compile(`--sbin-path=(\S+)`)
	match := r.FindStringSubmatch(out)
	if len(match) < 1 {
		logger.Error("nginx.GetPIDPath len(match) < 1")
		return ""
	}
	path = match[1]

	return resolvePath(path)
}

// GetAccessLogPath returns the path to the nginx access log file
func GetAccessLogPath() (path string) {
	if settings.NginxSettings.AccessLogPath == "" {
		out := getNginxV()
		r, _ := regexp.Compile(`--http-log-path=(\S+)`)
		match := r.FindStringSubmatch(out)
		if len(match) < 1 {
			logger.Error("nginx.GetAccessLogPath len(match) < 1")
			return ""
		}
		path = match[1]
	} else {
		path = settings.NginxSettings.AccessLogPath
	}

	return resolvePath(path)
}

// GetErrorLogPath returns the path to the nginx error log file
func GetErrorLogPath() string {
	if settings.NginxSettings.ErrorLogPath == "" {
		out := getNginxV()
		r, _ := regexp.Compile(`--error-log-path=(\S+)`)
		match := r.FindStringSubmatch(out)
		if len(match) < 1 {
			logger.Error("nginx.GetErrorLogPath len(match) < 1")
			return ""
		}
		return resolvePath(match[1])
	} else {
		return resolvePath(settings.NginxSettings.ErrorLogPath)
	}
}

// GetModulesPath returns the nginx modules path
func GetModulesPath() string {
	// First try to get from nginx -V output
	stdOut, stdErr := execCommand(getNginxExePath(), "-V")
	if stdErr != nil {
		return ""
	}
	if stdOut != "" {
		// Look for --modules-path in the output
		if strings.Contains(stdOut, "--modules-path=") {
			parts := strings.Split(stdOut, "--modules-path=")
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
