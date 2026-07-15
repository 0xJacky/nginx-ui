package site

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

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
