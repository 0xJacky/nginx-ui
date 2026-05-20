package nginx

import (
	"context"

	"github.com/0xJacky/Nginx-UI/settings"
)

// Runner abstracts how nginx commands are executed against the active
// control target. File I/O is intentionally NOT in this interface — it
// goes through the OS filesystem (bind-mount for local & SSH modes) or
// docker CopyTo/CopyFromContainer (external_container mode) as before.
type Runner interface {
	Exec(ctx context.Context, name string, args ...string) (stdout string, err error)
	Stat(path string) bool
}

// resolveRunner returns the active runner based on the configured control mode.
func resolveRunner() Runner {
	switch settings.NginxSettings.ControlMode() {
	case settings.ControlModeHostViaSSH:
		return newSSHRunner()
	case settings.ControlModeExternalContainer:
		return newDockerRunner()
	default:
		return &localRunner{}
	}
}
