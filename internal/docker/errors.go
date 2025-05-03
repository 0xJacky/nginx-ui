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
	ErrFailedToGetContainerID            = e.New(500009, "failed to get container id: {0}")
	ErrFailedToPullImage                 = e.New(500010, "failed to pull image: {0}")
	ErrFailedToInspectCurrentContainer   = e.New(500011, "failed to inspect current container: {0}")
	ErrFailedToCreateTempContainer       = e.New(500012, "failed to create temp container: {0}")
	ErrFailedToStartTempContainer        = e.New(500013, "failed to start temp container: {0}")
)
