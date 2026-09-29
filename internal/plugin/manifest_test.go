package plugin

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/0xJacky/Nginx-UI/internal/plugin/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// validManifest is the baseline the table tests mutate.
func validManifest() *protocol.Manifest {
	return &protocol.Manifest{
		ID:         "official.cloudflare",
		Name:       "Cloudflare",
		Version:    "1.2.3",
		APIVersion: protocol.APIVersion,
		IconPath:   "assets/icon.svg",
		Server: &protocol.ManifestServer{
			Executables: map[string]string{"linux-amd64": "bin/plugin-linux-amd64"},
			Lifecycle:   protocol.LifecycleOnDemand,
		},
		Capabilities: []string{protocol.CapabilityDNS01},
		Permissions: []string{
			protocol.PermissionKV,
			protocol.PermissionCredentialsReadPrefix + "dns",
		},
		DNS01: &protocol.ManifestDNS01{
			Providers: []protocol.DNS01Provider{
				{Name: "Cloudflare", Code: "cloudflare", Form: &protocol.DNS01ProviderForm{Fields: []protocol.DNS01ProviderField{{Key: "CF_DNS_API_TOKEN", Label: "API token", Group: "credential", Secret: true}}}},
				{Name: "Cloudflare DNS", Code: "cf-dns", Form: &protocol.DNS01ProviderForm{Fields: []protocol.DNS01ProviderField{{Key: "CF_DNS_API_TOKEN", Label: "API token", Group: "credential", Secret: true}}}},
			},
		},
		SettingsSchema: &protocol.SettingsSchema{
			Settings: []protocol.SettingsField{
				{Key: "token", Type: "secret", DisplayName: "API Token"},
				{Key: "zone", Type: "select", DisplayName: "Zone", Options: []protocol.SettingsOption{{Value: "a", Label: "A"}}},
			},
		},
	}
}

func writeManifest(t *testing.T, dir string, m any) {
	t.Helper()
	data, err := json.Marshal(m)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, ManifestFileName), data, 0o644))
}

func TestLoadManifest(t *testing.T) {
	dir := t.TempDir()
	writeManifest(t, dir, validManifest())

	m, err := LoadManifest(dir)
	require.NoError(t, err)
	assert.Equal(t, "official.cloudflare", m.ID)
	assert.Equal(t, protocol.APIVersion, m.APIVersion)
	require.NoError(t, ValidateManifest(m))
	assert.True(t, IsCompatible(m))
}

func TestLoadManifestFailures(t *testing.T) {
	_, err := LoadManifest(t.TempDir())
	assert.ErrorIs(t, err, ErrManifestInvalid)

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, ManifestFileName), []byte("{not json"), 0o644))
	_, err = LoadManifest(dir)
	assert.ErrorIs(t, err, ErrManifestInvalid)
}

func TestIsCompatible(t *testing.T) {
	assert.False(t, IsCompatible(nil))
	assert.True(t, IsCompatible(&protocol.Manifest{APIVersion: protocol.APIVersion}))
	assert.False(t, IsCompatible(&protocol.Manifest{APIVersion: protocol.APIVersion + 1}))
}

