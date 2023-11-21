package nginx

import (
	"github.com/0xJacky/Nginx-UI/server/internal/logger"
	"github.com/0xJacky/Nginx-UI/server/settings"
	"os/exec"
	"path/filepath"
	"regexp"
)

func execShell(cmd string) (out string, err error) {
	bytes, err := exec.Command("/bin/sh", "-c", cmd).CombinedOutput()
	out = string(bytes)
	return
}

func TestConf() (string, error) {
	if settings.NginxSettings.TestConfigCmd != "" {
		out, err := execShell(settings.NginxSettings.TestConfigCmd)

		if err != nil {
			logger.Error(err)
			return out, err
		}

		return out, nil
	}

	out, err := exec.Command("nginx", "-t").CombinedOutput()
	if err != nil {
		logger.Error(err)
		return string(out), err
	}

	return string(out), nil
}

func Reload() (string, error) {
	if settings.NginxSettings.ReloadCmd != "" {
		out, err := execShell(settings.NginxSettings.ReloadCmd)

		if err != nil {
			logger.Error(err)
			return out, err
		}

		return out, nil

	} else {
		out, err := exec.Command("nginx", "-s", "reload").CombinedOutput()

		if err != nil {
			logger.Error(err)
			return string(out), err
		}

		return string(out), nil
	}

}

func Restart() (string, error) {
	if settings.NginxSettings.RestartCmd != "" {
		out, err := execShell(settings.NginxSettings.RestartCmd)

		if err != nil {
			logger.Error(err)
			return "", err
		}

		return out, nil
	} else {

		out, err := exec.Command("nginx", "-s", "reopen").CombinedOutput()

		if err != nil {
			logger.Error(err)
			return "", err
		}

		return string(out), nil
	}

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
