package nginx

import (
	"context"
	"runtime"

	"github.com/0xJacky/Nginx-UI/settings"
)

func execShell(cmd string) (stdOut string, stdErr error) {
	return execShellContext(context.Background(), cmd)
}

func execShellContext(ctx context.Context, cmd string) (stdOut string, stdErr error) {
	remoteTarget := settings.NginxSettings.ControlMode() != settings.ControlModeLocal
	name, args := shellCommand(runtime.GOOS, remoteTarget, cmd)
	return execCommandContext(ctx, name, args...)
}

func shellCommand(goos string, remoteTarget bool, cmd string) (name string, args []string) {
	// A remote target runs its own OS, not this one. An external Nginx
	// container is Linux, and an SSH host is Linux or macOS, so a Windows
	// hosted Nginx UI must not send cmd /c to either. Route the shell itself
	// through execCommand so custom test, reload, and restart commands use the
	// configured target.
	if remoteTarget || goos != "windows" {
		return "/bin/sh", []string{"-c", cmd}
	}

	return "cmd", []string{"/c", cmd}
}

// execCommand routes nginx invocations through the Runner chosen by the
// current control mode. Callers should keep using execCommand as before —
// the routing is transparent.
func execCommand(name string, args ...string) (stdOut string, stdErr error) {
	return execCommandContext(context.Background(), name, args...)
}

// execCommandContext is the context-aware variant of execCommand. The context
// bounds the command on every control target, so a hung nginx test or reload
// cannot block the caller forever.
func execCommandContext(ctx context.Context, name string, args ...string) (stdOut string, stdErr error) {
	runner := resolveRunner()
	return runner.Exec(ctx, name, args...)
}
