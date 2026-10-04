package plugin

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/plugin/protocol"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testBlockTemplate = `# Nginx UI Template Start
name = "Cache static files"
author = "@example"
description = { en = "Cache images and scripts", zh_CN = "缓存静态文件" }

[variables.expires]
type = "string"
name = { en = "Expires" }
value = "30d"

[variables.public]
type = "boolean"
name = { en = "Public" }
value = true
# Nginx UI Template End
location ~* \.(?:css|js|png)$ {
    expires {{.expires}};
    {{if .public}}add_header Cache-Control "public";{{end}}
}
`

const testConfTemplate = `# Nginx UI Template Start
name = "Ghost"
author = "@example"
# Nginx UI Template End
location / {
    proxy_pass http://127.0.0.1:2368;
}
`

const testPOFile = `msgid ""
msgstr ""
"Language: de_DE\n"
"Content-Type: text/plain; charset=UTF-8\n"

msgid "Cache static files"
msgstr "Statische Dateien zwischenspeichern"

#, fuzzy
msgid "Draft"
msgstr "Entwurf"
`

// contentManifest is a plugin without a process that ships templates and
// translations.
func contentManifest(id string) *protocol.Manifest {
	return &protocol.Manifest{
		ID:         id,
		Name:       id,
		Version:    "1.0.0",
		APIVersion: protocol.APIVersion,
		Content:    &protocol.ManifestContent{Templates: "templates", Locales: "locales"},
	}
}

// contentFiles is a valid templates and locales tree.
func contentFiles() map[string]string {
	return map[string]string{
		"templates/block/cache-static.conf": testBlockTemplate,
		"templates/conf/ghost.conf":         testConfTemplate,
		"locales/de_DE.po":                  testPOFile,
		"README.md":                         "# Snippets\n\n## Permissions\n\nNone.\n\n## Supported platforms\n\nAll.\n",
	}
}

// writeContentDir materialises a plugin directory with the given files.
func writeContentDir(t *testing.T, manifest *protocol.Manifest, files map[string]string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), manifest.ID)
	writePluginDir(t, dir, manifest)
	for name, body := range files {
		target := filepath.Join(dir, filepath.FromSlash(name))
		require.NoError(t, os.MkdirAll(filepath.Dir(target), 0o755))
		require.NoError(t, os.WriteFile(target, []byte(body), 0o644))
	}
	return dir
}

func problemRules(problems []ContentProblem, level Level) []string {
	var rules []string
	for _, problem := range problems {
		if problem.Level == level {
			rules = append(rules, problem.Rule)
		}
	}
	return rules
}

func TestCheckContentAcceptsValidFiles(t *testing.T) {
	dir := writeContentDir(t, contentManifest("io.github.example.snippets"), contentFiles())
	problems := CheckContent(contentManifest("io.github.example.snippets"), dir)
	assert.Empty(t, problemRules(problems, LevelError), "%v", problems)
	assert.Empty(t, problemRules(problems, LevelWarning), "%v", problems)
}

func TestCheckContentReportsBrokenFiles(t *testing.T) {
	manifest := contentManifest("io.github.example.snippets")

	tests := []struct {
		name  string
		edit  func(files map[string]string)
		level Level
		rule  string
	}{
		{"missing markers", func(f map[string]string) { f["templates/block/bad.conf"] = "location / {}\n" }, LevelError, RuleContentTemplate},
		{"invalid toml", func(f map[string]string) {
			f["templates/block/bad.conf"] = "# Nginx UI Template Start\nname = \n# Nginx UI Template End\n"
		}, LevelError, RuleContentTemplate},
		{"loop", func(f map[string]string) {
			f["templates/block/bad.conf"] = "# Nginx UI Template Start\nname = \"x\"\n# Nginx UI Template End\n{{range $i := 1000000000}}{{end}}\n"
		}, LevelError, RuleContentTemplate},
		{"printf", func(f map[string]string) {
			f["templates/block/bad.conf"] = "# Nginx UI Template Start\nname = \"x\"\n# Nginx UI Template End\n{{printf \"%1000000000d\" 1}}\n"
		}, LevelError, RuleContentTemplate},
		{"nginx syntax", func(f map[string]string) {
			f["templates/block/bad.conf"] = "# Nginx UI Template Start\nname = \"x\"\n# Nginx UI Template End\nlocation / {\n"
		}, LevelError, RuleContentTemplate},
		{"unknown variable type", func(f map[string]string) {
			f["templates/block/bad.conf"] = "# Nginx UI Template Start\nname = \"x\"\n[variables.a]\ntype = \"file\"\nvalue = \"b\"\n# Nginx UI Template End\n"
		}, LevelError, RuleContentTemplate},
		{"no name", func(f map[string]string) {
			f["templates/block/anonymous.conf"] = "# Nginx UI Template Start\nauthor = \"x\"\n# Nginx UI Template End\nclient_max_body_size 1m;\n"
		}, LevelWarning, RuleContentTemplate},
		{"stray file", func(f map[string]string) { f["templates/block/notes.txt"] = "x" }, LevelWarning, RuleContentTemplates},
		{"stray directory", func(f map[string]string) { f["templates/http/a.conf"] = testBlockTemplate }, LevelWarning, RuleContentTemplates},
		{"no template", func(f map[string]string) {
			delete(f, "templates/block/cache-static.conf")
			delete(f, "templates/conf/ghost.conf")
			f["templates/README"] = "empty"
		}, LevelError, RuleContentTemplates},
		{"language this host lacks", func(f map[string]string) { f["locales/xx_XX.po"] = testPOFile }, LevelWarning, RuleContentLocales},
		{"file named in another spelling", func(f map[string]string) { f["locales/de-DE.po"] = testPOFile }, LevelError, RuleContentLocales},
		{"stray locale file", func(f map[string]string) { f["locales/de_DE.mo"] = "x" }, LevelWarning, RuleContentLocales},
		{"no locale", func(f map[string]string) {
			delete(f, "locales/de_DE.po")
			f["locales/README"] = "empty"
		}, LevelError, RuleContentLocales},
		{"broken po", func(f map[string]string) { f["locales/de_DE.po"] = "this is not a catalog\n" }, LevelError, RuleContentLocale},
		{"po without header", func(f map[string]string) { f["locales/de_DE.po"] = "msgid \"a\"\nmsgstr \"b\"\n" }, LevelError, RuleContentLocale},
		{"po entry without msgstr", func(f map[string]string) {
			f["locales/de_DE.po"] = "msgid \"\"\nmsgstr \"\"\n\nmsgid \"a\"\n"
		}, LevelError, RuleContentLocale},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			files := contentFiles()
			tc.edit(files)
			dir := writeContentDir(t, manifest, files)
			problems := CheckContent(manifest, dir)
			assert.Contains(t, problemRules(problems, tc.level), tc.rule, "%v", problems)
		})
	}

	// A declared directory that is missing.
	dir := writeContentDir(t, manifest, map[string]string{})
	problems := CheckContent(manifest, dir)
	assert.Equal(t, []string{RuleContentTemplates, RuleContentLocales}, problemRules(problems, LevelError), "%v", problems)
}

func TestCheckContentRefusesSymlinkedTemplates(t *testing.T) {
	manifest := contentManifest("io.github.example.snippets")
	files := contentFiles()
	dir := writeContentDir(t, manifest, files)
	outside := filepath.Join(t.TempDir(), "outside.conf")
	require.NoError(t, os.WriteFile(outside, []byte(testBlockTemplate), 0o644))
	require.NoError(t, os.Symlink(outside, filepath.Join(dir, "templates", "block", "linked.conf")))

	problems := CheckContent(manifest, dir)
	assert.Contains(t, problemRules(problems, LevelWarning), RuleContentTemplates, "%v", problems)
}

func TestValidateManifestRejectsProcessWorkWithoutServer(t *testing.T) {
	manifest := contentManifest("io.github.example.snippets")
	require.NoError(t, ValidateManifest(manifest))

	withCapability := contentManifest("io.github.example.snippets")
	withCapability.Capabilities = []string{protocol.CapabilityNotify}
	withCapability.Notify = &protocol.ManifestNotify{Channels: []protocol.NotifyChannel{{Code: "chat", Name: "Chat"}}}
	assertPluginError(t, ValidateManifest(withCapability), ErrManifestInvalid)

	withCron := contentManifest("io.github.example.snippets")
	withCron.Cron = []protocol.ManifestCron{{ID: "sync", Schedule: "@every 1h", Method: "sync"}}
	assertPluginError(t, ValidateManifest(withCron), ErrManifestInvalid)

	withEvents := contentManifest("io.github.example.snippets")
	withEvents.Events = []string{"cert.issued"}
	assertPluginError(t, ValidateManifest(withEvents), ErrManifestInvalid)
}

func TestLintReportsContentFindings(t *testing.T) {
	manifest := contentManifest("io.github.example.snippets")
	manifest.Capabilities = []string{protocol.CapabilityProbe}
	files := contentFiles()
	files["locales/xx_XX.po"] = testPOFile
	dir := writeContentDir(t, manifest, files)

	report, err := Lint(dir)
	require.NoError(t, err)
	rules := map[string]bool{}
	for _, finding := range report.Findings {
		rules[finding.Rule] = true
	}
	assert.True(t, rules[RuleContentProcessLess], "%+v", report.Findings)
	assert.True(t, rules[RuleContentLocales], "%+v", report.Findings)
}

func TestInstallValidatesContent(t *testing.T) {
	settings.PluginSettings.Enabled = true
	m := newTestManager(t)

	files := contentFiles()
	files["templates/block/bad.conf"] = "location / {}\n"
	archive := buildTestPackage(t, contentManifest("io.github.example.snippets"), files)
	_, err := m.Install(context.Background(), archive, InstallOptions{Enable: true})
	assertPluginError(t, err, ErrContentInvalid)
	_, err = os.Stat(filepath.Join(m.Dir(), "io.github.example.snippets"))
	assert.True(t, os.IsNotExist(err), "a refused package leaves nothing behind")
}

func TestContentPluginHasNoProcess(t *testing.T) {
	settings.PluginSettings.Enabled = true
	m := newTestManager(t)

	archive := buildTestPackage(t, contentManifest("io.github.example.snippets"), contentFiles())
	info, err := m.Install(context.Background(), archive, InstallOptions{Enable: true})
	require.NoError(t, err)
	assert.False(t, info.HasServer)
	assert.Equal(t, StatusRunning, info.Status)

	assert.Nil(t, supervisorOf(m, "io.github.example.snippets"), "a content plugin never gets a supervisor")

	entries := m.ContentEntries()
	require.Len(t, entries, 1)
	assert.Equal(t, "io.github.example.snippets", entries[0].PluginID)
	assert.DirExists(t, entries[0].TemplatesDir)
	assert.DirExists(t, entries[0].LocalesDir)

	catalog, errs := LoadLocales(entries[0].LocalesDir)
	assert.Empty(t, errs)
	assert.Equal(t, "Statische Dateien zwischenspeichern", catalog["de_DE"]["Cache static files"])
	_, fuzzy := catalog["de_DE"]["Draft"]
	assert.False(t, fuzzy, "fuzzy entries are skipped")

	info, err = m.Disable(context.Background(), "io.github.example.snippets")
	require.NoError(t, err)
	assert.Equal(t, StatusInstalled, info.Status)
	assert.Empty(t, m.ContentEntries())

	info, err = m.Enable(context.Background(), "io.github.example.snippets", false)
	require.NoError(t, err)
	assert.Equal(t, StatusRunning, info.Status)
	assert.Len(t, m.ContentEntries(), 1)
}

func TestConformanceChecksContentPluginsStatically(t *testing.T) {
	manifest := contentManifest("io.github.example.snippets")
	dir := writeContentDir(t, manifest, contentFiles())

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	report, err := Conformance(ctx, dir, ConformanceOptions{HandshakeTimeout: testHandshakeTimeout})
	require.NoError(t, err)
	assert.True(t, report.Passed(), "%+v", report.Cases)

	var rules []string
	for _, c := range report.Cases {
		rules = append(rules, c.Rule)
	}
	assert.True(t, slices.Contains(rules, RuleContentTemplate), "%v", rules)
	assert.True(t, slices.Contains(rules, RuleContentLocale), "%v", rules)
	assert.False(t, slices.Contains(rules, RuleLifecycleHandshake), "no process is started: %v", rules)

	files := contentFiles()
	files["locales/de_DE.po"] = "broken\n"
	dir = writeContentDir(t, manifest, files)
	report, err = Conformance(ctx, dir, ConformanceOptions{HandshakeTimeout: testHandshakeTimeout})
	require.NoError(t, err)
	assert.False(t, report.Passed())
}
