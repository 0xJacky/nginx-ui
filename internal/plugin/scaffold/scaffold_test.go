package scaffold

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/0xJacky/Nginx-UI/internal/plugin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestInitProducesLintCleanRepositories scaffolds each supported language and
// runs the plugin linter over the result. The only finding tolerated is the
// "server.command[0] not found on PATH" warning: whether python3 or
// node happen to be installed on the machine running this test is not
// something the scaffold itself controls, and the spec only asks for a
// warning there, never an error.
func TestInitProducesLintCleanRepositories(t *testing.T) {
	for _, lang := range []string{"go", "rust", "python", "node"} {
		t.Run(lang, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "plugin")
			require.NoError(t, Init(dir, InitOptions{
				ID:   "io.github.example.demo",
				Name: "Demo",
				Lang: lang,
			}))

			report, err := plugin.Lint(dir)
			require.NoError(t, err)

			for _, f := range report.Findings {
				if f.Level == plugin.LevelWarning && f.Rule == plugin.RuleServerPaths {
					continue
				}
				t.Errorf("unexpected finding for lang %s: %+v", lang, f)
			}
			assert.False(t, report.HasErrors(), "%+v", report.Findings)

			// The manifest shows how to translate the name and description.
			manifest, err := plugin.LoadManifest(dir)
			require.NoError(t, err)
			assert.Equal(t, "Demo", manifest.I18n["zh_CN"].Name)
			assert.NotEmpty(t, manifest.I18n["zh_CN"].Description)
		})
	}
}

func TestInitRejectsBadID(t *testing.T) {
	err := Init(t.TempDir(), InitOptions{ID: "NotValid", Name: "Demo", Lang: "go"})
	assert.Error(t, err)
}

func TestInitRejectsEmptyName(t *testing.T) {
	err := Init(t.TempDir(), InitOptions{ID: "io.github.example.demo", Lang: "go"})
	assert.Error(t, err)
}

func TestInitRejectsUnknownLang(t *testing.T) {
	err := Init(t.TempDir(), InitOptions{ID: "io.github.example.demo", Name: "Demo", Lang: "cobol"})
	assert.Error(t, err)
}

func TestInitRejectsUnknownCapability(t *testing.T) {
	err := Init(t.TempDir(), InitOptions{ID: "io.github.example.demo", Name: "Demo", Lang: "go", Capability: "http"})
	assert.Error(t, err)
}

func TestInitGoWritesAPlaceholderBinary(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "plugin")
	require.NoError(t, Init(dir, InitOptions{ID: "io.github.example.demo", Name: "Demo", Lang: "go"}))

	manifest, err := plugin.LoadManifest(dir)
	require.NoError(t, err)
	require.NoError(t, plugin.ValidateManifest(manifest))

	argv, err := plugin.ResolveExecutable(manifest, dir)
	require.NoError(t, err)
	require.NotEmpty(t, argv)
}

func TestPascalCase(t *testing.T) {
	assert.Equal(t, "MyDns", pascalCase("my-dns"))
	assert.Equal(t, "Provider", pascalCase("---"))
}

func TestSanitizeProviderCode(t *testing.T) {
	assert.Equal(t, "a-x", sanitizeProviderCode("a"))
	assert.Len(t, sanitizeProviderCode(strings.Repeat("a", 40)), 32)
	assert.Equal(t, "mydns", sanitizeProviderCode("MyDNS"))
}
