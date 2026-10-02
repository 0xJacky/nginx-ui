package snippet

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/0xJacky/Nginx-UI/internal/template"
	"github.com/0xJacky/Nginx-UI/internal/testdb"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uozi-tech/cosy"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// setupSnippetTest points nginx at a temporary configuration directory whose
// test and reload always succeed, with an in-memory database for the
// configuration records Save keeps.
func setupSnippetTest(t *testing.T) string {
	return setupSnippetTestB(t)
}

func setupSnippetTestB(t testing.TB) string {
	t.Helper()
	confDir := t.TempDir()

	original := *settings.NginxSettings
	settings.NginxSettings.ConfigDir = confDir
	settings.NginxSettings.PIDPath = filepath.Join(confDir, "nginx.pid")
	settings.NginxSettings.TestConfigCmd = "true"
	settings.NginxSettings.ReloadCmd = "true"
	settings.NginxSettings.RestartCmd = "true"
	require.NoError(t, os.WriteFile(settings.NginxSettings.PIDPath, []byte(strconv.Itoa(os.Getpid())), 0o644))
	t.Cleanup(func() { *settings.NginxSettings = original })

	db, err := gorm.Open(sqlite.Open(testdb.DSN(t)), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Config{}, &model.ConfigBackup{}, &model.Node{}))
	model.Use(db)
	query.Use(db)
	query.SetDefault(db)
	return confDir
}

func writeFile(t *testing.T, path, content string) { writeFileB(t, path, content) }

func writeFileB(t testing.TB, path, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}

func TestHeaderIsWrittenAsComments(t *testing.T) {
	h := header{
		Name:        "Static cache",
		Author:      "admin",
		Description: map[string]string{"en": "Cache static files"},
		Variables: map[string]template.Variable{
			"days": {Type: "string", Name: map[string]string{"en": "Days"}, Value: "7"},
		},
	}
	content, err := compose(h, "expires {{.days}}d;\n\n\n")
	require.NoError(t, err)

	lines := strings.Split(string(content), "\n")
	assert.Equal(t, template.HeaderStart, lines[0])
	for _, line := range lines[1:] {
		if line == template.HeaderEnd {
			break
		}
		assert.True(t, strings.HasPrefix(line, "#"), "header line %q must be a comment", line)
	}
	assert.True(t, strings.HasSuffix(string(content), "\nexpires {{.days}}d;\n"))

	parsed, body, err := parse(content)
	require.NoError(t, err)
	assert.Equal(t, h, parsed)
	assert.Equal(t, "expires {{.days}}d;\n", body)
}

func TestParseWithoutHeader(t *testing.T) {
	h, body, err := parse([]byte("gzip on;\n"))
	require.NoError(t, err)
	assert.Equal(t, header{}, h)
	assert.Equal(t, "gzip on;\n", body)

	// A snippet without anything to describe needs no header.
	content, err := compose(header{}, "gzip on;")
	require.NoError(t, err)
	assert.Equal(t, "gzip on;\n", string(content))

	_, _, err = parse([]byte(template.HeaderStart + "\n# name = \"x\"\n"))
	assert.ErrorIs(t, err, errHeaderUnclosed)
}

func TestValidFile(t *testing.T) {
	for _, file := range []string{"gzip.conf", "a.b-c_d.conf", "1.conf"} {
		assert.True(t, ValidFile(file), file)
	}
	for _, file := range []string{"", "gzip", ".hidden.conf", "a/b.conf", "../a.conf", "a b.conf", strings.Repeat("a", 65) + ".conf"} {
		assert.False(t, ValidFile(file), file)
	}
}

