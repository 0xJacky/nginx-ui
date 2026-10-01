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

	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Config{}, &model.ConfigBackup{}, &model.Node{}))
	model.Use(db)
	query.Use(db)
	query.SetDefault(db)
	return confDir
}

func writeFile(t *testing.T, path, content string) {
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
		File: "cache.conf", Name: " Static cache ", Description: map[string]string{"en": "Cache static files", "zh_CN": " "},
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
	edited, err := Save(ctx, SaveParams{File: "hsts.conf", Name: "Strict transport", Content: `add_header Strict-Transport-Security "max-age={{.maxAge}}" always;`}, "admin")
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
