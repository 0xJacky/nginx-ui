package nginx

import (
	"github.com/0xJacky/Nginx-UI/server/settings"
	"log"
	"os/exec"
	"path/filepath"
	"regexp"
)

func TestConf() string {
	out, err := exec.Command("nginx", "-t").CombinedOutput()
	if err != nil {
		log.Println("[error] nginx.TestConf", err)
	}

	return string(out)
}

func Reload() string {
	out, err := exec.Command("nginx", "-s", "reload").CombinedOutput()

	if err != nil {
		log.Println("[error] nginx.Reload", err)
	}

	return string(out)
}

func Restart() string {
	out, err := exec.Command("nginx", "-s", "reopen").CombinedOutput()

	if err != nil {
		log.Println("[error] nginx.Restart", err)
	}

	return string(out)
}

func GetConfPath(dir ...string) string {
	var confPath string

	if settings.ServerSettings.NginxConfigDir == "" {
		out, err := exec.Command("nginx", "-V").CombinedOutput()
		if err != nil {
			log.Println("nginx.GetConfPath exec.Command error", err)
			return ""
		}
		r, _ := regexp.Compile("--conf-path=(.*)/(.*.conf)")
		match := r.FindStringSubmatch(string(out))
		if len(match) < 1 {
			log.Println("nginx.GetConfPath len(match) < 1")
			return ""
		}
		confPath = r.FindStringSubmatch(string(out))[1]
	} else {
		confPath = settings.ServerSettings.NginxConfigDir
	}

	return filepath.Join(confPath, filepath.Join(dir...))
}

func GetNginxPIDPath() string {
	var confPath string

	if settings.ServerSettings.NginxPIDPath == "" {
		out, err := exec.Command("nginx", "-V").CombinedOutput()
		if err != nil {
			log.Println("nginx.GetNginxPIDPath exec.Command error", err)
			return ""
		}
		r, _ := regexp.Compile("--pid-path=(.*.pid)")
		match := r.FindStringSubmatch(string(out))
		if len(match) < 1 {
			log.Println("nginx.GetNginxPIDPath len(match) < 1")
			return ""
		}
		confPath = r.FindStringSubmatch(string(out))[1]
	} else {
		confPath = settings.ServerSettings.NginxPIDPath
	}

	return confPath
}
