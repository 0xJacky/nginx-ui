package nginx

import (
	"context"
	"os/exec"
	"runtime"

	"github.com/0xJacky/Nginx-UI/internal/docker"
	"github.com/0xJacky/Nginx-UI/settings"
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

func execCommand(name string, cmd ...string) (stdOut string, stdErr error) {
	switch settings.NginxSettings.RunningInAnotherContainer() {
	case true:
		cmd = append([]string{name}, cmd...)
		stdOut, stdErr = docker.Exec(context.Background(), cmd)
	case false:
		execCmd := exec.Command(name, cmd...)
		// fix #1046
		execCmd.Dir = GetNginxExeDir()
		bytes, err := execCmd.CombinedOutput()
		stdOut = string(bytes)
		if err != nil {
			stdErr = err
		}
	}
	return
}
