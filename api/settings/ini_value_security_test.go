package settings

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/0xJacky/Nginx-UI/internal/middleware"
	appsettings "github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/ini.v1"
)

// These tests cover GHSA-jjg7-4xmx-2vvh: a settings value that app.ini cannot
// store faithfully must be refused, so one field can never add or replace
// other keys or sections when the file is read back.

// unstorableValue returns a value whose triple-quoted form in app.ini would end
// early, leaving the given lines to be read as separate lines of the file.
func unstorableValue(lines ...string) string {
	return "x\"\"\"\n" + strings.Join(lines, "\n") + "\nz = \"\"\""
}

func jsonBody(t *testing.T, value any) string {
	t.Helper()
	body, err := json.Marshal(value)
	require.NoError(t, err)
	return string(body)
}

// keepNginxAndTerminalSettings restores the live sections these tests change.
func keepNginxAndTerminalSettings(t *testing.T) {
	t.Helper()
	originalNginx := *appsettings.NginxSettings
	originalTerminal := *appsettings.TerminalSettings
	originalPlugin := *appsettings.PluginSettings
	t.Cleanup(func() {
		*appsettings.NginxSettings = originalNginx
		*appsettings.TerminalSettings = originalTerminal
		*appsettings.PluginSettings = originalPlugin
	})
}

func loadWrittenSettings(t *testing.T, confPath string) *ini.File {
	t.Helper()
	conf, err := ini.LoadSources(ini.LoadOptions{Loose: true, AllowShadows: true}, confPath)
	require.NoError(t, err)
	return conf
}

