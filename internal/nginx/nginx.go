package nginx

import (
	"context"
	"errors"
	"fmt"
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

const reloadControlTimeout = 30 * time.Second

// TestConfig tests the nginx config
func TestConfig() (stdOut string, stdErr error) {
	commandMutex.Lock()
	defer commandMutex.Unlock()
	return testConfig()
}

func testConfig() (stdOut string, stdErr error) {
	return testConfigContext(context.Background())
}

func testConfigContext(ctx context.Context) (stdOut string, stdErr error) {
	if settings.NginxSettings.TestConfigCmd != "" {
		return execShellContext(ctx, settings.NginxSettings.TestConfigCmd)
	}
	sbin := GetSbinPath()
	if sbin == "" {
		return execCommandContext(ctx, "nginx", "-t")
	}
	return execCommandContext(ctx, sbin, "-t")
}

// Reload reloads the nginx
func Reload() (stdOut string, stdErr error) {
	commandMutex.Lock()
	defer commandMutex.Unlock()
	return reload()
}

func reload() (stdOut string, stdErr error) {
	return reloadContext(context.Background())
}

func reloadContext(ctx context.Context) (stdOut string, stdErr error) {

	// Clear the modules cache when reloading Nginx
	clearModulesCache()

	if !IsRunning() {
		stdOut, stdErr = restartContext(ctx)
		setLastResult(stdOut, stdErr)
		return stdOut, stdErr
	}

	// An operator-authored command wins on every target, as it does for
	// TestConfigCmd; execShellContext already runs it through the SSH runner.
	if settings.NginxSettings.ReloadCmd != "" {
		return execShellContext(ctx, settings.NginxSettings.ReloadCmd)
	}

	// SSH mode controls the native host service without crossing PID namespaces.
	if settings.NginxSettings.ControlMode() == settings.ControlModeHostViaSSH {
		name, args := hostReloadCommand(settings.NginxSettings)
		return execCommandContext(ctx, name, args...)
	}

	sbin := GetSbinPath()

	if sbin == "" {
		return execCommandContext(ctx, "nginx", "-s", "reload")
	}
	return execCommandContext(ctx, sbin, "-s", "reload")
}

// TryTestAndReload validates and reloads Nginx while holding the control lock
// for the complete sequence. It fails fast when another test, reload, or restart
// is already running so HTTP handlers do not accumulate behind commandMutex.
func TryTestAndReload() (testResult, reloadResult *ControlResult, ok bool) {
	if !commandMutex.TryLock() {
		return nil, nil, false
	}
	defer commandMutex.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), reloadControlTimeout)
	defer cancel()

	stdOut, stdErr := testConfigContext(ctx)
	testResult = &ControlResult{stdOut: stdOut, stdErr: stdErr}
	if testResult.IsError() {
		return testResult, nil, true
	}

	stdOut, stdErr = reloadContext(ctx)
	reloadResult = &ControlResult{stdOut: stdOut, stdErr: stdErr}
	return testResult, reloadResult, true
}

func restart() (stdOut string, stdErr error) {
	return restartContext(context.Background())
}

func restartContext(ctx context.Context) (stdOut string, stdErr error) {
	// fix(docker): nginx restart always output network error
	time.Sleep(500 * time.Millisecond)

	// An operator-authored command wins on every target, as it does for
	// TestConfigCmd; execShellContext already runs it through the SSH runner.
	if settings.NginxSettings.RestartCmd != "" {
		return execShellContext(ctx, settings.NginxSettings.RestartCmd)
	}

	// SSH mode routes restart through the host's native service manager.
	if settings.NginxSettings.ControlMode() == settings.ControlModeHostViaSSH {
		runner := resolveRunner()
		name, args, err := hostRestartCommand(runner, settings.NginxSettings)
		if err != nil {
			return "", err
		}
		return runner.Exec(ctx, name, args...)
	}

	pidPath := GetPIDPath()
	daemon := GetSbinPath()

	// Check if nginx is running before attempting to stop it
	if IsRunning() {
		stdOut, stdErr = execCommandContext(ctx, "start-stop-daemon", "--stop", "--quiet", "--oknodo", "--retry=TERM/30/KILL/5", "--pidfile", pidPath)
		if stdErr != nil {
			return stdOut, stdErr
		}
	}

	if daemon == "" {
		return execCommandContext(ctx, "nginx")
	}

	return execCommandContext(ctx, "start-stop-daemon", "--start", "--quiet", "--pidfile", pidPath, "--exec", daemon)
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
		return isRunningViaHostService()
	case settings.ControlModeExternalContainer:
		return docker.StatPath(pidPath)
	default:
		return isProcessRunning(pidPath)
	}
}

