package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"aead.dev/minisign"
	"github.com/0xJacky/Nginx-UI/internal/plugin/protocol"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uozi-tech/cosy"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// testPluginBinEnv points the launcher script at the test binary, which
// TestMain turns into a JSON-RPC peer.
const testPluginBinEnv = "PLUGIN_TEST_BIN"

// testLauncherName is the executable every generated test plugin ships.
const testLauncherName = "run.sh"

// testLauncher re-execs the test binary so a plugin directory needs no second
// compiled artifact.
const testLauncher = "#!/bin/sh\nexec \"$" + testPluginBinEnv + "\"\n"

// setupPluginTestDB wires the query package to a private in-memory database.
func setupPluginTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Plugin{}, &model.PluginKV{}, &model.DnsCredential{}))
	model.Use(db)
	query.Init(db)
	t.Cleanup(func() { model.Use(nil) })
	return db
}

// usePluginProcesses points the generated plugins at the test binary. Only the
// tests that actually spawn a plugin need it.
// testHandshakeTimeout replaces the ten second default for the fake plugin,
// which is this race instrumented test binary and can take that long to start
// while the whole suite runs.
const testHandshakeTimeout = 60 * time.Second

func usePluginProcesses(t *testing.T, mode string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the test plugin launcher is a shell script")
	}
	executable, err := os.Executable()
	require.NoError(t, err)
	t.Setenv(testPluginBinEnv, executable)
	t.Setenv(testPluginModeEnv, mode)
}

// newTestManager returns a manager rooted at a fresh directory, wired to a
// fresh database. The test packages are unsigned, so developer mode is on.
func newTestManager(t *testing.T) *Manager {
	t.Helper()
	setupPluginTestDB(t)
	useDeveloperMode(t, true)
	m := newManager(t.TempDir())
	m.handshakeTimeout = testHandshakeTimeout
	t.Cleanup(func() { m.Stop(context.Background()) })
	return m
}

// testManifest2 builds a manifest for a plugin that speaks the test protocol.
func pluginManifest(id string) *protocol.Manifest {
	return &protocol.Manifest{
		ID:         id,
		Name:       id,
		Version:    "1.0.0",
		APIVersion: protocol.APIVersion,
		Server: &protocol.ManifestServer{
			Executables: map[string]string{runtime.GOOS + "-" + runtime.GOARCH: testLauncherName},
		},
		Capabilities: []string{protocol.CapabilityDNS01},
		DNS01: &protocol.ManifestDNS01{
			Providers: []protocol.DNS01Provider{{Name: "Test", Code: "test", Form: &protocol.DNS01ProviderForm{Fields: []protocol.DNS01ProviderField{{Key: "TEST_TOKEN", Label: "API token", Group: "credential"}}}}},
		},
	}
}

// writePluginDir materialises a plugin directory: the manifest plus the
// launcher the manifest points at.
func writePluginDir(t *testing.T, dir string, manifest *protocol.Manifest) string {
	t.Helper()
	require.NoError(t, os.MkdirAll(dir, 0o755))

	encoded, err := json.MarshalIndent(manifest, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, ManifestFileName), encoded, 0o644))

	if manifest.Server != nil {
		require.NoError(t, os.WriteFile(filepath.Join(dir, testLauncherName), []byte(testLauncher), 0o755))
	}
	return dir
}

// useDeveloperMode switches developer mode for one test.
func useDeveloperMode(t *testing.T, enabled bool) {
	t.Helper()
	previous := settings.PluginSettings.DeveloperMode
	settings.PluginSettings.DeveloperMode = enabled
	t.Cleanup(func() { settings.PluginSettings.DeveloperMode = previous })
}

// buildTestPackage writes a plugin into a staging directory and packs it.
func buildTestPackage(t *testing.T, manifest *protocol.Manifest, extra map[string]string) string {
	t.Helper()
	return buildSignedTestPackage(t, manifest, extra, nil)
}

// buildSignedTestPackage is buildTestPackage with an embedded signature when
// signer is set.
func buildSignedTestPackage(t *testing.T, manifest *protocol.Manifest, extra map[string]string,
	signer *minisign.PrivateKey,
) string {
	t.Helper()
	staging := filepath.Join(t.TempDir(), manifest.ID)
	writePluginDir(t, staging, manifest)
	for name, content := range extra {
		target := filepath.Join(staging, filepath.FromSlash(name))
		require.NoError(t, os.MkdirAll(filepath.Dir(target), 0o755))
		require.NoError(t, os.WriteFile(target, []byte(content), 0o644))
	}

	archive := filepath.Join(t.TempDir(), manifest.ID+".tar.gz")
	if signer != nil {
		require.NoError(t, BuildSignedPackage(staging, archive, *signer))
		return archive
	}
	require.NoError(t, BuildPackage(staging, archive))
	return archive
}

// assertPluginError compares the cosy error code, which is what survives
// cosy.WrapErrorWithParams: the wrapper copies the code instead of chaining.
func assertPluginError(t *testing.T, err error, expected error) {
	t.Helper()
	var actual, want *cosy.Error
	require.True(t, errors.As(err, &actual), "expected a cosy error, got %v", err)
	require.True(t, errors.As(expected, &want))
	assert.Equal(t, want.Code, actual.Code, "unexpected error %v", err)
}

// supervisorOf reads the supervisor of a plugin under the manager lock, which
// keeps the race detector happy in the tests that inspect plugin logs.
func supervisorOf(m *Manager, id string) *Supervisor {
	m.mu.RLock()
	defer m.mu.RUnlock()
	item, ok := m.entries[id]
	if !ok {
		return nil
	}
	return item.supervisor
}

