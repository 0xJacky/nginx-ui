package nginx

import (
	"os"
	"strconv"
	"strings"
	"sync"
	"syscall"
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

	// Clear the modules cache when reloading Nginx
	clearModulesCache()

	if !IsRunning() {
		restart()
		return
	}

	if settings.NginxSettings.ReloadCmd != "" {
		return execShell(settings.NginxSettings.ReloadCmd)
	}
	return execCommand("nginx", "-s", "reload")
}

func restart() {
	// fix(docker): nginx restart always output network error
	time.Sleep(500 * time.Millisecond)

	if settings.NginxSettings.RestartCmd != "" {
		lastStdOut, lastStdErr = execShell(settings.NginxSettings.RestartCmd)
		return
	}

	pidPath := GetPIDPath()
	daemon := GetSbinPath()

	// Check if nginx is running before attempting to stop it
	if IsRunning() {
		lastStdOut, lastStdErr = execCommand("start-stop-daemon", "--stop", "--quiet", "--oknodo", "--retry=TERM/30/KILL/5", "--pidfile", pidPath)
		if lastStdErr != nil {
			return
		}
	}

	if daemon == "" {
		lastStdOut, lastStdErr = execCommand("nginx")
		return
	}

	lastStdOut, lastStdErr = execCommand("start-stop-daemon", "--start", "--quiet", "--pidfile", pidPath, "--exec", daemon)
}

// Restart restarts the nginx
func Restart() {
	mutex.Lock()
	defer mutex.Unlock()

	// Clear the modules cache when restarting Nginx
	clearModulesCache()

	restart()
}

// GetLastOutput returns the last output of the nginx command
func GetLastResult() *ControlResult {
	mutex.Lock()
	defer mutex.Unlock()
	return &ControlResult{
		stdOut: lastStdOut,
		stdErr: lastStdErr,
	}
}

func IsRunning() bool {
	pidPath := GetPIDPath()
	switch settings.NginxSettings.RunningInAnotherContainer() {
	case true:
		return docker.StatPath(pidPath)
	case false:
		return isProcessRunning(pidPath)
	}
	return false
}

// isProcessRunning checks if the process with the PID from pidPath is actually running
func isProcessRunning(pidPath string) bool {
	// Check if PID file exists
	if fileInfo, err := os.Stat(pidPath); err != nil || fileInfo.Size() == 0 {
		return false
	}

	// Read PID from file
	pidBytes, err := os.ReadFile(pidPath)
	if err != nil {
		return false
	}

	pidStr := strings.TrimSpace(string(pidBytes))
	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		return false
	}

	// Cross-platform process existence check
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	// On Unix systems, FindProcess always succeeds and returns a Process for the given pid,
	// regardless of whether the process exists. To test whether the process actually exists,
	// see whether p.Signal(syscall.Signal(0)) reports an error.
	err = process.Signal(syscall.Signal(0))
	if err == nil {
		// Process exists and we can signal it
		return true
	}
	return false
}
