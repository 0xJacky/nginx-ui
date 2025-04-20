package docker

import (
	"context"

	"github.com/docker/docker/client"
	"github.com/uozi-tech/cosy"
)

type ContainerStatus int

const (
	ContainerStatusCreated ContainerStatus = iota
	ContainerStatusRunning
	ContainerStatusPaused
	ContainerStatusRestarting
	ContainerStatusRemoving
	ContainerStatusExited
	ContainerStatusDead
	ContainerStatusUnknown
	ContainerStatusNotFound
)

var (
	containerStatusMap = map[string]ContainerStatus{
		"created":    ContainerStatusCreated,
		"running":    ContainerStatusRunning,
		"paused":     ContainerStatusPaused,
		"restarting": ContainerStatusRestarting,
		"removing":   ContainerStatusRemoving,
		"exited":     ContainerStatusExited,
		"dead":       ContainerStatusDead,
	}
)

// GetContainerStatus checks the status of a given container.
func GetContainerStatus(ctx context.Context, containerID string) (ContainerStatus, error) {
	cli, err := initClient()
	if err != nil {
		return ContainerStatusUnknown, cosy.WrapErrorWithParams(ErrClientNotInitialized, err.Error())
	}
	defer cli.Close()

	containerJSON, err := cli.ContainerInspect(ctx, containerID)
	if err != nil {
		if client.IsErrNotFound(err) {
			return ContainerStatusNotFound, nil // Container doesn't exist
		}
		return ContainerStatusUnknown, cosy.WrapErrorWithParams(ErrInspectContainer, err.Error())
	}

	// Can be one of "created", "running", "paused", "restarting", "removing", "exited", or "dead"
	status, ok := containerStatusMap[containerJSON.State.Status]
	if !ok {
		return ContainerStatusUnknown, ErrContainerStatusUnknown
	}
	return status, nil
}
