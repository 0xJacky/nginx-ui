package nginx

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/0xJacky/Nginx-UI/settings"
)

type hostControlTestRunner struct {
	responses map[string]struct {
		out string
		err error
	}
}

func (r *hostControlTestRunner) Exec(_ context.Context, name string, args ...string) (string, error) {
	key := name
	for _, arg := range args {
		key += " " + arg
	}
	response, ok := r.responses[key]
	if !ok {
		return "", errors.New("unexpected command: " + key)
	}
	return response.out, response.err
}

func (*hostControlTestRunner) Stat(string) bool { return false }

func TestHostReloadCommand(t *testing.T) {
	tests := []struct {
		name     string
		settings settings.Nginx
		wantName string
		wantArgs []string
	}{
		{name: "systemd defaults", settings: settings.Nginx{}, wantName: "/bin/systemctl", wantArgs: []string{"reload", "nginx.service"}},
		{name: "launchd homebrew defaults", settings: settings.Nginx{HostServiceManager: settings.HostServiceManagerLaunchd}, wantName: settings.DefaultHostSbinPathLaunchd, wantArgs: []string{"-s", "reload"}},
		{name: "launchd configured sbin", settings: settings.Nginx{HostServiceManager: settings.HostServiceManagerLaunchd, SbinPath: "/usr/local/bin/nginx"}, wantName: "/usr/local/bin/nginx", wantArgs: []string{"-s", "reload"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotName, gotArgs := newHostService(&tt.settings).reloadCommand()
			if gotName != tt.wantName || !reflect.DeepEqual(gotArgs, tt.wantArgs) {
				t.Fatalf("reloadCommand() = %q %v, want %q %v", gotName, gotArgs, tt.wantName, tt.wantArgs)
			}
		})
	}
}

// The service manager strategy owns every systemctl/launchctl invocation, so
// the commands must stay byte-identical to what the host sudoers whitelist and
// the wizard expect.
func TestHostServiceRestartAndStatusCommands(t *testing.T) {
	idRunner := &hostControlTestRunner{responses: map[string]struct {
		out string
		err error
	}{"/usr/bin/id -u": {out: "501\n"}}}

	tests := []struct {
		name        string
		settings    settings.Nginx
		wantRestart string
		wantStatus  string
	}{
		{
			name:        "systemd defaults",
			settings:    settings.Nginx{},
			wantRestart: "/bin/systemctl restart nginx.service",
			wantStatus:  "/bin/systemctl is-active nginx.service",
		},
		{
			name:        "systemd configured paths",
			settings:    settings.Nginx{HostSystemctlPath: "/usr/bin/systemctl", HostSystemdUnitName: "openresty.service"},
			wantRestart: "/usr/bin/systemctl restart openresty.service",
			wantStatus:  "/usr/bin/systemctl is-active openresty.service",
		},
		{
			name:        "launchd defaults",
			settings:    settings.Nginx{HostServiceManager: settings.HostServiceManagerLaunchd},
			wantRestart: "/bin/launchctl kickstart -k gui/501/homebrew.mxcl.nginx",
			wantStatus:  "/bin/launchctl print gui/501/homebrew.mxcl.nginx",
		},
		{
			name:        "launchd configured label",
			settings:    settings.Nginx{HostServiceManager: settings.HostServiceManagerLaunchd, HostLaunchctlPath: "/usr/bin/launchctl", HostLaunchdService: "org.example.nginx"},
			wantRestart: "/usr/bin/launchctl kickstart -k gui/501/org.example.nginx",
			wantStatus:  "/usr/bin/launchctl print gui/501/org.example.nginx",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := newHostService(&tt.settings)

			name, args, err := service.restartCommand(idRunner)
			if err != nil {
				t.Fatalf("restartCommand() error = %v", err)
			}
			if got := joinCommand(name, args); got != tt.wantRestart {
				t.Fatalf("restartCommand() = %q, want %q", got, tt.wantRestart)
			}

			name, args, err = service.statusCommand(idRunner)
			if err != nil {
				t.Fatalf("statusCommand() error = %v", err)
			}
			if got := joinCommand(name, args); got != tt.wantStatus {
				t.Fatalf("statusCommand() = %q, want %q", got, tt.wantStatus)
			}
		})
	}
}

// launchd commands depend on the remote uid, so a failed lookup must surface
// as an error instead of a command against the wrong domain.
func TestLaunchdServiceCommandsPropagateUIDFailure(t *testing.T) {
	runner := &hostControlTestRunner{responses: map[string]struct {
		out string
		err error
	}{}}
	service := newHostService(&settings.Nginx{HostServiceManager: settings.HostServiceManagerLaunchd})

	if _, _, err := service.restartCommand(runner); err == nil {
		t.Fatal("restartCommand() error = nil, want the uid lookup failure")
	}
	if _, _, err := service.statusCommand(runner); err == nil {
		t.Fatal("statusCommand() error = nil, want the uid lookup failure")
	}
}

