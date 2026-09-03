package nginx

import (
	"context"
)

func execShell(cmd string) (stdOut string, stdErr error) {
	return execShellContext(context.Background(), cmd)
}

func execShellContext(ctx context.Context, cmd string) (stdOut string, stdErr error) {
	// The shell must match the OS of the target that runs the command, which
	// the runner reports: an external container is Linux and an SSH host is
	// Linux or macOS, so a Windows hosted Nginx UI must not send cmd /c to
	// either. Custom test, reload, and restart commands therefore run through
	// the same runner as every other nginx invocation.
	runner := resolveRunner()
	name, args := shellCommand(runner.GOOS(), cmd)
	return runner.Exec(ctx, name, args...)
}

// shellCommand wraps cmd in the shell of the operating system named by goos.
func shellCommand(goos string, cmd string) (name string, args []string) {
	if goos != "windows" {
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