func hostReloadCommand(n *settings.Nginx) (string, []string) {
	if n.GetHostServiceManager() == settings.HostServiceManagerLaunchd {
		sbin := n.SbinPath
		if sbin == "" {
			sbin = "/opt/homebrew/bin/nginx"
		}
		return sbin, []string{"-s", "reload"}
	}
	systemctl := n.HostSystemctlPath
	if systemctl == "" {
		systemctl = "/bin/systemctl"
	}
	unit := n.HostSystemdUnitName
	if unit == "" {
		unit = "nginx.service"
	}
	return systemctl, []string{"reload", unit}
}

func hostRestartCommand(runner Runner, n *settings.Nginx) (string, []string, error) {
	if n.GetHostServiceManager() == settings.HostServiceManagerLaunchd {
		target, err := launchdTarget(runner, n.GetHostLaunchdService())
		if err != nil {
			return "", nil, err
		}
		return n.GetHostLaunchctlPath(), []string{"kickstart", "-k", target}, nil
	}
	systemctl := n.HostSystemctlPath
	if systemctl == "" {
		systemctl = "/bin/systemctl"
	}
	unit := n.HostSystemdUnitName
	if unit == "" {
		unit = "nginx.service"
	}
	return systemctl, []string{"restart", unit}, nil
}

func launchdTarget(runner Runner, service string) (string, error) {
	out, err := runner.Exec(context.Background(), "/usr/bin/id", "-u")
	if err != nil {
		return "", fmt.Errorf("resolve launchd user domain: %w", err)
	}
	uid := strings.TrimSpace(out)
	if parsed, parseErr := strconv.ParseUint(uid, 10, 32); parseErr != nil || parsed == 0 {
		return "", fmt.Errorf("invalid launchd user id %q", uid)
	}
	service = strings.TrimSpace(service)
	if service == "" {
		return "", errors.New("launchd service label is empty")
	}
	return "gui/" + uid + "/" + service, nil
}

// isRunningViaHostService queries the configured host service manager over SSH.
// On manager errors it validates the remote PID instead of inspecting the container PID namespace.
func isRunningViaHostService() bool {
	runner := resolveRunner()
	n := settings.NginxSettings
	if n.GetHostServiceManager() == settings.HostServiceManagerLaunchd {
		target, err := launchdTarget(runner, n.GetHostLaunchdService())
		if err == nil {
			if _, err = runner.Exec(context.Background(), n.GetHostLaunchctlPath(), "print", target); err == nil {
				return true
			}
		}
	} else {
		name, args := hostSystemdStatusCommand(n)
		out, err := runner.Exec(context.Background(), name, args...)
		if err == nil && strings.TrimSpace(out) == "active" {
			return true
		}
	}
	return isRemotePIDRunning(runner, GetPIDPath())
}

func hostSystemdStatusCommand(n *settings.Nginx) (string, []string) {
	systemctl := n.HostSystemctlPath
	if systemctl == "" {
		systemctl = "/bin/systemctl"
	}
	unit := n.HostSystemdUnitName
	if unit == "" {
		unit = "nginx.service"
	}
	return systemctl, []string{"is-active", unit}
}

func isRemotePIDRunning(runner Runner, pidPath string) bool {
	if pidPath == "" {
		return false
	}
	out, err := runner.Exec(context.Background(), "/bin/cat", pidPath)
	if err != nil {
		return false
	}
	pid := strings.TrimSpace(out)
	parsed, err := strconv.ParseInt(pid, 10, 32)
	if err != nil || parsed <= 0 {
		return false
	}
	_, err = runner.Exec(context.Background(), "/bin/kill", "-0", pid)
	return err == nil
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
