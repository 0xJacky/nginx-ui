package nginx

import (
	"os"
	"strings"
	"sync"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/docker"
	"github.com/0xJacky/Nginx-UI/settings"
)

var (
	mutex      sync.Mutex
	lastStdOut string
	lastStdErr error
)

// TestConfig tests the nginx config
func TestConfig() (stdOut string, stdErr error) {
	mutex.Lock()
	defer mutex.Unlock()
	if settings.NginxSettings.TestConfigCmd != "" {
		return execShell(settings.NginxSettings.TestConfigCmd)
	}
	return execCommand("nginx", "-t")
}

// Reload reloads the nginx
func Reload() (stdOut string, stdErr error) {
	mutex.Lock()
	defer mutex.Unlock()
	if settings.NginxSettings.ReloadCmd != "" {
		return execShell(settings.NginxSettings.ReloadCmd)
	}
	return execCommand("nginx", "-s", "reload")
}

// Restart restarts the nginx
func Restart() {
	mutex.Lock()
	defer mutex.Unlock()

	// fix(docker): nginx restart always output network error
	time.Sleep(500 * time.Millisecond)

	if settings.NginxSettings.RestartCmd != "" {
		lastStdOut, lastStdErr = execShell(settings.NginxSettings.RestartCmd)
		return
	}

	pidPath := GetPIDPath()
	daemon := GetSbinPath()

	lastStdOut, lastStdErr = execCommand("start-stop-daemon", "--stop", "--quiet", "--oknodo", "--retry=TERM/30/KILL/5", "--pidfile", pidPath)
	if lastStdErr != nil {
		return
	}

	if daemon == "" {
		lastStdOut, lastStdErr = execCommand("nginx")
		return
	}

	lastStdOut, lastStdErr = execCommand("start-stop-daemon", "--start", "--quiet", "--pidfile", pidPath, "--exec", daemon)
	return
}

// GetLastOutput returns the last output of the nginx command
func GetLastOutput() (stdOut string, stdErr error) {
	mutex.Lock()
	defer mutex.Unlock()
	return lastStdOut, lastStdErr
}

// GetModulesPath returns the nginx modules path
func GetModulesPath() string {
	// First try to get from nginx -V output
	stdOut, stdErr := execCommand("nginx", "-V")
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
				return path
			}
		}
	}

	// Default path if not found
	return "/usr/lib/nginx/modules"
}

func IsNginxRunning() bool {
	pidPath := GetPIDPath()
	switch settings.NginxSettings.RunningInAnotherContainer() {
	case true:
		return docker.StatPath(pidPath)
	case false:
		if fileInfo, err := os.Stat(pidPath); err != nil || fileInfo.Size() == 0 {
			return false
		}
		return true
	}
	return false
}
