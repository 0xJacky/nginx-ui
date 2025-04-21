package nginx

import (
	"context"
	"os/exec"

	"github.com/0xJacky/Nginx-UI/internal/docker"
	"github.com/0xJacky/Nginx-UI/settings"
)

func execShell(cmd string) (stdOut string, stdErr error) {
	return execCommand("/bin/sh", "-c", cmd)
}

func execCommand(name string, cmd ...string) (stdOut string, stdErr error) {
	switch settings.NginxSettings.RunningInAnotherContainer() {
	case true:
		cmd = append([]string{name}, cmd...)
		stdOut, stdErr = docker.Exec(context.Background(), cmd)
	case false:
		bytes, err := exec.Command(name, cmd...).CombinedOutput()
		stdOut = string(bytes)
		if err != nil {
			stdErr = err
		}
	}
	return
}