func infoOfID(infos []Info, id string) (Info, bool) {
	for _, info := range infos {
		if info.ID == id {
			return info, true
		}
	}
	return Info{}, false
}

func TestManagerDiscoveryAlignsTheDatabase(t *testing.T) {
	m := newTestManager(t)
	ctx := context.Background()

	writePluginDir(t, filepath.Join(m.Dir(), "official.alpha"), pluginManifest("official.alpha"))

	future := pluginManifest("official.beta")
	future.APIVersion = protocol.APIVersion + 42
	writePluginDir(t, filepath.Join(m.Dir(), "official.beta"), future)

	// A row whose directory is gone must survive as "missing".
	require.NoError(t, query.Plugin.WithContext(ctx).Create(&model.Plugin{
		PluginID: "official.gone", Version: "0.9.0", Enabled: true,
	}))

	require.NoError(t, m.LoadOffline(ctx))
	infos := m.List()
	require.Len(t, infos, 3)

	alpha, ok := infoOfID(infos, "official.alpha")
	require.True(t, ok)
	assert.Equal(t, StatusInstalled, alpha.Status)
	assert.False(t, alpha.Enabled)
	assert.Equal(t, "1.0.0", alpha.Version)
	assert.Equal(t, model.PluginSyncPolicyManual, alpha.SyncPolicy)

	beta, ok := infoOfID(infos, "official.beta")
	require.True(t, ok)
	assert.Equal(t, StatusIncompatible, beta.Status)

	gone, ok := infoOfID(infos, "official.gone")
	require.True(t, ok)
	assert.Equal(t, StatusMissing, gone.Status)

	// The new plugin was persisted, the missing one was not deleted.
	rows, err := query.Plugin.WithContext(ctx).Find()
	require.NoError(t, err)
	assert.Len(t, rows, 3)
}

func TestInfoAndInspectCarryTranslations(t *testing.T) {
	m := newTestManager(t)
	ctx := context.Background()

	manifest := pluginManifest("official.alpha")
	manifest.Description = "Alpha plugin"
	manifest.I18n = map[string]protocol.ManifestI18n{
		"zh_CN": {Name: "阿尔法", Description: "阿尔法插件"},
		"ja_JP": {Name: "アルファ"},
		"zh_TW": {},
	}
	writePluginDir(t, filepath.Join(m.Dir(), "official.alpha"), manifest)
	writePluginDir(t, filepath.Join(m.Dir(), "official.beta"), pluginManifest("official.beta"))
	require.NoError(t, m.LoadOffline(ctx))

	infos := m.List()
	alpha, ok := infoOfID(infos, "official.alpha")
	require.True(t, ok)
	assert.Equal(t, "official.alpha", alpha.Name)
	assert.Equal(t, map[string]string{"zh_CN": "阿尔法", "ja_JP": "アルファ"}, alpha.NameI18n)
	assert.Equal(t, map[string]string{"zh_CN": "阿尔法插件"}, alpha.DescriptionI18n)

	// Without translations the fields stay out of the JSON.
	beta, ok := infoOfID(infos, "official.beta")
	require.True(t, ok)
	encoded, err := json.Marshal(beta)
	require.NoError(t, err)
	assert.NotContains(t, string(encoded), "name_i18n")
	assert.NotContains(t, string(encoded), "description_i18n")

	result, err := m.Inspect(buildTestPackage(t, manifest, nil))
	require.NoError(t, err)
	assert.Equal(t, alpha.NameI18n, result.NameI18n)
	assert.Equal(t, alpha.DescriptionI18n, result.DescriptionI18n)
	assert.Equal(t, manifest.I18n, result.Manifest.I18n)
}

func TestManagerInstallEnableDisableUninstall(t *testing.T) {
	usePluginProcesses(t, pluginModeNormal)
	m := newTestManager(t)
	ctx := context.Background()
	m.Start(ctx)

	archive := buildTestPackage(t, pluginManifest("official.alpha"), nil)
	info, err := m.Install(ctx, archive, InstallOptions{Enable: true})
	require.NoError(t, err)
	assert.Equal(t, StatusRunning, info.Status)
	assert.True(t, info.Enabled)
	assert.DirExists(t, filepath.Join(m.Dir(), "official.alpha"))
	assert.DirExists(t, m.DataDir("official.alpha"))

	// The running plugin answers capability calls through the manager.
	client, release, err := m.Acquire(ctx, "official.alpha")
	require.NoError(t, err)
	var echo struct {
		Text string `json:"text"`
	}
	require.NoError(t, client.Call(ctx, "echo.hello", map[string]string{"text": "hi"}, &echo))
	assert.Equal(t, "HI", echo.Text)
	release()

	assert.Equal(t, []string{"official.alpha"}, m.EnabledWithCapability(protocol.CapabilityDNS01))
	owner, ok := m.OwnerOf(protocol.CapabilityDNS01, "test")
	assert.True(t, ok)
	assert.Equal(t, "official.alpha", owner)
	require.Len(t, m.DNS01Providers(), 1)

	disabled, err := m.Disable(ctx, "official.alpha")
	require.NoError(t, err)
	assert.Equal(t, StatusInstalled, disabled.Status)
	assert.False(t, disabled.Enabled)
	assert.Empty(t, m.EnabledWithCapability(protocol.CapabilityDNS01))

	enabled, err := m.Enable(ctx, "official.alpha", false)
	require.NoError(t, err)
	assert.Equal(t, StatusRunning, enabled.Status)

	require.NoError(t, m.hostBackend().KVSet("official.alpha", "state", json.RawMessage(`1`)))
	require.NoError(t, m.Uninstall(ctx, "official.alpha", false))
	assert.NoDirExists(t, filepath.Join(m.Dir(), "official.alpha"))
	assert.NoDirExists(t, m.DataDir("official.alpha"))
	_, err = m.Get("official.alpha")
	assert.ErrorIs(t, err, ErrPluginNotFound)

	// The rows are gone for good, not soft deleted, so the unique indexes on
	// the plugin id are free for a reinstall.
	rows, err := query.Plugin.WithContext(ctx).Unscoped().Find()
	require.NoError(t, err)
	assert.Empty(t, rows)
	kvRows, err := query.PluginKV.WithContext(ctx).Unscoped().Find()
	require.NoError(t, err)
	assert.Empty(t, kvRows)

	info, err = m.Install(ctx, archive, InstallOptions{Enable: true})
	require.NoError(t, err)
	assert.Equal(t, StatusRunning, info.Status)
	require.NoError(t, m.hostBackend().KVSet("official.alpha", "state", json.RawMessage(`2`)))
}

