package nginx

import (
	"context"
	"os/exec"
	"runtime"

	"github.com/0xJacky/Nginx-UI/internal/docker"
	"github.com/0xJacky/Nginx-UI/settings"
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
