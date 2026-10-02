package serverstate

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/0xJacky/Nginx-UI/internal/site"
	"github.com/0xJacky/Nginx-UI/internal/stream"
	"github.com/0xJacky/Nginx-UI/internal/testdb"
	"github.com/0xJacky/Nginx-UI/internal/upstream"
	"github.com/0xJacky/Nginx-UI/internal/upstream/managed"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uozi-tech/cosy"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

const failingTestConfigCmd = "echo 'nginx: [emerg] unknown directive \"brokn\" in /etc/nginx/conf.d/other.conf:1' >&2; exit 1"

const siteWithUpstream = `upstream site_pool {
    server 127.0.0.1:8081;
    server 127.0.0.1:8082 max_fails=2;   # second backend
}

server {
    listen 80;
    server_name example.com;
    location / {
        proxy_pass http://site_pool;
    }
}
`

// setupToggleTest points nginx at an isolated configuration tree whose test and
// reload commands succeed, with an in-memory database for the save paths.
func setupToggleTest(t *testing.T) string {
	t.Helper()

	confDir := t.TempDir()
	for _, dir := range []string{"conf.d", "sites-available", "sites-enabled", "streams-available", "streams-enabled"} {
		require.NoError(t, os.MkdirAll(filepath.Join(confDir, dir), 0o755))
	}

	original := *settings.NginxSettings
	settings.NginxSettings.ConfigDir = confDir
	settings.NginxSettings.PIDPath = filepath.Join(confDir, "nginx.pid")
	settings.NginxSettings.ReloadCmd = "true"
	settings.NginxSettings.RestartCmd = "true"
	settings.NginxSettings.TestConfigCmd = "true"
	require.NoError(t, os.WriteFile(settings.NginxSettings.PIDPath, []byte(strconv.Itoa(os.Getpid())), 0o644))
	t.Cleanup(func() {
		// site.Save and stream.Save replicate in the background and read the
		// nginx settings while doing so; let them finish before restoring.
		site.WaitForSync()
		stream.WaitForSync()
		*settings.NginxSettings = original
	})

	db, err := gorm.Open(sqlite.Open(testdb.DSN(t)), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Config{}, &model.ConfigBackup{}, &model.Node{}, &model.Site{},
		&model.Stream{}, &model.Namespace{}, &model.LLMSession{}))
	model.Use(db)
	query.Use(db)
	query.SetDefault(db)

	upstream.GetUpstreamService().ClearTargets()
	t.Cleanup(upstream.GetUpstreamService().ClearTargets)

	return confDir
}

// writeConfig writes a file and registers it with the upstream scanner, as
// the file watcher would.
func writeConfig(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	require.NoError(t, upstream.ScanConfig(path, []byte(content)))
}

// enabledSite writes an enabled site: sites-available plus the sites-enabled
// symlink, both registered with the scanner like a real rescan does.
func enabledSite(t *testing.T, confDir, name, content string) string {
	t.Helper()
	available := filepath.Join(confDir, "sites-available", name)
	enabled := filepath.Join(confDir, "sites-enabled", name)
	writeConfig(t, available, content)
	require.NoError(t, os.Symlink(available, enabled))
	require.NoError(t, upstream.ScanConfig(enabled, []byte(content)))
	return available
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(content)
}

func serverByAddress(t *testing.T, group *Group, address string) Server {
	t.Helper()
	for _, s := range group.Servers {
		if s.Address == address {
			return s
		}
	}
	t.Fatalf("server %s not in group %s", address, group.Name)
	return Server{}
}

func requireCosyCode(t *testing.T, err error, code int32) {
	t.Helper()
	require.Error(t, err)
	var cErr *cosy.Error
	require.ErrorAs(t, err, &cErr)
	assert.Equal(t, code, cErr.Code, err.Error())
}

func TestSetEnabledTogglesServerInSiteFile(t *testing.T) {
	confDir := setupToggleTest(t)
	path := enabledSite(t, confDir, "example.com", siteWithUpstream)

	group, err := SetEnabled(Request{Upstream: "site_pool", ConfigPath: path, Address: "127.0.0.1:8082"}, "admin")
	require.NoError(t, err)
	assert.True(t, serverByAddress(t, group, "127.0.0.1:8082").Down)
	assert.False(t, serverByAddress(t, group, "127.0.0.1:8081").Down)
	assert.Equal(t, Source{Type: SourceSite, Name: "example.com"}, group.Source)

	content := readFile(t, path)
	assert.Contains(t, content, "    server 127.0.0.1:8082 max_fails=2 down;   # second backend\n")
	assert.Contains(t, content, "    server 127.0.0.1:8081;\n")

	// Enabling it again puts the original file back byte for byte.
	group, err = SetEnabled(Request{Upstream: "site_pool", ConfigPath: path, Address: "127.0.0.1:8082", Enabled: true}, "admin")
	require.NoError(t, err)
	assert.False(t, serverByAddress(t, group, "127.0.0.1:8082").Down)
	assert.Equal(t, siteWithUpstream, readFile(t, path))
}

