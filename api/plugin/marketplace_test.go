package plugin

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	plugin "github.com/0xJacky/Nginx-UI/internal/plugin"
	"github.com/0xJacky/Nginx-UI/internal/plugin/protocol"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// marketplaceFixture serves a catalog plus the packages it advertises.
type marketplaceFixture struct {
	server   *httptest.Server
	document map[string]any
	files    map[string][]byte
}

// newMarketplaceFixture points the node at a throwaway catalog and restores
// the settings afterwards.
func newMarketplaceFixture(t *testing.T) *marketplaceFixture {
	t.Helper()

	fixture := &marketplaceFixture{files: map[string][]byte{}}
	fixture.document = map[string]any{
		"schema_version": plugin.CatalogSchemaVersion,
		"plugins":        []any{},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/index.json", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(fixture.document)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		body, ok := fixture.files[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(body)
	})
	fixture.server = httptest.NewServer(mux)
	t.Cleanup(fixture.server.Close)

	previousSources := settings.PluginSettings.MarketplaceSources
	previousEnabled := settings.PluginSettings.MarketplaceEnabled
	previousDeveloperMode := settings.PluginSettings.DeveloperMode
	previousInsecure := settings.PluginSettings.AllowInsecureDownloadURL
	t.Cleanup(func() {
		settings.PluginSettings.MarketplaceSources = previousSources
		settings.PluginSettings.MarketplaceEnabled = previousEnabled
		settings.PluginSettings.DeveloperMode = previousDeveloperMode
		settings.PluginSettings.AllowInsecureDownloadURL = previousInsecure
		plugin.GetManager().Marketplace().ClearCache()
	})

	settings.PluginSettings.MarketplaceSources = []string{fixture.server.URL + "/index.json"}
	settings.PluginSettings.MarketplaceEnabled = true
	settings.PluginSettings.DeveloperMode = true
	settings.PluginSettings.AllowInsecureDownloadURL = true
	// The manager is process wide, so a catalog from another test must go.
	plugin.GetManager().Marketplace().ClearCache()
	return fixture
}

func (f *marketplaceFixture) sourceURL() string { return f.server.URL + "/index.json" }

// publish packs a plugin and adds it to the catalog.
func (f *marketplaceFixture) publish(t *testing.T, manifest *protocol.Manifest, mutate func(entry map[string]any)) {
	t.Helper()

	staging := filepath.Join(t.TempDir(), manifest.ID)
	require.NoError(t, os.MkdirAll(filepath.Join(staging, "webapp"), 0o755))
	encoded, err := json.MarshalIndent(manifest, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(staging, plugin.ManifestFileName), encoded, 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(staging, "webapp", "main.js"), []byte("export default {}"), 0o644))

	archive := filepath.Join(t.TempDir(), manifest.ID+".tar.gz")
	require.NoError(t, plugin.BuildPackage(staging, archive))
	body, err := os.ReadFile(archive)
	require.NoError(t, err)

	name := "/pkg/" + manifest.ID + "-" + manifest.Version + ".tar.gz"
	f.files[name] = body
	digest := sha256.Sum256(body)

	entry := map[string]any{
		"id":         manifest.ID,
		"name":       map[string]string{"en": manifest.Name},
		"trust":      plugin.TrustOfficial,
		"categories": []string{"dns01"},
		"releases": []any{map[string]any{
			"version":      manifest.Version,
			"api_version":  manifest.APIVersion,
			"platforms":    []string{"any"},
			"download_url": f.server.URL + name,
			"sha256":       hex.EncodeToString(digest[:]),
			"manifest":     manifest,
		}},
	}
	if mutate != nil {
		mutate(entry)
	}
	f.document["plugins"] = append(f.document["plugins"].([]any), entry)
}

// marketplaceTestManifest is a plugin that never spawns a process.
func marketplaceTestManifest(id, pluginVersion string) *protocol.Manifest {
	return &protocol.Manifest{
		ID:         id,
		Name:       "Test " + id,
		Version:    pluginVersion,
		APIVersion: protocol.APIVersion,
		Webapp:     &protocol.ManifestWebapp{BundlePath: "webapp/main.js"},
	}
}