func TestHostServiceIsActiveOutput(t *testing.T) {
	tests := []struct {
		name     string
		settings settings.Nginx
		stdout   string
		want     bool
	}{
		{name: "systemd active", settings: settings.Nginx{}, stdout: "active\n", want: true},
		{name: "systemd inactive", settings: settings.Nginx{}, stdout: "inactive\n", want: false},
		{name: "systemd activating", settings: settings.Nginx{}, stdout: "activating\n", want: false},
		{name: "launchd print succeeded", settings: settings.Nginx{HostServiceManager: settings.HostServiceManagerLaunchd}, stdout: "gui/501/homebrew.mxcl.nginx = {\n\tstate = running\n}", want: true},
		{name: "launchd empty output", settings: settings.Nginx{HostServiceManager: settings.HostServiceManagerLaunchd}, stdout: "", want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := newHostService(&tt.settings).isActiveOutput(tt.stdout); got != tt.want {
				t.Fatalf("isActiveOutput(%q) = %v, want %v", tt.stdout, got, tt.want)
			}
		})
	}
}

func TestResolveHostServicePicksConfiguredManager(t *testing.T) {
	originalSettings := *settings.NginxSettings
	t.Cleanup(func() { *settings.NginxSettings = originalSettings })

	settings.NginxSettings.HostServiceManager = settings.HostServiceManagerSystemd
	if _, ok := resolveHostService().(*systemdService); !ok {
		t.Fatalf("resolveHostService() = %T, want *systemdService", resolveHostService())
	}

	settings.NginxSettings.HostServiceManager = settings.HostServiceManagerLaunchd
	if _, ok := resolveHostService().(*launchdService); !ok {
		t.Fatalf("resolveHostService() = %T, want *launchdService", resolveHostService())
	}
}

// IsRunning in SSH mode must fall back to the remote PID when the service
// manager does not report nginx as active, regardless of the manager.
func TestIsRunningViaHostServiceFallsBackToRemotePID(t *testing.T) {
	originalSettings := *settings.NginxSettings
	originalResolve := resolveRunner
	t.Cleanup(func() {
		*settings.NginxSettings = originalSettings
		resolveRunner = originalResolve
	})

	tests := []struct {
		name      string
		settings  settings.Nginx
		responses map[string]struct {
			out string
			err error
		}
		want bool
	}{
		{
			name:     "systemd active",
			settings: settings.Nginx{HostMode: settings.HostModeSSH},
			responses: map[string]struct {
				out string
				err error
			}{"/bin/systemctl is-active nginx.service": {out: "active\n"}},
			want: true,
		},
		{
			name:     "systemd inactive but pid alive",
			settings: settings.Nginx{HostMode: settings.HostModeSSH, PIDPath: "/run/nginx.pid"},
			responses: map[string]struct {
				out string
				err error
			}{
				"/bin/systemctl is-active nginx.service": {out: "inactive\n", err: errors.New("exit status 3")},
				"/bin/cat /run/nginx.pid":                {out: "4242\n"},
				"/bin/kill -0 4242":                      {},
			},
			want: true,
		},
		{
			name:     "systemd inactive and no pid",
			settings: settings.Nginx{HostMode: settings.HostModeSSH, PIDPath: "/run/nginx.pid"},
			responses: map[string]struct {
				out string
				err error
			}{"/bin/systemctl is-active nginx.service": {out: "inactive\n", err: errors.New("exit status 3")}},
			want: false,
		},
		{
			name:     "launchd loaded",
			settings: settings.Nginx{HostMode: settings.HostModeSSH, HostServiceManager: settings.HostServiceManagerLaunchd},
			responses: map[string]struct {
				out string
				err error
			}{
				"/usr/bin/id -u": {out: "501\n"},
				"/bin/launchctl print gui/501/homebrew.mxcl.nginx": {out: "state = running"},
			},
			want: true,
		},
		{
			name:     "launchd uid lookup fails but pid alive",
			settings: settings.Nginx{HostMode: settings.HostModeSSH, HostServiceManager: settings.HostServiceManagerLaunchd, PIDPath: "/opt/homebrew/var/run/nginx.pid"},
			responses: map[string]struct {
				out string
				err error
			}{
				"/bin/cat /opt/homebrew/var/run/nginx.pid": {out: "42109\n"},
				"/bin/kill -0 42109":                       {},
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			*settings.NginxSettings = tt.settings
			resolveRunner = func() Runner { return &hostControlTestRunner{responses: tt.responses} }
			if got := isRunningViaHostService(); got != tt.want {
				t.Fatalf("isRunningViaHostService() = %v, want %v", got, tt.want)
			}
		})
	}
}