func TestSetEnabledThroughEnabledSymlinkWritesAvailableFile(t *testing.T) {
	confDir := setupToggleTest(t)
	path := enabledSite(t, confDir, "example.com", siteWithUpstream)
	enabled := filepath.Join(confDir, "sites-enabled", "example.com")

	_, err := SetEnabled(Request{Upstream: "site_pool", ConfigPath: enabled, Address: "127.0.0.1:8081"}, "admin")
	require.NoError(t, err)

	info, err := os.Lstat(enabled)
	require.NoError(t, err)
	assert.NotZero(t, info.Mode()&os.ModeSymlink, "sites-enabled entry must stay a symlink")
	assert.Contains(t, readFile(t, path), "server 127.0.0.1:8081 down;")
}

func TestSetEnabledKeepsSiteSyncSettings(t *testing.T) {
	confDir := setupToggleTest(t)
	path := enabledSite(t, confDir, "example.com", siteWithUpstream)
	namespace := &model.Namespace{Name: "edge"}
	require.NoError(t, query.Namespace.Create(namespace))
	require.NoError(t, query.Site.Create(&model.Site{Path: path, NamespaceID: namespace.ID, SyncNodeIDs: []uint64{7}}))

	_, err := SetEnabled(Request{Upstream: "site_pool", ConfigPath: path, Address: "127.0.0.1:8081"}, "admin")
	require.NoError(t, err)

	// site.Save stores the namespace and sync targets it is handed, so the
	// toggle must pass the current ones through instead of clearing them.
	record, err := query.Site.Where(query.Site.Path.Eq(path)).First()
	require.NoError(t, err)
	assert.Equal(t, namespace.ID, record.NamespaceID)
	assert.Equal(t, []uint64{7}, record.SyncNodeIDs)
}

func TestSetEnabledManagedGroupKeepsGeneratedShape(t *testing.T) {
	setupToggleTest(t)
	u := &managed.Upstream{Name: "backend_pool", Method: managed.MethodLeastConn, Zone: true,
		Servers: []managed.Server{{Address: "127.0.0.1:8081"}, {Address: "127.0.0.1:8082"}}}
	detail, err := managed.Save(u, true, "admin")
	require.NoError(t, err)

	group, err := SetEnabled(Request{Upstream: "backend_pool", ConfigPath: detail.Path, Address: "127.0.0.1:8082"}, "admin")
	require.NoError(t, err)
	assert.Equal(t, SourceManaged, group.Source.Type)
	assert.True(t, serverByAddress(t, group, "127.0.0.1:8082").Down)

	// The file is still exactly what the managed editor renders, so it keeps
	// being listed and edited as a managed group.
	parsed, _, content, err := managed.Read("backend_pool")
	require.NoError(t, err)
	assert.True(t, parsed.Servers[1].Down)
	assert.False(t, parsed.Servers[0].Down)
	assert.Equal(t, parsed.Render(), content)
	assert.Contains(t, content, "    server 127.0.0.1:8082 down;\n")

	_, err = SetEnabled(Request{Upstream: "backend_pool", ConfigPath: detail.Path, Address: "127.0.0.1:8082", Enabled: true}, "admin")
	require.NoError(t, err)
	parsed, _, _, err = managed.Read("backend_pool")
	require.NoError(t, err)
	assert.False(t, parsed.Servers[1].Down)
}

func TestSetEnabledTogglesStreamAndPlainConfig(t *testing.T) {
	confDir := setupToggleTest(t)

	streamPath := filepath.Join(confDir, "streams-available", "dns")
	writeConfig(t, streamPath, "upstream dns_pool {\n    server 10.0.0.1:53;\n    server 10.0.0.2:53;\n}\nserver { listen 53 udp; proxy_pass dns_pool; }\n")
	group, err := SetEnabled(Request{Upstream: "dns_pool", ConfigPath: streamPath, Address: "10.0.0.2:53"}, "admin")
	require.NoError(t, err)
	assert.Equal(t, Source{Type: SourceStream, Name: "dns"}, group.Source)
	assert.Contains(t, readFile(t, streamPath), "server 10.0.0.2:53 down;")

	plainPath := filepath.Join(confDir, "conf.d", "legacy.conf")
	writeConfig(t, plainPath, "upstream legacy {\n    server 10.0.1.1:80;\n}\n")
	group, err = SetEnabled(Request{Upstream: "legacy", ConfigPath: plainPath, Address: "10.0.1.1:80"}, "admin")
	require.NoError(t, err)
	assert.Equal(t, Source{Type: SourceConfig, Name: filepath.Join("conf.d", "legacy.conf")}, group.Source)
	assert.Equal(t, "upstream legacy {\n    server 10.0.1.1:80 down;\n}\n", readFile(t, plainPath))
}

