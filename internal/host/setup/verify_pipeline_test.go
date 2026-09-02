package setup

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeRunner answers a fixed script of commands. An unscripted command is an
// error, so a check that starts issuing a different command is caught rather
// than silently passing. The pipeline runs checks concurrently, so the call
// log is guarded and every Exec can be slowed down to force overlap.
type fakeRunner struct {
	responses map[string]struct {
		out string
		err error
	}
	// delay is applied to every Exec so concurrent checks overlap in tests.
	delay time.Duration

	mu       sync.Mutex
	calls    []string
	inFlight int
	// maxInFlight records the largest number of Exec calls that overlapped.
	maxInFlight int
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
	r.mu.Lock()
	r.calls = append(r.calls, key)
	r.inFlight++
	if r.inFlight > r.maxInFlight {
		r.maxInFlight = r.inFlight
	}
	r.mu.Unlock()
	defer func() {
		r.mu.Lock()
		r.inFlight--
		r.mu.Unlock()
	}()

	if r.delay > 0 {
		time.Sleep(r.delay)
	}
	response, ok := r.responses[key]
	if !ok {
		return "", errors.New("unscripted command: " + key)
	}
	return response.out, response.err
}

// recordedCalls returns a copy of the command log.
func (r *fakeRunner) recordedCalls() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.calls...)
}

// peakInFlight reports how many Exec calls overlapped at most.
func (r *fakeRunner) peakInFlight() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.maxInFlight
}

func systemdParams() SetupParams {
	return SetupParams{
		HostAddress:    "192.168.1.10:22",
		HostUser:       "nginxui",
		AccessMode:     "mounted",
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

func TestVerifySFTPModeChecksRemotePathsWithoutMountChecks(t *testing.T) {
	params := systemdParams()
	params.AccessMode = "sftp"
	runner := newFakeRunner().
		on("/bin/echo ok", "ok", nil).
		on("/usr/bin/uname -s", "Linux", nil).
		on("/bin/systemctl is-active nginx.service", "active", nil).
		on("/bin/systemctl show nginx.service --property=ExecReload", "ExecReload=/bin/kill -HUP $MAINPID", nil).
		on("/bin/test -d /etc/nginx -a -r /etc/nginx -a -x /etc/nginx -a -w /etc/nginx", "", nil).
		on("/bin/test -d /var/log/nginx -a -r /var/log/nginx -a -x /var/log/nginx", "", nil).
		on("/bin/test -e /var/run/nginx.pid", "", nil)

	result := Verify(context.Background(), VerifyOptions{
		Client: runner,
		Params: params,
		Groups: []CheckGroup{CheckGroupPlatform},
	})

	if _, exists := result.Steps["config_dir_shared"]; exists {
		t.Fatal("SFTP verification must not run a bind-mount check")
	}
	for _, key := range []string{"config_dir_writable", "log_dir_readable", "pid_file_present"} {
		if step := result.Steps[key]; !step.OK {
			t.Fatalf("remote SFTP check %q failed: %+v", key, step)
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
	if calls := runner.recordedCalls(); len(calls) != 0 {
		t.Fatalf("the pipeline touched the target before validating: %v", calls)
	}
}

// The checks share one multiplexed SSH connection, so they run concurrently,
// but sshd caps sessions per connection. The pipeline must overlap calls
// without ever exceeding its own limit, and the two sequential gates must
// stay outside the concurrent window.
func TestVerifyRunsChecksConcurrentlyWithinTheSessionCap(t *testing.T) {
	params := systemdParams()
	params.AccessMode = "sftp"
	runner := newFakeRunner().
		on("/bin/echo ok", "ok", nil).
		on("/usr/bin/uname -s", "Linux", nil).
		on("/bin/cat /etc/machine-id", "abc", nil).
		on("/bin/systemctl is-active nginx.service", "active", nil).
		on("/bin/systemctl show nginx.service --property=ExecReload", "ExecReload=/bin/kill -HUP $MAINPID", nil).
		on("/bin/test -d /etc/nginx -a -r /etc/nginx -a -x /etc/nginx -a -w /etc/nginx", "", nil).
		on("/bin/test -d /var/log/nginx -a -r /var/log/nginx -a -x /var/log/nginx", "", nil).
		on("/bin/test -e /var/run/nginx.pid", "", nil).
		on("/usr/bin/id -u", "1000", nil).
		on("/usr/bin/sudo -n /bin/true", "", nil).
		on("/usr/bin/sudo -n -l", "(root) NOPASSWD: /bin/systemctl reload nginx.service\n(root) NOPASSWD: /bin/systemctl restart nginx.service\n(root) NOPASSWD: /usr/sbin/nginx -t\n(root) NOPASSWD: /usr/sbin/nginx -T", nil).
		on("/usr/sbin/nginx -t", "syntax is ok", nil)
	runner.delay = 20 * time.Millisecond

	result := Verify(context.Background(), VerifyOptions{Client: runner, Params: params})

	for _, key := range []string{
		"ssh_connect", "same_host", "host_platform", "systemctl_is_active", "unit_has_execreload",
		"config_dir_writable", "log_dir_readable", "pid_file_present",
		"sudo_available", "sudoers_coverage", "nginx_test",
	} {
		step, ok := result.Steps[key]
		if !ok {
			t.Fatalf("step %q is missing from %v", key, result.Steps)
		}
		if !step.OK {
			t.Fatalf("step %q failed: %+v", key, step)
		}
	}

	peak := runner.peakInFlight()
	if peak > maxConcurrentChecks {
		t.Fatalf("%d checks ran at once, the cap is %d", peak, maxConcurrentChecks)
	}
	if peak < 2 {
		t.Fatalf("the checks ran sequentially (peak %d)", peak)
	}

	calls := runner.recordedCalls()
	if calls[0] != "/bin/echo ok" {
		t.Fatalf("ssh_connect must gate the pipeline, first call was %q", calls[0])
	}
	if last := calls[len(calls)-1]; last != "/usr/sbin/nginx -t" {
		t.Fatalf("nginx -t must run after every other check, last call was %q", last)
	}
	// Both privilege checks share the single uid probe.
	uidProbes := 0
	for _, call := range calls {
		if call == "/usr/bin/id -u" {
			uidProbes++
		}
	}
	if uidProbes != 1 {
		t.Fatalf("id -u ran %d times, want once", uidProbes)
	}
}

// The bounded runner is what keeps the pipeline under the session cap, so it
// is pinned on its own with more tasks than slots.
func TestRunBoundedHonoursTheCap(t *testing.T) {
	var mu sync.Mutex
	inFlight, peak, done := 0, 0, 0
	tasks := make([]func(), 0, maxConcurrentChecks*3)
	for i := 0; i < cap(tasks); i++ {
		tasks = append(tasks, func() {
			mu.Lock()
			inFlight++
			if inFlight > peak {
				peak = inFlight
			}
			mu.Unlock()
			time.Sleep(5 * time.Millisecond)
			mu.Lock()
			inFlight--
			done++
			mu.Unlock()
		})
	}

	runBounded(tasks)

	if done != len(tasks) {
		t.Fatalf("%d of %d tasks finished before runBounded returned", done, len(tasks))
	}
	if peak > maxConcurrentChecks {
		t.Fatalf("peak %d exceeded the cap %d", peak, maxConcurrentChecks)
	}
	if peak < 2 {
		t.Fatalf("tasks did not overlap (peak %d)", peak)
	}
}
