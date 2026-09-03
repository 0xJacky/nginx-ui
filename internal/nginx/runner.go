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
	// GOOS names the operating system nginx runs on, in runtime.GOOS terms.
	// A remote target runs its own OS, so path conventions and shell choice
	// must follow this value rather than the OS hosting nginx-ui.
	GOOS() string
}

// targetGOOS returns the operating system of the machine that runs nginx.
func targetGOOS() string {
	return resolveRunner().GOOS()
}

// StatOnTarget reports whether path exists on the machine that runs nginx,
// which is not this container in external container or SSH mode.
func StatOnTarget(path string) bool {
	return resolveRunner().Stat(path)
}

// resolveRunner returns the active runner based on the configured control mode.
// It is a variable so tests can substitute a runner without a live target.
var resolveRunner = func() Runner {
	switch settings.NginxSettings.ControlMode() {
	case settings.ControlModeHostViaSSH:
		return newSSHRunner()
	case settings.ControlModeExternalContainer:
		return newDockerRunner()
	default:
		return &localRunner{}
	}
}