func TestSetEnabledRollsBackWhenNginxTestFails(t *testing.T) {
	confDir := setupToggleTest(t)
	sitePath := enabledSite(t, confDir, "example.com", siteWithUpstream)
	plainPath := filepath.Join(confDir, "conf.d", "legacy.conf")
	writeConfig(t, plainPath, "upstream legacy {\n    server 10.0.1.1:80;\n}\n")
	detail, err := managed.Save(&managed.Upstream{Name: "backend_pool",
		Servers: []managed.Server{{Address: "127.0.0.1:8081"}}}, true, "admin")
	require.NoError(t, err)

	cases := []struct {
		name, upstream, path, address string
	}{
		{"site", "site_pool", sitePath, "127.0.0.1:8081"},
		{"config", "legacy", plainPath, "10.0.1.1:80"},
		{"managed", "backend_pool", detail.Path, "127.0.0.1:8081"},
	}

	settings.NginxSettings.TestConfigCmd = failingTestConfigCmd
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			before := readFile(t, tc.path)
			_, err := SetEnabled(Request{Upstream: tc.upstream, ConfigPath: tc.path, Address: tc.address}, "admin")
			require.Error(t, err)
			assert.Contains(t, err.Error(), "unknown directive")
			assert.Equal(t, before, readFile(t, tc.path), "a rejected toggle must be rolled back")
		})
	}
}

func TestSetEnabledRejectsUnknownTargets(t *testing.T) {
	confDir := setupToggleTest(t)
	path := enabledSite(t, confDir, "example.com", siteWithUpstream)

	_, err := SetEnabled(Request{Upstream: "site_pool", ConfigPath: path}, "admin")
	requireCosyCode(t, err, 40016)

	// A file the scanner never reported as defining an upstream is refused,
	// even inside the configuration directory.
	unscanned := filepath.Join(confDir, "conf.d", "unscanned.conf")
	require.NoError(t, os.WriteFile(unscanned, []byte("upstream site_pool { server 1.1.1.1:80; }\n"), 0o644))
	_, err = SetEnabled(Request{Upstream: "site_pool", ConfigPath: unscanned, Address: "1.1.1.1:80"}, "admin")
	requireCosyCode(t, err, 40402)

	_, err = SetEnabled(Request{Upstream: "site_pool", ConfigPath: "/etc/passwd", Address: "127.0.0.1:8081"}, "admin")
	requireCosyCode(t, err, 40402)

	_, err = SetEnabled(Request{Upstream: "other_pool", ConfigPath: path, Address: "127.0.0.1:8081"}, "admin")
	requireCosyCode(t, err, 40402)

	_, err = SetEnabled(Request{Upstream: "site_pool", ConfigPath: path, Address: "127.0.0.1:9999"}, "admin")
	requireCosyCode(t, err, 40403)

	assert.Equal(t, siteWithUpstream, readFile(t, path))
}

func TestListReportsEveryUpstreamOnce(t *testing.T) {
	confDir := setupToggleTest(t)
	sitePath := enabledSite(t, confDir, "example.com", siteWithUpstream)
	_, err := managed.Save(&managed.Upstream{Name: "backend_pool",
		Servers: []managed.Server{{Address: "127.0.0.1:8083"}, {Address: "127.0.0.1:8084", Down: true}}}, true, "admin")
	require.NoError(t, err)

	groups := List()
	require.Len(t, groups, 2, "the site reached through sites-enabled is listed once")

	assert.Equal(t, "backend_pool", groups[0].Name)
	assert.Equal(t, SourceManaged, groups[0].Source.Type)
	assert.True(t, serverByAddress(t, &groups[0], "127.0.0.1:8084").Down)

	assert.Equal(t, "site_pool", groups[1].Name)
	assert.Equal(t, Source{Type: SourceSite, Name: "example.com"}, groups[1].Source)
	assert.Equal(t, resolvePath(sitePath), groups[1].ConfigPath)
	second := serverByAddress(t, &groups[1], "127.0.0.1:8082")
	assert.Equal(t, "127.0.0.1:8082", second.Socket)
	assert.Equal(t, "max_fails=2", second.Params)
}

func TestListReportsTheZoneDirective(t *testing.T) {
	confDir := setupToggleTest(t)
	enabledSite(t, confDir, "zoned.test", "upstream own_pool {\n    zone own_pool 128k;\n    server 127.0.0.1:8081;\n}\n"+
		"upstream joined_pool {\n    zone shared;\n    server 127.0.0.1:8082;\n}\n"+
		"upstream plain_pool {\n    server 127.0.0.1:8083;\n}\n")

	zones := map[string]string{}
	for _, group := range List() {
		zones[group.Name] = group.Zone
	}
	assert.Equal(t, map[string]string{"own_pool": "own_pool 128k", "joined_pool": "shared", "plain_pool": ""}, zones)
}
