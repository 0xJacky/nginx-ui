package nginx

import (
	"github.com/0xJacky/Nginx-UI/settings"
	"os/exec"
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

	pidPath := GetPIDPath()
	daemon := GetSbinPath()

	out = execCommand("start-stop-daemon", "--stop", "--quiet", "--oknodo", "--retry=TERM/30/KILL/5", "--pidfile", pidPath)

	if daemon == "" {
		out += execCommand("nginx")

		return
	}

	out += execCommand("start-stop-daemon", "--start", "--quiet", "--pidfile", pidPath, "--exec", daemon)

	return
}