func TestGetMarketplaceListReturnsCatalogAndSources(t *testing.T) {
	setupManager(t)
	fixture := newMarketplaceFixture(t)
	fixture.publish(t, marketplaceTestManifest("com.example.alpha", "1.0.0"), nil)
	fixture.publish(t, marketplaceTestManifest("com.example.beta", "1.0.0"), func(entry map[string]any) {
		entry["categories"] = []string{"metrics"}
	})

	c, recorder := newContext(http.MethodGet, "/api/plugins/marketplace?refresh=true", nil, nil)
	GetMarketplaceList(c)
	require.Equal(t, http.StatusOK, recorder.Code)

	var body struct {
		Plugins []plugin.CatalogEntry `json:"plugins"`
		Sources []string              `json:"sources"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	require.Len(t, body.Plugins, 2)
	assert.Equal(t, []string{fixture.sourceURL()}, body.Sources)
	assert.Equal(t, "1.0.0", body.Plugins[0].InstallableRelease.Version)

	// The category filter reaches the marketplace.
	c, recorder = newContext(http.MethodGet, "/api/plugins/marketplace?category=metrics", nil, nil)
	GetMarketplaceList(c)
	require.Equal(t, http.StatusOK, recorder.Code)
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	require.Len(t, body.Plugins, 1)
	assert.Equal(t, "com.example.beta", body.Plugins[0].ID)
}

func TestGetMarketplacePluginReturnsReadme(t *testing.T) {
	setupManager(t)
	fixture := newMarketplaceFixture(t)
	fixture.files["/readme.md"] = []byte("# Alpha")
	fixture.publish(t, marketplaceTestManifest("com.example.alpha", "1.0.0"), func(entry map[string]any) {
		entry["readme_url"] = fixture.server.URL + "/readme.md"
	})

	c, recorder := newContext(http.MethodGet, "/api/plugins/marketplace/com.example.alpha", nil,
		gin.Params{{Key: "id", Value: "com.example.alpha"}})
	GetMarketplacePlugin(c)
	require.Equal(t, http.StatusOK, recorder.Code)

	var body struct {
		Plugin *plugin.CatalogEntry `json:"plugin"`
		Readme string               `json:"readme"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	require.NotNil(t, body.Plugin)
	assert.Equal(t, "# Alpha", body.Readme)
}

func TestGetMarketplacePluginRejectsAMalformedID(t *testing.T) {
	setupManager(t)
	newMarketplaceFixture(t)

	c, recorder := newContext(http.MethodGet, "/api/plugins/marketplace/..%2Fetc", nil,
		gin.Params{{Key: "id", Value: "../etc"}})
	GetMarketplacePlugin(c)
	assert.NotEqual(t, http.StatusOK, recorder.Code)
}

func TestInstallFromMarketplaceInstallsTheRelease(t *testing.T) {
	setupManager(t)
	fixture := newMarketplaceFixture(t)
	fixture.publish(t, marketplaceTestManifest("com.example.alpha", "1.0.0"), nil)

	payload := `{"id":"com.example.alpha","enable":true,"approve_permissions":true}`
	c, recorder := newContext(http.MethodPost, "/api/plugins/marketplace/install", strings.NewReader(payload), nil)
	InstallFromMarketplace(c)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())

	var info plugin.Info
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &info))
	assert.Equal(t, "com.example.alpha", info.ID)
	assert.True(t, info.Enabled)

	// The updates endpoint sees nothing to do right after the install.
	c, recorder = newContext(http.MethodGet, "/api/plugins/updates", nil, nil)
	GetPluginUpdates(c)
	require.Equal(t, http.StatusOK, recorder.Code)
	var updates []plugin.UpdateInfo
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &updates))
	assert.Empty(t, updates)
}

func TestInstallFromMarketplaceReportsErrors(t *testing.T) {
	setupManager(t)
	fixture := newMarketplaceFixture(t)
	fixture.publish(t, marketplaceTestManifest("com.example.alpha", "1.0.0"), func(entry map[string]any) {
		releases := entry["releases"].([]any)
		releases[0].(map[string]any)["sha256"] = strings.Repeat("ab", 32)
	})

	for _, testCase := range []struct{ name, payload string }{
		{"malformed id", `{"id":"../etc"}`},
		{"unknown plugin", `{"id":"com.example.absent"}`},
		{"digest mismatch", `{"id":"com.example.alpha"}`},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			c, recorder := newContext(http.MethodPost, "/api/plugins/marketplace/install",
				strings.NewReader(testCase.payload), nil)
			InstallFromMarketplace(c)
			assert.NotEqual(t, http.StatusOK, recorder.Code, recorder.Body.String())
		})
	}
}

