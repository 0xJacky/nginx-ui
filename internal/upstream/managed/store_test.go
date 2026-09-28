package managed

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/0xJacky/Nginx-UI/internal/upstream"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

const failingTestConfigCmd = "echo 'nginx: [emerg] host not found in upstream' >&2; exit 1"

// setupStoreTest points nginx at an empty configuration tree whose test and
// reload commands succeed, and gives config.Save an in-memory database.
func setupStoreTest(t *testing.T) string {
	t.Helper()

	confDir := t.TempDir()
	for _, dir := range []string{"conf.d", "sites-available", "sites-enabled"} {
		require.NoError(t, os.MkdirAll(filepath.Join(confDir, dir), 0o755))
	}

	original := *settings.NginxSettings
	settings.NginxSettings.ConfigDir = confDir
	settings.NginxSettings.PIDPath = filepath.Join(confDir, "nginx.pid")
	settings.NginxSettings.ReloadCmd = "true"
	settings.NginxSettings.RestartCmd = "true"
	settings.NginxSettings.TestConfigCmd = "true"
	require.NoError(t, os.WriteFile(settings.NginxSettings.PIDPath, []byte(strconv.Itoa(os.Getpid())), 0o644))
	t.Cleanup(func() { *settings.NginxSettings = original })

	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Config{}, &model.ConfigBackup{}, &model.Node{}, &model.LLMSession{}))
	model.Use(db)
	query.Use(db)
	query.SetDefault(db)

	upstream.GetUpstreamService().ClearTargets()
	t.Cleanup(upstream.GetUpstreamService().ClearTargets)

	return confDir
}

func pool(name string, servers ...string) *Upstream {
	u := &Upstream{Name: name, Method: MethodLeastConn}
	for _, s := range servers {
		u.Servers = append(u.Servers, Server{Address: s})
	}
	return u
}

func writeSite(t *testing.T, confDir, name, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(confDir, "sites-available", name), []byte(content), 0o644))
}

func TestSaveCreatesFileInConfD(t *testing.T) {
	confDir := setupStoreTest(t)

	detail, err := Save(pool("backend_pool", "127.0.0.1:8081", "127.0.0.1:8082"), true, "admin")
	require.NoError(t, err)

	path := filepath.Join(confDir, "conf.d", "upstream-backend_pool.conf")
	assert.Equal(t, path, detail.Path)
	content, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(content), "upstream backend_pool {\n    least_conn;\n    server 127.0.0.1:8081;\n    server 127.0.0.1:8082;\n}\n")
	assert.Equal(t, string(content), detail.Content)
	assert.Empty(t, detail.References)

	// The availability service learns about the group without a rescan.
	def, ok := upstream.GetUpstreamService().GetUpstreamDefinition("backend_pool")
	require.True(t, ok)
	assert.Len(t, def.Servers, 2)
}

func TestSaveRejectsDuplicateCreate(t *testing.T) {
	setupStoreTest(t)

	_, err := Save(pool("backend_pool", "127.0.0.1:8081"), true, "")
	require.NoError(t, err)

	_, err = Save(pool("backend_pool", "127.0.0.1:9999"), true, "")
	assertCosyError(t, err, ErrUpstreamExists)

	u, _, _, err := Read("backend_pool")
	require.NoError(t, err)
	assert.Equal(t, "127.0.0.1:8081", u.Servers[0].Address)
}

func TestSaveRejectsNameDefinedInAnotherFile(t *testing.T) {
	confDir := setupStoreTest(t)

	sitePath := filepath.Join(confDir, "sites-available", "legacy")
	writeSite(t, confDir, "legacy", "upstream backend_pool { server 10.0.0.1:80; }\n")
	upstream.GetUpstreamService().UpdateUpstreamDefinition("backend_pool",
		[]upstream.ProxyTarget{{Host: "10.0.0.1", Port: "80", Type: "upstream"}}, sitePath)

	_, err := Save(pool("backend_pool", "127.0.0.1:8081"), true, "")
	assertCosyError(t, err, ErrUpstreamNameConflict)
	assert.Contains(t, err.Error(), sitePath)
	assert.False(t, Exists("backend_pool"))
}

