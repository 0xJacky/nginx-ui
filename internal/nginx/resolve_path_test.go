package nginx

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"testing"

	"github.com/0xJacky/Nginx-UI/settings"
)

func TestExtractConfigureArg(t *testing.T) {
	t.Parallel()

	output := `
nginx version: nginx/1.25.2
configure arguments: --prefix="/Program Files/Nginx" --conf-path='/Program Files/Nginx/conf/nginx.conf' --pid-path=/var/run/nginx.pid
`

	tests := []struct {
		name string
		flag string
		want string
	}{
		{
			name: "double quoted conf path",
			flag: "--conf-path",
			want: "/Program Files/Nginx/conf/nginx.conf",
		},
		{
			name: "single quoted conf path alias",
			flag: "conf-path",
			want: "/Program Files/Nginx/conf/nginx.conf",
		},
		{
			name: "unquoted pid path",
			flag: "pid-path",
			want: "/var/run/nginx.pid",
		},
		{
			name: "missing flag",
			flag: "--http-log-path",
			want: "",
		},
		{
			name: "prefix parsing",
			flag: "prefix",
			want: "/Program Files/Nginx",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			if got := ExtractConfigureArg(output, tt.flag); got != tt.want {
				t.Fatalf("ExtractConfigureArg(%q) = %q, want %q", tt.flag, got, tt.want)
			}
		})
	}
}

// A warm lookup cache must not shadow the configured path, otherwise changing
// the nginx binary in settings only takes effect after a restart.
func TestGetSbinPathPrefersSettingsOverWarmCache(t *testing.T) {
	originalSbinPath := settings.NginxSettings.SbinPath
	originalCache := nginxSbinPathCache.value

	t.Cleanup(func() {
		settings.NginxSettings.SbinPath = originalSbinPath
		nginxSbinPathCache.set(originalCache)
	})

	settings.NginxSettings.SbinPath = ""
	nginxSbinPathCache.set("/discovered/bin/nginx")

	settings.NginxSettings.SbinPath = "/usr/sbin/nginx"

	if got := GetSbinPath(); got != "/usr/sbin/nginx" {
		t.Fatalf("GetSbinPath() = %q, want the configured path", got)
	}
}

// GetPrefix must resolve through the serialized cache. A plain package level
// string here races under concurrent cold callers such as the log indexer and
// the log path whitelist.
func TestGetPrefixSerializesColdLoad(t *testing.T) {
	originalNginxVOutput := nginxVOutputCache.value
	originalPrefix := nginxPrefixCache.value

	t.Cleanup(func() {
		nginxVOutputCache.set(originalNginxVOutput)
		nginxPrefixCache.set(originalPrefix)
	})

	nginxVOutputCache.set(`
nginx version: nginx/1.25.2
configure arguments: --prefix=/opt/nginx-prefix
`)
	nginxPrefixCache.set("")

	const callers = 32
	results := make([]string, callers)
	var waitGroup sync.WaitGroup

	for i := range callers {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			results[i] = GetPrefix()
		}()
	}
	waitGroup.Wait()

	if results[0] == "" {
		t.Fatal("GetPrefix() = empty, want the configured prefix")
	}
	for i, got := range results {
		if got != results[0] {
			t.Fatalf("GetPrefix() call %d = %q, want %q", i, got, results[0])
		}
	}
}

