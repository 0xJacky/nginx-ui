package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/version"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/pkg/errors"
	"github.com/uozi-tech/cosy"
	"github.com/uozi-tech/cosy/logger"
)

const (
	ImageName  = "uozi/nginx-ui"
	TempPrefix = "nginx-ui-temp-"
	OldSuffix  = "_old"
)

// getTimestampedTempName returns a temporary container name with timestamp
func getTimestampedTempName() string {
	return fmt.Sprintf("%s%d", TempPrefix, time.Now().Unix())
}

// removeAllTempContainers removes all containers with the TempPrefix
func removeAllTempContainers(ctx context.Context, cli *client.Client) (err error) {
	containers, err := cli.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		return
	}

	for _, c := range containers {
		for _, name := range c.Names {
			processedName := strings.TrimPrefix(name, "/")
			if strings.HasPrefix(processedName, TempPrefix) {
				err = cli.ContainerRemove(ctx, c.ID, container.RemoveOptions{Force: true})
				if err != nil {
					logger.Error("Failed to remove temp container:", err)
				} else {
					logger.Info("Successfully removed temp container:", processedName)
				}
				break
			}
		}
	}

	return nil
}

// UpgradeStepOne Trigger in the OTA upgrade
func UpgradeStepOne(channel string, progressChan chan<- float64) (err error) {
	ctx := context.Background()

	// 1. Get the tag of the latest release
	release, err := version.GetRelease(channel)
	if err != nil {
		return err
	}
	tag := release.TagName

	// 2. Pull the image
	cli, err := initClient()
	if err != nil {
		return cosy.WrapErrorWithParams(ErrClientNotInitialized, err.Error())
	}
	defer cli.Close()

	// Pull the image with the specified tag
	out, err := cli.ImagePull(ctx, fmt.Sprintf("%s:%s", ImageName, tag), image.PullOptions{})
	if err != nil {
		return cosy.WrapErrorWithParams(ErrFailedToPullImage, err.Error())
	}
	defer out.Close()

	// Parse JSON stream and send progress updates through channel
	decoder := json.NewDecoder(out)
	type ProgressDetail struct {
		Current int64 `json:"current"`
		Total   int64 `json:"total"`
	}
	type PullStatus struct {
		Status         string         `json:"status"`
		ProgressDetail ProgressDetail `json:"progressDetail"`
		ID             string         `json:"id"`
	}

	layers := make(map[string]float64)
	var status PullStatus
	var lastProgress float64

	for {
		if err := decoder.Decode(&status); err != nil {
			if err == io.EOF {
				break
			}
			logger.Error("Error decoding Docker pull status:", err)
			continue
		}

		// Only process layers with progress information
		if status.ProgressDetail.Total > 0 {
			progress := float64(status.ProgressDetail.Current) / float64(status.ProgressDetail.Total) * 100
			layers[status.ID] = progress

			// Calculate overall progress (average of all layers)
			var totalProgress float64
			for _, p := range layers {
				totalProgress += p
			}
			overallProgress := totalProgress / float64(len(layers))

			// Only send progress updates when there's a meaningful change
			if overallProgress > lastProgress+1 || overallProgress >= 100 {
				if progressChan != nil {
					progressChan <- overallProgress
				}
				lastProgress = overallProgress
			}
		}
	}

	// Ensure we send 100% at the end
	if progressChan != nil && lastProgress < 100 {
		progressChan <- 100
	}

	// 3. Create a temp container
	// Clean up any existing temp containers
	err = removeAllTempContainers(ctx, cli)
	if err != nil {
		logger.Error("Failed to clean up existing temp containers:", err)
		// Continue execution despite cleanup errors
	}

	// Generate timestamped temp container name
	tempContainerName := getTimestampedTempName()

	// Get current container name
	containerID, err := GetContainerID()
	if err != nil {
		return cosy.WrapErrorWithParams(ErrFailedToGetContainerID, err.Error())
	}
	containerInfo, err := cli.ContainerInspect(ctx, containerID)
	if err != nil {
		return cosy.WrapErrorWithParams(ErrFailedToInspectCurrentContainer, err.Error())
	}
	currentContainerName := strings.TrimPrefix(containerInfo.Name, "/")

	// Set up the command for the temp container to execute step 2
	upgradeCmd := []string{"./nginx-ui", "upgrade-docker-step2"}

	// Add old container name as environment variable
	containerEnv := containerInfo.Config.Env
	containerEnv = append(containerEnv, fmt.Sprintf("NGINX_UI_CONTAINER_NAME=%s", currentContainerName))

	// Create temp container using new image
	_, err = cli.ContainerCreate(
		ctx,
		&container.Config{
			Image: fmt.Sprintf("%s:%s", ImageName, tag),
			Cmd:   upgradeCmd, // Use upgrade command instead of original command
			Env:   containerEnv,
		},
		&container.HostConfig{
			Binds: containerInfo.HostConfig.Binds,
		},
		nil,
		nil,
		tempContainerName,
	)
	if err != nil {
		return cosy.WrapErrorWithParams(ErrFailedToCreateTempContainer, err.Error())
	}

	// Start the temp container to execute step 2
	err = cli.ContainerStart(ctx, tempContainerName, container.StartOptions{})
	if err != nil {
		return cosy.WrapErrorWithParams(ErrFailedToStartTempContainer, err.Error())
	}

	// Output status information
	logger.Info("Docker OTA upgrade step 1 completed. Temp container started to execute step 2.")

	return nil
}

