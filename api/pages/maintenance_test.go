package pages

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/0xJacky/Nginx-UI/settings"
)

func setupMaintenanceTemplateSettings(t *testing.T, dir, template string) {
	t.Helper()
	originalDir := settings.NginxSettings.MaintenanceDir
	originalTemplate := settings.NginxSettings.MaintenanceTemplate
	t.Cleanup(func() {
		settings.NginxSettings.MaintenanceDir = originalDir
		settings.NginxSettings.MaintenanceTemplate = originalTemplate
	})

	settings.NginxSettings.MaintenanceDir = dir
	settings.NginxSettings.MaintenanceTemplate = template
}

func TestReadMaintenanceTemplate(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "maintenance.html"), []byte("generic"), 0644); err != nil {
		t.Fatalf("failed to write generic template: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "example.com.maintenance.html"), []byte("site"), 0644); err != nil {
		t.Fatalf("failed to write site template: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "empty.com.maintenance.html"), nil, 0644); err != nil {
		t.Fatalf("failed to write empty site template: %v", err)
	}

	setupMaintenanceTemplateSettings(t, dir, "maintenance.html")

	tests := []struct {
		name     string
		siteName string
		want     string
	}{
		{name: "site specific template wins", siteName: "example.com", want: "site"},
		{name: "unknown site falls back to generic", siteName: "unknown.com", want: "generic"},
		{name: "empty site template falls back to generic", siteName: "empty.com", want: "generic"},
		{name: "missing site header falls back to generic", siteName: "", want: "generic"},
		{name: "path traversal is stripped to the base name", siteName: "../../etc/example.com", want: "site"},
		{name: "dot segment falls back to generic", siteName: "..", want: "generic"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := string(readMaintenanceTemplate(test.siteName)); got != test.want {
				t.Fatalf("readMaintenanceTemplate(%q) = %q, want %q", test.siteName, got, test.want)
			}
		})
	}
}

func TestReadMaintenanceTemplateWithoutConfiguredTemplate(t *testing.T) {
	setupMaintenanceTemplateSettings(t, t.TempDir(), "")

	if content := readMaintenanceTemplate("example.com"); content != nil {
		t.Fatalf("readMaintenanceTemplate() = %q, want nil so the built-in page is used", content)
	}
}

func TestGetMaintenanceDirFallsBackToDefault(t *testing.T) {
	setupMaintenanceTemplateSettings(t, "", "maintenance.html")

	if got := settings.NginxSettings.GetMaintenanceDir(); got != settings.DefaultMaintenanceDir {
		t.Fatalf("GetMaintenanceDir() = %q, want %q", got, settings.DefaultMaintenanceDir)
	}
}