func TestManagerFailedUpgradeRestartsThePreviousVersion(t *testing.T) {
	usePluginProcesses(t, pluginModeNormal)
	m := newTestManager(t)
	ctx := context.Background()
	m.Start(ctx)

	_, err := m.Install(ctx, buildTestPackage(t, pluginManifest("official.alpha"), nil), InstallOptions{Enable: true})
	require.NoError(t, err)

	// The row update fails once the new files are in place.
	db := query.Plugin.WithContext(ctx).UnderlyingDB()
	require.NoError(t, db.Migrator().DropTable(&model.Plugin{}))

	second := pluginManifest("official.alpha")
	second.Version = "2.0.0"
	_, err = m.Install(ctx, buildTestPackage(t, second, nil), InstallOptions{Enable: true})
	require.Error(t, err)

	// The previous version is back, running and answering calls, with its
	// event queue restored.
	info, err := m.Get("official.alpha")
	require.NoError(t, err)
	assert.Equal(t, "1.0.0", info.Version)
	assert.Equal(t, StatusRunning, info.Status)

	client, release, err := m.Acquire(ctx, "official.alpha")
	require.NoError(t, err)
	defer release()
	require.NoError(t, client.Call(ctx, "echo.hello", map[string]string{"text": "hi"}, nil))

	item, ok := m.lookup("official.alpha")
	require.True(t, ok)
	m.mu.RLock()
	assert.NotNil(t, item.events)
	m.mu.RUnlock()
}

func TestManagerUpgradeKeepsSettingsAndRollsBack(t *testing.T) {
	m := newTestManager(t)
	ctx := context.Background()
	require.NoError(t, m.LoadOffline(ctx))

	first := pluginManifest("official.alpha")
	first.SettingsSchema = &protocol.SettingsSchema{
		Settings: []protocol.SettingsField{{Key: "token", Type: "text", DisplayName: "Token"}},
	}
	_, err := m.Install(ctx, buildTestPackage(t, first, nil), InstallOptions{Enable: true})
	require.NoError(t, err)
	require.NoError(t, m.SaveSettings(ctx, "official.alpha", map[string]any{"token": "kept"}))

	second := pluginManifest("official.alpha")
	second.Version = "2.0.0"
	second.SettingsSchema = first.SettingsSchema
	info, err := m.Install(ctx, buildTestPackage(t, second, nil), InstallOptions{Enable: true})
	require.NoError(t, err)
	assert.Equal(t, "2.0.0", info.Version)
	assert.NoDirExists(t, filepath.Join(m.Dir(), "official.alpha"+backupSuffix))

	_, values, err := m.Settings("official.alpha")
	require.NoError(t, err)
	assert.Equal(t, "kept", values["token"])

	// A failure after the files moved has to restore the previous version.
	db := query.Plugin.WithContext(ctx).UnderlyingDB()
	require.NoError(t, db.Migrator().DropTable(&model.Plugin{}))

	third := pluginManifest("official.alpha")
	third.Version = "3.0.0"
	_, err = m.Install(ctx, buildTestPackage(t, third, nil), InstallOptions{Enable: true})
	require.Error(t, err)
	assert.NoDirExists(t, filepath.Join(m.Dir(), "official.alpha"+backupSuffix))

	restored, err := LoadManifest(filepath.Join(m.Dir(), "official.alpha"))
	require.NoError(t, err)
	assert.Equal(t, "2.0.0", restored.Version)
}

func TestMoveIntoPlaceCommitsAndRollsBack(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "official.alpha")
	require.NoError(t, os.MkdirAll(target, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(target, "marker"), []byte("old"), 0o644))

	staged := filepath.Join(root, "staged")
	require.NoError(t, os.MkdirAll(staged, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(staged, "marker"), []byte("new"), 0o644))

	finish, err := moveIntoPlace(staged, target)
	require.NoError(t, err)
	finish(false)

	content, err := os.ReadFile(filepath.Join(target, "marker"))
	require.NoError(t, err)
	assert.Equal(t, "old", string(content))
	assert.NoDirExists(t, target+backupSuffix)

	require.NoError(t, os.MkdirAll(staged, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(staged, "marker"), []byte("new"), 0o644))
	finish, err = moveIntoPlace(staged, target)
	require.NoError(t, err)
	finish(true)

	content, err = os.ReadFile(filepath.Join(target, "marker"))
	require.NoError(t, err)
	assert.Equal(t, "new", string(content))
	assert.NoDirExists(t, target+backupSuffix)
}

