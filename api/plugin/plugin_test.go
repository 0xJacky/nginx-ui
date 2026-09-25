package plugin

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/middleware"
	plugin "github.com/0xJacky/Nginx-UI/internal/plugin"
	"github.com/0xJacky/Nginx-UI/internal/plugin/protocol"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// The handlers reach the process wide manager, so the whole package shares one
// plugin directory that is fixed before the singleton resolves it.
func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)

	dir, err := os.MkdirTemp("", "nginx-ui-plugin-api-")
	if err != nil {
		panic(err)
	}
	settings.PluginSettings.Dir = dir
	// The test packages are unsigned.
	settings.PluginSettings.DeveloperMode = true
	// No test reaches the real partner keyring.
	restore := plugin.SetOfficialSourceForTesting("")

	code := m.Run()
	restore()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}

// setupManager wires a fresh database and reloads the plugin inventory without
// starting any plugin process.
func setupManager(t *testing.T) *plugin.Manager {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Plugin{}, &model.PluginKV{}))
	model.Use(db)
	query.Init(db)
	t.Cleanup(func() { model.Use(nil) })

	// Every test starts from an empty directory.
	entries, err := os.ReadDir(settings.PluginSettings.Dir)
	require.NoError(t, err)
	for _, item := range entries {
		require.NoError(t, os.RemoveAll(filepath.Join(settings.PluginSettings.Dir, item.Name())))
	}

	manager := plugin.GetManager()
	require.NoError(t, manager.LoadOffline(context.Background()))
	return manager
}

// installTestPlugin builds a package and installs it through the manager.
func installTestPlugin(t *testing.T, manager *plugin.Manager, manifest *protocol.Manifest, files map[string]string, enable bool) {
	t.Helper()

	archive := buildTestPackage(t, manifest, files)
	_, err := manager.Install(context.Background(), archive, plugin.InstallOptions{Enable: enable})
	require.NoError(t, err)
}

// buildTestPackage packs a manifest and its files into a package archive.
func buildTestPackage(t *testing.T, manifest *protocol.Manifest, files map[string]string) string {
	t.Helper()

	staging := filepath.Join(t.TempDir(), manifest.ID)
	require.NoError(t, os.MkdirAll(staging, 0o755))
	encoded, err := json.MarshalIndent(manifest, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(staging, plugin.ManifestFileName), encoded, 0o644))
	for name, content := range files {
		target := filepath.Join(staging, filepath.FromSlash(name))
		require.NoError(t, os.MkdirAll(filepath.Dir(target), 0o755))
		require.NoError(t, os.WriteFile(target, []byte(content), 0o644))
	}

	archive := filepath.Join(t.TempDir(), manifest.ID+".tar.gz")
	require.NoError(t, plugin.BuildPackage(staging, archive))
	return archive
}

// webappManifest describes a plugin that only ships browser assets, so no
// process is ever spawned by these tests.
func webappManifest(id string) *protocol.Manifest {
	return &protocol.Manifest{
		ID:         id,
		Name:       "Test " + id,
		Version:    "1.2.3",
		APIVersion: protocol.APIVersion,
		Webapp:     &protocol.ManifestWebapp{BundlePath: "webapp/main.js"},
		SettingsSchema: &protocol.SettingsSchema{
			Settings: []protocol.SettingsField{
				{Key: "endpoint", Type: "text", DisplayName: "Endpoint", Default: "https://example.com"},
				{Key: "token", Type: "secret", DisplayName: "Token"},
			},
		},
	}
}

func newContext(method, target string, body io.Reader, params gin.Params) (*gin.Context, *httptest.ResponseRecorder) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(method, target, body)
	if body != nil {
		c.Request.Header.Set("Content-Type", "application/json")
	}
	c.Params = params
	return c, recorder
}

func TestGetPluginListReturnsEveryPlugin(t *testing.T) {
	manager := setupManager(t)
	installTestPlugin(t, manager, webappManifest("official.alpha"), map[string]string{
		"webapp/main.js": "export default {}",
	}, true)

	c, recorder := newContext(http.MethodGet, "/api/plugins", nil, nil)
	GetPluginList(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	var infos []plugin.Info
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &infos))
	require.Len(t, infos, 1)
	assert.Equal(t, "official.alpha", infos[0].ID)
	assert.Equal(t, "1.2.3", infos[0].Version)
	assert.True(t, infos[0].Enabled)
	assert.True(t, infos[0].HasWebapp)
	assert.False(t, infos[0].HasServer)
	assert.NotNil(t, infos[0].SettingsSchema)
}

