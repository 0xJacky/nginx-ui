package nginx

import (
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
			if got := extractConfigureArg(output, tt.flag); got != tt.want {
				t.Fatalf("extractConfigureArg(%q) = %q, want %q", tt.flag, got, tt.want)
			}
		})
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

	t.Cleanup(func() {
		settings.NginxSettings.ConfigDir = originalConfigDir
		settings.NginxSettings.ConfigPath = originalConfigPath
		settings.NginxSettings.PIDPath = originalPIDPath
		nginxVOutputCache.set(originalNginxVOutput)
	})

	settings.NginxSettings.ConfigDir = ""
	settings.NginxSettings.ConfigPath = ""
	settings.NginxSettings.PIDPath = ""

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