func TestManagerPermissionApprovalGating(t *testing.T) {
	m := newTestManager(t)
	ctx := context.Background()
	require.NoError(t, m.LoadOffline(ctx))

	first := pluginManifest("official.alpha")
	first.Permissions = []string{protocol.PermissionKV}
	info, err := m.Install(ctx, buildTestPackage(t, first, nil), InstallOptions{Enable: true})
	require.NoError(t, err)
	// The manager runs offline here, so an approved plugin is enabled but the
	// process itself is not spawned.
	assert.Equal(t, StatusStopped, info.Status)
	assert.True(t, info.Enabled)

	// An upgrade that asks for more stays down until the user approves it.
	second := pluginManifest("official.alpha")
	second.Version = "2.0.0"
	second.Permissions = []string{protocol.PermissionKV, protocol.PermissionNotify}
	info, err = m.Install(ctx, buildTestPackage(t, second, nil), InstallOptions{Enable: true})
	require.NoError(t, err)
	assert.Equal(t, StatusNeedsApproval, info.Status)
	assert.False(t, info.Enabled)

	_, err = m.Enable(ctx, "official.alpha", false)
	assert.ErrorIs(t, err, ErrPermissionApprovalRequired)

	info, err = m.Enable(ctx, "official.alpha", true)
	require.NoError(t, err)
	assert.Equal(t, StatusStopped, info.Status)
	assert.True(t, info.Enabled)

	// The same permission set upgrades without asking again.
	third := pluginManifest("official.alpha")
	third.Version = "3.0.0"
	third.Permissions = second.Permissions
	info, err = m.Install(ctx, buildTestPackage(t, third, nil), InstallOptions{Enable: true})
	require.NoError(t, err)
	assert.Equal(t, StatusStopped, info.Status)
	assert.True(t, info.Enabled)

	// Dropping a permission asks for nothing new.
	fourth := pluginManifest("official.alpha")
	fourth.Version = "4.0.0"
	fourth.Permissions = []string{protocol.PermissionKV}
	info, err = m.Install(ctx, buildTestPackage(t, fourth, nil), InstallOptions{Enable: true})
	require.NoError(t, err)
	assert.Equal(t, StatusStopped, info.Status)
	assert.True(t, info.Enabled)

	row, err := query.Plugin.Where(query.Plugin.PluginID.Eq("official.alpha")).First()
	require.NoError(t, err)
	assert.Equal(t, []string{protocol.PermissionKV}, row.ApprovedPermissions)

	// The dropped permission is forgotten, so asking for it again needs a
	// new approval.
	fifth := pluginManifest("official.alpha")
	fifth.Version = "5.0.0"
	fifth.Permissions = second.Permissions
	info, err = m.Install(ctx, buildTestPackage(t, fifth, nil), InstallOptions{Enable: true})
	require.NoError(t, err)
	assert.Equal(t, StatusNeedsApproval, info.Status)
}

func TestManagerApprovalOfPluginWithoutPermissions(t *testing.T) {
	m := newTestManager(t)
	ctx := context.Background()
	require.NoError(t, m.LoadOffline(ctx))

	_, err := m.Install(ctx, buildTestPackage(t, pluginManifest("official.beta"), nil), InstallOptions{Enable: true})
	require.NoError(t, err)

	// An empty approved set survives the database, so the first permission
	// a later version asks for still needs an approval.
	row, err := query.Plugin.Where(query.Plugin.PluginID.Eq("official.beta")).First()
	require.NoError(t, err)
	assert.NotNil(t, row.ApprovedPermissions)
	assert.Empty(t, row.ApprovedPermissions)

	next := pluginManifest("official.beta")
	next.Version = "2.0.0"
	next.Permissions = []string{protocol.PermissionKV}
	info, err := m.Install(ctx, buildTestPackage(t, next, nil), InstallOptions{Enable: true})
	require.NoError(t, err)
	assert.Equal(t, StatusNeedsApproval, info.Status)
}

func TestManagerDependencyRules(t *testing.T) {
	m := newTestManager(t)
	ctx := context.Background()
	require.NoError(t, m.LoadOffline(ctx))

	dependent := pluginManifest("official.beta")
	dependent.Requires = []protocol.ManifestRequirement{{ID: "official.alpha", Version: "^1.0.0"}}
	_, err := m.Install(ctx, buildTestPackage(t, dependent, nil), InstallOptions{Enable: true})
	assertPluginError(t, err, ErrDependencyMissing)

	_, err = m.Install(ctx, buildTestPackage(t, pluginManifest("official.alpha"), nil), InstallOptions{Enable: true})
	require.NoError(t, err)
	_, err = m.Install(ctx, buildTestPackage(t, dependent, nil), InstallOptions{Enable: true})
	require.NoError(t, err)

	order, err := m.startOrder()
	require.NoError(t, err)
	assert.Equal(t, []string{"official.alpha", "official.beta"}, order)

	assertPluginError(t, m.Uninstall(ctx, "official.alpha", false), ErrPluginInUse)

	// Disabling a dependency takes its dependents down with it.
	_, err = m.Disable(ctx, "official.alpha")
	require.NoError(t, err)
	beta, err := m.Get("official.beta")
	require.NoError(t, err)
	assert.Contains(t, beta.LastError, "official.alpha")

	require.NoError(t, m.Uninstall(ctx, "official.alpha", true))
	_, err = m.Get("official.beta")
	assert.ErrorIs(t, err, ErrPluginNotFound)
}

