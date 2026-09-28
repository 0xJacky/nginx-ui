package access_list

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// useConfDir points the Nginx configuration directory at a temporary one.
func useConfDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	previous := settings.NginxSettings.ConfigDir
	settings.NginxSettings.ConfigDir = dir
	t.Cleanup(func() { settings.NginxSettings.ConfigDir = previous })
	return dir
}

func TestReferencesIn(t *testing.T) {
	content := `server {
    server_name a.example.com;
    include nginx-ui/access/lan.conf;
    location /admin {
        include nginx-ui/access/office.conf;
        location /admin/deep {
            include /etc/nginx/nginx-ui/access/deep.conf;
        }
    }
    include snippets/other.conf;
}
`
	refs, err := ReferencesIn(content)
	require.NoError(t, err)
	assert.Equal(t, []Reference{
		{Server: 0, ServerName: "a.example.com", Slug: "lan"},
		{Server: 0, ServerName: "a.example.com", Location: "/admin", Slug: "office"},
		{Server: 0, ServerName: "a.example.com", Location: "/admin/deep", Slug: "deep"},
	}, refs)

	slugs, err := Slugs(content)
	require.NoError(t, err)
	assert.Equal(t, []string{"lan", "office", "deep"}, slugs)
}

func TestScanReferences(t *testing.T) {
	dir := useConfDir(t)
	write := func(rel, content string) {
		path := filepath.Join(dir, rel)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	}
	write("sites-available/nas", "server {\n include nginx-ui/access/lan.conf;\n}\n")
	write("sites-available/broken", "server {\n include nginx-ui/access/lan.conf;\n")
	write("streams-available/ssh", "server {\n listen 2222;\n include nginx-ui/access/lan.conf;\n}\n")
	write("sites-available/blog", "server {\n listen 80;\n}\n")

	refs := FilterReferences(ScanReferences(), "lan")
	require.Len(t, refs, 2)
	assert.Equal(t, RefKindSite, refs[0].Kind)
	assert.Equal(t, "nas", refs[0].Name)
	assert.Equal(t, RefKindStream, refs[1].Kind)
	assert.Equal(t, "ssh", refs[1].Name)

	assert.Empty(t, FilterReferences(ScanReferences(), "office"))
}

func TestSlugFromInclude(t *testing.T) {
	cases := map[string]string{
		"nginx-ui/access/lan.conf":             "lan",
		"/etc/nginx/nginx-ui/access/lan.conf":  "lan",
		`C:\nginx\conf\nginx-ui\access\a.conf`: "a",
	}
	for in, want := range cases {
		got, ok := SlugFromInclude(in)
		assert.True(t, ok, in)
		assert.Equal(t, want, got, in)
	}
	for _, in := range []string{"nginx-ui/access/*.conf", "access/lan.conf", "nginx-ui/access/LAN.conf", "nginx-ui/access/lan.conf.bak"} {
		_, ok := SlugFromInclude(in)
		assert.False(t, ok, in)
	}
}
