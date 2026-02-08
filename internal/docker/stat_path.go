package docker

import (
	"context"

	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/docker/docker/client"
	"github.com/uozi-tech/cosy/logger"
)

// StatPath checks if a path exists in the container
func StatPath(path string) bool {
	if !settings.NginxSettings.RunningInAnotherContainer() {
		return false
	}

	containerName := settings.NginxSettings.ContainerName
	cli, err := initClient()
	if err != nil {
		logger.Error("Failed to initialize Docker client", "error", err)
		return false
	}
	defer cli.Close()

	ctx := context.Background()

	// First, verify the container exists and is accessible
	containerInfo, err := cli.ContainerInspect(ctx, containerName)
	if err != nil {
		if client.IsErrNotFound(err) {
			logger.Error("Container not found. Please verify the container name matches exactly.",
				"containerName", containerName,
				"hint", "Check if the container is running with 'docker ps' and verify the NGINX_UI_NGINX_CONTAINER_NAME setting")
		} else {
			logger.Error("Failed to inspect container",
				"containerName", containerName,
				"error", err)
		}
		return false
	}

	// Log container status for debugging
	if !containerInfo.State.Running {
		logger.Warn("Container is not running",
			"containerName", containerName,
			"containerState", containerInfo.State.Status)
	}

	_, err = cli.ContainerStatPath(ctx, containerName, path)
	if err != nil {
		logger.Error("Failed to stat path in container",
			"containerName", containerName,
			"path", path,
			"containerRunning", containerInfo.State.Running,
			"error", err)
		return false
	}

	return true
}
