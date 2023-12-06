package nginx

import (
	"github.com/0xJacky/Nginx-UI/internal/logger"
	"github.com/0xJacky/Nginx-UI/settings"
	"os/exec"
	"path/filepath"
	"regexp"
)

func execShell(cmd string) (out string) {
	bytes, err := exec.Command("/bin/sh", "-c", cmd).CombinedOutput()
	out = string(bytes)
	if err != nil {
		out += " " + err.Error()
	}
	return
}

func execCommand(name string, cmd ...string) (out string) {
	bytes, err := exec.Command(name, cmd...).CombinedOutput()
	out = string(bytes)
	if err != nil {
		out += " " + err.Error()
	}
	return
}

func TestConf() (out string) {
	if settings.NginxSettings.TestConfigCmd != "" {
		out = execShell(settings.NginxSettings.TestConfigCmd)

		return
	}

	out = execCommand("nginx", "-t")

	return
}

func Reload() (out string) {
	if settings.NginxSettings.ReloadCmd != "" {
		out = execShell(settings.NginxSettings.ReloadCmd)
		return
	}

	out = execCommand("nginx", "-s", "reload")

	return
}

func Restart() (out string) {
	if settings.NginxSettings.RestartCmd != "" {
		out = execShell(settings.NginxSettings.RestartCmd)

		return
	}

	out = execCommand("nginx", "-s", "stop")

	out += execCommand("nginx")

	return
}

func GetConfPath(dir ...string) string {
	var confPath string

	if settings.NginxSettings.ConfigDir == "" {
		out, err := exec.Command("nginx", "-V").CombinedOutput()
		if err != nil {
			logger.Error(err)
			return ""
		}
		r, _ := regexp.Compile("--conf-path=(.*)/(.*.conf)")
		match := r.FindStringSubmatch(string(out))
		if len(match) < 1 {
			logger.Error("nginx.GetConfPath len(match) < 1")
			return ""
		}
		confPath = r.FindStringSubmatch(string(out))[1]
	} else {
		confPath = settings.NginxSettings.ConfigDir
	}

	return filepath.Join(confPath, filepath.Join(dir...))
}

func GetNginxPIDPath() string {
	var confPath string

	if settings.NginxSettings.PIDPath == "" {
		out, err := exec.Command("nginx", "-V").CombinedOutput()
		if err != nil {
			logger.Error(err)
			return ""
		}
		r, _ := regexp.Compile("--pid-path=(.*.pid)")
		match := r.FindStringSubmatch(string(out))
		if len(match) < 1 {
			logger.Error("nginx.GetNginxPIDPath len(match) < 1")
			return ""
		}
		confPath = r.FindStringSubmatch(string(out))[1]
	} else {
		confPath = settings.NginxSettings.PIDPath
	}

	return confPath
}