func TestTopoSortDetectsCycles(t *testing.T) {
	ordered, err := topoSort([]string{"a", "b", "c"}, map[string][]string{"b": {"a"}, "c": {"b"}})
	require.NoError(t, err)
	assert.Equal(t, []string{"a", "b", "c"}, ordered)

	_, err = topoSort([]string{"a", "b"}, map[string][]string{"a": {"b"}, "b": {"a"}})
	assert.ErrorIs(t, err, ErrDependencyCycle)
}

func TestManagerSettingsRedactionAndConfigurePush(t *testing.T) {
	usePluginProcesses(t, pluginModeNormal)
	m := newTestManager(t)
	ctx := context.Background()
	m.Start(ctx)

	manifest := pluginManifest("official.alpha")
	manifest.SettingsSchema = &protocol.SettingsSchema{
		Settings: []protocol.SettingsField{
			{Key: "endpoint", Type: "text", DisplayName: "Endpoint", Default: "https://example.com"},
			{Key: "token", Type: "secret", DisplayName: "Token"},
			{Key: "retries", Type: "number", DisplayName: "Retries"},
		},
	}
	_, err := m.Install(ctx, buildTestPackage(t, manifest, nil), InstallOptions{Enable: true})
	require.NoError(t, err)

	schema, values, err := m.Settings("official.alpha")
	require.NoError(t, err)
	require.NotNil(t, schema)
	assert.Equal(t, "https://example.com", values["endpoint"])

	require.NoError(t, m.SaveSettings(ctx, "official.alpha", map[string]any{
		"endpoint": "https://plugin.example",
		"token":    "s3cret",
		"retries":  float64(3),
	}))

	_, values, err = m.Settings("official.alpha")
	require.NoError(t, err)
	assert.Equal(t, "https://plugin.example", values["endpoint"])
	assert.Equal(t, settings.RedactedSensitiveValue, values["token"])
	assert.InDelta(t, 3, values["retries"], 0.001)

	// Echoing the placeholder back keeps the stored secret.
	require.NoError(t, m.SaveSettings(ctx, "official.alpha", map[string]any{
		"endpoint": "https://plugin.example",
		"token":    settings.RedactedSensitiveValue,
	}))
	stored, err := query.Plugin.WithContext(ctx).Where(query.Plugin.PluginID.Eq("official.alpha")).First()
	require.NoError(t, err)
	assert.Equal(t, "s3cret", stored.Settings["token"])

	// A wrong type is refused instead of reaching the plugin.
	assertPluginError(t, m.SaveSettings(ctx, "official.alpha", map[string]any{"retries": "many"}), ErrSettingsInvalid)

	// The running plugin was reconfigured.
	assert.Eventually(t, func() bool {
		return logsContain(supervisorOf(m, "official.alpha"), "plugin configured")
	}, 10*time.Second, 20*time.Millisecond, "plugin.configure never reached the plugin")
}

func TestHostBackendKeyValueRoundTrip(t *testing.T) {
	m := newTestManager(t)
	backend := m.hostBackend()

	require.NoError(t, backend.KVSet("official.alpha", "a/one", json.RawMessage(`{"n":1}`)))
	require.NoError(t, backend.KVSet("official.alpha", "a/two", json.RawMessage(`2`)))
	require.NoError(t, backend.KVSet("official.beta", "a/one", json.RawMessage(`"other"`)))

	value, found, err := backend.KVGet("official.alpha", "a/one")
	require.NoError(t, err)
	require.True(t, found)
	assert.JSONEq(t, `{"n":1}`, string(value))

	// Overwriting replaces the value instead of adding a second row.
	require.NoError(t, backend.KVSet("official.alpha", "a/one", json.RawMessage(`{"n":2}`)))
	value, found, err = backend.KVGet("official.alpha", "a/one")
	require.NoError(t, err)
	require.True(t, found)
	assert.JSONEq(t, `{"n":2}`, string(value))

	// Prefixes are literal and case sensitive whatever the database makes
	// of LIKE: pattern characters match themselves and "A/" is not "a/".
	require.NoError(t, backend.KVSet("official.alpha", "a_1", json.RawMessage(`1`)))
	require.NoError(t, backend.KVSet("official.alpha", "ab1", json.RawMessage(`1`)))
	require.NoError(t, backend.KVSet("official.alpha", "a%1", json.RawMessage(`1`)))
	require.NoError(t, backend.KVSet("official.alpha", "A/upper", json.RawMessage(`1`)))

	keys, err := backend.KVList("official.alpha", "a/")
	require.NoError(t, err)
	assert.Equal(t, []string{"a/one", "a/two"}, keys)
	keys, err = backend.KVList("official.alpha", "a_")
	require.NoError(t, err)
	assert.Equal(t, []string{"a_1"}, keys)
	keys, err = backend.KVList("official.alpha", "a%")
	require.NoError(t, err)
	assert.Equal(t, []string{"a%1"}, keys)

	require.NoError(t, backend.KVDelete("official.alpha", "a/one"))
	_, found, err = backend.KVGet("official.alpha", "a/one")
	require.NoError(t, err)
	assert.False(t, found)

	// The store is private per plugin.
	value, found, err = backend.KVGet("official.beta", "a/one")
	require.NoError(t, err)
	require.True(t, found)
	assert.JSONEq(t, `"other"`, string(value))
}

