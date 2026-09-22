package capability

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/0xJacky/Nginx-UI/internal/plugin"
	"github.com/0xJacky/Nginx-UI/internal/template"
	"github.com/0xJacky/Nginx-UI/internal/translation"
)

const contentBlockTemplate = `# Nginx UI Template Start
name = "Deny dotfiles"
author = "@example"
# Nginx UI Template End
location ~ /\. {
    deny all;
}
`

const contentPOFile = `msgid ""
msgstr ""
"Language: zh_CN\n"

msgid "Deny dotfiles"
msgstr "拒绝访问隐藏文件"
`

// writeContent lays out a templates and a locales directory.
func writeContent(t *testing.T) (templatesDir, localesDir string) {
	t.Helper()
	root := t.TempDir()
	templatesDir = filepath.Join(root, "templates")
	localesDir = filepath.Join(root, "locales")
	for path, body := range map[string]string{
		filepath.Join(templatesDir, "block", "deny-dotfiles.conf"): contentBlockTemplate,
		filepath.Join(localesDir, "zh_CN.po"):                      contentPOFile,
		filepath.Join(localesDir, "xx_XX.po"):                      contentPOFile,
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return templatesDir, localesDir
}

func TestTemplateSourceFollowsTheEnabledPlugins(t *testing.T) {
	templatesDir, _ := writeContent(t)
	host := newCapabilityHost()
	host.setContent([]plugin.ContentEntry{{PluginID: "io.github.example.snippets", TemplatesDir: templatesDir}})

	roots := NewTemplateSource(host).TemplateRoots()
	if len(roots) != 1 || roots[0].PluginID != "io.github.example.snippets" || roots[0].Dir != templatesDir {
		t.Fatalf("roots = %+v", roots)
	}

	template.RegisterSource(NewTemplateSource(host))
	info, err := template.GetPluginTemplateInfo("io.github.example.snippets", "block", "deny-dotfiles.conf")
	if err != nil || info.Name != "Deny dotfiles" || info.Origin != template.OriginPlugin {
		t.Fatalf("info = %+v, %v", info, err)
	}

	// Disabling the plugin takes its templates away at once.
	host.setContent(nil)
	if _, err = template.GetPluginTemplateInfo("io.github.example.snippets", "block", "deny-dotfiles.conf"); err == nil {
		t.Fatal("the template of a disabled plugin is still served")
	}
}

func TestLocaleBridgeMergesAndRemovesCatalogs(t *testing.T) {
	_, localesDir := writeContent(t)
	host := newCapabilityHost()
	host.setContent([]plugin.ContentEntry{{PluginID: "io.github.example.snippets", LocalesDir: localesDir}})
	t.Cleanup(func() { translation.SetPluginCatalogs(nil) })

	bridge := NewLocaleBridge(host)
	bridge.Sync()
	if got := translation.GetTranslation("zh_CN")["Deny dotfiles"]; got != "拒绝访问隐藏文件" {
		t.Fatalf("translation = %v", got)
	}
	if ids := translation.PluginIDs(); len(ids) != 1 || ids[0] != "io.github.example.snippets" {
		t.Fatalf("plugins = %v", ids)
	}

	host.setContent(nil)
	bridge.Sync()
	if _, ok := translation.GetTranslation("zh_CN")["Deny dotfiles"]; ok {
		t.Fatal("the entries of a disabled plugin are still served")
	}
}