// UpgradeStepTwo Trigger in the temp container
func UpgradeStepTwo(ctx context.Context) (err error) {
	// 1. Copy the old config
	cli, err := initClient()
	if err != nil {
		return
	}
	defer cli.Close()

	// Get old container name from environment variable, fallback to settings if not available
	currentContainerName := os.Getenv("NGINX_UI_CONTAINER_NAME")
	if currentContainerName == "" {
		return errors.New("could not find old container name")
	}
	// Get the current running temp container name
	// Since we can't directly get our own container name from inside, we'll search all temp containers
	containers, err := cli.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		return errors.Wrap(err, "failed to list containers")
	}

	// Find containers with the temp prefix
	var tempContainerName string
	for _, c := range containers {
		for _, name := range c.Names {
			processedName := strings.TrimPrefix(name, "/")
			if strings.HasPrefix(processedName, TempPrefix) {
				tempContainerName = processedName
				break
			}
		}
		if tempContainerName != "" {
			break
		}
	}

	if tempContainerName == "" {
		return errors.New("could not find temp container")
	}

	// Get temp container info to get the new image
	tempContainerInfo, err := cli.ContainerInspect(ctx, tempContainerName)
	if err != nil {
		return errors.Wrap(err, "failed to inspect temp container")
	}
	newImage := tempContainerInfo.Config.Image

	// Get current container info
	oldContainerInfo, err := cli.ContainerInspect(ctx, currentContainerName)
	if err != nil {
		return errors.Wrap(err, "failed to inspect current container")
	}

	// 2. Stop the old container and rename to _old
	err = cli.ContainerStop(ctx, currentContainerName, container.StopOptions{})
	if err != nil {
		return errors.Wrap(err, "failed to stop current container")
	}

	// Rename the old container with _old suffix
	err = cli.ContainerRename(ctx, currentContainerName, currentContainerName+OldSuffix)
	if err != nil {
		return errors.Wrap(err, "failed to rename old container")
	}

	// 3. Use the old config to create and start a new container with the updated image
	// Create new container with original config but using the new image
	newContainerEnv := oldContainerInfo.Config.Env
	// Pass the old container name to the new container
	newContainerEnv = append(newContainerEnv, fmt.Sprintf("NGINX_UI_CONTAINER_NAME=%s", currentContainerName))

	_, err = cli.ContainerCreate(
		ctx,
		&container.Config{
			Image:        newImage,
			Cmd:          oldContainerInfo.Config.Cmd,
			Env:          newContainerEnv,
			Entrypoint:   oldContainerInfo.Config.Entrypoint,
			Labels:       oldContainerInfo.Config.Labels,
			ExposedPorts: oldContainerInfo.Config.ExposedPorts,
			Volumes:      oldContainerInfo.Config.Volumes,
			WorkingDir:   oldContainerInfo.Config.WorkingDir,
		},
		&container.HostConfig{
			Binds:         oldContainerInfo.HostConfig.Binds,
			PortBindings:  oldContainerInfo.HostConfig.PortBindings,
			RestartPolicy: oldContainerInfo.HostConfig.RestartPolicy,
			NetworkMode:   oldContainerInfo.HostConfig.NetworkMode,
			Mounts:        oldContainerInfo.HostConfig.Mounts,
			Privileged:    oldContainerInfo.HostConfig.Privileged,
		},
		nil,
		nil,
		currentContainerName,
	)
	if err != nil {
		// If creation fails, try to recover
		recoverErr := cli.ContainerRename(ctx, currentContainerName+OldSuffix, currentContainerName)
		if recoverErr == nil {
			// Start old container
			recoverErr = cli.ContainerStart(ctx, currentContainerName, container.StartOptions{})
			if recoverErr == nil {
				return errors.Wrap(err, "failed to create new container, recovered to old container")
			}
		}
		return errors.Wrap(err, "failed to create new container and failed to recover")
	}

	// Start the new container
	err = cli.ContainerStart(ctx, currentContainerName, container.StartOptions{})
	if err != nil {
		// If startup fails, try to recover
		// First remove the failed new container
		removeErr := cli.ContainerRemove(ctx, currentContainerName, container.RemoveOptions{Force: true})
		if removeErr != nil {
			logger.Error("Failed to remove failed new container:", removeErr)
		}

		// Rename the old container back to original
		recoverErr := cli.ContainerRename(ctx, currentContainerName+OldSuffix, currentContainerName)
		if recoverErr == nil {
			// Start old container
			recoverErr = cli.ContainerStart(ctx, currentContainerName, container.StartOptions{})
			if recoverErr == nil {
				return errors.Wrap(err, "failed to start new container, recovered to old container")
			}
		}
		return errors.Wrap(err, "failed to start new container and failed to recover")
	}

	logger.Info("Docker OTA upgrade step 2 completed successfully. New container is running.")
	return nil
}