func joinCommand(name string, args []string) string {
	for _, arg := range args {
		name += " " + arg
	}
	return name
}

func TestLaunchdTargetUsesRemoteUID(t *testing.T) {
	runner := &hostControlTestRunner{responses: map[string]struct {
		out string
		err error
	}{"/usr/bin/id -u": {out: "501\n"}}}
	target, err := launchdTarget(runner, "homebrew.mxcl.nginx")
	if err != nil {
		t.Fatal(err)
	}
	if target != "gui/501/homebrew.mxcl.nginx" {
		t.Fatalf("launchdTarget() = %q", target)
	}
}

func TestIsRemotePIDRunning(t *testing.T) {
	runner := &hostControlTestRunner{responses: map[string]struct {
		out string
		err error
	}{
		"/bin/cat /opt/homebrew/var/run/nginx.pid": {out: "42109\n"},
		"/bin/kill -0 42109":                       {},
	}}
	if !isRemotePIDRunning(runner, "/opt/homebrew/var/run/nginx.pid") {
		t.Fatal("expected remote PID to be running")
	}
}

// restart() used to stash the SSH result in package globals and return the
// zero values, so a failed host restart was reported as success.
func TestRestartPropagatesHostServiceFailure(t *testing.T) {
	originalSettings := *settings.NginxSettings
	originalResolve := resolveRunner
	t.Cleanup(func() {
		*settings.NginxSettings = originalSettings
		resolveRunner = originalResolve
	})

	settings.NginxSettings.HostMode = settings.HostModeSSH
	settings.NginxSettings.HostSystemctlPath = "/bin/systemctl"
	settings.NginxSettings.HostSystemdUnitName = "nginx.service"

	wantErr := errors.New("unit nginx.service is masked")
	resolveRunner = func() Runner {
		return &hostControlTestRunner{responses: map[string]struct {
			out string
			err error
		}{
			"/bin/systemctl restart nginx.service": {out: "Failed to restart", err: wantErr},
		}}
	}

	stdOut, stdErr := restart()
	if !errors.Is(stdErr, wantErr) {
		t.Fatalf("restart() error = %v, want %v", stdErr, wantErr)
	}
	if stdOut != "Failed to restart" {
		t.Fatalf("restart() output = %q, want the runner output", stdOut)
	}
}

// reload() and restart() used to take the host_via_ssh branch before checking
// the operator-authored ReloadCmd/RestartCmd, so a custom command was silently
// replaced by systemctl even though TestConfigCmd was honoured.
func TestCustomCommandsWinOverHostServiceInSSHMode(t *testing.T) {
	originalSettings := *settings.NginxSettings
	originalResolve := resolveRunner
	t.Cleanup(func() {
		*settings.NginxSettings = originalSettings
		resolveRunner = originalResolve
	})

	const reloadCmd = "/usr/local/openresty/bin/openresty -s reload"
	const restartCmd = "/usr/local/openresty/bin/openresty -s stop && /usr/local/openresty/bin/openresty"

	settings.NginxSettings.HostMode = settings.HostModeSSH
	settings.NginxSettings.HostSystemctlPath = "/bin/systemctl"
	settings.NginxSettings.HostSystemdUnitName = "nginx.service"
	settings.NginxSettings.ReloadCmd = reloadCmd
	settings.NginxSettings.RestartCmd = restartCmd

	// Every other command, including systemctl reload/restart, is rejected by
	// the runner, so falling back to the host service fails the test.
	resolveRunner = func() Runner {
		return &hostControlTestRunner{responses: map[string]struct {
			out string
			err error
		}{
			"/bin/systemctl is-active nginx.service": {out: "active\n"},
			"/bin/sh -c " + reloadCmd:                {out: "custom reload"},
			"/bin/sh -c " + restartCmd:               {out: "custom restart"},
		}}
	}

	stdOut, stdErr := reload()
	if stdErr != nil {
		t.Fatalf("reload() error = %v, want the custom command to run", stdErr)
	}
	if stdOut != "custom reload" {
		t.Fatalf("reload() output = %q, want the custom command output", stdOut)
	}

	stdOut, stdErr = restart()
	if stdErr != nil {
		t.Fatalf("restart() error = %v, want the custom command to run", stdErr)
	}
	if stdOut != "custom restart" {
		t.Fatalf("restart() output = %q, want the custom command output", stdOut)
	}
}