func TestInstallFromMarketplaceRefusesWhenDisabled(t *testing.T) {
	setupManager(t)
	fixture := newMarketplaceFixture(t)
	fixture.publish(t, marketplaceTestManifest("com.example.alpha", "1.0.0"), nil)
	settings.PluginSettings.MarketplaceEnabled = false

	c, recorder := newContext(http.MethodPost, "/api/plugins/marketplace/install",
		strings.NewReader(`{"id":"com.example.alpha"}`), nil)
	InstallFromMarketplace(c)
	assert.NotEqual(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "55101")
}

func TestUpdatePluginUpgradesFromTheCatalog(t *testing.T) {
	manager := setupManager(t)
	fixture := newMarketplaceFixture(t)
	fixture.publish(t, marketplaceTestManifest("com.example.alpha", "1.0.0"), nil)

	_, err := manager.Marketplace().Install(context.Background(), "com.example.alpha", "", "",
		plugin.InstallOptions{Enable: true, ApprovePermissions: true})
	require.NoError(t, err)

	// A newer release shows up.
	fixture.document["plugins"] = []any{}
	fixture.publish(t, marketplaceTestManifest("com.example.alpha", "1.1.0"), nil)
	manager.Marketplace().ClearCache()

	c, recorder := newContext(http.MethodGet, "/api/plugins/updates", nil, nil)
	GetPluginUpdates(c)
	require.Equal(t, http.StatusOK, recorder.Code)
	var updates []plugin.UpdateInfo
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &updates))
	require.Len(t, updates, 1)
	assert.Equal(t, "1.1.0", updates[0].LatestVersion)

	c, recorder = newContext(http.MethodPost, "/api/plugins/com.example.alpha/update",
		strings.NewReader(`{}`), gin.Params{{Key: "id", Value: "com.example.alpha"}})
	UpdatePlugin(c)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())

	var info plugin.Info
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &info))
	assert.Equal(t, "1.1.0", info.Version)
}

func TestGetMarketplaceSourcesReportsTheDefault(t *testing.T) {
	setupManager(t)
	fixture := newMarketplaceFixture(t)

	c, recorder := newContext(http.MethodGet, "/api/plugins/marketplace/sources", nil, nil)
	GetMarketplaceSources(c)
	require.Equal(t, http.StatusOK, recorder.Code)

	var body marketplaceSourcesResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	assert.Equal(t, []string{fixture.sourceURL()}, body.Sources)
	assert.Equal(t, settings.DefaultPluginMarketplaceSource, body.Default)
}

// The literal marketplace routes share a segment with /plugins/:id, so gin has
// to accept both trees side by side.
func TestMarketplaceRoutesDoNotCollideWithThePluginRoutes(t *testing.T) {
	engine := gin.New()
	group := engine.Group("/api")

	require.NotPanics(t, func() {
		InitRouter(group)
		InitMarketplaceRouter(group)
	})

	registered := make(map[string]struct{})
	for _, route := range engine.Routes() {
		registered[route.Method+" "+route.Path] = struct{}{}
	}
	for _, want := range []string{
		"GET /api/plugins/marketplace",
		"GET /api/plugins/marketplace/sources",
		"GET /api/plugins/marketplace/:id",
		"GET /api/plugins/updates",
		"POST /api/plugins/marketplace/install",
		"POST /api/plugins/marketplace/sources",
		"POST /api/plugins/:id/update",
	} {
		_, ok := registered[want]
		assert.True(t, ok, "route %s is not registered", want)
	}
}

func TestNormalizeSources(t *testing.T) {
	previous := settings.PluginSettings.AllowInsecureDownloadURL
	t.Cleanup(func() { settings.PluginSettings.AllowInsecureDownloadURL = previous })
	settings.PluginSettings.AllowInsecureDownloadURL = false

	sources, err := normalizeSources([]string{
		" https://example.com/a.json ",
		"https://example.com/a.json",
		"",
	})
	require.NoError(t, err)
	assert.Equal(t, []string{"https://example.com/a.json"}, sources)

	_, err = normalizeSources([]string{"not a url"})
	assert.Error(t, err)

	_, err = normalizeSources([]string{"http://example.com/a.json"})
	assert.ErrorIs(t, err, plugin.ErrInsecureURL)

	settings.PluginSettings.AllowInsecureDownloadURL = true
	sources, err = normalizeSources([]string{"http://example.com/a.json"})
	require.NoError(t, err)
	assert.Len(t, sources, 1)
}