func TestWebappEntriesOnlyListWhatCanBeServed(t *testing.T) {
	m := newTestManager(t)
	ctx := context.Background()
	require.NoError(t, m.LoadOffline(ctx))

	withBundle := pluginManifest("official.alpha")
	withBundle.Server = nil
	withBundle.Webapp = &protocol.ManifestWebapp{
		BundlePath: "webapp/main.js",
		StylePath:  "webapp/main.css",
		Shared:     map[string]string{"vue": "^3.5.0"},
	}
	withBundle.Capabilities = nil
	withBundle.DNS01 = nil
	withBundle.IconPath = "webapp/icon.svg"
	_, err := m.Install(ctx, buildTestPackage(t, withBundle, map[string]string{
		"webapp/main.js":  "export default {}",
		"webapp/main.css": ".a{}",
		"webapp/icon.svg": "<svg/>",
	}), InstallOptions{Enable: true})
	require.NoError(t, err)

	// A manifest that promises a bundle it does not ship is skipped.
	broken := pluginManifest("official.beta")
	broken.Server = nil
	broken.Capabilities = nil
	broken.DNS01 = nil
	broken.Webapp = &protocol.ManifestWebapp{BundlePath: "webapp/main.js"}
	_, err = m.Install(ctx, buildTestPackage(t, broken, nil), InstallOptions{Enable: true})
	require.NoError(t, err)

	// Pages alone are enough to be listed.
	pagesOnly := pluginManifest("official.gamma")
	pagesOnly.Server = nil
	pagesOnly.Capabilities = nil
	pagesOnly.DNS01 = nil
	pagesOnly.Webapp = &protocol.ManifestWebapp{
		Pages: []protocol.ManifestPage{{Path: "overview", Title: map[string]string{"en": "Overview"}, File: "pages/overview.html"}},
	}
	_, err = m.Install(ctx, buildTestPackage(t, pagesOnly, map[string]string{
		"pages/overview.html": "<html></html>",
	}), InstallOptions{Enable: false})
	require.NoError(t, err)

	entries := m.WebappEntries()
	require.Len(t, entries, 1)
	assert.Equal(t, "official.alpha", entries[0].ID)
	// Relative to the application root, so a sub path deployment works.
	assert.Equal(t, "plugins/official.alpha/webapp/main.js", entries[0].BundleURL)
	assert.Equal(t, "plugins/official.alpha/webapp/main.css", entries[0].StyleURL)
	assert.Equal(t, map[string]string{"vue": "^3.5.0"}, entries[0].Shared)

	alpha, err := m.Get("official.alpha")
	require.NoError(t, err)
	assert.Equal(t, "plugins/official.alpha/webapp/icon.svg", alpha.IconURL)

	// A disabled plugin is never served, enabling it makes the pages appear.
	_, err = m.Enable(ctx, "official.gamma", false)
	require.NoError(t, err)
	entries = m.WebappEntries()
	require.Len(t, entries, 2)

	root, err := m.StaticRoot("official.gamma", "pages")
	require.NoError(t, err)
	assert.FileExists(t, filepath.Join(root, "overview.html"))

	_, err = m.StaticRoot("official.beta", "webapp")
	require.NoError(t, err)
	_, err = m.StaticRoot("official.unknown", "webapp")
	assert.ErrorIs(t, err, ErrPluginNotFound)

	// A disabled plugin keeps only its icon.
	_, err = m.Disable(ctx, "official.alpha")
	require.NoError(t, err)
	_, err = m.StaticRoot("official.alpha", "webapp")
	assert.ErrorIs(t, err, ErrPluginNotFound)
	root, err = m.IconRoot("official.alpha", "webapp", "icon.svg")
	require.NoError(t, err)
	assert.FileExists(t, filepath.Join(root, "icon.svg"))
	_, err = m.IconRoot("official.alpha", "webapp", "main.js")
	assert.ErrorIs(t, err, ErrPluginNotFound)
	_, err = m.IconRoot("official.alpha", "pages", "icon.svg")
	assert.ErrorIs(t, err, ErrPluginNotFound)
	_, err = m.IconRoot("official.gamma", "pages", "overview.html")
	assert.ErrorIs(t, err, ErrPluginNotFound, "a plugin without an icon serves nothing")
}

func TestManagerRefusesWhenTheSystemIsDisabled(t *testing.T) {
	m := newTestManager(t)
	previous := settings.PluginSettings.Enabled
	settings.PluginSettings.Enabled = false
	t.Cleanup(func() { settings.PluginSettings.Enabled = previous })

	ctx := context.Background()
	_, err := m.Install(ctx, "missing.tar.gz", InstallOptions{})
	assert.ErrorIs(t, err, ErrPluginsDisabled)
	_, err = m.Enable(ctx, "official.alpha", true)
	assert.ErrorIs(t, err, ErrPluginsDisabled)
	_, err = m.Disable(ctx, "official.alpha")
	assert.ErrorIs(t, err, ErrPluginsDisabled)
	assert.ErrorIs(t, m.Uninstall(ctx, "official.alpha", false), ErrPluginsDisabled)
	assert.ErrorIs(t, m.SaveSettings(ctx, "official.alpha", nil), ErrPluginsDisabled)
}

