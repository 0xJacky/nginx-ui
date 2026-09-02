package nginx

import (
	"context"
	"runtime"
	"strings"
	"testing"

	"github.com/0xJacky/Nginx-UI/settings"
)

// recordingRunner reports a fixed GOOS and records every command instead of
// running it, so resolvers can be exercised against a target OS that differs
// from the one running the tests.
type recordingRunner struct {
	goos     string
	commands []string
}

func (r *recordingRunner) Exec(_ context.Context, name string, args ...string) (string, error) {
	r.commands = append(r.commands, strings.Join(append([]string{name}, args...), " "))
	return "", nil
}

func (r *recordingRunner) Stat(string) bool { return false }

func (r *recordingRunner) GOOS() string { return r.goos }

func stubTargetGOOS(t *testing.T, goos string) *recordingRunner {
	t.Helper()
	originalResolve := resolveRunner
	t.Cleanup(func() { resolveRunner = originalResolve })
	runner := &recordingRunner{goos: goos}
	resolveRunner = func() Runner { return runner }
	return runner
}

func stubNginxSettings(t *testing.T, n settings.Nginx) {
	t.Helper()
	originalSettings := *settings.NginxSettings
	originalSbin := nginxSbinPathCache.value
	originalV := nginxVOutputCache.value
	originalT := nginxTOutputCache.value
	originalPrefix := nginxPrefixCache.value
	originalPID := nginxPIDPathCache.value
	t.Cleanup(func() {
		*settings.NginxSettings = originalSettings
		nginxSbinPathCache.set(originalSbin)
		nginxVOutputCache.set(originalV)
		nginxTOutputCache.set(originalT)
		nginxPrefixCache.set(originalPrefix)
		nginxPIDPathCache.set(originalPID)
	})
	*settings.NginxSettings = n
	resetPathCaches()
}

func TestRunnerGOOS(t *testing.T) {
	if got := (&localRunner{}).GOOS(); got != runtime.GOOS {
		t.Fatalf("localRunner.GOOS() = %q, want %q", got, runtime.GOOS)
	}
	if got := (&dockerRunner{}).GOOS(); got != "linux" {
		t.Fatalf("dockerRunner.GOOS() = %q, want linux", got)
	}

	tests := []struct {
		name           string
		serviceManager string
		want           string
	}{
		{name: "systemd host is linux", serviceManager: settings.HostServiceManagerSystemd, want: "linux"},
		{name: "launchd host is darwin", serviceManager: settings.HostServiceManagerLaunchd, want: "darwin"},
		{name: "unset manager defaults to systemd", want: "linux"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stubNginxSettings(t, settings.Nginx{HostMode: settings.HostModeSSH, HostServiceManager: tt.serviceManager})
			t.Cleanup(ResetSSHClient)
			if got := newSSHRunner().GOOS(); got != tt.want {
				t.Fatalf("sshRunner.GOOS() = %q, want %q", got, tt.want)
			}
		})
	}
}

// resolvePath used to consult runtime.GOOS, so a Windows hosted Nginx UI
// joined relative paths from a Linux target with its own exe dir, and a Linux
// hosted one left relative Windows paths untouched.
func TestResolvePathFollowsTargetGOOS(t *testing.T) {
	tests := []struct {
		name string
		goos string
		path string
		want string
	}{
		{name: "windows relative path joins exe dir", goos: "windows", path: "conf/nginx.conf", want: "/opt/nginx/conf/nginx.conf"},
		{name: "windows absolute path unchanged", goos: "windows", path: "/opt/nginx/conf/nginx.conf", want: "/opt/nginx/conf/nginx.conf"},
		{name: "linux relative path unchanged", goos: "linux", path: "conf/nginx.conf", want: "conf/nginx.conf"},
		{name: "darwin relative path unchanged", goos: "darwin", path: "conf/nginx.conf", want: "conf/nginx.conf"},
		{name: "empty path", goos: "windows", path: "", want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stubNginxSettings(t, settings.Nginx{SbinPath: "/opt/nginx/nginx.exe"})
			stubTargetGOOS(t, tt.goos)
			if got := resolvePath(tt.path); got != tt.want {
				t.Fatalf("resolvePath(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestPathFallbacksFollowTargetGOOS(t *testing.T) {
	// nginx -V without --prefix, --conf-path or --modules-path forces every
	// resolver onto its OS dependent default.
	const nginxV = "nginx version: nginx/1.25.2\nconfigure arguments: --with-http_ssl_module"

	tests := []struct {
		name        string
		goos        string
		wantPrefix  string
		wantConf    string
		wantModules string
	}{
		{name: "windows target", goos: "windows", wantPrefix: "/opt/nginx", wantConf: "/opt/nginx", wantModules: "/opt/nginx/modules"},
		{name: "linux target", goos: "linux", wantPrefix: "/usr/local/nginx", wantConf: "/etc/nginx", wantModules: "/usr/lib/nginx/modules"},
		{name: "darwin target", goos: "darwin", wantPrefix: "/usr/local/nginx", wantConf: "/etc/nginx", wantModules: "/usr/lib/nginx/modules"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stubNginxSettings(t, settings.Nginx{SbinPath: "/opt/nginx/nginx.exe"})
			runner := stubTargetGOOS(t, tt.goos)
			nginxVOutputCache.set(nginxV)

			if got := GetPrefix(); got != tt.wantPrefix {
				t.Fatalf("GetPrefix() = %q, want %q", got, tt.wantPrefix)
			}
			if got := GetConfPath(); got != tt.wantConf {
				t.Fatalf("GetConfPath() = %q, want %q", got, tt.wantConf)
			}
			if got := GetModulesPath(); got != tt.wantModules {
				t.Fatalf("GetModulesPath() = %q, want %q", got, tt.wantModules)
			}
			if len(runner.commands) != 0 {
				t.Fatalf("resolvers ran %v, want only cached nginx -V output", runner.commands)
			}
		})
	}
}

// Non-local modes must not look nginx up in this container's PATH: SSH uses
// the host default and docker lets the container's PATH resolve the name.
func TestGetNginxSbinPathSkipsLocalLookupForRemoteTargets(t *testing.T) {
	tests := []struct {
		name     string
		settings settings.Nginx
		want     string
	}{
		{name: "external container", settings: settings.Nginx{ContainerName: "nginx"}, want: "nginx"},
		{name: "external container configured path wins", settings: settings.Nginx{ContainerName: "nginx", SbinPath: "/usr/local/openresty/bin/openresty"}, want: "/usr/local/openresty/bin/openresty"},
		{name: "ssh systemd host", settings: settings.Nginx{HostMode: settings.HostModeSSH}, want: settings.DefaultHostSbinPathSystemd},
		{name: "ssh launchd host", settings: settings.Nginx{HostMode: settings.HostModeSSH, HostServiceManager: settings.HostServiceManagerLaunchd}, want: settings.DefaultHostSbinPathLaunchd},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stubNginxSettings(t, tt.settings)
			stubTargetGOOS(t, "linux")
			// A warm local lookup must be ignored for a remote target.
			nginxSbinPathCache.set("/usr/sbin/nginx")

			if got := getNginxSbinPath(); got != tt.want {
				t.Fatalf("getNginxSbinPath() = %q, want %q", got, tt.want)
			}
		})
	}
}