func TestSaveGetAndList(t *testing.T) {
	confDir := setupSnippetTest(t)
	ctx := context.Background()

	saved, err := Save(ctx, SaveParams{
		File: "cache.conf", Names: map[string]string{"en": " Static cache "}, Description: map[string]string{"en": "Cache static files", "zh_CN": " "},
		Content: "expires 7d;", Create: true,
	}, "admin")
	require.NoError(t, err)
	assert.Equal(t, "Static cache", saved.Name)
	assert.Equal(t, map[string]string{"en": "Cache static files"}, saved.Description)
	assert.Equal(t, "include snippets/cache.conf;", saved.Include)
	assert.Equal(t, "expires 7d;\n", saved.Content)

	_, err = Save(ctx, SaveParams{File: "cache.conf", Content: "expires 1d;", Create: true}, "admin")
	assert.ErrorIs(t, err, ErrAlreadyExists)

	// Variables and the author of a header written by hand survive an edit.
	writeFile(t, filepath.Join(confDir, "snippets", "hsts.conf"), template.HeaderStart+`
# name = "HSTS"
# author = "ops"
# [variables.maxAge]
# type = "string"
# value = "31536000"
`+template.HeaderEnd+`
add_header Strict-Transport-Security "max-age={{.maxAge}}";
`)
	edited, err := Save(ctx, SaveParams{File: "hsts.conf", Names: map[string]string{"en": "Strict transport"}, Content: `add_header Strict-Transport-Security "max-age={{.maxAge}}" always;`}, "admin")
	require.NoError(t, err)
	assert.Equal(t, "ops", edited.Author)
	assert.Equal(t, "31536000", edited.Variables["maxAge"].Value)

	// Files of other software and other kinds are left out of the list.
	writeFile(t, filepath.Join(confDir, "snippets", "fastcgi-php.conf"), "fastcgi_index index.php;\n")
	writeFile(t, filepath.Join(confDir, "snippets", "notes.txt"), "not a snippet")
	writeFile(t, filepath.Join(confDir, "snippets", "plugins", "x", "a.conf"), "# owned by a plugin\n")

	snippets, err := List()
	require.NoError(t, err)
	var files []string
	for _, s := range snippets {
		files = append(files, s.File)
	}
	assert.Equal(t, []string{"cache.conf", "fastcgi-php.conf", "hsts.conf"}, files)
	assert.Equal(t, "fastcgi-php", snippets[1].Name, "a snippet without a header is named after its file")
	assert.Empty(t, snippets[0].Content, "the list carries no bodies")
}

func TestSaveReplacesAuthorAndVariables(t *testing.T) {
	confDir := setupSnippetTest(t)
	ctx := context.Background()
	author := " ops "

	saved, err := Save(ctx, SaveParams{
		File: "redirect.conf", Names: map[string]string{"en": "Redirect"}, Author: &author, Create: true,
		Variables: map[string]template.Variable{
			"status": {Type: VariableSelect, Name: map[string]string{"en": " Status ", "zh_CN": ""}, Value: "301", Mask: map[string]map[string]string{
				"301": {"en": "Moved Permanently"},
				"302": {"en": "Found"},
			}},
			"target": {Type: VariableString, Name: map[string]string{"en": "Target"}},
			"keep":   {Type: VariableBoolean, Value: true},
		},
		Content: "return {{ .status }} {{ .target }};",
	}, "admin")
	require.NoError(t, err)
	assert.Equal(t, "ops", saved.Author)
	assert.Equal(t, map[string]string{"en": "Status"}, saved.Variables["status"].Name)
	assert.Equal(t, "", saved.Variables["target"].Value)
	assert.Equal(t, true, saved.Variables["keep"].Value)

	raw, err := os.ReadFile(filepath.Join(confDir, "snippets", "redirect.conf"))
	require.NoError(t, err)
	assert.Contains(t, string(raw), "# [variables.status]")
	assert.Contains(t, string(raw), "# type = \"select\"")

	// An empty set removes the variables.
	edited, err := Save(ctx, SaveParams{File: "redirect.conf", Names: map[string]string{"en": "Redirect"}, Variables: map[string]template.Variable{}, Content: "return 301 https://example.com;"}, "admin")
	require.NoError(t, err)
	assert.Empty(t, edited.Variables)
	assert.Equal(t, "ops", edited.Author, "a nil author keeps the current one")
}