func TestValidateManifest(t *testing.T) {
	assert.NoError(t, ValidateManifest(validManifest()))

	tests := []struct {
		name   string
		mutate func(m *protocol.Manifest)
		reason string
	}{
		{"empty id", func(m *protocol.Manifest) { m.ID = "" }, "id is required"},
		{"id without namespace", func(m *protocol.Manifest) { m.ID = "cloudflare" }, "vendor.name"},
		{"id with upper case", func(m *protocol.Manifest) { m.ID = "Official.Cloudflare" }, "vendor.name"},
		{"id too long", func(m *protocol.Manifest) { m.ID = "official." + strings.Repeat("a", 60) }, "at most 64"},
		{"empty name", func(m *protocol.Manifest) { m.Name = "" }, "name is required"},
		{"empty version", func(m *protocol.Manifest) { m.Version = "" }, "version is required"},
		{"version is not semver", func(m *protocol.Manifest) { m.Version = "1.2" }, "semantic version"},
		{"version with v prefix", func(m *protocol.Manifest) { m.Version = "v1.2.3" }, "semantic version"},
		{"missing api version", func(m *protocol.Manifest) { m.APIVersion = 0 }, "api_version is required"},
		{"absolute icon path", func(m *protocol.Manifest) { m.IconPath = "/etc/passwd" }, "relative path"},
		{"unknown i18n locale", func(m *protocol.Manifest) {
			m.I18n = map[string]protocol.ManifestI18n{"zh_CN": {Name: "Cloudflare"}, "klingon": {Name: "Cloudflare"}}
		}, `i18n: "klingon" is not a language of the host`},
		{"i18n locale in another spelling", func(m *protocol.Manifest) {
			m.I18n = map[string]protocol.ManifestI18n{"zh-CN": {Name: "Cloudflare"}}
		}, `i18n: "zh-CN"`},
		{"nothing to contribute", func(m *protocol.Manifest) { m.Server = nil }, "at least one of server"},
		{"unknown lifecycle", func(m *protocol.Manifest) { m.Server.Lifecycle = "forever" }, "server.lifecycle"},
		{"escaping executable", func(m *protocol.Manifest) {
			m.Server.Executables["linux-amd64"] = "../../bin/sh"
		}, "relative path"},
		{"absolute executable", func(m *protocol.Manifest) {
			m.Server.Executables["linux-amd64"] = "/bin/sh"
		}, "relative path"},
		{"no way to start", func(m *protocol.Manifest) { m.Server.Executables = nil }, "executables or command"},
		{"escaping command", func(m *protocol.Manifest) {
			m.Server.Executables = nil
			m.Server.Command = []string{"../python", "main.py"}
		}, "relative path"},
		{"unknown capability", func(m *protocol.Manifest) { m.Capabilities = []string{"telepathy"} }, "unknown capability"},
		{"duplicate capability", func(m *protocol.Manifest) {
			m.Capabilities = []string{protocol.CapabilityDNS01, protocol.CapabilityDNS01}
		}, "declared twice"},
		{"dns01 without providers", func(m *protocol.Manifest) { m.DNS01 = nil }, "at least one provider"},
		{"provider without name", func(m *protocol.Manifest) { m.DNS01.Providers[0].Name = "" }, "name is required"},
		{"provider code too short", func(m *protocol.Manifest) { m.DNS01.Providers[0].Code = "c" }, "provider code"},
		{"provider code with upper case", func(m *protocol.Manifest) { m.DNS01.Providers[0].Code = "CloudFlare" }, "provider code"},
		{"duplicate provider code", func(m *protocol.Manifest) { m.DNS01.Providers[1].Code = "cloudflare" }, "declared twice"},
		{"http without block", func(m *protocol.Manifest) {
			m.Capabilities = []string{protocol.CapabilityHTTP}
			m.DNS01 = nil
		}, "requires the http block"},
		{"unknown http listen", func(m *protocol.Manifest) {
			m.Capabilities = []string{protocol.CapabilityHTTP}
			m.DNS01 = nil
			m.HTTP = &protocol.ManifestHTTP{Listen: "tcp"}
		}, "http.listen"},
		{"unknown permission", func(m *protocol.Manifest) { m.Permissions = []string{"root"} }, "unknown permission"},
		{"credentials permission without kind", func(m *protocol.Manifest) {
			m.Permissions = []string{protocol.PermissionCredentialsReadPrefix}
		}, "unknown permission"},
		{"webapp page escapes", func(m *protocol.Manifest) {
			m.Webapp = &protocol.ManifestWebapp{Pages: []protocol.ManifestPage{{Path: "/a", File: "../x.html"}}}
		}, "relative path"},
		{"content locales escape", func(m *protocol.Manifest) {
			m.Content = &protocol.ManifestContent{Locales: "a/../../b"}
		}, "relative path"},
		{"settings key missing", func(m *protocol.Manifest) { m.SettingsSchema.Settings[0].Key = "" }, "key is required"},
		{"duplicate settings key", func(m *protocol.Manifest) { m.SettingsSchema.Settings[1].Key = "token" }, "declared twice"},
		{"unknown settings type", func(m *protocol.Manifest) { m.SettingsSchema.Settings[0].Type = "color" }, "unknown type"},
		{"select without options", func(m *protocol.Manifest) { m.SettingsSchema.Settings[1].Options = nil }, "needs options"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := validManifest()
			tc.mutate(m)
			err := ValidateManifest(m)
			require.Error(t, err)
			assert.ErrorIs(t, err, ErrManifestInvalid)
			assert.Contains(t, err.Error(), tc.reason)
		})
	}
}

func TestValidateManifestAcceptsI18n(t *testing.T) {
	m := validManifest()
	m.I18n = map[string]protocol.ManifestI18n{
		"en":    {Name: "Cloudflare"},
		"zh_CN": {Name: "Cloudflare 验证", Description: "通过 Cloudflare 完成验证"},
		"ja_JP": {Description: "Cloudflare で検証する"},
		"zh_TW": {},
	}
	assert.NoError(t, ValidateManifest(m))
}

