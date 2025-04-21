package docker

import (
	"context"

	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/uozi-tech/cosy/logger"
)

// StatPath checks if a path exists in the container
func StatPath(path string) bool {
	if !settings.NginxSettings.RunningInAnotherContainer() {
		return false
	}

	cli, err := initClient()
	if err != nil {
		return false
	}
	defer cli.Close()

	_, err = cli.ContainerStatPath(context.Background(), settings.NginxSettings.ContainerName, path)
	if err != nil {
		logger.Error("Failed to stat path", "error", err)
		return false
	}

	return true
}