func TestNamesPerLanguage(t *testing.T) {
	confDir := setupSnippetTest(t)
	ctx := context.Background()
	read := func(file string) string {
		raw, err := os.ReadFile(filepath.Join(confDir, "snippets", file))
		require.NoError(t, err)
		return string(raw)
	}

	// An English only name stays a plain string, as in the built-in templates.
	saved, err := Save(ctx, SaveParams{File: "a.conf", Names: map[string]string{"en": "Gzip"}, Content: "gzip on;", Create: true}, "admin")
	require.NoError(t, err)
	assert.Contains(t, read("a.conf"), `# name = "Gzip"`)
	assert.Equal(t, "Gzip", saved.Name)
	assert.Equal(t, map[string]string{"en": "Gzip"}, saved.NameI18n)

	// Other languages make it a table, and the name to show falls back to
	// the English one.
	saved, err = Save(ctx, SaveParams{File: "b.conf", Names: map[string]string{"en": "Gzip", "zh_CN": "压缩", "ja_JP": " "}, Content: "gzip on;", Create: true}, "admin")
	require.NoError(t, err)
	assert.Contains(t, read("b.conf"), "# [name]")
	assert.Equal(t, "Gzip", saved.Name)
	assert.Equal(t, map[string]string{"en": "Gzip", "zh_CN": "压缩"}, saved.NameI18n)

	saved, err = Save(ctx, SaveParams{File: "c.conf", Names: map[string]string{"zh_CN": "压缩"}, Content: "gzip on;", Create: true}, "admin")
	require.NoError(t, err)
	assert.Equal(t, "压缩", saved.Name)
	assert.Equal(t, map[string]string{"zh_CN": "压缩"}, saved.NameI18n)

	// Without a name the file name is shown.
	saved, err = Save(ctx, SaveParams{File: "d.conf", Content: "gzip on;", Create: true}, "admin")
	require.NoError(t, err)
	assert.Equal(t, "d", saved.Name)
	assert.Empty(t, saved.NameI18n)
}

func TestSaveRejectsInvalidVariables(t *testing.T) {
	setupSnippetTest(t)
	cases := map[string]struct {
		vars    map[string]template.Variable
		content string
		want    error
	}{
		"bad key":          {map[string]template.Variable{"max-age": {Type: VariableString}}, "x;", ErrInvalidVariable},
		"unknown type":     {map[string]template.Variable{"a": {Type: "number"}}, "x;", ErrInvalidVariable},
		"boolean as text":  {map[string]template.Variable{"a": {Type: VariableBoolean, Value: "yes"}}, "x;", ErrInvalidVariable},
		"select no option": {map[string]template.Variable{"a": {Type: VariableSelect, Value: "1"}}, "x;", ErrInvalidVariable},
		"select default":   {map[string]template.Variable{"a": {Type: VariableSelect, Value: "3", Mask: map[string]map[string]string{"1": {}}}}, "x;", ErrInvalidVariable},
		"broken template":  {map[string]template.Variable{"a": {Type: VariableString}}, "return {{ .a ;", ErrInvalidTemplate},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := Save(context.Background(), SaveParams{File: "x.conf", Variables: tc.vars, Content: tc.content, Create: true}, "admin")
			var cosyErr *cosy.Error
			require.ErrorAs(t, err, &cosyErr)
			assert.Equal(t, tc.want.(*cosy.Error).Code, cosyErr.Code)
		})
	}
}

func TestSaveKeepsThePreviousSnippetWhenNginxRejectsIt(t *testing.T) {
	confDir := setupSnippetTest(t)
	file := filepath.Join(confDir, "snippets", "cache.conf")
	writeFile(t, file, "expires 7d;\n")

	settings.NginxSettings.TestConfigCmd = "echo 'nginx: [emerg] unknown directive' >&2; exit 1"
	_, err := Save(context.Background(), SaveParams{File: "cache.conf", Content: "nope;"}, "admin")
	require.Error(t, err)

	content, err := os.ReadFile(file)
	require.NoError(t, err)
	assert.Equal(t, "expires 7d;\n", string(content))
}

func TestIncludedFiles(t *testing.T) {
	confDir := setupSnippetTest(t)
	writeFile(t, filepath.Join(confDir, "snippets", "wild-a.conf"), "")
	writeFile(t, filepath.Join(confDir, "snippets", "wild-b.conf"), "")

	content := fmt.Sprintf(`server {
    include snippets/cache.conf;
    include "%s/snippets/headers.conf";
    include 'snippets/quoted.conf';
    # include snippets/commented.conf;
    include snippets/wild-*.conf;
    include snippets/plugins/x/a.conf;
    include conf.d/other.conf;
}`, filepath.ToSlash(confDir))

	assert.Equal(t, []string{"cache.conf", "headers.conf", "quoted.conf", "wild-a.conf", "wild-b.conf"}, IncludedFiles(content))
	assert.Empty(t, IncludedFiles("server { listen 80; }"))
}

type fakeRemover struct {
	removed []string
	err     error
}

func (r *fakeRemover) Remove(path string) (bool, error) {
	if r.err != nil {
		return false, r.err
	}
	if _, err := os.Stat(path); err != nil {
		return false, nil
	}
	r.removed = append(r.removed, path)
	return true, os.Remove(path)
}

