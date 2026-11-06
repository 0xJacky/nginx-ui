package site

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/0xJacky/Nginx-UI/internal/config"
	"github.com/0xJacky/Nginx-UI/internal/nginx_log/utils"
	"github.com/0xJacky/Nginx-UI/settings"
)

func TestBuildLogEntries(t *testing.T) {
	allValid := func(string) bool { return true }

	tests := []struct {
		name             string
		directives       []utils.LogDirective
		defaultAccessLog string
		defaultErrorLog  string
		isValid          func(string) bool
		want             []LogEntry
	}{
		{
			name: "site with own access and error logs",
			directives: []utils.LogDirective{
				{Type: "access", Path: "/var/log/nginx/site.access.log"},
				{Type: "error", Path: "/var/log/nginx/site.error.log"},
			},
			defaultAccessLog: "/var/log/nginx/access.log",
			defaultErrorLog:  "/var/log/nginx/error.log",
			isValid:          allValid,
			want: []LogEntry{
				{Type: "access", Path: "/var/log/nginx/site.access.log", Valid: true},
				{Type: "error", Path: "/var/log/nginx/site.error.log", Valid: true},
			},
		},
		{
			name:             "no directives falls back to inherited defaults",
			directives:       nil,
			defaultAccessLog: "/var/log/nginx/access.log",
			defaultErrorLog:  "/var/log/nginx/error.log",
			isValid:          allValid,
			want: []LogEntry{
				{Type: "access", Path: "/var/log/nginx/access.log", Inherited: true, Valid: true},
				{Type: "error", Path: "/var/log/nginx/error.log", Inherited: true, Valid: true},
			},
		},
		{
			name: "access log only falls back to inherited error log",
			directives: []utils.LogDirective{
				{Type: "access", Path: "/var/log/nginx/site.access.log"},
			},
			defaultAccessLog: "/var/log/nginx/access.log",
			defaultErrorLog:  "/var/log/nginx/error.log",
			isValid:          allValid,
			want: []LogEntry{
				{Type: "access", Path: "/var/log/nginx/site.access.log", Valid: true},
				{Type: "error", Path: "/var/log/nginx/error.log", Inherited: true, Valid: true},
			},
		},
		{
			name: "duplicate directives deduplicated",
			directives: []utils.LogDirective{
				{Type: "access", Path: "/var/log/nginx/site.access.log"},
				{Type: "access", Path: "/var/log/nginx/site.access.log"},
				{Type: "access", Path: "/var/log/nginx/other.access.log"},
			},
			defaultAccessLog: "/var/log/nginx/access.log",
			defaultErrorLog:  "",
			isValid:          allValid,
			want: []LogEntry{
				{Type: "access", Path: "/var/log/nginx/site.access.log", Valid: true},
				{Type: "access", Path: "/var/log/nginx/other.access.log", Valid: true},
			},
		},
		{
			name:             "empty defaults produce no inherited entries",
			directives:       nil,
			defaultAccessLog: "",
			defaultErrorLog:  "",
			isValid:          allValid,
			want:             []LogEntry{},
		},
		{
			name: "invalid path flagged",
			directives: []utils.LogDirective{
				{Type: "access", Path: "/opt/outside/whitelist.log"},
			},
			defaultAccessLog: "",
			defaultErrorLog:  "",
			isValid:          func(string) bool { return false },
			want: []LogEntry{
				{Type: "access", Path: "/opt/outside/whitelist.log", Valid: false},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildLogEntries(tt.directives, tt.defaultAccessLog, tt.defaultErrorLog, tt.isValid)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("buildLogEntries() = %v, want %v", got, tt.want)
			}
		})
	}
}

// setupLogsTestSettings snapshots and restores the settings globals mutated by
// the GetLogs integration test, then installs canonical test values.
func setupLogsTestSettings(t *testing.T, configDir string) {
	t.Helper()
	originalConfigDir := settings.NginxSettings.ConfigDir
	originalAccessLogPath := settings.NginxSettings.AccessLogPath
	originalErrorLogPath := settings.NginxSettings.ErrorLogPath
	originalLogDirWhiteList := settings.NginxSettings.LogDirWhiteList
	t.Cleanup(func() {
		settings.NginxSettings.ConfigDir = originalConfigDir
		settings.NginxSettings.AccessLogPath = originalAccessLogPath
		settings.NginxSettings.ErrorLogPath = originalErrorLogPath
		settings.NginxSettings.LogDirWhiteList = originalLogDirWhiteList
	})

	settings.NginxSettings.ConfigDir = configDir
	settings.NginxSettings.AccessLogPath = "/var/log/nginx/access.log"
	settings.NginxSettings.ErrorLogPath = "/var/log/nginx/error.log"
	settings.NginxSettings.LogDirWhiteList = []string{"/var/log/nginx"}
}

