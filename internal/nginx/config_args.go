package nginx

import (
	"github.com/0xJacky/Nginx-UI/internal/logger"
	"github.com/0xJacky/Nginx-UI/settings"
	"os/exec"
	"path/filepath"
	"regexp"
)

func getNginxV() string {
	out, err := exec.Command("nginx", "-V").CombinedOutput()
	if err != nil {
		logger.Error(err)
		return ""
	}
	return string(out)
}

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

	return filepath.Join(confPath, filepath.Join(dir...))
}

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

	return path
}

func GetSbinPath() (path string) {
	out := getNginxV()
	r, _ := regexp.Compile("--sbin-path=(\\S+)")
	match := r.FindStringSubmatch(out)
	if len(match) < 1 {
		logger.Error("nginx.GetPIDPath len(match) < 1")
		return ""
	}
	path = match[1]

	return path
}

func GetAccessLogPath() (path string) {
	if settings.NginxSettings.AccessLogPath == "" {
		out := getNginxV()
		r, _ := regexp.Compile("--http-log-path=(\\S+)")
		match := r.FindStringSubmatch(out)
		if len(match) < 1 {
			logger.Error("nginx.GetAccessLogPath len(match) < 1")
			return ""
		}
		path = match[1]
	} else {
		path = settings.NginxSettings.AccessLogPath
	}

	return path
}

func GetErrorLogPath() string {
	if settings.NginxSettings.ErrorLogPath == "" {
		out := getNginxV()
		r, _ := regexp.Compile("--error-log-path=(\\S+)")
		match := r.FindStringSubmatch(out)
		if len(match) < 1 {
			logger.Error("nginx.GetErrorLogPath len(match) < 1")
			return ""
		}
		return match[1]
	} else {
		return settings.NginxSettings.ErrorLogPath
	}
}
