package nginx

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/0xJacky/Nginx-UI/settings"
)

func TestLocalRunner_Stat(t *testing.T) {
	r := &localRunner{}
	tmp, err := os.CreateTemp("", "runner-stat-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmp.Name())
	tmp.Close()

	if !r.Stat(tmp.Name()) {
		t.Errorf("Stat(%q) = false, want true", tmp.Name())
	}
	if r.Stat("/nonexistent/path/that/should/not/exist") {
		t.Errorf("Stat(nonexistent) = true, want false")
	}
}

func TestLocalRunner_Exec_Echo(t *testing.T) {
	r := &localRunner{}
	out, err := r.Exec(context.Background(), "echo", "hello")
	if err != nil {
		t.Fatalf("Exec returned err: %v", err)
	}
	if want := "hello\n"; out != want && out != "hello\r\n" {
		t.Errorf("Exec output = %q, want %q", out, want)
	}
}

func TestDockerRunner_RoutesToDockerExec(t *testing.T) {
	// We can't actually exercise docker.Exec without a docker daemon,
	// so this is a smoke test ensuring the type satisfies the interface.
	var _ Runner = (*dockerRunner)(nil)
}

// ResetSSHClient used to write the shared client without a lock, so a
// concurrent newSSHRunner could hand out a runner holding a nil client.
func TestResetSSHClientIsSafeUnderConcurrentUse(t *testing.T) {
	t.Cleanup(func() {
		sshMutex.Lock()
		sshShared = nil
		sshMutex.Unlock()
	})

	var waitGroup sync.WaitGroup
	for range 16 {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			for range 32 {
				runner, ok := newSSHRunner().(*sshRunner)
				if !ok || runner.client == nil {
					t.Error("newSSHRunner() produced a runner without a client")
					return
				}
			}
		}()
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			for range 32 {
				ResetSSHClient()
			}
		}()
	}
	waitGroup.Wait()
}

// Paths resolved from nginx -V/-T describe one control target, so switching
// targets must not keep serving the previous one.
func TestResetHostNginxStateDropsResolvedPaths(t *testing.T) {
	originalSbin := nginxSbinPathCache.value
	originalV := nginxVOutputCache.value
	originalT := nginxTOutputCache.value
	originalPrefix := nginxPrefixCache.value
	originalPID := nginxPIDPathCache.value
	t.Cleanup(func() {
		nginxSbinPathCache.set(originalSbin)
		nginxVOutputCache.set(originalV)
		nginxTOutputCache.set(originalT)
		nginxPrefixCache.set(originalPrefix)
		nginxPIDPathCache.set(originalPID)
	})

	nginxSbinPathCache.set("/remote/sbin/nginx")
	nginxVOutputCache.set("configure arguments: --prefix=/remote")
	nginxTOutputCache.set("pid /remote/nginx.pid;")
	nginxPrefixCache.set("/remote")
	nginxPIDPathCache.set("/remote/nginx.pid")

	ResetHostNginxState()

	for name, cache := range map[string]*nginxStringCache{
		"sbin path": &nginxSbinPathCache,
		"nginx -V":  &nginxVOutputCache,
		"nginx -T":  &nginxTOutputCache,
		"prefix":    &nginxPrefixCache,
		"pid path":  &nginxPIDPathCache,
	} {
		if cache.value != "" {
			t.Errorf("%s cache still holds %q after a target change", name, cache.value)
		}
	}
}

// With SbinPath empty, SSH mode used to fall back to exec.LookPath inside this
// container and hand hostssh.Config an empty NginxSbinPath, so nginx -t/-T ran
// on the host unprivileged or with a path the host does not have.
func TestSSHModeResolvesHostSbinPathDefault(t *testing.T) {
	originalSettings := *settings.NginxSettings
	t.Cleanup(func() {
		*settings.NginxSettings = originalSettings
		resetPathCaches()
	})

	tests := []struct {
		name           string
		serviceManager string
		sbinPath       string
		want           string
	}{
		{name: "systemd default", serviceManager: settings.HostServiceManagerSystemd, want: settings.DefaultHostSbinPathSystemd},
		{name: "launchd default", serviceManager: settings.HostServiceManagerLaunchd, want: settings.DefaultHostSbinPathLaunchd},
		{name: "configured path wins", serviceManager: settings.HostServiceManagerLaunchd, sbinPath: "/usr/local/bin/nginx", want: "/usr/local/bin/nginx"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetPathCaches()
			*settings.NginxSettings = settings.Nginx{
				HostMode:           settings.HostModeSSH,
				HostServiceManager: tt.serviceManager,
				SbinPath:           tt.sbinPath,
				HostKnownHostsPath: filepath.Join(t.TempDir(), "known_hosts"),
			}

			if got := getNginxSbinPath(); got != tt.want {
				t.Fatalf("getNginxSbinPath() = %q, want %q", got, tt.want)
			}
			if got := buildSSHOptions().Config.NginxSbinPath; got != tt.want {
				t.Fatalf("buildSSHOptions().Config.NginxSbinPath = %q, want %q", got, tt.want)
			}
		})
	}
}
