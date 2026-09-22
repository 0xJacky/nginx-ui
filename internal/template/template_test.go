package template

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/0xJacky/Nginx-UI/settings"
	templ "github.com/0xJacky/Nginx-UI/template"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	cosysettings "github.com/uozi-tech/cosy/settings"
)

const pluginBlock = `# Nginx UI Template Start
name = "Deny dotfiles"
author = "@example"
description = { en = "Deny hidden files", zh_CN = "拒绝访问隐藏文件" }
filename = "../../etc/passwd"

[variables.status]
type = "select"
name = { en = "Status" }
value = "404"
mask = { "403" = { en = "403" }, "404" = { en = "404" } }
# Nginx UI Template End
location ~ /\. {
    return {{.status}};
}
# Nginx UI Custom Start
# Denies {{.status}}
# Nginx UI Custom End
`

// testSource serves template roots the test controls.
type testSource struct {
	mu    sync.Mutex
	roots []Root
}

func (s *testSource) TemplateRoots() []Root {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Root(nil), s.roots...)
}

func (s *testSource) set(roots ...Root) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.roots = roots
}

var registerTestSource = sync.OnceValue(func() *testSource {
	source := &testSource{}
	RegisterSource(source)
	return source
})

func writeFiles(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for name, body := range files {
		target := filepath.Join(root, filepath.FromSlash(name))
		require.NoError(t, os.MkdirAll(filepath.Dir(target), 0o755))
		require.NoError(t, os.WriteFile(target, []byte(body), 0o644))
	}
}

func TestBuiltinTemplatesPassValidation(t *testing.T) {
	for _, kind := range Kinds {
		entries, err := fs.ReadDir(templ.DistFS, kind)
		require.NoError(t, err)
		for _, entry := range entries {
			assert.NoError(t, ValidateFile(templ.DistFS, kind, entry.Name()), "%s/%s", kind, entry.Name())
		}
	}
}

func TestBuiltinTemplatesAreTagged(t *testing.T) {
	registerTestSource().set()
	list, err := GetTemplateList(KindBlock)
	require.NoError(t, err)
	require.NotEmpty(t, list)
	for _, item := range list {
		assert.Equal(t, OriginBuiltin, item.Origin)
		assert.Empty(t, item.PluginID)
	}
	hsts := GetTemplateInfo(KindBlock, "hsts.conf")
	assert.Equal(t, "HSTS", hsts.Name)
	assert.Equal(t, "hsts.conf", hsts.Filename)
}

func TestPluginTemplatesJoinTheLists(t *testing.T) {
	source := registerTestSource()
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"block/deny-dotfiles.conf": pluginBlock,
		"block/broken.conf":        "no header\n",
		"block/notes.txt":          "ignored",
	})
	source.set(Root{PluginID: "io.github.example.snippets", Dir: root})
	t.Cleanup(func() { source.set() })

	list, err := GetTemplateList(KindBlock)
	require.NoError(t, err)
	var fromPlugin []ConfigInfoItem
	for _, item := range list {
		if item.Origin == OriginPlugin {
			fromPlugin = append(fromPlugin, item)
		}
	}
	require.Len(t, fromPlugin, 1, "a broken template is skipped")
	assert.Equal(t, "Deny dotfiles", fromPlugin[0].Name)
	assert.Equal(t, "io.github.example.snippets", fromPlugin[0].PluginID)
	assert.Equal(t, "deny-dotfiles.conf", fromPlugin[0].Filename, "the header cannot change the file name")

	detail, err := ParsePluginTemplate("io.github.example.snippets", KindBlock, "deny-dotfiles.conf", fromPlugin[0].Variables)
	require.NoError(t, err)
	require.Len(t, detail.Locations, 1)
	assert.Contains(t, detail.Locations[0].Content, "return 404")
	assert.Equal(t, "# Denies 404", detail.Custom)

	// Unknown plugins, unsafe names and a built-in name are refused.
	_, err = GetPluginTemplateInfo("io.github.other", KindBlock, "deny-dotfiles.conf")
	assert.ErrorIs(t, err, ErrTemplateNotFound)
	_, err = ParsePluginTemplate("io.github.example.snippets", KindBlock, "../block/deny-dotfiles.conf", nil)
	assert.ErrorIs(t, err, ErrTemplateNotFound)
	_, err = GetPluginTemplateInfo("io.github.example.snippets", KindBlock, "hsts.conf")
	assert.Error(t, err)
}

func TestPluginTemplatesDoNotFollowSymlinks(t *testing.T) {
	source := registerTestSource()
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside.conf")
	require.NoError(t, os.WriteFile(outside, []byte(pluginBlock), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(root, KindBlock), 0o755))
	require.NoError(t, os.Symlink(outside, filepath.Join(root, KindBlock, "linked.conf")))
	source.set(Root{PluginID: "io.github.example.snippets", Dir: root})
	t.Cleanup(func() { source.set() })

	_, err := GetPluginTemplateInfo("io.github.example.snippets", KindBlock, "linked.conf")
	assert.ErrorIs(t, err, ErrTemplateNotFound)
	assert.Error(t, ValidateFile(os.DirFS(root), KindBlock, "linked.conf"))
	for _, item := range pluginTemplateList(KindBlock) {
		assert.NotEqual(t, "linked.conf", item.Filename)
	}
}

