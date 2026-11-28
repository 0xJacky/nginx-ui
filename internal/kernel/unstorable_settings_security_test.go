package kernel

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	appSettings "github.com/0xJacky/Nginx-UI/settings"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	cSettings "github.com/uozi-tech/cosy/settings"
	"gopkg.in/ini.v1"
)

// These tests cover the boot side of GHSA-jjg7-4xmx-2vvh. A setting app.ini
// cannot store that an environment variable overrides must not stop the
// boot, and the generated values are saved. A value app.ini stores itself and
// cannot write back stops the boot instead of letting a secret be generated
// anew on every start.

// setupUnstorableSettings starts from an app.ini with the given content, as
// settings.Init would at boot, and restores the sections it changes.
func setupUnstorableSettings(t *testing.T, content string) string {
	t.Helper()
	originalNginx := *appSettings.NginxSettings
	originalBackup := *appSettings.BackupSettings
	t.Cleanup(func() {
		*appSettings.NginxSettings = originalNginx
		*appSettings.BackupSettings = originalBackup
	})
	setupSettingsFile(t)

	confPath := filepath.Join(t.TempDir(), "app.ini")
	require.NoError(t, os.WriteFile(confPath, []byte(content), 0o644))
	appSettings.Init(confPath)
	cSettings.ConfPath = confPath
	return confPath
}

// bootGenerators are the boot steps that generate and save a value, with the
// key they save it under.
var bootGenerators = map[string]struct {
	run     func()
	clear   func()
	section string
	key     string
	value   func() string
}{
	"node secret": {
		run:     InitNodeSecret,
		clear:   func() { appSettings.NodeSettings.Secret = "" },
		section: "node", key: "Secret",
		value: func() string { return appSettings.NodeSettings.Secret },
	},
	"crypto secret": {
		run:     InitCryptoSecret,
		clear:   func() { appSettings.CryptoSettings.Secret = "" },
		section: "crypto", key: "Secret",
		value: func() string { return appSettings.CryptoSettings.Secret },
	},
	"instance ID": {
		run:     InitNodeInstanceID,
		clear:   func() { appSettings.NodeSettings.InstanceID = "" },
		section: "node", key: "InstanceID",
		value: func() string { return appSettings.NodeSettings.InstanceID },
	},
	"skip install": {
		run: skipInstall,
		clear: func() {
			cSettings.AppSettings.JwtSecret = ""
			appSettings.NodeSettings.Secret = ""
		},
		section: "app", key: "JwtSecret",
		value: func() string { return cSettings.AppSettings.JwtSecret },
	},
}

func TestBootSavesGeneratedValuesNextToAnUnstorableEnvironmentOverride(t *testing.T) {
	for name, generator := range bootGenerators {
		t.Run(name, func(t *testing.T) {
			t.Setenv("NGINX_UI_NGINX_CONFIG_DIR", `C:\nginx\conf\`)
			confPath := setupUnstorableSettings(t, "[app]\n[server]\nPort = 9000\n[nginx]\nConfigDir = /etc/nginx\n")
			require.Equal(t, `C:\nginx\conf\`, appSettings.NginxSettings.ConfigDir)
			generator.clear()

			generator.run()

			generated := generator.value()
			require.NotEmpty(t, generated)
			conf, err := ini.LoadSources(ini.LoadOptions{Loose: true, AllowShadows: true}, confPath)
			require.NoError(t, err)
			assert.Equal(t, generated, conf.Section(generator.section).Key(generator.key).String())
			assert.Equal(t, "/etc/nginx", conf.Section("nginx").Key("ConfigDir").String())
			assert.Equal(t, `C:\nginx\conf\`, appSettings.NginxSettings.ConfigDir, "the override stays in effect")
		})
	}
}

// The fatal paths run in a child test process, selected by this variable.
const unstorableBootChildEnv = "NGINX_UI_TEST_UNSTORABLE_BOOT_CHILD"

func TestBootStopsWhenAppIniHoldsUnstorableValue(t *testing.T) {
	const content = "[app]\n[server]\nPort = 9000\n[backup]\nGrantedAccessPath = \"D:\\\"\n"

	if name := os.Getenv(unstorableBootChildEnv); name != "" {
		generator := bootGenerators[name]
		setupUnstorableSettings(t, content)
		generator.clear()
		generator.run()
		// Reaching here means the boot went on; the parent sees exit code 0.
		return
	}

	for name := range bootGenerators {
		t.Run(name, func(t *testing.T) {
			cmd := exec.Command(os.Args[0], "-test.run=^TestBootStopsWhenAppIniHoldsUnstorableValue$")
			cmd.Env = append(os.Environ(), unstorableBootChildEnv+"="+name)
			output, err := cmd.CombinedOutput()

			var exitErr *exec.ExitError
			require.ErrorAs(t, err, &exitErr, "the boot must stop, output:\n%s", output)
			assert.Contains(t, string(output), "stores a value for backup.GrantedAccessPath that cannot be written back")
			assert.Contains(t, string(output), "it ends with a backslash")
			assert.Contains(t, string(output), "Correct it in app.ini before Nginx UI can start")
		})
	}
}
