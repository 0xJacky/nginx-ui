package convert

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/0xJacky/Nginx-UI/internal/nodeauth"
	"github.com/0xJacky/Nginx-UI/internal/site"
	"github.com/0xJacky/Nginx-UI/internal/stream"
	"github.com/0xJacky/Nginx-UI/internal/upstream"
	"github.com/0xJacky/Nginx-UI/internal/upstream/managed"
	"github.com/0xJacky/Nginx-UI/internal/upstream/serverstate"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uozi-tech/cosy"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

const failingTestConfigCmd = "echo 'nginx: [emerg] unknown directive \"brokn\" in /etc/nginx/conf.d/broken.conf:1' >&2; exit 1"

const legacySite = `# Backends of the legacy application.
upstream legacy_pool {
    least_conn;
    server 127.0.0.1:8081 weight=2 max_fails=3 fail_timeout=10s;   # primary
    server 127.0.0.1:8082 max_conns=100;
    server 127.0.0.1:8083 backup;
    # server 127.0.0.1:8084;
    keepalive 16;
    keepalive_timeout 60s;
}

upstream other_pool {
    server 127.0.0.1:9000;
}

server {
    listen 80;
    server_name legacy.test;

    location / {
        proxy_pass http://legacy_pool;
    }

    location /other/ {
        proxy_pass http://other_pool;
    }
}
`

const legacySiteAfter = `# Backends of the legacy application.

upstream other_pool {
    server 127.0.0.1:9000;
}

server {
    listen 80;
    server_name legacy.test;

    location / {
        proxy_pass http://legacy_pool;
    }

    location /other/ {
        proxy_pass http://other_pool;
    }
}
`

const legacyGroup = "# Managed by Nginx UI: edit this upstream from the Upstream page.\n" +
	"# Sites reference it with `proxy_pass http://legacy_pool;`.\n" +
	`upstream legacy_pool {
    least_conn;
    zone legacy_pool 64k;
    server 127.0.0.1:8081 weight=2 max_fails=3 fail_timeout=10s;
    server 127.0.0.1:8082 max_conns=100;
    server 127.0.0.1:8083 backup;
    keepalive 16;
    keepalive_timeout 60s;
    # primary
    # server 127.0.0.1:8084;
}
`

// legacyGroupNoZone is legacyGroup converted with the zone switch off.
var legacyGroupNoZone = strings.Replace(legacyGroup, "    zone legacy_pool 64k;\n", "", 1)

// setupConvertTest points nginx at an isolated configuration tree whose test
// and reload commands succeed, with an in-memory database.
func setupConvertTest(t *testing.T) string {
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
		// Background replications read the nginx settings; let them finish
		// before the settings are restored.
		WaitForSync()
		site.WaitForSync()
		stream.WaitForSync()
		*settings.NginxSettings = original
	})

	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Config{}, &model.ConfigBackup{}, &model.Node{}, &model.Site{},
		&model.Stream{}, &model.Namespace{}, &model.Notification{}, &model.ExternalNotify{}))
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

// enabledSite writes sites-available/<name> plus its sites-enabled symlink.
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

func requireCosyCode(t *testing.T, err error, code int32) {
	t.Helper()
	require.Error(t, err)
	var cErr *cosy.Error
	require.ErrorAs(t, err, &cErr)
	assert.Equal(t, code, cErr.Code, err.Error())
}

func codeOf(err error) int32 {
	var cErr *cosy.Error
	if errors.As(err, &cErr) {
		return cErr.Code
	}
	return 0
}

func groupPath(confDir, name string) string {
	return filepath.Join(confDir, "conf.d", "upstream-"+name+".conf")
}