func TestWebappEntriesListChunkURLs(t *testing.T) {
	m := newTestManager(t)
	ctx := context.Background()
	require.NoError(t, m.LoadOffline(ctx))

	manifest := pluginManifest("official.alpha")
	manifest.Server = nil
	manifest.Capabilities = nil
	manifest.DNS01 = nil
	manifest.Webapp = &protocol.ManifestWebapp{
		BundlePath: "webapp/main.js",
		Chunks:     map[string]string{"search": "webapp/search.js", "deep": "webapp/chunks/deep.js", "elsewhere": "other/x.js"},
	}
	_, err := m.Install(ctx, buildTestPackage(t, manifest, map[string]string{
		"webapp/main.js": "a", "webapp/search.js": "b", "webapp/chunks/deep.js": "c", "other/x.js": "d",
	}), InstallOptions{Enable: true})
	require.NoError(t, err)

	entries := m.WebappEntries()
	require.Len(t, entries, 1)
	// A chunk the webapp route cannot serve is not offered.
	assert.Equal(t, map[string]string{
		"search": "plugins/official.alpha/webapp/search.js",
		"deep":   "plugins/official.alpha/webapp/chunks/deep.js",
	}, entries[0].Chunks)
}

// conflictManifest builds a test plugin that declares the given conflicts.
func conflictManifest(id string, conflicts ...string) *protocol.Manifest {
	manifest := pluginManifest(id)
	manifest.Conflicts = conflicts
	return manifest
}

func TestManagerEnableRefusesAConflictUnlessItReplaces(t *testing.T) {
	m := newTestManager(t)
	ctx := context.Background()
	require.NoError(t, m.LoadOffline(ctx))

	// official.beta declares the conflict, official.alpha does not know it.
	_, err := m.Install(ctx, buildTestPackage(t, pluginManifest("official.alpha"), nil), InstallOptions{Enable: true})
	require.NoError(t, err)
	beta, err := m.Install(ctx, buildTestPackage(t, conflictManifest("official.beta", "official.alpha"), nil), InstallOptions{})
	require.NoError(t, err)
	assert.Equal(t, []string{"official.alpha"}, beta.ConflictsEnabled)
	assert.Equal(t, []string{"official.alpha"}, beta.Conflicts)

	// The relation is symmetric, alpha sees beta as a conflict too.
	alpha, err := m.Get("official.alpha")
	require.NoError(t, err)
	assert.Empty(t, alpha.Conflicts)
	assert.Empty(t, alpha.ConflictsEnabled)
	require.NoError(t, m.setEnabledForTest(ctx, "official.beta", true))
	alpha, err = m.Get("official.alpha")
	require.NoError(t, err)
	assert.Equal(t, []string{"official.beta"}, alpha.ConflictsEnabled)
	require.NoError(t, m.setEnabledForTest(ctx, "official.beta", false))

	_, err = m.Enable(ctx, "official.beta", false)
	assertPluginError(t, err, ErrPluginConflict)
	assert.Contains(t, err.Error(), "official.alpha")
	alpha, err = m.Get("official.alpha")
	require.NoError(t, err)
	assert.True(t, alpha.Enabled)

	// Replacing turns the other plugin off, in either direction.
	beta, err = m.EnableWith(ctx, "official.beta", EnableOptions{ReplaceConflicts: true})
	require.NoError(t, err)
	assert.True(t, beta.Enabled)
	assert.Empty(t, beta.ConflictsEnabled)
	alpha, err = m.Get("official.alpha")
	require.NoError(t, err)
	assert.False(t, alpha.Enabled)

	_, err = m.Enable(ctx, "official.alpha", false)
	assertPluginError(t, err, ErrPluginConflict)
	_, err = m.EnableWith(ctx, "official.alpha", EnableOptions{ReplaceConflicts: true})
	require.NoError(t, err)
	beta, err = m.Get("official.beta")
	require.NoError(t, err)
	assert.False(t, beta.Enabled)
}

// setEnabledForTest flips the row flag without any conflict handling.
func (m *Manager) setEnabledForTest(ctx context.Context, id string, enabled bool) error {
	item, ok := m.lookup(id)
	if !ok {
		return ErrPluginNotFound
	}
	m.mu.Lock()
	item.row.Enabled = enabled
	m.mu.Unlock()
	return m.saveRow(ctx, item.row)
}

func TestManagerReplacingAConflictStopsItsDependents(t *testing.T) {
	m := newTestManager(t)
	ctx := context.Background()
	require.NoError(t, m.LoadOffline(ctx))

	_, err := m.Install(ctx, buildTestPackage(t, pluginManifest("official.alpha"), nil), InstallOptions{Enable: true})
	require.NoError(t, err)
	dependent := pluginManifest("official.gamma")
	dependent.Requires = []protocol.ManifestRequirement{{ID: "official.alpha"}}
	_, err = m.Install(ctx, buildTestPackage(t, dependent, nil), InstallOptions{Enable: true})
	require.NoError(t, err)
	_, err = m.Install(ctx, buildTestPackage(t, conflictManifest("official.beta", "official.alpha"), nil), InstallOptions{})
	require.NoError(t, err)

	_, err = m.EnableWith(ctx, "official.beta", EnableOptions{ReplaceConflicts: true})
	require.NoError(t, err)

	gamma, err := m.Get("official.gamma")
	require.NoError(t, err)
	assert.Contains(t, gamma.LastError, "official.alpha")
	alpha, err := m.Get("official.alpha")
	require.NoError(t, err)
	assert.False(t, alpha.Enabled)
}