func TestGetConfAndPidPathsHandleSpaces(t *testing.T) {
	originalConfigDir := settings.NginxSettings.ConfigDir
	originalConfigPath := settings.NginxSettings.ConfigPath
	originalPIDPath := settings.NginxSettings.PIDPath
	originalNginxVOutput := nginxVOutputCache.value
	originalPIDCache := nginxPIDPathCache.value

	t.Cleanup(func() {
		settings.NginxSettings.ConfigDir = originalConfigDir
		settings.NginxSettings.ConfigPath = originalConfigPath
		settings.NginxSettings.PIDPath = originalPIDPath
		nginxVOutputCache.set(originalNginxVOutput)
		nginxPIDPathCache.set(originalPIDCache)
	})

	settings.NginxSettings.ConfigDir = ""
	settings.NginxSettings.ConfigPath = ""
	settings.NginxSettings.PIDPath = ""
	nginxPIDPathCache.set("")

	sampleConf := "/Program Files/nginx/conf/nginx.conf"
	samplePID := "/Program Files/nginx/logs/nginx.pid"

	nginxVOutputCache.set(fmt.Sprintf(`
nginx version: nginx/1.25.2
configure arguments: --conf-path="%s" --pid-path="%s"
`, sampleConf, samplePID))

	confDir := GetConfPath()
	expectedConfDir := filepath.Dir(sampleConf)
	if confDir != expectedConfDir {
		t.Fatalf("GetConfPath() = %q, want %q", confDir, expectedConfDir)
	}

	confEntry := GetConfEntryPath()
	if confEntry != sampleConf {
		t.Fatalf("GetConfEntryPath() = %q, want %q", confEntry, sampleConf)
	}

	pidPath := GetPIDPath()
	if pidPath != samplePID {
		t.Fatalf("GetPIDPath() = %q, want %q", pidPath, samplePID)
	}
}

type countingStatRunner struct {
	mutex sync.Mutex
	stats int
}

func (r *countingStatRunner) Exec(context.Context, string, ...string) (string, error) {
	return "", nil
}

func (r *countingStatRunner) Stat(string) bool {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	r.stats++
	return true
}

func (r *countingStatRunner) count() int {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	return r.stats
}

// In SSH mode every probe is a remote exec, so the discovered PID path must be
// served from the cache until the control target changes.
func TestGetPIDPathIsMemoizedUntilTargetChanges(t *testing.T) {
	originalSettings := *settings.NginxSettings
	originalRunner := resolveRunner
	originalNginxVOutput := nginxVOutputCache.value
	originalNginxTOutput := nginxTOutputCache.value
	originalPIDPath := nginxPIDPathCache.value
	t.Cleanup(func() {
		*settings.NginxSettings = originalSettings
		resolveRunner = originalRunner
		nginxVOutputCache.set(originalNginxVOutput)
		nginxTOutputCache.set(originalNginxTOutput)
		nginxPIDPathCache.set(originalPIDPath)
	})

	runner := &countingStatRunner{}
	resolveRunner = func() Runner { return runner }
	*settings.NginxSettings = settings.Nginx{HostMode: settings.HostModeSSH}
	// nginx -T has no pid directive, so the compiled default is probed on the target.
	nginxTOutputCache.set("events {}")
	nginxVOutputCache.set("configure arguments: --pid-path=/remote/run/nginx.pid")
	nginxPIDPathCache.set("")

	const want = "/remote/run/nginx.pid"
	for i := range 3 {
		if got := GetPIDPath(); got != want {
			t.Fatalf("GetPIDPath() call %d = %q, want %q", i, got, want)
		}
	}
	if got := runner.count(); got != 1 {
		t.Fatalf("runner.Stat called %d times, want 1 (cached after the first resolution)", got)
	}

	// A settings override bypasses the cache without touching it.
	settings.NginxSettings.PIDPath = "/override/nginx.pid"
	if got := GetPIDPath(); got != "/override/nginx.pid" {
		t.Fatalf("GetPIDPath() = %q, want the configured override", got)
	}
	settings.NginxSettings.PIDPath = ""

	// Switching targets must drop the memoized path and probe again.
	ResetHostNginxState()
	nginxTOutputCache.set("events {}")
	nginxVOutputCache.set("configure arguments: --pid-path=/other/run/nginx.pid")
	if got := GetPIDPath(); got != "/other/run/nginx.pid" {
		t.Fatalf("GetPIDPath() after reset = %q, want the new target's path", got)
	}
	if got := runner.count(); got != 2 {
		t.Fatalf("runner.Stat called %d times, want 2 (one probe per target)", got)
	}
}
