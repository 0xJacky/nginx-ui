package nginx

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"math"

	"github.com/0xJacky/Nginx-UI/internal/docker"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/google/uuid"
	"github.com/shirou/gopsutil/v4/process"
	"github.com/uozi-tech/cosy/logger"
)

var (
	commandMutex         sync.Mutex
	resultMutex          sync.RWMutex
	lastStdOut           string
	lastStdErr           error
	lastControlOperation *ControlOperation
)

type ControlOperationState string

const (
	ControlOperationRunning   ControlOperationState = "running"
	ControlOperationSucceeded ControlOperationState = "succeeded"
	ControlOperationFailed    ControlOperationState = "failed"
)

type ControlOperation struct {
	ID         string                `json:"id"`
	Action     string                `json:"action"`
	State      ControlOperationState `json:"state"`
	StartedAt  time.Time             `json:"started_at"`
	FinishedAt *time.Time            `json:"finished_at,omitempty"`
	Message    string                `json:"message,omitempty"`
	Level      int                   `json:"level"`
	ExitCode   *int                  `json:"exit_code,omitempty"`
}

var ErrControlOperationRunning = errors.New("another nginx control operation is already running")

// TestConfig tests the nginx config
func TestConfig() (stdOut string, stdErr error) {
	commandMutex.Lock()
	defer commandMutex.Unlock()
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
	commandMutex.Lock()
	defer commandMutex.Unlock()

	// Clear the modules cache when reloading Nginx
	clearModulesCache()

	if !IsRunning() {
		stdOut, stdErr = restart()
		setLastResult(stdOut, stdErr)
		return stdOut, stdErr
	}

	// SSH mode: prefer systemctl reload over nginx -s reload because the
	// container's PID namespace cannot reach the host's nginx master PID.
	if settings.NginxSettings.ControlMode() == settings.ControlModeHostViaSSH {
		systemctl := settings.NginxSettings.HostSystemctlPath
		if systemctl == "" {
			systemctl = "/bin/systemctl"
		}
		unit := settings.NginxSettings.HostSystemdUnitName
		if unit == "" {
			unit = "nginx.service"
		}
		return execCommand(systemctl, "reload", unit)
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

func restart() (stdOut string, stdErr error) {
	// fix(docker): nginx restart always output network error
	time.Sleep(500 * time.Millisecond)

	// SSH mode: route through systemctl for correct cross-namespace lifecycle.
	if settings.NginxSettings.ControlMode() == settings.ControlModeHostViaSSH {
		systemctl := settings.NginxSettings.HostSystemctlPath
		if systemctl == "" {
			systemctl = "/bin/systemctl"
		}
		unit := settings.NginxSettings.HostSystemdUnitName
		if unit == "" {
			unit = "nginx.service"
		}
		lastStdOut, lastStdErr = execCommand(systemctl, "restart", unit)
		return
	}

	if settings.NginxSettings.RestartCmd != "" {
		return execShell(settings.NginxSettings.RestartCmd)
	}

	pidPath := GetPIDPath()
	daemon := GetSbinPath()

	// Check if nginx is running before attempting to stop it
	if IsRunning() {
		stdOut, stdErr = execCommand("start-stop-daemon", "--stop", "--quiet", "--oknodo", "--retry=TERM/30/KILL/5", "--pidfile", pidPath)
		if stdErr != nil {
			return stdOut, stdErr
		}
	}

	if daemon == "" {
		return execCommand("nginx")
	}

	return execCommand("start-stop-daemon", "--start", "--quiet", "--pidfile", pidPath, "--exec", daemon)
}

// Restart restarts the nginx
func Restart() {
	commandMutex.Lock()
	defer commandMutex.Unlock()

	stdOut, stdErr := executeRestart()
	setLastResult(stdOut, stdErr)
}

// StartRestart starts a tracked asynchronous restart operation.
func StartRestart(operationID string) (*ControlOperation, error) {
	return startRestart(operationID, executeRestart)
}

func startRestart(operationID string, executor ControlFunc) (*ControlOperation, error) {
	if operationID == "" {
		operationID = uuid.NewString()
	}

	resultMutex.RLock()
	if lastControlOperation != nil && lastControlOperation.ID == operationID {
		operation := *lastControlOperation
		resultMutex.RUnlock()
		return &operation, nil
	}
	if lastControlOperation != nil && lastControlOperation.State == ControlOperationRunning {
		resultMutex.RUnlock()
		return nil, ErrControlOperationRunning
	}
	resultMutex.RUnlock()

	if !commandMutex.TryLock() {
		return nil, ErrControlOperationRunning
	}

	operation := &ControlOperation{
		ID:        operationID,
		Action:    "restart",
		State:     ControlOperationRunning,
		StartedAt: time.Now(),
		Level:     Unknown,
	}
	resultMutex.Lock()
	lastControlOperation = operation
	resultMutex.Unlock()

	operationSnapshot := *operation
	logger.Infof("Nginx control operation started: id=%s action=restart", operationID)
	go finishRestart(operationID, executor)

	return &operationSnapshot, nil
}

func executeRestart() (stdOut string, stdErr error) {
	clearModulesCache()
	return restart()
}

func finishRestart(operationID string, executor ControlFunc) {
	defer commandMutex.Unlock()
	startedAt := time.Now()

	stdOut, stdErr := executor()
	finishedAt := time.Now()
	result := &ControlResult{stdOut: stdOut, stdErr: stdErr}
	level := result.GetLevel()
	state := ControlOperationSucceeded
	if result.IsError() {
		state = ControlOperationFailed
		if stdErr != nil && level < Error {
			level = Error
		}
	}
	message := result.GetOutput()
	exitCode := getExitCode(stdErr)

	resultMutex.Lock()
	lastStdOut = stdOut
	lastStdErr = stdErr
	if lastControlOperation != nil && lastControlOperation.ID == operationID {
		lastControlOperation.State = state
		lastControlOperation.FinishedAt = &finishedAt
		lastControlOperation.Message = message
		lastControlOperation.Level = level
		lastControlOperation.ExitCode = exitCode
	}
	resultMutex.Unlock()

	duration := time.Since(startedAt)
	if state == ControlOperationFailed {
		logger.Errorf("Nginx control operation failed: id=%s action=restart duration=%s exit_code=%v output=%q", operationID, duration, exitCodeValue(exitCode), truncateControlOutput(message))
		return
	}
	logger.Infof("Nginx control operation completed: id=%s action=restart duration=%s output=%q", operationID, duration, truncateControlOutput(message))
}

// GetLastResult returns the last output of the nginx command
func GetLastResult() *ControlResult {
	resultMutex.RLock()
	defer resultMutex.RUnlock()
	return &ControlResult{
		stdOut: lastStdOut,
		stdErr: lastStdErr,
	}
}

// GetControlOperation returns a snapshot of the latest tracked control operation.
func GetControlOperation() *ControlOperation {
	resultMutex.RLock()
	defer resultMutex.RUnlock()
	if lastControlOperation == nil {
		return nil
	}
	operation := *lastControlOperation
	return &operation
}

// GetStatusSnapshot returns the command result and operation from the same state revision.
func GetStatusSnapshot() (*ControlResult, *ControlOperation) {
	resultMutex.RLock()
	defer resultMutex.RUnlock()

	result := &ControlResult{
		stdOut: lastStdOut,
		stdErr: lastStdErr,
	}
	if lastControlOperation == nil {
		return result, nil
	}
	operation := *lastControlOperation
	return result, &operation
}

func setLastResult(stdOut string, stdErr error) {
	resultMutex.Lock()
	lastStdOut = stdOut
	lastStdErr = stdErr
	resultMutex.Unlock()
}

func getExitCode(err error) *int {
	var exitError *exec.ExitError
	if !errors.As(err, &exitError) {
		return nil
	}
	exitCode := exitError.ExitCode()
	return &exitCode
}

func exitCodeValue(exitCode *int) any {
	if exitCode == nil {
		return "unknown"
	}
	return *exitCode
}

func truncateControlOutput(output string) string {
	const maxLength = 4096
	if len(output) <= maxLength {
		return output
	}
	return output[:maxLength] + "..."
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
