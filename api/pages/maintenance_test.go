package pages

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
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
		{name: "disallowed characters fall back to generic", siteName: "example.com; rm -rf", want: "generic"},
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

func TestSanitizeMaintenanceFileNameRejectsTraversal(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "plain name", input: "maintenance.html", want: "maintenance.html"},
		{name: "absolute posix path", input: "/etc/passwd", want: "passwd"},
		{name: "absolute windows path", input: `C:\Windows\System32\config`, want: "config"},
		{name: "traversal", input: "..", want: ""},
		{name: "nul byte", input: "maintenance.html\x00.png", want: ""},
		{name: "shell metacharacters", input: "a;b", want: ""},
		{name: "space", input: "a b", want: ""},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := sanitizeMaintenanceFileName(test.input); got != test.want {
				t.Fatalf("sanitizeMaintenanceFileName(%q) = %q, want %q", test.input, got, test.want)
			}
		})
	}
}

func TestMaintenanceMeta(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodGet, "/pages/maintenance/meta", nil)
	req.Header.Set(maintenanceSiteHeader, "example.com")
	req.Header.Set(maintenanceStartTimeHeader, "2026-09-09 10:00:00")
	req.Header.Set(maintenanceEndTimeHeader, "2026-09-09 12:00:00")
	req.Header.Set(maintenanceContactHeader, "ops@example.com")
	req.Header.Set(maintenanceAdditionalInfoHeader, "planned db migration")
	c.Request = req

	MaintenanceMeta(c)

	if w.Code != http.StatusOK {
		t.Fatalf("MaintenanceMeta() status = %d, want %d", w.Code, http.StatusOK)
	}

	var payload map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if payload["site_name"] != "example.com" {
		t.Fatalf("site_name = %q, want %q", payload["site_name"], "example.com")
	}
	if payload["start_time"] != "2026-09-09 10:00:00" {
		t.Fatalf("start_time = %q, want %q", payload["start_time"], "2026-09-09 10:00:00")
	}
	if payload["end_time"] != "2026-09-09 12:00:00" {
		t.Fatalf("end_time = %q, want %q", payload["end_time"], "2026-09-09 12:00:00")
	}
	if payload["contact"] != "ops@example.com" {
		t.Fatalf("contact = %q, want %q", payload["contact"], "ops@example.com")
	}
	if payload["additioninfomation"] != "planned db migration" {
		t.Fatalf("additioninfomation = %q, want %q", payload["additioninfomation"], "planned db migration")
	}
}