func TestSaveUpdateKeepsNameAndRewritesFile(t *testing.T) {
	setupStoreTest(t)

	_, err := Save(pool("backend_pool", "127.0.0.1:8081", "127.0.0.1:8082"), true, "")
	require.NoError(t, err)

	updated := pool("backend_pool", "127.0.0.1:8081", "127.0.0.1:8082", "127.0.0.1:8083")
	updated.Method = MethodRoundRobin
	updated.Servers[1].Down = true
	detail, err := Save(updated, false, "")
	require.NoError(t, err)
	assert.Equal(t, MethodRoundRobin, detail.Method)
	assert.Len(t, detail.Servers, 3)
	assert.True(t, detail.Servers[1].Down)
	assert.NotContains(t, detail.Content, "least_conn")
	assert.Contains(t, detail.Content, "server 127.0.0.1:8082 down;")
}

func TestSaveUpdateOfMissingUpstreamFails(t *testing.T) {
	setupStoreTest(t)
	_, err := Save(pool("ghost", "127.0.0.1:8081"), false, "")
	assertCosyError(t, err, ErrUpstreamNotFound)
}

func TestSaveUpdateRefusesHandWrittenFile(t *testing.T) {
	confDir := setupStoreTest(t)
	path := filepath.Join(confDir, "conf.d", "upstream-legacy.conf")
	original := "upstream legacy { server 10.0.0.1:80; }\nupstream other { server 10.0.0.2:80; }\n"
	require.NoError(t, os.WriteFile(path, []byte(original), 0o644))

	_, err := Save(pool("legacy", "127.0.0.1:8081"), false, "")
	assertCosyError(t, err, ErrUpstreamFileNotManaged)

	content, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, original, string(content))

	// List skips it instead of failing.
	list, err := List()
	require.NoError(t, err)
	assert.Empty(t, list)
}

func TestSaveRollsBackWhenNginxTestFails(t *testing.T) {
	confDir := setupStoreTest(t)

	_, err := Save(pool("backend_pool", "127.0.0.1:8081"), true, "")
	require.NoError(t, err)
	path := filepath.Join(confDir, "conf.d", "upstream-backend_pool.conf")
	before, err := os.ReadFile(path)
	require.NoError(t, err)

	settings.NginxSettings.TestConfigCmd = failingTestConfigCmd

	_, err = Save(pool("backend_pool", "does-not-resolve.invalid:80"), false, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "host not found in upstream")

	after, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, string(before), string(after))

	// A rejected create leaves nothing behind.
	_, err = Save(pool("new_pool", "127.0.0.1:8081"), true, "")
	require.Error(t, err)
	assert.False(t, Exists("new_pool"))
}

func TestSaveRejectsInvalidInputBeforeWriting(t *testing.T) {
	confDir := setupStoreTest(t)

	_, err := Save(&Upstream{Name: "../escape", Servers: []Server{{Address: "127.0.0.1:80"}}}, true, "")
	assertCosyError(t, err, ErrInvalidName)

	_, err = Save(pool("pool"), true, "")
	assertCosyError(t, err, ErrNoServers)

	entries, err := os.ReadDir(filepath.Join(confDir, "conf.d"))
	require.NoError(t, err)
	assert.Empty(t, entries)
}

func TestFindReferences(t *testing.T) {
	confDir := setupStoreTest(t)

	writeSite(t, confDir, "a.example.com", "server {\n  location / {\n    proxy_pass http://backend_pool/;\n  }\n}\n")
	writeSite(t, confDir, "b.example.com", "server {\n  location /api { proxy_pass https://backend_pool:8443; }\n}\n")
	writeSite(t, confDir, "grpc.example.com", "server {\n  location / { grpc_pass grpc://backend_pool; }\n}\n")
	// Not references: a longer name, a commented line, and a host with a dot.
	writeSite(t, confDir, "c.example.com", "server {\n  location / { proxy_pass http://backend_pool_v2/; }\n}\n")
	writeSite(t, confDir, "d.example.com", "server {\n  # proxy_pass http://backend_pool/;\n  location / { proxy_pass http://127.0.0.1:9000; }\n}\n")
	writeSite(t, confDir, "e.example.com", "server {\n  location / { proxy_pass http://backend_pool.example.com/; }\n}\n")
	require.NoError(t, os.WriteFile(filepath.Join(confDir, "conf.d", "shared.conf"),
		[]byte("server { location / { proxy_pass http://backend_pool; } }\n"), 0o644))

	_, err := Save(pool("backend_pool", "127.0.0.1:8081"), true, "")
	require.NoError(t, err)

	refs, err := FindReferences("backend_pool")
	require.NoError(t, err)

	names := make([]string, 0, len(refs))
	for _, ref := range refs {
		names = append(names, ref.Type+":"+ref.Name)
	}
	assert.ElementsMatch(t, []string{
		"site:a.example.com",
		"site:b.example.com",
		"site:grpc.example.com",
		"config:shared.conf",
	}, names)

	list, err := List()
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Len(t, list[0].References, 4)
}