func TestPluginTemplatesAreRestricted(t *testing.T) {
	root := t.TempDir()
	header := "# Nginx UI Template Start\nname = \"x\"\n# Nginx UI Template End\n"
	cases := map[string]string{
		"range.conf":    header + "{{range $i := 3}}deny all;{{end}}\n",
		"printf.conf":   header + "{{printf \"%s\" \"x\"}}\n",
		"define.conf":   header + "{{define \"x\"}}deny all;{{end}}\n",
		"call.conf":     header + "{{call .HTTPPORT}}\n",
		"template.conf": header + "{{template \"x\"}}\n",
		"huge.conf":     header + "{{if true}}" + strings.Repeat("# padding padding padding padding\n", 40000) + "{{end}}\n",
		"allowed.conf":  header + "{{if and .HTTPPORT (ne .HTTP01PORT \"\")}}listen 80;{{else}}listen 81;{{end}}\n",
	}
	files := map[string]string{}
	for name, body := range cases {
		files["block/"+name] = body
	}
	writeFiles(t, root, files)
	fsys := os.DirFS(root)

	for name := range cases {
		err := ValidateFile(fsys, KindBlock, name)
		if name == "allowed.conf" {
			assert.NoError(t, err, name)
			continue
		}
		assert.Error(t, err, name)
	}
}

func TestIsValidFileName(t *testing.T) {
	for _, name := range []string{"a.conf", "cache-static.conf", "v1.2_x.conf"} {
		assert.True(t, IsValidFileName(name), name)
	}
	for _, name := range []string{"", ".conf", "../a.conf", "a/b.conf", "a.txt", ".hidden.conf", "a b.conf"} {
		assert.False(t, IsValidFileName(name), name)
	}
}

func TestNginxUIListenerTemplate(t *testing.T) {
	previous := *settings.ListenerSettings
	previousServer := *cosysettings.ServerSettings
	t.Cleanup(func() { *settings.ListenerSettings = previous; *cosysettings.ServerSettings = previousServer })
	cosysettings.ServerSettings.Port = 9000

	cases := []struct {
		socket string
		https  bool
		want   string
	}{
		{"", false, "proxy_pass http://127.0.0.1:9000/;"},
		{"", true, "proxy_pass https://127.0.0.1:9000/;"},
		{"/run/nginx-ui/nginx-ui.sock", false, "proxy_pass http://unix:/run/nginx-ui/nginx-ui.sock:/;"},
		{"/run/nginx-ui/nginx-ui.sock", true, "proxy_pass https://unix:/run/nginx-ui/nginx-ui.sock:/;"},
	}
	for _, tc := range cases {
		settings.ListenerSettings.UnixSocket = tc.socket
		cosysettings.ServerSettings.EnableHTTPS = tc.https
		result, err := ParseTemplate("block", "nginx-ui.conf", nil)
		require.NoError(t, err)
		require.Len(t, result.Locations, 1)
		content := result.Locations[0].Content
		require.Contains(t, content, tc.want)
		if tc.socket != "" {
			require.NotContains(t, content, "127.0.0.1:9000")
		}
		require.NotContains(t, content, `"`)
	}
}

func TestBuiltinBlockSource(t *testing.T) {
	info, body, err := BuiltinBlockSource("hsts.conf")
	require.NoError(t, err)
	require.NotEmpty(t, info.Name)
	require.NotEmpty(t, info.Variables)
	require.NotContains(t, body, HeaderStart)
	require.NotContains(t, body, HeaderEnd)
	require.Contains(t, body, "Strict-Transport-Security")

	for _, name := range []string{"../config/nginx.conf", "missing.conf", "hsts"} {
		_, _, err := BuiltinBlockSource(name)
		require.ErrorIs(t, err, ErrBuiltinNotFound, name)
	}
}

func TestTrimActionLines(t *testing.T) {
	content := "location / {\n    {{ if .keep }}\n    return 301 x$request_uri;\n    {{- else }}\n    return 301 x;\n    {{ end }}{{/* done */}}\n    add_header A {{ .a }};\n}\n"
	rendered, err := RenderText("t", content, map[string]Variable{"keep": {Value: false}, "a": {Value: "b"}})
	require.NoError(t, err)
	require.Equal(t, "location / {\n    return 301 x;\n    add_header A b;\n}\n", rendered)

	rendered, err = RenderText("t", "{{ if .keep }}\nkeep;\n{{ end }}\nlast;\n", map[string]Variable{"keep": {Value: true}})
	require.NoError(t, err)
	require.Equal(t, "keep;\nlast;\n", rendered)

	// Actions written without spaces inside the braces are trimmed as well.
	rendered, err = RenderText("t", "{{if .keep}}\nkeep;\n{{else}}\ndrop;\n{{end}}\n{{/* done */}}\nlast;\n", map[string]Variable{"keep": {Value: true}})
	require.NoError(t, err)
	require.Equal(t, "keep;\nlast;\n", rendered)

	rendered, err = RenderText("t", "{{range $i := 2}}deny all;\n{{end}}\n", nil)
	require.NoError(t, err)
	require.Equal(t, "deny all;deny all;\n", rendered)

	rendered, err = RenderText("t", "{{- if .keep -}}\nkeep;\n{{- end }}\n", map[string]Variable{"keep": {Value: true}})
	require.NoError(t, err)
	require.Equal(t, "keep;\n", rendered)

	// An action inside a line, or one that prints a value, keeps the line.
	require.Equal(t, "gzip {{ if .g }}on{{ else }}off{{ end }};", TrimActionLines("gzip {{ if .g }}on{{ else }}off{{ end }};"))
	require.Equal(t, "server {\n    {{ .extra }}\n}", TrimActionLines("server {\n    {{ .extra }}\n}"))
}