func requireUnstorableValueError(t *testing.T, w *httptest.ResponseRecorder, key string) {
	t.Helper()
	require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	var response struct {
		Scope  string   `json:"scope"`
		Code   int32    `json:"code"`
		Params []string `json:"params"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	assert.Equal(t, "settings", response.Scope)
	assert.Equal(t, int32(40007), response.Code)
	assert.Equal(t, []string{key}, response.Params)
}

func TestSaveSettingsSectionRejectsValueThatWouldAddSections(t *testing.T) {
	confPath := useTempSettingsFile(t)
	keepNginxAndTerminalSettings(t)
	appsettings.TerminalSettings.StartCmd = "saved-start-cmd"
	appsettings.NginxSettings.MaintenanceTemplate = "maintenance.html"
	require.NoError(t, appsettings.Save())
	before, err := os.ReadFile(confPath)
	require.NoError(t, err)

	w := postSection(t, "nginx", jsonBody(t, map[string]string{
		"maintenance_template": unstorableValue("[terminal]", "StartCmd = section-marker", "[nginx]"),
	}))

	requireUnstorableValueError(t, w, "nginx.MaintenanceTemplate")

	written, err := os.ReadFile(confPath)
	require.NoError(t, err)
	assert.Equal(t, string(before), string(written), "a refused save must leave app.ini untouched")
	conf := loadWrittenSettings(t, confPath)
	assert.Equal(t, "saved-start-cmd", conf.Section("terminal").Key("StartCmd").String())

	// The refused value is rolled back in memory too, so it neither shows up
	// in the settings nor blocks the next save.
	assert.Equal(t, "maintenance.html", appsettings.NginxSettings.MaintenanceTemplate)
	assert.Equal(t, "saved-start-cmd", appsettings.TerminalSettings.StartCmd)

	w = postSection(t, "nginx", `{"maintenance_template":"other.html"}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	conf = loadWrittenSettings(t, confPath)
	assert.Equal(t, "other.html", conf.Section("nginx").Key("MaintenanceTemplate").String())
	assert.Equal(t, "saved-start-cmd", conf.Section("terminal").Key("StartCmd").String())
}

// TestSaveSettingsSectionRejectsValueThatWouldShadowKeysInItsSection covers the
// variant that adds a second copy of a protected key inside the same section,
// followed by another save that could otherwise reorder the copies.
func TestSaveSettingsSectionRejectsValueThatWouldShadowKeysInItsSection(t *testing.T) {
	confPath := useTempSettingsFile(t)
	keepNginxAndTerminalSettings(t)
	appsettings.NginxSettings.TestConfigCmd = "saved-test-cmd"
	require.NoError(t, appsettings.Save())

	w := postSection(t, "nginx", jsonBody(t, map[string]string{
		"maintenance_template": unstorableValue("TestConfigCmd = shadow-marker"),
	}))
	requireUnstorableValueError(t, w, "nginx.MaintenanceTemplate")

	w = postSection(t, "nginx", `{"maintenance_host":"maintenance.example.com"}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	written, err := os.ReadFile(confPath)
	require.NoError(t, err)
	assert.NotContains(t, string(written), "shadow-marker")
	conf := loadWrittenSettings(t, confPath)
	assert.Equal(t, []string{"saved-test-cmd"}, conf.Section("nginx").Key("TestConfigCmd").ValueWithShadows())
	assert.Equal(t, "saved-test-cmd", appsettings.NginxSettings.TestConfigCmd)
}

// A trailing backslash continues an ini value onto the next line, which would
// swallow the key written after it.
func TestSaveSettingsSectionRejectsValueThatWouldSwallowTheNextKey(t *testing.T) {
	confPath := useTempSettingsFile(t)
	keepNginxAndTerminalSettings(t)
	appsettings.NginxSettings.MaintenanceHost = "maintenance.example.com"
	require.NoError(t, appsettings.Save())

	w := postSection(t, "nginx", jsonBody(t, map[string]string{
		"maintenance_template": `C:\maintenance\`,
	}))
	requireUnstorableValueError(t, w, "nginx.MaintenanceTemplate")

	conf := loadWrittenSettings(t, confPath)
	assert.Equal(t, "maintenance.example.com", conf.Section("nginx").Key("MaintenanceHost").String())
}

func TestSaveSettingsRejectsValueThatCannotBeStored(t *testing.T) {
	confPath := useTempSettingsFile(t)
	keepNginxAndTerminalSettings(t)
	appsettings.TerminalSettings.StartCmd = "saved-start-cmd"
	require.NoError(t, appsettings.Save())

	w := postSettings(t, jsonBody(t, map[string]any{
		"nginx": map[string]string{
			"maintenance_host": unstorableValue("[terminal]", "StartCmd = combined-marker"),
		},
	}))
	requireUnstorableValueError(t, w, "nginx.MaintenanceHost")

	written, err := os.ReadFile(confPath)
	require.NoError(t, err)
	assert.NotContains(t, string(written), "combined-marker")
	assert.Equal(t, "saved-start-cmd", appsettings.TerminalSettings.StartCmd)
}

// The nginx control endpoint is allowed to change protected nginx settings,
// but it still must not write any key or section other than its own fields.
func TestSaveNginxControlSettingsRejectsValueThatCannotBeStored(t *testing.T) {
	confPath := useTempSettingsFile(t)
	keepNginxAndTerminalSettings(t)
	appsettings.TerminalSettings.StartCmd = "saved-start-cmd"
	appsettings.NginxSettings.HostMode = ""
	appsettings.NginxSettings.ReloadCmd = "saved-reload-cmd"
	require.NoError(t, appsettings.Save())

	body := jsonBody(t, nginxControlSettingsPayload{
		Mode:               appsettings.ControlModeHostViaSSH,
		HostAddress:        "host.docker.internal:22",
		HostUser:           "nginxui",
		HostAccessMode:     appsettings.HostAccessModeSFTP,
		HostKeySource:      appsettings.HostKeySourceGenerated,
		HostPrivateKeyPath: "/etc/nginx-ui/host_key",
		HostKnownHostsPath: "/etc/nginx-ui/known_hosts",
		HostServiceManager: appsettings.HostServiceManagerSystemd,
		HostSudoPrefix:     unstorableValue("ReloadCmd = control-marker", "[terminal]", "StartCmd = control-marker"),
	})
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set(middleware.SecureSessionVerifiedKey, true)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/settings/nginx/control", bytes.NewBufferString(body))
	c.Request.Header.Set("Content-Type", "application/json")

	SaveNginxControlSettings(c)

	requireUnstorableValueError(t, w, "nginx.HostSudoPrefix")
	written, err := os.ReadFile(confPath)
	require.NoError(t, err)
	assert.NotContains(t, string(written), "control-marker")
	assert.Equal(t, "", appsettings.NginxSettings.HostMode)
	assert.Equal(t, "saved-reload-cmd", appsettings.NginxSettings.ReloadCmd)
	assert.Equal(t, "saved-start-cmd", appsettings.TerminalSettings.StartCmd)
}

// ini.v1 expands %(name)s with another key of the section when the settings
// are mapped, so a stored reference would not read back as stored. Expanding
// to a value that cannot be stored would also block every later save.
func TestSaveSettingsSectionRejectsReferenceToAnotherSetting(t *testing.T) {
	values := map[string]string{
		"expands to a trailing backslash": `C:\x\%(ContainerName)s`,
		"reads a sibling setting":         "%(TestConfigCmd)s",
		"names a key that does not exist": "prefix-%(Missing)s",
	}

	for name, value := range values {
		t.Run(name, func(t *testing.T) {
			confPath := useTempSettingsFile(t)
			keepNginxAndTerminalSettings(t)
			appsettings.NginxSettings.TestConfigCmd = "saved-test-cmd"
			appsettings.NginxSettings.ContainerName = ""
			appsettings.NginxSettings.MaintenanceHost = "maintenance.example.com"
			require.NoError(t, appsettings.Save())
			before, err := os.ReadFile(confPath)
			require.NoError(t, err)

			w := postSection(t, "nginx", jsonBody(t, map[string]string{"maintenance_host": value}))

			requireUnstorableValueError(t, w, "nginx.MaintenanceHost")
			after, err := os.ReadFile(confPath)
			require.NoError(t, err)
			assert.Equal(t, string(before), string(after))
			assert.Equal(t, "maintenance.example.com", appsettings.NginxSettings.MaintenanceHost)

			// An unrelated save afterwards is not blocked.
			w = postSection(t, "openai", `{"model":"gpt-5","token":"new-token"}`)
			require.Equal(t, http.StatusOK, w.Code, w.Body.String())
			assert.Equal(t, "maintenance.example.com",
				loadWrittenSettings(t, confPath).Section("nginx").Key("MaintenanceHost").String())
		})
	}
}

// A value app.ini cannot store may already be in effect without this request
// setting it, from an environment variable or app.ini itself. It must not
// block saving other settings, and app.ini keeps its stored value for it.
func TestSaveSettingsSectionKeepsUnstorableValueItDidNotSet(t *testing.T) {
	confPath := useTempSettingsFile(t)
	keepNginxAndTerminalSettings(t)
	appsettings.NginxSettings.ConfigDir = "/etc/nginx"
	require.NoError(t, appsettings.Save())
	// As an environment override would set it after app.ini was loaded.
	appsettings.NginxSettings.ConfigDir = `C:\nginx\conf\`

	w := postSection(t, "nginx", `{"maintenance_host":"maintenance.example.com"}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	conf := loadWrittenSettings(t, confPath)
	assert.Equal(t, "maintenance.example.com", conf.Section("nginx").Key("MaintenanceHost").String())
	assert.Equal(t, "/etc/nginx", conf.Section("nginx").Key("ConfigDir").String())
	assert.Equal(t, `C:\nginx\conf\`, appsettings.NginxSettings.ConfigDir, "the override stays in effect")

	// A value the request sets is still refused, and named, next to it.
	w = postSection(t, "nginx", jsonBody(t, map[string]string{"maintenance_template": `D:\maintenance\`}))
	requireUnstorableValueError(t, w, "nginx.MaintenanceTemplate")
	assert.Equal(t, `C:\nginx\conf\`, appsettings.NginxSettings.ConfigDir)
}

// TestSaveSettingsSectionKeepsValuesTheFileCanStore guards the other side:
// values with ini metacharacters, multi-line values and list fields stored as
// repeated keys must keep saving and read back unchanged.
func TestSaveSettingsSectionKeepsValuesTheFileCanStore(t *testing.T) {
	values := []string{
		"maintenance#1.html",
		"a;b",
		`"quoted"`,
		`'single'`,
		"`backtick`",
		"a=b=c",
		"ユニコード 维护.html",
		"https://example.com/path?q=1&x=y#frag",
		`C:\nginx\maintenance\page.html`,
		`\\server\share\page.html`,
		"  padded  ",
		"line one\nline two",
		"multi\n`backtick`\nline",
		`a"""b`,
		`"""leading`,
		"ends with quotes\"\"\"",
		"line\r\nbreaks",
		"100%",
		"/maintenance%20page.html?q=%2F",
		"%s and %d",
		"%(unclosed",
		"50%(off)",
		"%()s",
	}

	for _, value := range values {
		t.Run(value, func(t *testing.T) {
			confPath := useTempSettingsFile(t)
			keepNginxAndTerminalSettings(t)

			w := postSection(t, "nginx", jsonBody(t, map[string]string{"maintenance_template": value}))
			require.Equal(t, http.StatusOK, w.Code, w.Body.String())
			assert.Equal(t, value, appsettings.NginxSettings.MaintenanceTemplate)
			first, err := os.ReadFile(confPath)
			require.NoError(t, err)
			assert.Equal(t, value, loadWrittenSettings(t, confPath).Section("nginx").Key("MaintenanceTemplate").String())

			// Saving another section writes the value again from the reloaded
			// file; it must come out byte for byte the same.
			w = postSection(t, "openai", `{"model":"gpt-5","token":"new-token"}`)
			require.Equal(t, http.StatusOK, w.Code, w.Body.String())
			assert.Equal(t, value, appsettings.NginxSettings.MaintenanceTemplate)
			second, err := os.ReadFile(confPath)
			require.NoError(t, err)
			assert.Equal(t,
				loadWrittenSettings(t, confPath).Section("nginx").Key("MaintenanceTemplate").String(), value)
			assert.Equal(t, sectionLines(string(first), "nginx"), sectionLines(string(second), "nginx"))
		})
	}
}

func TestSaveSettingsSectionKeepsListValues(t *testing.T) {
	confPath := useTempSettingsFile(t)
	keepNginxAndTerminalSettings(t)

	sources := []string{"https://plugins.example.com/v1/index.json", "https://mirror.example.com/index.json?a=1;b=2"}
	w := postSection(t, "plugin", jsonBody(t, map[string]any{"marketplace_sources": sources}))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	assert.Equal(t, sources, loadWrittenSettings(t, confPath).Section("plugin").Key("MarketplaceSources").ValueWithShadows())

	w = postSection(t, "nginx", `{"maintenance_template":"maintenance.html"}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, sources, appsettings.PluginSettings.MarketplaceSources)
	assert.Equal(t, sources, loadWrittenSettings(t, confPath).Section("plugin").Key("MarketplaceSources").ValueWithShadows())
}

// sectionLines returns the lines of one section of an ini file as written.
func sectionLines(content, section string) string {
	var out []string
	inSection := false
	for _, line := range strings.Split(content, "\n") {
		if strings.HasPrefix(line, "[") && strings.HasSuffix(strings.TrimSpace(line), "]") {
			inSection = strings.TrimSpace(line) == "["+section+"]"
		}
		if inSection {
			out = append(out, line)
		}
	}
	return strings.Join(out, "\n")
}