func TestGetLogs(t *testing.T) {
	configDir := t.TempDir()
	sitesAvailable := filepath.Join(configDir, "sites-available")
	if err := os.MkdirAll(sitesAvailable, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	siteConfig := `server {
    listen 80;
    server_name example.com;
    access_log /var/log/nginx/example.access.log main;
    # access_log /var/log/nginx/commented.log;
}`
	if err := os.WriteFile(filepath.Join(sitesAvailable, "example.com"), []byte(siteConfig), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	setupLogsTestSettings(t, configDir)

	logs, err := GetLogs("example.com")
	if err != nil {
		t.Fatalf("GetLogs() error = %v", err)
	}

	want := []LogEntry{
		{Type: "access", Path: "/var/log/nginx/example.access.log", Valid: true},
		{Type: "error", Path: "/var/log/nginx/error.log", Inherited: true, Valid: true},
	}
	if !reflect.DeepEqual(logs, want) {
		t.Errorf("GetLogs() = %v, want %v", logs, want)
	}
}

func TestGetLogsSiteNotFound(t *testing.T) {
	configDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(configDir, "sites-available"), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	setupLogsTestSettings(t, configDir)

	_, err := GetLogs("not-exist.com")
	if err == nil {
		t.Fatal("GetLogs() expected error for missing site, got nil")
	}
}

func TestResolveLogPaths(t *testing.T) {
	allValid := func(string) bool { return true }
	validUnder := func(prefix string) func(string) bool {
		return func(p string) bool { return strings.HasPrefix(p, prefix) }
	}

	tests := []struct {
		name             string
		directives       []utils.LogDirective
		defaultAccessLog string
		defaultErrorLog  string
		isValid          func(string) bool
		want             LogPaths
	}{
		{
			name: "own directives win over the defaults",
			directives: []utils.LogDirective{
				{Type: "access", Path: "/var/log/nginx/site.access.log"},
				{Type: "error", Path: "/var/log/nginx/site.error.log"},
			},
			defaultAccessLog: "/var/log/nginx/access.log",
			defaultErrorLog:  "/var/log/nginx/error.log",
			isValid:          allValid,
			want: LogPaths{
				AccessPath: "/var/log/nginx/site.access.log",
				ErrorPath:  "/var/log/nginx/site.error.log",
			},
		},
		{
			name:             "no directives inherit the defaults",
			defaultAccessLog: "/var/log/nginx/access.log",
			defaultErrorLog:  "/var/log/nginx/error.log",
			isValid:          allValid,
			want: LogPaths{
				AccessPath:      "/var/log/nginx/access.log",
				AccessInherited: true,
				ErrorPath:       "/var/log/nginx/error.log",
				ErrorInherited:  true,
			},
		},
		{
			name: "each kind falls back on its own",
			directives: []utils.LogDirective{
				{Type: "access", Path: "/var/log/nginx/site.access.log"},
			},
			defaultAccessLog: "/var/log/nginx/access.log",
			defaultErrorLog:  "/var/log/nginx/error.log",
			isValid:          allValid,
			want: LogPaths{
				AccessPath:     "/var/log/nginx/site.access.log",
				ErrorPath:      "/var/log/nginx/error.log",
				ErrorInherited: true,
			},
		},
		{
			name: "the first valid own directive is used",
			directives: []utils.LogDirective{
				{Type: "access", Path: "/opt/outside.log"},
				{Type: "access", Path: "/var/log/nginx/second.log"},
			},
			defaultAccessLog: "/var/log/nginx/access.log",
			isValid:          validUnder("/var/log/nginx"),
			want:             LogPaths{AccessPath: "/var/log/nginx/second.log"},
		},
		{
			name: "an invalid own directive does not fall back to the default",
			directives: []utils.LogDirective{
				{Type: "access", Path: "/opt/outside.log"},
			},
			defaultAccessLog: "/var/log/nginx/access.log",
			isValid:          validUnder("/var/log/nginx"),
			want:             LogPaths{},
		},
		{
			name:             "an invalid default is left out",
			defaultAccessLog: "/opt/outside.log",
			defaultErrorLog:  "/var/log/nginx/error.log",
			isValid:          validUnder("/var/log/nginx"),
			want:             LogPaths{ErrorPath: "/var/log/nginx/error.log", ErrorInherited: true},
		},
		{
			name:    "nothing declared and no default",
			isValid: allValid,
			want:    LogPaths{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveLogPaths(tt.directives, tt.defaultAccessLog, tt.defaultErrorLog, tt.isValid)
			if got != tt.want {
				t.Errorf("resolveLogPaths() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestScanForSiteRecordsLogDirectives(t *testing.T) {
	configDir := t.TempDir()
	setupLogsTestSettings(t, configDir)

	const name = "logs-scan-test"
	t.Cleanup(func() {
		siteIndexMutex.Lock()
		delete(IndexedSites, name)
		siteIndexMutex.Unlock()
	})

	// No valid server_name, so the site only enters the index through its logs.
	content := []byte(`server {
    listen 80;
    access_log /var/log/nginx/logs-scan.access.log main;
    # error_log /var/log/nginx/commented.log;
}`)
	if err := scanForSite(filepath.Join(configDir, "sites-available", name), content); err != nil {
		t.Fatalf("scanForSite() error = %v", err)
	}

	got := GetIndexedSite(name).LogDirectives
	want := []utils.LogDirective{{Type: "access", Path: "/var/log/nginx/logs-scan.access.log"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("LogDirectives = %v, want %v", got, want)
	}

	if err := scanForSite(filepath.Join(configDir, "sites-available", name), nil); err != nil {
		t.Fatalf("scanForSite(remove) error = %v", err)
	}
	if got := GetIndexedSite(name).LogDirectives; len(got) != 0 {
		t.Errorf("LogDirectives after removal = %v, want none", got)
	}
}

func TestBuildConfigReportsSiteLogPaths(t *testing.T) {
	configDir := t.TempDir()
	setupLogsTestSettings(t, configDir)

	info, err := os.Stat(configDir)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}

	const own, inheriting = "logs-own-test", "logs-inherit-test"
	siteIndexMutex.Lock()
	IndexedSites[own] = &Index{
		Urls: []string{"http://own.example.com"},
		LogDirectives: []utils.LogDirective{
			{Type: "access", Path: "/var/log/nginx/own.access.log"},
			{Type: "error", Path: "/var/log/nginx/own.error.log"},
		},
	}
	IndexedSites[inheriting] = &Index{Urls: []string{"http://inherit.example.com"}}
	siteIndexMutex.Unlock()
	t.Cleanup(func() {
		siteIndexMutex.Lock()
		delete(IndexedSites, own)
		delete(IndexedSites, inheriting)
		siteIndexMutex.Unlock()
	})

	build := newConfigBuilder()

	ownConfig := build(own, info, config.StatusEnabled, 0, 0, nil)
	if ownConfig.AccessLogPath != "/var/log/nginx/own.access.log" || ownConfig.AccessLogInherited ||
		ownConfig.ErrorLogPath != "/var/log/nginx/own.error.log" || ownConfig.ErrorLogInherited {
		t.Errorf("own logs = %+v", ownConfig)
	}

	inheritConfig := build(inheriting, info, config.StatusEnabled, 0, 0, nil)
	if inheritConfig.AccessLogPath != "/var/log/nginx/access.log" || !inheritConfig.AccessLogInherited ||
		inheritConfig.ErrorLogPath != "/var/log/nginx/error.log" || !inheritConfig.ErrorLogInherited {
		t.Errorf("inherited logs = %+v", inheritConfig)
	}

	unknown := build("logs-unknown-test", info, config.StatusEnabled, 0, 0, nil)
	if unknown.AccessLogPath != "/var/log/nginx/access.log" || !unknown.AccessLogInherited {
		t.Errorf("a site outside the index inherits the default log, got %+v", unknown)
	}
}
