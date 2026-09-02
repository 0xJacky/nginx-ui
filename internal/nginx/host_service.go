package nginx

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/0xJacky/Nginx-UI/settings"
)

// hostService describes how the native service manager on the SSH host
// controls nginx. The nginx control paths only ask for the command to run and
// how to read its status, so adding a manager means adding one implementation
// here instead of another branch in reload, restart and IsRunning.
//
// Every method returns the command as name plus args so it can be handed to
// Runner.Exec unchanged; none of them run anything except where a Runner is
// passed in to resolve a value the command depends on.
type hostService interface {
	// reloadCommand returns the command that asks the running nginx to reload.
	reloadCommand() (name string, args []string)
	// restartCommand returns the command that restarts the nginx service. The
	// runner resolves values the command depends on, such as the launchd user
	// domain.
	restartCommand(runner Runner) (name string, args []string, err error)
	// statusCommand returns the command whose result isActiveOutput interprets.
	statusCommand(runner Runner) (name string, args []string, err error)
	// isActiveOutput reports whether a successful statusCommand run means the
	// service manager considers nginx running.
	isActiveOutput(stdout string) bool
}

// resolveHostService picks the service manager strategy for the configured
// SSH host.
func resolveHostService() hostService {
	return newHostService(settings.NginxSettings)
}

// newHostService builds the strategy for the service manager configured in n.
// Every path and name comes from the settings getters so each default has a
// single home.
func newHostService(n *settings.Nginx) hostService {
	if n.GetHostServiceManager() == settings.HostServiceManagerLaunchd {
		return &launchdService{
			launchctl: n.GetHostLaunchctlPath(),
			label:     n.GetHostLaunchdService(),
			sbin:      n.GetHostSbinPath(),
		}
	}
	return &systemdService{
		systemctl: n.GetHostSystemctlPath(),
		unit:      n.GetHostSystemdUnitName(),
	}
}

// systemdService controls nginx through systemctl on a Linux host.
type systemdService struct {
	systemctl string
	unit      string
}

func (s *systemdService) reloadCommand() (string, []string) {
	return s.systemctl, []string{"reload", s.unit}
}

func (s *systemdService) restartCommand(Runner) (string, []string, error) {
	return s.systemctl, []string{"restart", s.unit}, nil
}

func (s *systemdService) statusCommand(Runner) (string, []string, error) {
	return s.systemctl, []string{"is-active", s.unit}, nil
}

func (s *systemdService) isActiveOutput(stdout string) bool {
	return strings.TrimSpace(stdout) == "active"
}

// launchdService controls a Homebrew style nginx through launchctl in the
// SSH user's GUI domain on a macOS host.
type launchdService struct {
	launchctl string
	label     string
	sbin      string
}

// reloadCommand signals the nginx binary directly: launchd has no reload verb
// and kickstart would restart the service.
func (l *launchdService) reloadCommand() (string, []string) {
	return l.sbin, []string{"-s", "reload"}
}

func (l *launchdService) restartCommand(runner Runner) (string, []string, error) {
	target, err := launchdTarget(runner, l.label)
	if err != nil {
		return "", nil, err
	}
	return l.launchctl, []string{"kickstart", "-k", target}, nil
}

func (l *launchdService) statusCommand(runner Runner) (string, []string, error) {
	target, err := launchdTarget(runner, l.label)
	if err != nil {
		return "", nil, err
	}
	return l.launchctl, []string{"print", target}, nil
}

// isActiveOutput treats any successful `launchctl print` as running: the
// command fails when the service is not loaded in the domain.
func (l *launchdService) isActiveOutput(string) bool {
	return true
}

// launchdTarget resolves the launchd domain target for the SSH user, e.g.
// gui/501/homebrew.mxcl.nginx.
func launchdTarget(runner Runner, service string) (string, error) {
	out, err := runner.Exec(context.Background(), "/usr/bin/id", "-u")
	if err != nil {
		return "", fmt.Errorf("resolve launchd user domain: %w", err)
	}
	uid := strings.TrimSpace(out)
	if parsed, parseErr := strconv.ParseUint(uid, 10, 32); parseErr != nil || parsed == 0 {
		return "", fmt.Errorf("invalid launchd user id %q", uid)
	}
	service = strings.TrimSpace(service)
	if service == "" {
		return "", errors.New("launchd service label is empty")
	}
	return "gui/" + uid + "/" + service, nil
}
