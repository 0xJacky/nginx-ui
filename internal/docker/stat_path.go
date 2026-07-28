package docker

import (
	"context"

	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/uozi-tech/cosy/logger"
)

type pathStatClient interface {
	ContainerInspect(ctx context.Context, container string) (container.InspectResponse, error)
	ContainerStatPath(ctx context.Context, container, path string) (container.PathStat, error)
}

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

	exists, err := statPath(context.Background(), cli, containerName, path)
	if err != nil {
		logger.Error("Failed to stat path", "container", containerName, "path", path, "error", err)
		return false
	}
	if !exists {
		logger.Debug("Path not found in container", "container", containerName, "path", path)
	}

	return exists
}

func statPath(ctx context.Context, cli pathStatClient, containerName, path string) (bool, error) {
	_, err := cli.ContainerStatPath(ctx, containerName, path)
	if err == nil {
		return true, nil
	}
	if !client.IsErrNotFound(err) {
		return false, err
	}

	// Docker returns the same 404 response when either the container or the
	// requested path does not exist. Inspect the container to distinguish an
	// expected missing-path probe from a real container access failure.
	_, inspectErr := cli.ContainerInspect(ctx, containerName)
	if inspectErr != nil {
		return false, inspectErr
	}

	return false, nil
}