func TestConvertMovesBlockIntoManagedGroup(t *testing.T) {
	confDir := setupConvertTest(t)
	sitePath := enabledSite(t, confDir, "legacy.test", legacySite)

	detail, err := Convert(context.Background(), Request{Site: "legacy.test", Upstream: "legacy_pool"}, "admin")
	require.NoError(t, err)

	// The group file has the shape the Upstream Groups editor generates.
	assert.Equal(t, legacyGroup, readFile(t, groupPath(confDir, "legacy_pool")))
	// Only the block left the site; the comment above it, the other upstream
	// and the server block stay byte for byte.
	assert.Equal(t, legacySiteAfter, readFile(t, sitePath))

	assert.Equal(t, "legacy_pool", detail.Name)
	assert.Equal(t, managed.MethodLeastConn, detail.Method)
	assert.Equal(t, 16, detail.Keepalive)
	// A block without a zone gets the one new groups get.
	assert.True(t, detail.Zone)
	assert.Equal(t, managed.DefaultZoneSize, detail.ZoneSize)
	require.Len(t, detail.Servers, 3)
	assert.Equal(t, 2, *detail.Servers[0].Weight)
	assert.Equal(t, 3, *detail.Servers[0].MaxFails)
	assert.Equal(t, "10s", detail.Servers[0].FailTimeout)
	assert.Equal(t, "max_conns=100", detail.Servers[1].Params)
	assert.True(t, detail.Servers[2].Backup)
	assert.Equal(t, "127.0.0.1:8081", detail.Servers[0].Socket)
	require.Len(t, detail.References, 1)
	assert.Equal(t, "legacy.test", detail.References[0].Name)

	// The group reads back as managed, so the Upstream Groups page lists it.
	names := managed.Names()
	assert.Equal(t, []string{"legacy_pool"}, names)

	// The scanner knows the block moved: the upstream is defined by the group
	// file and the site's own blocks no longer include it.
	def, ok := upstream.GetUpstreamService().GetUpstreamDefinition("legacy_pool")
	require.True(t, ok)
	assert.Equal(t, groupPath(confDir, "legacy_pool"), def.ConfigPath)
	for _, group := range serverstate.List() {
		if group.Name == "legacy_pool" {
			assert.Equal(t, serverstate.SourceManaged, group.Source.Type)
		}
	}

	// The site kept a history entry of its previous content.
	backups, err := query.ConfigBackup.Where(query.ConfigBackup.FilePath.Eq(sitePath)).Find()
	require.NoError(t, err)
	require.Len(t, backups, 1)
	assert.Equal(t, legacySite, backups[0].Content)
}

func TestConvertMapsHashZoneAndQuotedValues(t *testing.T) {
	confDir := setupConvertTest(t)
	content := `upstream hashed {
    hash "$request_uri" consistent;
    zone hashed 128k;
    server backend.internal:8080 slow_start=30s;
    sticky_param "a b";
}
server { listen 80; location / { proxy_pass http://hashed/api/; } }
`
	sitePath := enabledSite(t, confDir, "hash.test", content)

	detail, err := Convert(context.Background(), Request{Site: "hash.test", Upstream: "hashed"}, "admin")
	require.NoError(t, err)
	assert.Equal(t, managed.MethodHash, detail.Method)
	assert.Equal(t, "$request_uri", detail.HashKey)
	assert.True(t, detail.Consistent)
	assert.True(t, detail.Zone)
	assert.Equal(t, "128k", detail.ZoneSize)
	assert.Equal(t, "slow_start=30s", detail.Servers[0].Params)
	// A quoted value that needs its quotes is copied as written.
	assert.Equal(t, `sticky_param "a b";`, detail.ExtraDirectives)

	assert.Equal(t, "server { listen 80; location / { proxy_pass http://hashed/api/; } }\n", readFile(t, sitePath))
}

func TestConvertKeepsNamespaceAndSyncSettings(t *testing.T) {
	confDir := setupConvertTest(t)
	sitePath := enabledSite(t, confDir, "legacy.test", legacySite)

	namespace := &model.Namespace{Name: "prod"}
	require.NoError(t, query.Namespace.Create(namespace))
	require.NoError(t, query.Site.Create(&model.Site{Path: sitePath, NamespaceID: namespace.ID, SyncNodeIDs: []uint64{}}))

	_, err := Convert(context.Background(), Request{Site: "legacy.test", Upstream: "legacy_pool"}, "admin")
	require.NoError(t, err)

	record, err := query.Site.Where(query.Site.Path.Eq(sitePath)).First()
	require.NoError(t, err)
	assert.Equal(t, namespace.ID, record.NamespaceID)
}

