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
		{name: "launchd homebrew defaults", settings: settings.Nginx{HostServiceManager: settings.HostServiceManagerLaunchd}, wantName: "/opt/homebrew/bin/nginx", wantArgs: []string{"-s", "reload"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotName, gotArgs := hostReloadCommand(&tt.settings)
			if gotName != tt.wantName || !reflect.DeepEqual(gotArgs, tt.wantArgs) {
				t.Fatalf("hostReloadCommand() = %q %v, want %q %v", gotName, gotArgs, tt.wantName, tt.wantArgs)
			}
		})
	}
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
