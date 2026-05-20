package nginx

import (
	"context"
	"os/exec"
	"runtime"
)

func execShell(cmd string) (stdOut string, stdErr error) {
	name, args := shellCommand(runtime.GOOS, settings.NginxSettings.RunningInAnotherContainer(), cmd)
	return execCommand(name, args...)
}

func shellCommand(goos string, externalContainer bool, cmd string) (name string, args []string) {
	// External Nginx containers are Linux containers even when Nginx UI runs
	// on a different host OS. Route the shell itself through execCommand so
	// custom test, reload, and restart commands use the configured target.
	if externalContainer || goos != "windows" {
		return "/bin/sh", []string{"-c", cmd}
	}

	return "cmd", []string{"/c", cmd}
}

// execCommand routes nginx invocations through the Runner chosen by the
// current control mode. Callers should keep using execCommand as before —
// the routing is transparent.
func execCommand(name string, args ...string) (stdOut string, stdErr error) {
	runner := resolveRunner()
	return runner.Exec(context.Background(), name, args...)
}