func TestConvertMirrorsToSyncNodes(t *testing.T) {
	confDir := setupConvertTest(t)
	sitePath := enabledSite(t, confDir, "legacy.test", legacySite)

	originalSecret := settings.CryptoSettings.Secret
	originalInstanceID := settings.NodeSettings.InstanceID
	t.Cleanup(func() {
		settings.CryptoSettings.Secret = originalSecret
		settings.NodeSettings.InstanceID = originalInstanceID
	})
	settings.CryptoSettings.Secret = "upstream-convert-test-root"
	settings.NodeSettings.InstanceID = "22222222-2222-4222-8222-222222222222"

	var (
		mu       sync.Mutex
		received []Request
		paths    []string
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		paths = append(paths, r.URL.Path)
		var req Request
		_ = json.NewDecoder(r.Body).Decode(&req)
		received = append(received, req)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"name":"legacy_pool"}`))
	}))
	t.Cleanup(server.Close)

	node := &model.Node{
		Name:             "node-1",
		URL:              server.URL,
		AuthMethod:       model.NodeAuthMethodLegacy,
		CredentialStatus: model.NodeCredentialStatusActive,
		Enabled:          true,
	}
	db := model.UseDB()
	require.NoError(t, db.Create(node).Error)
	secret, err := nodeauth.EncryptPrivateCredential(nodeauth.LegacyCredentialPurpose(node.ID), []byte("node-secret"))
	require.NoError(t, err)
	require.NoError(t, db.Model(node).Update("encrypted_legacy_secret", secret).Error)
	require.NoError(t, query.Site.Create(&model.Site{Path: sitePath, SyncNodeIDs: []uint64{node.ID}}))

	_, err = Convert(context.Background(), Request{Site: "legacy.test", Upstream: "legacy_pool"}, "admin")
	require.NoError(t, err)
	WaitForSync()

	mu.Lock()
	defer mu.Unlock()
	// The node converts its own copy instead of receiving the two files.
	assert.Equal(t, []string{"/api/upstream/convert"}, paths)
	// The zone choice is spelled out, so the node converts the same way.
	on := true
	assert.Equal(t, []Request{{Site: "legacy.test", Upstream: "legacy_pool", Zone: &on}}, received)

	record, err := query.Site.Where(query.Site.Path.Eq(sitePath)).First()
	require.NoError(t, err)
	assert.Equal(t, []uint64{node.ID}, []uint64(record.SyncNodeIDs))
}

func TestConvertRollsBackBothFilesWhenTestFails(t *testing.T) {
	confDir := setupConvertTest(t)
	sitePath := enabledSite(t, confDir, "legacy.test", legacySite)
	settings.NginxSettings.TestConfigCmd = failingTestConfigCmd

	_, err := Convert(context.Background(), Request{Site: "legacy.test", Upstream: "legacy_pool"}, "admin")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "brokn")

	assert.Equal(t, legacySite, readFile(t, sitePath))
	_, statErr := os.Stat(groupPath(confDir, "legacy_pool"))
	assert.True(t, os.IsNotExist(statErr), "the group file must be removed again")

	// Nothing points at a group file that does not exist.
	def, ok := upstream.GetUpstreamService().GetUpstreamDefinition("legacy_pool")
	require.True(t, ok)
	assert.NotEqual(t, groupPath(confDir, "legacy_pool"), def.ConfigPath)
}

func TestConvertRollsBackBothFilesWhenReloadFails(t *testing.T) {
	confDir := setupConvertTest(t)
	sitePath := enabledSite(t, confDir, "legacy.test", legacySite)
	settings.NginxSettings.ReloadCmd = "false"

	_, err := Convert(context.Background(), Request{Site: "legacy.test", Upstream: "legacy_pool"}, "admin")
	require.Error(t, err)

	assert.Equal(t, legacySite, readFile(t, sitePath))
	_, statErr := os.Stat(groupPath(confDir, "legacy_pool"))
	assert.True(t, os.IsNotExist(statErr))
}

func TestConvertRefusals(t *testing.T) {
	cases := []struct {
		name     string
		site     string
		upstream string
		content  string
		setup    func(t *testing.T, confDir string)
		code     int32
	}{
		{
			name: "group with the same name exists", site: "legacy.test", upstream: "legacy_pool", content: legacySite,
			setup: func(t *testing.T, confDir string) {
				u := &managed.Upstream{Name: "legacy_pool", Servers: []managed.Server{{Address: "127.0.0.1:9100"}}}
				content, err := managed.Prepare(u)
				require.NoError(t, err)
				writeConfig(t, groupPath(confDir, "legacy_pool"), content)
			},
			code: codeOf(ErrGroupExists),
		},
		{
			name: "name defined in another file", site: "legacy.test", upstream: "legacy_pool", content: legacySite,
			setup: func(t *testing.T, confDir string) {
				writeConfig(t, filepath.Join(confDir, "conf.d", "shared.conf"),
					"upstream legacy_pool {\n    server 127.0.0.1:9200;\n}\n")
			},
			code: codeOf(managed.ErrUpstreamNameConflict),
		},
		{
			name: "invalid upstream name", site: "legacy.test", upstream: "bad name", content: legacySite,
			code: codeOf(managed.ErrInvalidName),
		},
		{
			name: "missing fields", site: "", upstream: "legacy_pool", content: legacySite,
			code: codeOf(ErrInvalidRequest),
		},
		{
			name: "path outside sites-available", site: "../conf.d/legacy.test", upstream: "legacy_pool", content: legacySite,
			code: codeOf(ErrInvalidSiteName),
		},
		{
			name: "unknown site", site: "missing.test", upstream: "legacy_pool", content: legacySite,
			code: codeOf(ErrSiteNotFound),
		},
		{
			name: "block not in site", site: "legacy.test", upstream: "nope", content: legacySite,
			code: codeOf(serverstate.ErrUpstreamNotInConfig),
		},
		{
			name: "nested block", site: "legacy.test", upstream: "legacy_pool",
			content: "upstream legacy_pool {\n    server 127.0.0.1:8081;\n    check { interval 3; }\n}\n",
			code:    codeOf(ErrNestedBlock),
		},
		{
			name: "comment with braces", site: "legacy.test", upstream: "legacy_pool",
			content: "upstream legacy_pool {\n    # moved from { old }\n    server 127.0.0.1:8081;\n}\n",
			code:    codeOf(ErrCommentWithBraces),
		},
		{
			name: "multi-line value", site: "legacy.test", upstream: "legacy_pool",
			content: "upstream legacy_pool {\n    server 127.0.0.1:8081;\n    sticky_note \"a\nb\";\n}\n",
			code:    codeOf(ErrMultilineValue),
		},
		{
			name: "defined twice", site: "legacy.test", upstream: "legacy_pool",
			content: "upstream legacy_pool {\n    server 127.0.0.1:8081;\n}\nupstream legacy_pool {\n    server 127.0.0.1:8082;\n}\n",
			code:    codeOf(ErrUpstreamDefinedTwice),
		},
		{
			name: "stream context", site: "legacy.test", upstream: "legacy_pool",
			content: "stream {\n    upstream legacy_pool {\n        server 127.0.0.1:5353;\n    }\n}\n",
			code:    codeOf(ErrUnsupportedContext),
		},
		{
			name: "block nginx would reject", site: "legacy.test", upstream: "legacy_pool",
			content: "upstream legacy_pool {\n    hash $remote_addr;\n    server 127.0.0.1:8081 backup;\n}\n",
			code:    codeOf(managed.ErrBackupNotSupported),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			confDir := setupConvertTest(t)
			sitePath := enabledSite(t, confDir, "legacy.test", tc.content)
			if tc.setup != nil {
				tc.setup(t, confDir)
			}
			before, err := os.ReadDir(filepath.Join(confDir, "conf.d"))
			require.NoError(t, err)

			_, err = Convert(context.Background(), Request{Site: tc.site, Upstream: tc.upstream}, "admin")
			requireCosyCode(t, err, tc.code)

			assert.Equal(t, tc.content, readFile(t, sitePath), "the site must stay untouched")
			after, err := os.ReadDir(filepath.Join(confDir, "conf.d"))
			require.NoError(t, err)
			assert.Equal(t, len(before), len(after), "no group file may be created")
		})
	}
}

func TestConvertAllowsStreamUpstreamWithSameName(t *testing.T) {
	confDir := setupConvertTest(t)
	sitePath := enabledSite(t, confDir, "legacy.test", legacySite)
	writeConfig(t, filepath.Join(confDir, "streams-available", "dns"),
		"upstream legacy_pool {\n    server 127.0.0.1:5353;\n}\nserver { listen 53 udp; proxy_pass legacy_pool; }\n")

	_, err := Convert(context.Background(), Request{Site: "legacy.test", Upstream: "legacy_pool"}, "admin")
	require.NoError(t, err)
	assert.Equal(t, legacySiteAfter, readFile(t, sitePath))
}

func TestRemoveBlock(t *testing.T) {
	cases := []struct {
		name    string
		content string
		want    string
	}{
		{
			name:    "block between blank lines",
			content: "a;\n\nupstream x {\n    server b;\n}\n\nserver {}\n",
			want:    "a;\n\nserver {}\n",
		},
		{
			name:    "block at file start",
			content: "upstream x {\n    server b;\n}\n\nserver {}\n",
			want:    "server {}\n",
		},
		{
			name:    "indented block inside http",
			content: "http {\n    upstream x {\n        server b;\n    }\n    server {}\n}\n",
			want:    "http {\n    server {}\n}\n",
		},
		{
			name:    "comment after the closing brace stays",
			content: "upstream x { server b; } # pool\nserver {}\n",
			want:    " # pool\nserver {}\n",
		},
		{
			name:    "block shares its line",
			content: "a; upstream x { server b; } b;\n",
			want:    "a;  b;\n",
		},
		{
			name:    "CRLF line endings",
			content: "a;\r\n\r\nupstream x {\r\n    server b;\r\n}\r\n\r\nserver {}\r\n",
			want:    "a;\r\n\r\nserver {}\r\n",
		},
		{
			name:    "block at file end without newline",
			content: "server {}\n\nupstream x { server b; }",
			want:    "server {}\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			blocks := serverstate.ParseBlocks(tc.content)
			require.Len(t, blocks, 1)
			assert.Equal(t, tc.want, RemoveBlock(tc.content, blocks[0].Start, blocks[0].End))
		})
	}
}