func TestManagerReplaceKeepsAnUnapprovedPluginFromTurningOffTheOther(t *testing.T) {
	m := newTestManager(t)
	ctx := context.Background()
	require.NoError(t, m.LoadOffline(ctx))

	_, err := m.Install(ctx, buildTestPackage(t, pluginManifest("official.alpha"), nil), InstallOptions{Enable: true})
	require.NoError(t, err)
	first := conflictManifest("official.beta", "official.alpha")
	first.Permissions = []string{protocol.PermissionKV}
	_, err = m.Install(ctx, buildTestPackage(t, first, nil), InstallOptions{})
	require.NoError(t, err)
	second := conflictManifest("official.beta", "official.alpha")
	second.Version = "2.0.0"
	second.Permissions = []string{protocol.PermissionKV, protocol.PermissionNotify}
	_, err = m.Install(ctx, buildTestPackage(t, second, nil), InstallOptions{})
	require.NoError(t, err)
	_, err = m.EnableWith(ctx, "official.beta", EnableOptions{ApprovePermissions: true})
	assertPluginError(t, err, ErrPluginConflict)

	// A replace that fails on the approval leaves the other plugin on.
	_, err = m.EnableWith(ctx, "official.beta", EnableOptions{ReplaceConflicts: true})
	assert.ErrorIs(t, err, ErrPermissionApprovalRequired)
	alpha, err := m.Get("official.alpha")
	require.NoError(t, err)
	assert.True(t, alpha.Enabled)

	_, err = m.EnableWith(ctx, "official.beta", EnableOptions{ApprovePermissions: true, ReplaceConflicts: true})
	require.NoError(t, err)
	alpha, err = m.Get("official.alpha")
	require.NoError(t, err)
	assert.False(t, alpha.Enabled)
}

func TestManagerInstallWithEnableLeavesAConflictDisabled(t *testing.T) {
	m := newTestManager(t)
	ctx := context.Background()
	require.NoError(t, m.LoadOffline(ctx))

	_, err := m.Install(ctx, buildTestPackage(t, pluginManifest("official.alpha"), nil), InstallOptions{Enable: true})
	require.NoError(t, err)

	info, err := m.Install(ctx, buildTestPackage(t, conflictManifest("official.beta", "official.alpha"), nil), InstallOptions{Enable: true})
	require.NoError(t, err)
	assert.False(t, info.Enabled)
	assert.Equal(t, []string{"official.alpha"}, info.ConflictsEnabled)
	alpha, err := m.Get("official.alpha")
	require.NoError(t, err)
	assert.True(t, alpha.Enabled)

	// An upgrade that adds a conflict keeps the plugin off, and asking to
	// replace turns the other one off instead.
	upgrade := conflictManifest("official.alpha", "official.gamma")
	upgrade.Version = "2.0.0"
	_, err = m.Install(ctx, buildTestPackage(t, pluginManifest("official.gamma"), nil), InstallOptions{Enable: true})
	require.NoError(t, err)
	info, err = m.Install(ctx, buildTestPackage(t, upgrade, nil), InstallOptions{Enable: true})
	require.NoError(t, err)
	assert.False(t, info.Enabled)
	gamma, err := m.Get("official.gamma")
	require.NoError(t, err)
	assert.True(t, gamma.Enabled)

	upgrade.Version = "3.0.0"
	info, err = m.Install(ctx, buildTestPackage(t, upgrade, nil), InstallOptions{Enable: true, ReplaceConflicts: true})
	require.NoError(t, err)
	assert.True(t, info.Enabled)
	gamma, err = m.Get("official.gamma")
	require.NoError(t, err)
	assert.False(t, gamma.Enabled)

	// The new package of an enabled plugin replaces a conflict on request.
	other, err := m.Install(ctx, buildTestPackage(t, conflictManifest("official.beta", "official.alpha"), nil),
		InstallOptions{Enable: true, ReplaceConflicts: true})
	require.NoError(t, err)
	assert.True(t, other.Enabled)
	alpha, err = m.Get("official.alpha")
	require.NoError(t, err)
	assert.False(t, alpha.Enabled)
}

func TestManagerStartupTurnsOffALaterConflictingPlugin(t *testing.T) {
	m := newTestManager(t)
	ctx := context.Background()
	require.NoError(t, m.LoadOffline(ctx))

	_, err := m.Install(ctx, buildTestPackage(t, pluginManifest("official.alpha"), nil), InstallOptions{Enable: true})
	require.NoError(t, err)
	_, err = m.Install(ctx, buildTestPackage(t, pluginManifest("official.beta"), nil), InstallOptions{Enable: true})
	require.NoError(t, err)
	_, err = m.Install(ctx, buildTestPackage(t, pluginManifest("official.delta"), nil), InstallOptions{Enable: true})
	require.NoError(t, err)

	// The database can hold two enabled conflicting plugins, for instance
	// after an older host enabled them both.
	beta, _ := m.lookup("official.beta")
	m.mu.Lock()
	beta.manifest.Conflicts = []string{"official.alpha"}
	m.mu.Unlock()

	m.startEnabled(ctx)

	alpha, err := m.Get("official.alpha")
	require.NoError(t, err)
	assert.Empty(t, alpha.LastError)
	betaInfo, err := m.Get("official.beta")
	require.NoError(t, err)
	assert.False(t, betaInfo.Enabled)
	assert.Contains(t, betaInfo.LastError, "official.alpha")
	row, err := query.Plugin.WithContext(ctx).Where(query.Plugin.PluginID.Eq("official.beta")).First()
	require.NoError(t, err)
	assert.False(t, row.Enabled, "the loser must stay off after a restart")
	delta, err := m.Get("official.delta")
	require.NoError(t, err)
	assert.Empty(t, delta.LastError)
}
