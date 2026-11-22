package settings

import (
	"errors"
	"io"
	"math/rand/v2"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uozi-tech/cosy"
	"github.com/uozi-tech/cosy/logger"
	cosysettings "github.com/uozi-tech/cosy/settings"
	"gopkg.in/ini.v1"
)

// These tests cover GHSA-jjg7-4xmx-2vvh at the shared write path: every
// writer of app.ini must refuse a value the file cannot store faithfully,
// whichever setting it is in.

// useTempConf points the settings at an empty app.ini in a temp directory.
func useTempConf(t *testing.T) string {
	t.Helper()

	confPath := filepath.Join(t.TempDir(), "app.ini")
	require.NoError(t, os.WriteFile(confPath, []byte("[app]\n"), 0o600))

	originalPath := cosysettings.ConfPath
	originalConf := cosysettings.Conf
	originalNginx := *NginxSettings
	originalTerminal := *TerminalSettings
	originalPlugin := *PluginSettings
	originalNode := *NodeSettings
	originalBackup := *BackupSettings
	originalCrypto := *CryptoSettings
	t.Cleanup(func() {
		cosysettings.ConfPath = originalPath
		cosysettings.Conf = originalConf
		*NginxSettings = originalNginx
		*TerminalSettings = originalTerminal
		*PluginSettings = originalPlugin
		*NodeSettings = originalNode
		*BackupSettings = originalBackup
		*CryptoSettings = originalCrypto
	})

	cosysettings.ConfPath = confPath
	require.NoError(t, cosysettings.Reload())
	return confPath
}

func requireUnstorableValue(t *testing.T, err error, key string) {
	t.Helper()
	require.Error(t, err)
	require.True(t, IsUnstorableValue(err), "unexpected error: %v", err)
	var cErr *cosy.Error
	require.True(t, errors.As(err, &cErr))
	assert.Equal(t, "settings", cErr.Scope)
	assert.Equal(t, int32(40007), cErr.Code)
	assert.Equal(t, []string{key}, cErr.Params)
}

