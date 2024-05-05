package nginx

import (
	"github.com/0xJacky/Nginx-UI/settings"
	"os/exec"
	"sync"
	"time"
)

var (
	mutex      sync.Mutex
	lastOutput string
)

func TestConf() (out string) {
	mutex.Lock()
	defer mutex.Unlock()
	if settings.NginxSettings.TestConfigCmd != "" {
		out = execShell(settings.NginxSettings.TestConfigCmd)

		return
	}

	out = execCommand("nginx", "-t")

	return
}

func Reload() (out string) {
	mutex.Lock()
	defer mutex.Unlock()
	if settings.NginxSettings.ReloadCmd != "" {
		out = execShell(settings.NginxSettings.ReloadCmd)
		return
	}

	out = execCommand("nginx", "-s", "reload")

	return
}

func Restart() {
	mutex.Lock()
	defer mutex.Unlock()

	// fix(docker): nginx restart always output network error
	time.Sleep(500 * time.Millisecond)

	if settings.NginxSettings.RestartCmd != "" {
		lastOutput = execShell(settings.NginxSettings.RestartCmd)

		return
	}

	pidPath := GetPIDPath()
	daemon := GetSbinPath()

	lastOutput = execCommand("start-stop-daemon", "--stop", "--quiet", "--oknodo", "--retry=TERM/30/KILL/5", "--pidfile", pidPath)

	if daemon == "" {
		lastOutput += execCommand("nginx")

		return
	}

	lastOutput += execCommand("start-stop-daemon", "--start", "--quiet", "--pidfile", pidPath, "--exec", daemon)

	return
}

func GetLastOutput() string {
	mutex.Lock()
	defer mutex.Unlock()
	return lastOutput
}

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
