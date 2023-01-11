package nginx

import (
    "errors"
    "github.com/0xJacky/Nginx-UI/server/settings"
    "log"
    "os/exec"
    "path/filepath"
    "regexp"
    "strings"
)

func TestConf() error {
    out, err := exec.Command("nginx", "-t").CombinedOutput()
    if err != nil {
        log.Println(err)
    }
    output := string(out)
    log.Println(output)
    if strings.Contains(output, "failed") {
        return errors.New(output)
    }
    return nil
}

func Reload() string {
    out, err := exec.Command("nginx", "-s", "reload").CombinedOutput()

    if err != nil {
        log.Println(err)
        return err.Error()
    }

    output := string(out)
    log.Println(output)
    if strings.Contains(output, "failed") {
        return output
    }
    return ""
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