func TestGetPluginSettingsRedactsSecrets(t *testing.T) {
	manager := setupManager(t)
	installTestPlugin(t, manager, webappManifest("official.alpha"), nil, true)
	require.NoError(t, manager.SaveSettings(context.Background(), "official.alpha", map[string]any{
		"endpoint": "https://plugin.example",
		"token":    "s3cret",
	}))

	c, recorder := newContext(http.MethodGet, "/api/plugins/official.alpha/settings", nil,
		gin.Params{{Key: "id", Value: "official.alpha"}})
	GetPluginSettings(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	var body struct {
		Schema *protocol.SettingsSchema `json:"schema"`
		Values map[string]any           `json:"values"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	require.NotNil(t, body.Schema)
	assert.Equal(t, "https://plugin.example", body.Values["endpoint"])
	assert.Equal(t, settings.RedactedSensitiveValue, body.Values["token"])
}

func TestSavePluginSettingsKeepsTheStoredSecret(t *testing.T) {
	manager := setupManager(t)
	installTestPlugin(t, manager, webappManifest("official.alpha"), nil, true)
	require.NoError(t, manager.SaveSettings(context.Background(), "official.alpha", map[string]any{
		"token": "s3cret",
	}))

	payload := `{"settings":{"endpoint":"https://changed.example","token":"` + settings.RedactedSensitiveValue + `"}}`
	c, recorder := newContext(http.MethodPost, "/api/plugins/official.alpha/settings",
		strings.NewReader(payload), gin.Params{{Key: "id", Value: "official.alpha"}})
	SavePluginSettings(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	var body struct {
		Values map[string]any `json:"values"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	assert.Equal(t, "https://changed.example", body.Values["endpoint"])
	assert.Equal(t, settings.RedactedSensitiveValue, body.Values["token"])

	stored, err := query.Plugin.WithContext(context.Background()).
		Where(query.Plugin.PluginID.Eq("official.alpha")).First()
	require.NoError(t, err)
	assert.Equal(t, "s3cret", stored.Settings["token"])
}

func TestGetPluginLogsIsEmptyForAWebappPlugin(t *testing.T) {
	manager := setupManager(t)
	installTestPlugin(t, manager, webappManifest("official.alpha"), nil, true)

	c, recorder := newContext(http.MethodGet, "/api/plugins/official.alpha/logs?lines=10", nil,
		gin.Params{{Key: "id", Value: "official.alpha"}})
	GetPluginLogs(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	var body struct {
		Lines []plugin.LogLine `json:"lines"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	assert.NotNil(t, body.Lines)
	assert.Empty(t, body.Lines)
}

func TestHandlersRejectMalformedPluginIDs(t *testing.T) {
	setupManager(t)

	c, recorder := newContext(http.MethodGet, "/api/plugins/..%2f..%2fetc/settings", nil,
		gin.Params{{Key: "id", Value: "../../etc"}})
	GetPluginSettings(c)
	assert.Equal(t, http.StatusInternalServerError, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "55002")
}

func TestGetSpecAndWebappEntries(t *testing.T) {
	manager := setupManager(t)
	installTestPlugin(t, manager, webappManifest("official.alpha"), map[string]string{
		"webapp/main.js": "export default {}",
	}, true)
	// A plugin that is installed but off never reaches the browser.
	installTestPlugin(t, manager, webappManifest("official.beta"), map[string]string{
		"webapp/main.js": "export default {}",
	}, false)

	c, recorder := newContext(http.MethodGet, "/api/plugins/spec", nil, nil)
	GetSpec(c)
	require.Equal(t, http.StatusOK, recorder.Code)
	var spec plugin.Spec
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &spec))
	assert.Equal(t, []int{protocol.APIVersion}, spec.APIVersions)
	assert.Equal(t, []string{protocol.TransportStdio, protocol.TransportGRPC}, spec.Transports)
	assert.Contains(t, spec.Capabilities, protocol.CapabilityDNS01)

	c, recorder = newContext(http.MethodGet, "/api/plugins/webapp", nil, nil)
	GetWebappEntries(c)
	require.Equal(t, http.StatusOK, recorder.Code)
	var entries []plugin.WebappEntry
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &entries))
	require.Len(t, entries, 1)
	assert.Equal(t, "official.alpha", entries[0].ID)
	assert.Equal(t, "plugins/official.alpha/webapp/main.js", entries[0].BundleURL)
}

func TestServeWebappServesOnlyFilesInsideThePlugin(t *testing.T) {
	manager := setupManager(t)
	installTestPlugin(t, manager, webappManifest("official.alpha"), map[string]string{
		"webapp/main.js": "export default {}",
		"secret.txt":     "not served",
	}, true)

	c, recorder := newContext(http.MethodGet, "/plugins/official.alpha/webapp/main.js", nil,
		gin.Params{{Key: "id", Value: "official.alpha"}, {Key: "filepath", Value: "/main.js"}})
	ServeWebapp(c)
	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "export default")

	// Traversal out of the served directory is refused.
	c, recorder = newContext(http.MethodGet, "/plugins/official.alpha/webapp/x", nil,
		gin.Params{{Key: "id", Value: "official.alpha"}, {Key: "filepath", Value: "/../secret.txt"}})
	ServeWebapp(c)
	assert.Equal(t, http.StatusNotFound, recorder.Code)

	// A disabled plugin is not served at all.
	_, err := manager.Disable(context.Background(), "official.alpha")
	require.NoError(t, err)
	c, recorder = newContext(http.MethodGet, "/plugins/official.alpha/webapp/main.js", nil,
		gin.Params{{Key: "id", Value: "official.alpha"}, {Key: "filepath", Value: "/main.js"}})
	ServeWebapp(c)
	assert.Equal(t, http.StatusNotFound, recorder.Code)
}

func TestServeWebappNeverHandsOutTheManifest(t *testing.T) {
	manager := setupManager(t)

	// A manifest may keep its bundle at the plugin root, which makes the whole
	// plugin directory the served root.
	manifest := webappManifest("official.alpha")
	manifest.Webapp = &protocol.ManifestWebapp{BundlePath: "main.js"}
	installTestPlugin(t, manager, manifest, map[string]string{"main.js": "export default {}"}, true)

	c, recorder := newContext(http.MethodGet, "/plugins/official.alpha/webapp/main.js", nil,
		gin.Params{{Key: "id", Value: "official.alpha"}, {Key: "filepath", Value: "/main.js"}})
	ServeWebapp(c)
	assert.Equal(t, http.StatusOK, recorder.Code)

	c, recorder = newContext(http.MethodGet, "/plugins/official.alpha/webapp/plugin.json", nil,
		gin.Params{{Key: "id", Value: "official.alpha"}, {Key: "filepath", Value: "/" + plugin.ManifestFileName}})
	ServeWebapp(c)
	assert.Equal(t, http.StatusNotFound, recorder.Code)
}

// getStatic requests a plugin asset through the registered static routes.
func getStatic(engine *gin.Engine, target, modifiedSince string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, target, nil)
	if modifiedSince != "" {
		request.Header.Set("If-Modified-Since", modifiedSince)
	}
	engine.ServeHTTP(recorder, request)
	return recorder
}

func TestServePageServesIndexHTML(t *testing.T) {
	manager := setupManager(t)
	installTestPlugin(t, manager, webappManifest("official.alpha"), map[string]string{
		"pages/index.html":        "<h1>root</h1>",
		"pages/nested/index.html": "<h1>nested</h1>",
	}, true)

	engine := gin.New()
	InitStaticRouter(engine)

	for target, want := range map[string]string{
		"/plugins/official.alpha/pages/index.html":        "<h1>root</h1>",
		"/plugins/official.alpha/pages/nested/index.html": "<h1>nested</h1>",
	} {
		recorder := getStatic(engine, target, "")
		assert.Equal(t, http.StatusOK, recorder.Code, target)
		assert.Empty(t, recorder.Header().Get("Location"), target)
		assert.Equal(t, want, recorder.Body.String(), target)
		assert.Contains(t, recorder.Header().Get("Content-Type"), "text/html", target)
	}
}

func TestStaticRoutesRevalidateAgainstThePluginFile(t *testing.T) {
	// Embed builds run CacheJs in front of every route, so the handler has to
	// replace its headers as well.
	for name, embedChain := range map[string]bool{"static routes": false, "embed chain": true} {
		t.Run(name, func(t *testing.T) {
			manager := setupManager(t)
			installTestPlugin(t, manager, webappManifest("official.alpha"), map[string]string{
				"webapp/main.js": "export default 1",
			}, true)

			engine := gin.New()
			if embedChain {
				engine.Use(middleware.CacheJs())
			}
			InitStaticRouter(engine)
			const target = "/plugins/official.alpha/webapp/main.js"

			first := getStatic(engine, target, "")
			require.Equal(t, http.StatusOK, first.Code)
			assert.Equal(t, "no-cache", first.Header().Get("Cache-Control"))
			assert.Equal(t, "export default 1", first.Body.String())
			lastModified := first.Header().Get("Last-Modified")
			require.NotEmpty(t, lastModified)
			assert.NotEqual(t, settings.LastModified, lastModified)

			if !embedChain {
				// The host build time says nothing about a plugin file.
				assert.Equal(t, http.StatusOK, getStatic(engine, target, settings.LastModified).Code)
			}

			// An unchanged file only revalidates.
			assert.Equal(t, http.StatusNotModified, getStatic(engine, target, lastModified).Code)

			// An upgrade rewrites the file, which is then served fresh.
			path := filepath.Join(settings.PluginSettings.Dir, "official.alpha", "webapp", "main.js")
			require.NoError(t, os.WriteFile(path, []byte("export default 2"), 0o644))
			later := time.Now().Add(time.Minute)
			require.NoError(t, os.Chtimes(path, later, later))

			fresh := getStatic(engine, target, lastModified)
			assert.Equal(t, http.StatusOK, fresh.Code)
			assert.Equal(t, "no-cache", fresh.Header().Get("Cache-Control"))
			assert.Equal(t, "export default 2", fresh.Body.String())
		})
	}
}

func TestGetPluginLogsRejectsAnUnknownPlugin(t *testing.T) {
	setupManager(t)

	c, recorder := newContext(http.MethodGet, "/api/plugins/official.ghost/logs", nil,
		gin.Params{{Key: "id", Value: "official.ghost"}})
	GetPluginLogs(c)
	assert.Equal(t, http.StatusInternalServerError, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "55002")
}