func TestDeleteRefusesASnippetInUse(t *testing.T) {
	confDir := setupSnippetTest(t)
	writeFile(t, filepath.Join(confDir, "snippets", "cache.conf"), "expires 7d;\n")
	writeFile(t, filepath.Join(confDir, "snippets", "unused.conf"), "gzip on;\n")
	writeFile(t, filepath.Join(confDir, "sites-available", "example.com"), "server {\n    include snippets/cache.conf;\n}\n")
	writeFile(t, filepath.Join(confDir, "streams-available", "dns"), "server {\n    include snippets/cache.conf;\n}\n")

	got, err := Get("cache.conf")
	require.NoError(t, err)
	assert.Equal(t, []string{"sites-available/example.com", "streams-available/dns"}, got.UsedBy)

	remover := &fakeRemover{}
	_, err = Delete("cache.conf", remover)
	var cosyErr *cosy.Error
	require.ErrorAs(t, err, &cosyErr)
	assert.Equal(t, ErrInUse.(*cosy.Error).Code, cosyErr.Code)
	assert.Contains(t, err.Error(), "sites-available/example.com")
	assert.FileExists(t, filepath.Join(confDir, "snippets", "cache.conf"))

	removed, err := Delete("unused.conf", remover)
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(confDir, "snippets", "unused.conf"), removed)

	_, err = Delete("unused.conf", remover)
	assert.ErrorIs(t, err, ErrNotFound)

	// A rejected removal is reported and the file stays.
	remover.err = errors.New("nginx test failed")
	_, err = Delete("cache-other.conf", remover)
	assert.Error(t, err)
}

func TestRenderAsBlockTemplate(t *testing.T) {
	confDir := setupSnippetTest(t)
	writeFile(t, filepath.Join(confDir, "snippets", "plain.conf"), "gzip on;\nlocation /static/ {\n    expires 7d;\n}\n")
	writeFile(t, filepath.Join(confDir, "snippets", "vars.conf"), template.HeaderStart+`
# [variables.days]
# type = "string"
# value = "7"
`+template.HeaderEnd+`
expires {{.days}}d;
`)

	detail, err := Render("plain.conf", nil)
	require.NoError(t, err)
	require.Len(t, detail.Directives, 1)
	assert.Equal(t, "gzip", detail.Directives[0].Directive)
	require.Len(t, detail.Locations, 1)
	assert.Equal(t, "/static/", detail.Locations[0].Path)

	detail, err = Render("vars.conf", map[string]template.Variable{"days": {Value: "30"}})
	require.NoError(t, err)
	require.Len(t, detail.Directives, 1)
	assert.Equal(t, "30d", detail.Directives[0].Params)

	items := TemplateInfo()
	require.Len(t, items, 2)
	assert.Equal(t, template.OriginCustom, items[0].Origin)
	assert.Equal(t, "plain", items[0].Name)
}

func TestNginxRemoverPutsTheFileBackWhenNginxRejectsIt(t *testing.T) {
	confDir := setupSnippetTest(t)
	file := filepath.Join(confDir, "snippets", "cache.conf")
	writeFile(t, file, "expires 7d;\n")

	settings.NginxSettings.TestConfigCmd = "echo 'nginx: [emerg] open() failed' >&2; exit 1"
	removed, err := NginxRemover{}.Remove(file)
	require.Error(t, err)
	assert.False(t, removed)
	assert.FileExists(t, file)

	settings.NginxSettings.TestConfigCmd = "true"
	removed, err = NginxRemover{}.Remove(file)
	require.NoError(t, err)
	assert.True(t, removed)
	assert.NoFileExists(t, file)

	removed, err = NginxRemover{}.Remove(file)
	require.NoError(t, err)
	assert.False(t, removed)
}

// useScannerIndex resets the index the config scanner feeds and marks it
// ready, as after the first scan.
func useScannerIndex(t *testing.T) {
	t.Helper()
	reset := func(ready bool) {
		usage.Lock()
		usage.includes = map[string][]string{}
		usage.headers = map[string]header{}
		usage.ready = ready
		usage.Unlock()
	}
	reset(true)
	t.Cleanup(func() { reset(false) })
}