func TestDeleteIsBlockedWhileReferenced(t *testing.T) {
	confDir := setupStoreTest(t)

	_, err := Save(pool("backend_pool", "127.0.0.1:8081"), true, "")
	require.NoError(t, err)
	writeSite(t, confDir, "a.example.com", "server { location / { proxy_pass http://backend_pool/; } }\n")

	err = Delete("backend_pool")
	assertCosyError(t, err, ErrUpstreamInUse)
	assert.Contains(t, err.Error(), "a.example.com")
	assert.True(t, Exists("backend_pool"))

	// Once the site stops using it the upstream can go.
	writeSite(t, confDir, "a.example.com", "server { location / { proxy_pass http://127.0.0.1:8081/; } }\n")
	require.NoError(t, Delete("backend_pool"))
	assert.False(t, Exists("backend_pool"))
	_, ok := upstream.GetUpstreamService().GetUpstreamDefinition("backend_pool")
	assert.False(t, ok)
}

func TestDeleteRestoresFileWhenNginxTestFails(t *testing.T) {
	setupStoreTest(t)

	_, err := Save(pool("backend_pool", "127.0.0.1:8081"), true, "")
	require.NoError(t, err)

	settings.NginxSettings.TestConfigCmd = failingTestConfigCmd
	require.Error(t, Delete("backend_pool"))
	assert.True(t, Exists("backend_pool"))
}

func TestDeleteMissingUpstream(t *testing.T) {
	setupStoreTest(t)
	assertCosyError(t, Delete("ghost"), ErrUpstreamNotFound)
}

func TestSaveExplainsZoneNameClash(t *testing.T) {
	setupStoreTest(t)

	// nginx -t output when another directive already declared the zone name.
	settings.NginxSettings.TestConfigCmd = "echo 'nginx: [emerg] the shared memory zone \"clash_pool\" is already declared " +
		"for a different use in /etc/nginx/conf.d/upstream-clash_pool.conf:4' >&2; " +
		"echo 'nginx: configuration file /etc/nginx/nginx.conf test failed' >&2; exit 1"

	u := pool("clash_pool", "127.0.0.1:8081")
	u.Zone = true
	_, err := Save(u, true, "")
	assertCosyError(t, err, ErrZoneNameConflict)
	assert.Contains(t, err.Error(), `the shared memory zone "clash_pool" is already declared for a different use`)
	assert.NotContains(t, err.Error(), "[emerg]")
	assert.False(t, Exists("clash_pool"), "the rejected file is rolled back")

	// Other nginx -t failures are passed through untouched.
	settings.NginxSettings.TestConfigCmd = failingTestConfigCmd
	_, err = Save(u, true, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "host not found in upstream")
}

func TestSaveWritesZone(t *testing.T) {
	confDir := setupStoreTest(t)

	u := pool("zoned", "127.0.0.1:8081", "127.0.0.1:8082")
	u.Zone = true
	detail, err := Save(u, true, "")
	require.NoError(t, err)
	assert.True(t, detail.Zone)
	assert.Equal(t, "64k", detail.ZoneSize)

	content, err := os.ReadFile(filepath.Join(confDir, "conf.d", "upstream-zoned.conf"))
	require.NoError(t, err)
	assert.Contains(t, string(content), "    zone zoned 64k;\n")
}

func TestPathIsConfinedToConfD(t *testing.T) {
	confDir := setupStoreTest(t)

	path, err := Path("backend_pool")
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(confDir, "conf.d", "upstream-backend_pool.conf"), path)

	traversal := []string{
		"../../etc/passwd",
		"..",
		"../conf.d/x",
		"a/../../b",
		"a/b",
		`a\b`,
		"/etc/passwd",
		"",
	}
	for _, name := range traversal {
		_, err := Path(name)
		assert.Error(t, err, name)

		// The confinement holds on its own, without the name pattern.
		_, err = confinedPath(name)
		assert.Error(t, err, name)
	}

	// Nothing escapes conf.d through the store entry points either.
	_, err = Save(&Upstream{Name: "../../escape", Servers: []Server{{Address: "127.0.0.1:80"}}}, true, "")
	require.Error(t, err)
	require.Error(t, Delete("../../etc/passwd"))
	_, _, _, err = Read("../upstream-x")
	require.Error(t, err)
	assert.False(t, Exists("../../etc/passwd"))
	_, err = os.Stat(filepath.Join(confDir, "escape.conf"))
	assert.True(t, os.IsNotExist(err))
}
