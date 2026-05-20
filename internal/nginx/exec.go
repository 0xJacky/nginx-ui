package nginx

import (
	"context"
	"os/exec"
	"runtime"
)

func execShell(cmd string) (stdOut string, stdErr error) {
	var execCmd *exec.Cmd

	if runtime.GOOS == "windows" {
		execCmd = exec.Command("cmd", "/c", cmd)
	} else {
		execCmd = exec.Command("/bin/sh", "-c", cmd)
	}

	execCmd.Dir = GetNginxExeDir()
	bytes, err := execCmd.CombinedOutput()
	stdOut = string(bytes)
	if err != nil {
		stdErr = err
	}
	return
}

// execCommand routes nginx invocations through the Runner chosen by the
// current control mode. Callers should keep using execCommand as before —
// the routing is transparent.
func execCommand(name string, args ...string) (stdOut string, stdErr error) {
	runner := resolveRunner()
	return runner.Exec(context.Background(), name, args...)
}
