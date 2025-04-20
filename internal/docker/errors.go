package docker

import "github.com/uozi-tech/cosy"

var (
	e                                    = cosy.NewErrorScope("docker")
	ErrClientNotInitialized              = e.New(500001, "docker client not initialized")
	ErrFailedToExec                      = e.New(500002, "failed to exec command: {0}")
	ErrFailedToAttach                    = e.New(500003, "failed to attach to exec instance: {0}")
	ErrReadOutput                        = e.New(500004, "failed to read output: {0}")
	ErrExitUnexpected                    = e.New(500005, "command exited with unexpected exit code: {0}, error: {1}")
	ErrContainerStatusUnknown            = e.New(500006, "container status unknown")
	ErrInspectContainer                  = e.New(500007, "failed to inspect container: {0}")
	ErrNginxNotRunningInAnotherContainer = e.New(500008, "nginx is not running in another container")
)