func TestUpdateRejectsValueThatCannotBeStored(t *testing.T) {
	values := map[string]string{
		"ends a triple-quoted value early": "x\"\"\"\n[terminal]\nStartCmd = update-marker\n[nginx]\nz = \"\"\"",
		"adds a key to its own section":    "x\"\"\"\nTestConfigCmd = update-marker\nz = \"\"\"",
		"leaves an unparsable line":        "\"\"\"\n",
		"swallows the next key":            `C:\maintenance\`,
	}

	for name, value := range values {
		t.Run(name, func(t *testing.T) {
			confPath := useTempConf(t)
			TerminalSettings.StartCmd = "saved-start-cmd"
			NginxSettings.TestConfigCmd = "saved-test-cmd"
			NginxSettings.MaintenanceTemplate = "maintenance.html"
			NginxSettings.MaintenanceHost = "maintenance.example.com"
			require.NoError(t, Save())
			before, err := os.ReadFile(confPath)
			require.NoError(t, err)

			err = Update(func() {
				NodeSettings.Name = "changed-in-the-same-update"
				NginxSettings.MaintenanceTemplate = value
			})

			requireUnstorableValue(t, err, "nginx.MaintenanceTemplate")
			after, err := os.ReadFile(confPath)
			require.NoError(t, err)
			assert.Equal(t, string(before), string(after), "a refused update must leave app.ini untouched")

			// The whole update is rolled back, not only the refused field.
			assert.Equal(t, "maintenance.html", NginxSettings.MaintenanceTemplate)
			assert.NotEqual(t, "changed-in-the-same-update", NodeSettings.Name)
			assert.Equal(t, "saved-start-cmd", TerminalSettings.StartCmd)
			assert.Equal(t, "saved-test-cmd", NginxSettings.TestConfigCmd)

			// The next update is not blocked by the refused one.
			require.NoError(t, Update(func() { NginxSettings.MaintenanceHost = "other.example.com" }))
			conf, err := ini.LoadSources(confLoadOptions, confPath)
			require.NoError(t, err)
			assert.Equal(t, "other.example.com", conf.Section("nginx").Key("MaintenanceHost").String())
			assert.Equal(t, []string{"saved-start-cmd"}, conf.Section("terminal").Key("StartCmd").ValueWithShadows())
			assert.Equal(t, []string{"saved-test-cmd"}, conf.Section("nginx").Key("TestConfigCmd").ValueWithShadows())
		})
	}
}

// List fields are stored as repeated keys, and a trusted publisher entry may
// span several lines. Both must keep saving and read back unchanged.
func TestSaveKeepsMultiLineAndListValues(t *testing.T) {
	confPath := useTempConf(t)
	NodeSettings.Name = ""
	PluginSettings.MarketplaceSources = []string{
		"https://plugins.example.com/v1/index.json",
		"https://mirror.example.com/index.json?a=1;b=2#x",
	}
	PluginSettings.TrustedPublicKeys = []string{
		"untrusted comment: minisign public key\nRWQexamplekeyexamplekeyexamplekeyexamplekeyexample Example `Publisher`",
		"RWQsecondkeysecondkeysecondkeysecondkeysecondkeysec",
	}
	NginxSettings.TestConfigCmd = "nginx -t -c \"C:\\nginx\\conf\\nginx.conf\" # check"
	require.NoError(t, Save())

	conf, err := ini.LoadSources(confLoadOptions, confPath)
	require.NoError(t, err)
	assert.Equal(t, PluginSettings.MarketplaceSources, conf.Section("plugin").Key("MarketplaceSources").ValueWithShadows())
	assert.Equal(t, PluginSettings.TrustedPublicKeys, conf.Section("plugin").Key("TrustedPublicKeys").ValueWithShadows())
	assert.Equal(t, NginxSettings.TestConfigCmd, conf.Section("nginx").Key("TestConfigCmd").String())

	// Another update writes the values again from the reloaded file; they
	// must come out the same.
	require.NoError(t, Update(func() { NodeSettings.Name = "node-1" }))
	reloaded, err := ini.LoadSources(confLoadOptions, confPath)
	require.NoError(t, err)
	assert.Equal(t, "node-1", reloaded.Section("node").Key("Name").String())
	for _, section := range []string{"plugin", "nginx"} {
		for _, key := range conf.Section(section).Keys() {
			assert.Equal(t, key.ValueWithShadows(), reloaded.Section(section).Key(key.Name()).ValueWithShadows(),
				section+"."+key.Name())
		}
	}
}

func TestBuildRestoreConfigKeepsValuesTheFileCanStore(t *testing.T) {
	currentPath := writeRestoreConfigFixture(t, "current.ini", "[app]\nJwtSecret = destination\n")
	backupPath := writeRestoreConfigFixture(t, "backup.ini", "[nginx]\n"+
		"MaintenanceTemplate = \"\"\"line one\nline `two`\"\"\"\n"+
		"MaintenanceHost = `maintenance#1;a`\n"+
		"[plugin]\n"+
		"MarketplaceSources = https://plugins.example.com/v1/index.json\n"+
		"MarketplaceSources = https://mirror.example.com/index.json\n")

	merged, _, err := BuildRestoreConfig(backupPath, currentPath, false)
	require.NoError(t, err)

	conf, err := ini.LoadSources(confLoadOptions, merged)
	require.NoError(t, err)
	assert.Equal(t, "line one\nline `two`", conf.Section("nginx").Key("MaintenanceTemplate").String())
	assert.Equal(t, "maintenance#1;a", conf.Section("nginx").Key("MaintenanceHost").String())
	assert.Equal(t,
		[]string{"https://plugins.example.com/v1/index.json", "https://mirror.example.com/index.json"},
		conf.Section("plugin").Key("MarketplaceSources").ValueWithShadows())
}

// A quoted value in a backup can hold what app.ini cannot store unquoted, such
// as a trailing backslash. Restoring it would swallow the next key.
func TestBuildRestoreConfigRejectsValueThatCannotBeStored(t *testing.T) {
	currentPath := writeRestoreConfigFixture(t, "current.ini", "[app]\nJwtSecret = destination\n")
	backupPath := writeRestoreConfigFixture(t, "backup.ini", "[nginx]\n"+
		"MaintenanceTemplate = `C:\\maintenance\\`\n"+
		"MaintenanceHost = maintenance.example.com\n")

	_, _, err := BuildRestoreConfig(backupPath, currentPath, false)
	requireUnstorableValue(t, err, "nginx.MaintenanceTemplate")
}

