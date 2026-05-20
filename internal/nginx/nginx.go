package nginx

import (
	"context"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"math"

	"github.com/0xJacky/Nginx-UI/internal/docker"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/shirou/gopsutil/v4/process"
	"github.com/uozi-tech/cosy/logger"
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
	sbin := GetSbinPath()
	if sbin == "" {
		return execCommand("nginx", "-t")
	}
	return execCommand(sbin, "-t")
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

	sbin := GetSbinPath()

	if sbin == "" {
		return execCommand("nginx", "-s", "reload")
	}
	return execCommand(sbin, "-s", "reload")
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

// GetLastResult returns the last output of the nginx command
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
	switch settings.NginxSettings.ControlMode() {
	case settings.ControlModeHostViaSSH:
		return isRunningViaSystemd()
	case settings.ControlModeExternalContainer:
		return docker.StatPath(pidPath)
	default:
		return isProcessRunning(pidPath)
	}
}

// isRunningViaSystemd queries `systemctl is-active <unit>` over SSH.
// Falls back to PID-file existence check (via bind-mount) on systemctl failure.
func isRunningViaSystemd() bool {
	unit := settings.NginxSettings.HostSystemdUnitName
	if unit == "" {
		unit = "nginx.service"
	}
	systemctl := settings.NginxSettings.HostSystemctlPath
	if systemctl == "" {
		systemctl = "/bin/systemctl"
	}
	runner := resolveRunner()
	out, err := runner.Exec(context.Background(), systemctl, "is-active", unit)
	if err == nil && strings.TrimSpace(out) == "active" {
		return true
	}
	// Fallback: bind-mounted PID file is visible to the container as a local path.
	return isProcessRunning(GetPIDPath())
}

// isProcessRunning checks if the process with the PID from pidPath is actually running
func isProcessRunning(pidPath string) bool {
	logger.Debugf("isProcessRunning pidPath: %s", pidPath)
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

	// Check that pid is within int32 and positive bounds
	if pid <= 0 || pid > math.MaxInt32 {
		return false
	}

	// Use gopsutil for cross-platform process existence check
	exists, err := process.PidExists(int32(pid))
	if err != nil {
		return false
	}

	if exists {
		return true
	}

	return false
}