func TestUsageComesFromTheScannerOnceItIsReady(t *testing.T) {
	confDir := setupSnippetTest(t)
	writeFile(t, filepath.Join(confDir, "snippets", "cache.conf"), template.HeaderStart+"\n# name = \"On disk\"\n"+template.HeaderEnd+"\nexpires 7d;\n")
	writeFile(t, filepath.Join(confDir, "snippets", "wild-a.conf"), "")
	site := filepath.Join(confDir, "sites-available", "example.com")
	writeFile(t, site, "server {\n    include snippets/cache.conf;\n}\n")

	// Before the scanner is ready the files are read directly.
	assert.Equal(t, []string{"sites-available/example.com"}, usageIndex()[IncludePath("cache.conf")])

	useScannerIndex(t)
	require.NoError(t, scanIncludes(site, []byte("server {\n    include snippets/cache.conf;\n    include snippets/wild-*.conf;\n}\n")))
	require.NoError(t, scanIncludes(filepath.Join(confDir, "conf.d", "a.conf"), []byte("include snippets/cache.conf;\n")))
	require.NoError(t, scanIncludes(filepath.Join(confDir, "snippets", "cache.conf"),
		[]byte(template.HeaderStart+"\n# name = \"From the scanner\"\n"+template.HeaderEnd+"\nexpires 7d;\n")))
	require.NoError(t, scanIncludes("/elsewhere/x.conf", []byte("include snippets/cache.conf;\n")))

	index := usageIndex()
	assert.Equal(t, []string{"conf.d/a.conf", "sites-available/example.com"}, index[IncludePath("cache.conf")])
	assert.Equal(t, []string{"sites-available/example.com"}, index[IncludePath("wild-a.conf")])

	snippets, err := List()
	require.NoError(t, err)
	assert.Equal(t, "From the scanner", snippets[0].Name, "the header comes from the scanner, not the disk")

	// A removed file arrives as empty content.
	require.NoError(t, scanIncludes(filepath.Join(confDir, "conf.d", "a.conf"), nil))
	assert.Equal(t, []string{"sites-available/example.com"}, usageIndex()[IncludePath("cache.conf")])

	// Deleting reads the files as they are now, whatever the scanner says.
	require.NoError(t, scanIncludes(site, nil))
	assert.Empty(t, usageIndex()[IncludePath("cache.conf")])
	_, err = Delete("cache.conf", &fakeRemover{})
	var cosyErr *cosy.Error
	require.ErrorAs(t, err, &cosyErr)
	assert.Equal(t, ErrInUse.(*cosy.Error).Code, cosyErr.Code)
}

func TestPreview(t *testing.T) {
	setupSnippetTest(t)
	vars := map[string]template.Variable{
		"status":   {Type: VariableSelect, Value: "302"},
		"keepPath": {Type: VariableBoolean, Value: false},
	}
	content := "location / {\n    return {{ .status }} https://example.com{{ if .keepPath }}$request_uri{{ end }};\n}\n"

	result, err := Preview(content, vars)
	require.NoError(t, err)
	assert.Empty(t, result.Error)
	assert.Contains(t, result.Content, "return 302 https://example.com;")

	result, err = Preview("return {{ .status ;", vars)
	require.NoError(t, err)
	assert.NotEmpty(t, result.Error, "a template that does not parse is reported")

	result, err = Preview("location / {\n    return {{ .status }};\n", vars)
	require.NoError(t, err)
	assert.Contains(t, result.Content, "return 302;")
	assert.NotEmpty(t, result.Error, "a result that is not nginx configuration is reported")
}

func TestCheckRename(t *testing.T) {
	confDir := setupSnippetTest(t)
	writeFile(t, filepath.Join(confDir, "snippets", "cache.conf"), "expires 7d;\n")
	writeFile(t, filepath.Join(confDir, "snippets", "unused.conf"), "gzip on;\n")
	writeFile(t, filepath.Join(confDir, "sites-available", "example.com"), "server {\n    include snippets/cache.conf;\n}\n")

	assert.NoError(t, CheckRename("unused.conf", "compression.conf"))
	assert.ErrorIs(t, CheckRename("unused.conf", "../escape.conf"), ErrInvalidFile)
	assert.ErrorIs(t, CheckRename("missing.conf", "other.conf"), ErrNotFound)

	// Sites name the old file, so an included snippet keeps it.
	err := CheckRename("cache.conf", "static-cache.conf")
	var cosyErr *cosy.Error
	require.ErrorAs(t, err, &cosyErr)
	assert.Equal(t, ErrInUse.(*cosy.Error).Code, cosyErr.Code)
}
