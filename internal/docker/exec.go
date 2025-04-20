package docker

import (
	"bytes"
	"context"
	"io"
	"strconv"

	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/uozi-tech/cosy"
	"github.com/uozi-tech/cosy/logger"
)

// Exec executes a command in a specific container and returns the output.
func Exec(ctx context.Context, command []string) (string, error) {
	if !settings.NginxSettings.RunningInAnotherContainer() {
		return "", ErrNginxNotRunningInAnotherContainer
	}

	cli, err := initClient()
	if err != nil {
		return "", cosy.WrapErrorWithParams(ErrClientNotInitialized, err.Error())
	}
	defer cli.Close()

	execConfig := container.ExecOptions{
		AttachStdout: true,
		AttachStderr: true, // Also attach stderr to capture errors from the command
		Cmd:          command,
	}

	// Create the exec instance
	execCreateResp, err := cli.ContainerExecCreate(ctx, settings.NginxSettings.ContainerName, execConfig)
	if err != nil {
		return "", cosy.WrapErrorWithParams(ErrFailedToExec, err.Error())
	}
	execID := execCreateResp.ID

	// Attach to the exec instance
	hijackedResp, err := cli.ContainerExecAttach(ctx, execID, container.ExecAttachOptions{})
	if err != nil {
		return "", cosy.WrapErrorWithParams(ErrFailedToAttach, err.Error())
	}
	defer hijackedResp.Close()

	// Read the output
	var outBuf, errBuf bytes.Buffer
	outputDone := make(chan error)

	go func() {
		// stdcopy.StdCopy demultiplexes the stream into two buffers
		_, err = stdcopy.StdCopy(&outBuf, &errBuf, hijackedResp.Reader)
		outputDone <- err
	}()

	select {
	case err := <-outputDone:
		if err != nil && err != io.EOF { // io.EOF is expected when the stream finishes
			return "", cosy.WrapErrorWithParams(ErrReadOutput, err.Error())
		}
	case <-ctx.Done():
		return "", cosy.WrapErrorWithParams(ErrReadOutput, ctx.Err().Error())
	}

	// Optionally inspect the exec process to check the exit code
	execInspectResp, err := cli.ContainerExecInspect(ctx, execID)
	logger.Debug("docker exec result", outBuf.String(), errBuf.String())

	if err != nil {
		return "", cosy.WrapErrorWithParams(ErrExitUnexpected, err.Error())
	} else if execInspectResp.ExitCode != 0 {
		// Command exited with a non-zero status code. Return stderr as part of the error.
		return outBuf.String(), cosy.WrapErrorWithParams(ErrExitUnexpected, strconv.Itoa(execInspectResp.ExitCode), errBuf.String())
	}

	// Return stdout if successful
	return outBuf.String(), nil
}