func TestI18nMaps(t *testing.T) {
	names, descriptions := i18nMaps(validManifest())
	assert.Nil(t, names)
	assert.Nil(t, descriptions)

	m := validManifest()
	m.I18n = map[string]protocol.ManifestI18n{
		"zh_CN": {Name: "名称", Description: "描述"},
		"ja_JP": {Description: "説明"},
		"zh_TW": {},
	}
	names, descriptions = i18nMaps(m)
	assert.Equal(t, map[string]string{"zh_CN": "名称"}, names)
	assert.Equal(t, map[string]string{"zh_CN": "描述", "ja_JP": "説明"}, descriptions)

	names, descriptions = i18nMaps(nil)
	assert.Nil(t, names)
	assert.Nil(t, descriptions)
}

func TestValidateManifestAcceptsHTTPAndWebapp(t *testing.T) {
	m := validManifest()
	m.Capabilities = []string{protocol.CapabilityDNS01, protocol.CapabilityHTTP}
	m.HTTP = &protocol.ManifestHTTP{Listen: "unix"}
	m.Webapp = &protocol.ManifestWebapp{
		BundlePath: "dist/index.js",
		Pages:      []protocol.ManifestPage{{Path: "/hello", File: "pages/hello.html"}},
	}
	m.Content = &protocol.ManifestContent{Templates: "templates", Locales: "locales"}
	assert.NoError(t, ValidateManifest(m))

	m.HTTP.Listen = "rpc"
	assert.NoError(t, ValidateManifest(m))
}

func TestResolveExecutable(t *testing.T) {
	dir := t.TempDir()
	platform := runtime.GOOS + "-" + runtime.GOARCH

	m := validManifest()
	m.Server.Executables = map[string]string{platform: "bin/plugin"}
	argv, err := ResolveExecutable(m, dir)
	require.NoError(t, err)
	require.Len(t, argv, 1)
	assert.Equal(t, filepath.Join(dir, "bin", "plugin"), argv[0])
	assert.True(t, filepath.IsAbs(argv[0]))

	// No binary for this platform, the interpreted command takes over.
	m.Server.Executables = map[string]string{"plan9-386": "bin/plugin"}
	m.Server.Command = []string{"python3", "main.py"}
	argv, err = ResolveExecutable(m, dir)
	require.NoError(t, err)
	assert.Equal(t, []string{"python3", "main.py"}, argv)

	// A command with a separator is resolved inside the plugin directory.
	m.Server.Command = []string{"venv/bin/python", "main.py"}
	argv, err = ResolveExecutable(m, dir)
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(dir, "venv", "bin", "python"), argv[0])
	assert.Equal(t, "main.py", argv[1])

	m.Server.Command = nil
	_, err = ResolveExecutable(m, dir)
	assert.ErrorIs(t, err, ErrNoExecutableForPlatform)

	_, err = ResolveExecutable(&protocol.Manifest{}, dir)
	assert.ErrorIs(t, err, ErrNoExecutableForPlatform)

	_, err = ResolveExecutable(nil, dir)
	assert.ErrorIs(t, err, ErrNoExecutableForPlatform)
}

func TestPermissionsHash(t *testing.T) {
	first := &protocol.Manifest{Permissions: []string{"kv", "cron", "credentials.read:dns"}}
	second := &protocol.Manifest{Permissions: []string{"credentials.read:dns", "kv", "cron"}}
	assert.Equal(t, PermissionsHash(first), PermissionsHash(second), "order must not matter")

	more := &protocol.Manifest{Permissions: []string{"kv", "cron", "credentials.read:dns", "notify"}}
	assert.NotEqual(t, PermissionsHash(first), PermissionsHash(more))

	// An empty set still hashes, which keeps the stored value comparable.
	empty := PermissionsHash(&protocol.Manifest{})
	assert.Len(t, empty, 64)
	assert.Equal(t, empty, PermissionsHash(nil))
}

func TestIsSafeRelPath(t *testing.T) {
	for _, good := range []string{"plugin.json", "bin/plugin", "a/b/c.txt"} {
		assert.True(t, isSafeRelPath(good), good)
	}
	for _, bad := range []string{"", ".", "..", "../x", "a/../../b", "/abs", "./a", "a//b", "a/", `bin\plugin`, "C:/x"} {
		assert.False(t, isSafeRelPath(bad), bad)
	}
}
