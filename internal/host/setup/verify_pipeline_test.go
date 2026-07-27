//go:build linux

package setup

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// fakeRunner answers a fixed script of commands. An unscripted command is an
// error, so a check that starts issuing a different command is caught rather
// than silently passing.
type fakeRunner struct {
	responses map[string]struct {
		out string
		err error
	}
	calls []string
}

func newFakeRunner() *fakeRunner {
	return &fakeRunner{responses: map[string]struct {
		out string
		err error
	}{}}
}

func (r *fakeRunner) on(command, out string, err error) *fakeRunner {
	r.responses[command] = struct {
		out string
		err error
	}{out, err}
	return r
}

func (r *fakeRunner) Exec(_ context.Context, name string, args ...string) (string, error) {
	key := strings.TrimSpace(name + " " + strings.Join(args, " "))
	r.calls = append(r.calls, key)
	response, ok := r.responses[key]
	if !ok {
		return "", errors.New("unscripted command: " + key)
	}
	return response.out, response.err
}

func systemdParams() SetupParams {
	return SetupParams{
		HostAddress:    "192.168.1.10:22",
		HostUser:       "nginxui",
		ServiceManager: "systemd",
		SystemdUnit:    "nginx.service",
		SystemctlPath:  "/bin/systemctl",
		NginxSbinPath:  "/usr/sbin/nginx",
		HostConfigDir:  "/etc/nginx",
		HostLogDir:     "/var/log/nginx",
	}.FillDefaults()
}

// A failing nginx -t must reach the caller as a failing step rather than being
// folded into a generic pass.
func TestVerifyReportsNginxTestFailure(t *testing.T) {
	runner := newFakeRunner().
		on("/bin/echo ok", "ok", nil).
		on("/usr/sbin/nginx -t", "nginx: [emerg] unknown directive \"servr\"", errors.New("exit status 1"))

	result := Verify(context.Background(), VerifyOptions{
		Client: runner,
		Params: systemdParams(),
		Groups: []CheckGroup{CheckGroupNginx},
	})

	step, ok := result.Steps["nginx_test"]
	if !ok {
		t.Fatal("nginx_test step is missing")
	}
	if step.OK {
		t.Fatalf("a failing nginx -t reported OK: %+v", step)
	}
	if !strings.Contains(step.Detail, "unknown directive") {
		t.Fatalf("the nginx error was not surfaced: %q", step.Detail)
	}
}

// A skipped check must not read as a pass.
func TestVerifySkippedNginxTestIsNotAPass(t *testing.T) {
	result := Verify(context.Background(), VerifyOptions{
		Client:     newFakeRunner().on("/bin/echo ok", "ok", nil),
		Params:     systemdParams(),
		Groups:     []CheckGroup{CheckGroupNginx},
		SkipNginxT: true,
	})

	step := result.Steps["nginx_test"]
	if step.OK {
		t.Fatal("a skipped nginx -t reported OK")
	}
	if step.Level != "warning" {
		t.Fatalf("level = %q, want warning so the skip does not block saving", step.Level)
	}
	if !strings.Contains(step.Detail, "not validated") {
		t.Fatalf("the skip was not explained: %q", step.Detail)
	}
}

// An undeclared ExecReload prints as an empty property value, which the old
// check could never observe.
func TestVerifyDetectsUndeclaredExecReload(t *testing.T) {
	cases := []struct {
		name   string
		output string
		wantOK bool
	}{
		{"declared", "ExecReload={ path=/bin/kill ; argv[]=/bin/kill -s HUP $MAINPID }", true},
		{"undeclared", "ExecReload=", false},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			runner := newFakeRunner().
				on("/bin/echo ok", "ok", nil).
				on("/bin/systemctl is-active nginx.service", "active", nil).
				on("/bin/systemctl show nginx.service --property=ExecReload", tt.output, nil).
				on("/usr/bin/uname -s", "Linux", nil).
				on("/bin/cat /etc/machine-id", "abc", nil).
				on("/usr/bin/stat -c %i /etc/nginx", "1", nil)

			result := Verify(context.Background(), VerifyOptions{
				Client: runner,
				Params: systemdParams(),
				Groups: []CheckGroup{CheckGroupPlatform},
			})

			step := result.Steps["unit_has_execreload"]
			if step.OK != tt.wantOK {
				t.Fatalf("ExecReload %q reported OK=%v, want %v (detail %q)",
					tt.output, step.OK, tt.wantOK, step.Detail)
			}
		})
	}
}

// A group request must run only what it names.
func TestVerifyRunsOnlyTheRequestedGroup(t *testing.T) {
	runner := newFakeRunner().
		on("/bin/echo ok", "ok", nil).
		on("/usr/sbin/nginx -t", "syntax is ok", nil)

	result := Verify(context.Background(), VerifyOptions{
		Client: runner,
		Params: systemdParams(),
		Groups: []CheckGroup{CheckGroupNginx},
	})

	if _, ok := result.Steps["nginx_test"]; !ok {
		t.Fatal("the requested nginx group did not run")
	}
	// ssh_connect always runs: every other check needs a working session.
	if _, ok := result.Steps["ssh_connect"]; !ok {
		t.Fatal("ssh_connect must always run")
	}
	for _, unexpected := range []string{"systemctl_is_active", "unit_has_execreload", "same_host"} {
		if _, ok := result.Steps[unexpected]; ok {
			t.Fatalf("step %q ran for a nginx-only request", unexpected)
		}
	}
}

// A launchd target needs no sudoers entry, so the privilege checks must not be
// shown at all rather than shown as failures the operator cannot act on.
func TestVerifyOmitsSudoersForLaunchd(t *testing.T) {
	params := systemdParams()
	params.ServiceManager = "launchd"
	params.HostUser = "hintay"
	params = params.FillDefaults()

	result := Verify(context.Background(), VerifyOptions{
		Client: newFakeRunner().on("/bin/echo ok", "ok", nil),
		Params: params,
		Groups: []CheckGroup{CheckGroupPrivileges},
	})

	for _, step := range []string{"sudo_available", "sudoers_coverage"} {
		if _, ok := result.Steps[step]; ok {
			t.Fatalf("step %q was reported for a launchd target", step)
		}
	}
}

// The checks stat and open the configured paths, so a value that is unsafe to
// paste into a shell must not reach the filesystem either.
func TestVerifyRejectsUnsafeParameters(t *testing.T) {
	params := systemdParams()
	params.HostConfigDir = "/etc/nginx; rm -rf /"

	runner := newFakeRunner().on("/bin/echo ok", "ok", nil)
	result := Verify(context.Background(), VerifyOptions{Client: runner, Params: params})

	step, ok := result.Steps["parameters"]
	if !ok || step.OK {
		t.Fatalf("an unsafe host_config_dir was accepted: %+v", result.Steps)
	}
	if len(runner.calls) != 0 {
		t.Fatalf("the pipeline touched the target before validating: %v", runner.calls)
	}
}