// writeAndLoadConf replaces app.ini with content and maps it onto the settings,
// as a start with a hand-edited app.ini would.
func writeAndLoadConf(t *testing.T, confPath, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(confPath, []byte(content), 0o600))
	require.NoError(t, cosysettings.Reload())
	for _, target := range settingsTargets() {
		require.NoError(t, cosysettings.MapTo(target.name, target.ptr))
	}
}

func reflectedSettings() *ini.File {
	doc := ini.Empty(confLoadOptions)
	for _, target := range settingsTargets() {
		_ = doc.Section(target.name).ReflectFrom(target.ptr)
	}
	return doc
}

func TestUpdateRollsBackEditedSliceElements(t *testing.T) {
	useTempConf(t)
	PluginSettings.MarketplaceSources = []string{"https://plugins.example.com/v1/index.json"}
	require.NoError(t, Save())

	err := Update(func() {
		PluginSettings.MarketplaceSources[0] = `https://plugins.example.com/%(Dir)s`
	})

	requireUnstorableValue(t, err, "plugin.MarketplaceSources")
	assert.Equal(t, []string{"https://plugins.example.com/v1/index.json"}, PluginSettings.MarketplaceSources)
}

// A list keeps its first entry when one is added, so the whole list is
// compared to tell that the change set the refused value.
func TestUpdateRejectsUnstorableEntryAddedToAList(t *testing.T) {
	useTempConf(t)
	PluginSettings.MarketplaceSources = []string{"https://plugins.example.com/v1/index.json"}
	require.NoError(t, Save())

	err := Update(func() {
		PluginSettings.MarketplaceSources = append(PluginSettings.MarketplaceSources, `https://mirror.example.com/\`)
	})

	requireUnstorableValue(t, err, "plugin.MarketplaceSources")
	assert.Equal(t, []string{"https://plugins.example.com/v1/index.json"}, PluginSettings.MarketplaceSources)
}

func TestUpdateKeepsTheDocumentWhenAppIniCannotBeReread(t *testing.T) {
	confPath := useTempConf(t)
	require.NoError(t, Save())
	require.NoError(t, os.WriteFile(confPath, []byte("not an ini line\n"), 0o600))

	err := Update(func() { NginxSettings.MaintenanceHost = `C:\maintenance\` })

	requireUnstorableValue(t, err, "nginx.MaintenanceHost")
	require.NotNil(t, cosysettings.Conf)
	require.NoError(t, Update(func() { NginxSettings.MaintenanceHost = "maintenance.example.com" }))
}

// An environment override app.ini cannot store stays in effect, and the
// update is saved with app.ini keeping its stored value for that setting.
func TestUpdateKeepsStoredValueForEnvironmentOverride(t *testing.T) {
	confPath := useTempConf(t)
	writeAndLoadConf(t, confPath, "[nginx]\nConfigDir = /etc/nginx\nConfigPath = /etc/nginx/nginx.conf\n")
	t.Setenv("NGINX_UI_NGINX_CONFIG_DIR", `C:\nginx\conf\`)
	parseEnv(NginxSettings, "NGINX_")
	require.Equal(t, `C:\nginx\conf\`, NginxSettings.ConfigDir)

	message := describeUnstorable(reflectedSettings(), "nginx.ConfigDir")
	assert.Contains(t, message, "NGINX_UI_NGINX_CONFIG_DIR")
	assert.Contains(t, message, "ends with a backslash")

	require.NoError(t, Update(func() { CryptoSettings.Secret = "generated-secret" }))

	conf, err := ini.LoadSources(confLoadOptions, confPath)
	require.NoError(t, err)
	assert.Equal(t, "generated-secret", conf.Section("crypto").Key("Secret").String())
	assert.Equal(t, "/etc/nginx", conf.Section("nginx").Key("ConfigDir").String())
	assert.Equal(t, "/etc/nginx/nginx.conf", conf.Section("nginx").Key("ConfigPath").String())
	assert.Equal(t, `C:\nginx\conf\`, NginxSettings.ConfigDir, "the override stays in effect")
	assert.Equal(t, "/etc/nginx/nginx.conf", NginxSettings.ConfigPath)
	assert.Equal(t, "generated-secret", CryptoSettings.Secret)
}

// app.ini can store a value it cannot write back: a quoted value ending in a
// backslash, a %(name)s reference in a key no setting maps to, or a second
// copy of a key. Nothing can be saved then, so every update is refused and
// rolled back, and the error names the key to correct in app.ini.
func TestUpdateRollsBackWhenAppIniHoldsUnstorableValue(t *testing.T) {
	stored := map[string]struct {
		content string
		key     string
		hint    string
	}{
		"quoted value ending in a backslash": {
			content: "[backup]\nGrantedAccessPath = \"D:\\\"\n",
			key:     "backup.GrantedAccessPath",
			hint:    "ends with a backslash",
		},
		"reference in a DEFAULT key": {
			content: "Base = /srv\nLegacy = %(Base)s/legacy\n[nginx]\nConfigDir = /etc/nginx\n",
			key:     "DEFAULT.Legacy",
			hint:    "%(name)s reference",
		},
		"reference in an unmapped section with a dot": {
			content: "[legacy.v1]\nPath = %(Other)s\nOther = x\n",
			key:     "legacy.v1.Path",
			hint:    "%(name)s reference",
		},
		"second copy of a single value key": {
			content: "[nginx]\nMaintenanceHost = maintenance.example.com\nMaintenanceHost = \"D:\\\"\n",
			key:     "nginx.MaintenanceHost",
			hint:    "ends with a backslash",
		},
	}

	for name, tt := range stored {
		t.Run(name, func(t *testing.T) {
			confPath := useTempConf(t)
			writeAndLoadConf(t, confPath, tt.content)
			CryptoSettings.Secret = ""
			before, err := os.ReadFile(confPath)
			require.NoError(t, err)

			updateErr := Update(func() {
				CryptoSettings.Secret = "generated-secret"
				NodeSettings.Name = "changed-in-the-same-update"
			})

			requireUnstorableValue(t, updateErr, tt.key)
			assert.Empty(t, CryptoSettings.Secret, "a change that cannot be saved must not stay in effect")
			assert.NotEqual(t, "changed-in-the-same-update", NodeSettings.Name)
			after, err := os.ReadFile(confPath)
			require.NoError(t, err)
			assert.Equal(t, string(before), string(after))

			message := UnstorableStoredValueMessage(updateErr)
			assert.Contains(t, message, tt.key)
			assert.Contains(t, message, tt.hint)
			assert.Contains(t, message, "before Nginx UI can start")
		})
	}
}

func TestReportUnstorableSettingsNamesEveryKeyAndItsSource(t *testing.T) {
	confPath := useTempConf(t)
	writeAndLoadConf(t, confPath, "[backup]\nGrantedAccessPath = \"D:\\\"\n[nginx]\nConfigDir = /etc/nginx\n")
	t.Setenv("NGINX_UI_NGINX_CONFIG_DIR", `C:\nginx\conf\`)
	parseEnv(NginxSettings, "NGINX_")

	output := captureLog(t, reportUnstorableSettings)

	assert.Contains(t, output, "stores a value for backup.GrantedAccessPath that cannot be written back: it ends with a backslash")
	assert.Contains(t, output, "No settings change can be saved until it is corrected in app.ini")
	assert.Contains(t, output, "Setting nginx.ConfigDir (set by environment variable NGINX_UI_NGINX_CONFIG_DIR) cannot be stored in app.ini: it ends with a backslash")
}

// captureLog returns what fn logs.
func captureLog(t *testing.T, fn func()) string {
	t.Helper()
	originalStderr := os.Stderr
	reader, writer, err := os.Pipe()
	require.NoError(t, err)
	os.Stderr = writer
	logger.Init("debug")
	defer func() {
		os.Stderr = originalStderr
		logger.Init("debug")
	}()

	fn()

	require.NoError(t, writer.Close())
	output, err := io.ReadAll(reader)
	require.NoError(t, err)
	return string(output)
}

// The fatal paths run in a child test process, selected by this variable.
const unstorableChildEnv = "NGINX_UI_TEST_UNSTORABLE_CHILD"

// Writing migrated settings that read back differently would change them, and
// starting without them would look like a new installation, so the start
// stops and the v1 file is left untouched for the operator to correct.
func TestMigrationStopsWhenValueCannotBeStored(t *testing.T) {
	if confPath := os.Getenv(unstorableChildEnv); confPath != "" {
		migrate(confPath)
		return
	}

	confPath := filepath.Join(t.TempDir(), "app.ini")
	content := "[server]\nJwtSecret = v1-secret\nCADir = \"D:\\\"\n"
	require.NoError(t, os.WriteFile(confPath, []byte(content), 0o600))

	cmd := exec.Command(os.Args[0], "-test.run=^TestMigrationStopsWhenValueCannotBeStored$")
	cmd.Env = append(os.Environ(), unstorableChildEnv+"="+confPath)
	output, err := cmd.CombinedOutput()

	var exitErr *exec.ExitError
	require.ErrorAs(t, err, &exitErr, "migration must stop the start, output:\n%s", output)
	assert.Contains(t, string(output), "cannot be migrated, the file was left unchanged")
	assert.Contains(t, string(output), "cert.CADir: it ends with a backslash")
	assert.Contains(t, string(output), "before Nginx UI can start")
	after, err := os.ReadFile(confPath)
	require.NoError(t, err)
	assert.Equal(t, content, string(after))
}

// deepCopy cannot copy what a pointer or an interface refers to, so a snapshot
// would share it with the live settings.
func TestSettingsStructsHoldNoSharedReferences(t *testing.T) {
	var check func(path string, typ reflect.Type)
	check = func(path string, typ reflect.Type) {
		switch typ.Kind() {
		case reflect.Pointer, reflect.Interface, reflect.Func, reflect.Chan, reflect.UnsafePointer:
			t.Errorf("%s is a %s, which a settings snapshot would share", path, typ.Kind())
		case reflect.Slice, reflect.Array:
			check(path+"[]", typ.Elem())
		case reflect.Map:
			check(path+"[key]", typ.Key())
			check(path+"[value]", typ.Elem())
		case reflect.Struct:
			for i := 0; i < typ.NumField(); i++ {
				if typ.Field(i).IsExported() {
					check(path+"."+typ.Field(i).Name, typ.Field(i).Type)
				}
			}
		}
	}
	for _, target := range settingsTargets() {
		check(target.name, reflect.TypeOf(target.ptr).Elem())
	}
}

// A seeded property check: whatever value a setting is given from characters
// ini.v1 treats specially, either it is refused and app.ini is untouched, or
// it reads back exactly, also through interpolation, and the protected
// settings around it keep their values.
func TestUpdateNeverChangesOtherSettingsProperty(t *testing.T) {
	alphabet := []string{" ", "\n", `\`, "#", ";", "%", "(", ")", "s", "=", "[", "]", "\r", `"""`, "`", "a", "Key"}
	random := rand.New(rand.NewPCG(2026, 11))

	confPath := useTempConf(t)
	NginxSettings.TestConfigCmd = "protected-test-cmd"
	TerminalSettings.StartCmd = "protected-start-cmd"
	NginxSettings.MaintenanceHost = "maintenance.example.com"
	require.NoError(t, Save())

	for i := 0; i < 400; i++ {
		var value strings.Builder
		for n := random.IntN(12); n >= 0; n-- {
			value.WriteString(alphabet[random.IntN(len(alphabet))])
		}
		before, err := os.ReadFile(confPath)
		require.NoError(t, err)
		previous := NginxSettings.MaintenanceTemplate

		err = Update(func() { NginxSettings.MaintenanceTemplate = value.String() })

		conf, loadErr := ini.LoadSources(confLoadOptions, confPath)
		require.NoError(t, loadErr)
		if err != nil {
			requireUnstorableValue(t, err, "nginx.MaintenanceTemplate")
			after, readErr := os.ReadFile(confPath)
			require.NoError(t, readErr)
			require.Equal(t, string(before), string(after), "refused value %q", value.String())
			require.Equal(t, previous, NginxSettings.MaintenanceTemplate)
		} else {
			require.Equal(t, value.String(), conf.Section("nginx").Key("MaintenanceTemplate").String(), "value %q", value.String())
			require.Equal(t, value.String(), NginxSettings.MaintenanceTemplate)
		}
		require.Equal(t, []string{"protected-test-cmd"}, conf.Section("nginx").Key("TestConfigCmd").ValueWithShadows(), "value %q", value.String())
		require.Equal(t, []string{"protected-start-cmd"}, conf.Section("terminal").Key("StartCmd").ValueWithShadows(), "value %q", value.String())
		require.Equal(t, "maintenance.example.com", conf.Section("nginx").Key("MaintenanceHost").String(), "value %q", value.String())
		require.Equal(t, "protected-test-cmd", NginxSettings.TestConfigCmd)
		require.Equal(t, "protected-start-cmd", TerminalSettings.StartCmd)
	}
}