// UpgradeStepThree Trigger in the new container
func UpgradeStepThree() error {
	ctx := context.Background()
	// Remove the old container
	cli, err := initClient()
	if err != nil {
		return cosy.WrapErrorWithParams(ErrClientNotInitialized, err.Error())
	}
	defer cli.Close()

	// Get old container name from environment variable, fallback to settings if not available
	currentContainerName := os.Getenv("NGINX_UI_CONTAINER_NAME")
	if currentContainerName == "" {
		return nil
	}
	oldContainerName := currentContainerName + OldSuffix

	// Check if old container exists and remove it if it does
	containers, err := cli.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		return errors.Wrap(err, "failed to list containers")
	}

	for _, c := range containers {
		for _, name := range c.Names {
			processedName := strings.TrimPrefix(name, "/")
			// Remove old container
			if processedName == oldContainerName {
				err = cli.ContainerRemove(ctx, c.ID, container.RemoveOptions{Force: true})
				if err != nil {
					logger.Error("Failed to remove old container:", err)
					// Continue execution, don't interrupt because of failure to remove old container
				} else {
					logger.Info("Successfully removed old container:", oldContainerName)
				}
				break
			}
		}
	}

	// Clean up all temp containers
	err = removeAllTempContainers(ctx, cli)
	if err != nil {
		logger.Error("Failed to clean up temp containers:", err)
		// Continue execution despite cleanup errors
	}

	return nil
}
