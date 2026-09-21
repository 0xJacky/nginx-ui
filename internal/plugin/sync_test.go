package plugin

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/0xJacky/Nginx-UI/internal/analytic"
	"github.com/0xJacky/Nginx-UI/internal/plugin/protocol"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/go-resty/resty/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// fakeSyncNode is a child node exposing the plugin endpoints the syncer drives.
type fakeSyncNode struct {
	server *httptest.Server

	mu sync.Mutex
	// inventory is what GET /api/plugins answers.
	inventory []Info
	// apiVersions is what GET /api/plugins/spec advertises.
	apiVersions []int
	// marketplaceStatus lets a test simulate an offline catalog.
	marketplaceStatus int

	uploaded      []byte
	uploadEnable  string
	uploadCalls   int
	marketCalls   int
	enableCalls   []string
	disableCalls  []string
	settingsCalls []map[string]any
}

func newFakeSyncNode(t *testing.T) *fakeSyncNode {
	t.Helper()

	node := &fakeSyncNode{
		inventory:         []Info{},
		apiVersions:       []int{protocol.APIVersion},
		marketplaceStatus: http.StatusNotFound,
	}
	mux := http.NewServeMux()

	mux.HandleFunc("/api/plugins/spec", func(w http.ResponseWriter, _ *http.Request) {
		node.mu.Lock()
		spec := Spec{APIVersions: node.apiVersions, WebappAPIVersion: protocol.APIVersion}
		node.mu.Unlock()
		writeSyncJSON(w, spec)
	})

	mux.HandleFunc("/api/plugins/marketplace/install", func(w http.ResponseWriter, _ *http.Request) {
		node.mu.Lock()
		node.marketCalls++
		status := node.marketplaceStatus
		node.mu.Unlock()
		w.WriteHeader(status)
		_, _ = w.Write([]byte(`{}`))
	})

	mux.HandleFunc("/api/plugins", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			node.mu.Lock()
			inventory := append([]Info(nil), node.inventory...)
			node.mu.Unlock()
			writeSyncJSON(w, inventory)
			return
		}

		file, _, err := r.FormFile("file")
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		defer file.Close()
		payload, err := io.ReadAll(file)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		node.mu.Lock()
		node.uploadCalls++
		node.uploaded = payload
		node.uploadEnable = r.FormValue("enable")
		node.mu.Unlock()
		writeSyncJSON(w, Info{})
	})

	mux.HandleFunc("/api/plugins/", func(w http.ResponseWriter, r *http.Request) {
		id, action := splitSyncPluginPath(r.URL.Path)
		node.mu.Lock()
		defer node.mu.Unlock()
		switch action {
		case "enable":
			node.enableCalls = append(node.enableCalls, id)
		case "disable":
			node.disableCalls = append(node.disableCalls, id)
		case "settings":
			var body struct {
				Settings map[string]any `json:"settings"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			node.settingsCalls = append(node.settingsCalls, body.Settings)
		default:
			w.WriteHeader(http.StatusNotFound)
			return
		}
		writeSyncJSON(w, map[string]any{})
	})

	node.server = httptest.NewServer(mux)
	t.Cleanup(node.server.Close)
	return node
}

func writeSyncJSON(w http.ResponseWriter, payload any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(payload)
}

// splitSyncPluginPath reads "/api/plugins/<id>/<action>".
func splitSyncPluginPath(path string) (string, string) {
	rest := path[len("/api/plugins/"):]
	for index := len(rest) - 1; index >= 0; index-- {
		if rest[index] == '/' {
			return rest[:index], rest[index+1:]
		}
	}
	return rest, ""
}

// useFakeSyncCluster points the syncer at plain HTTP clients and at a fixed node
// status snapshot, so no cluster credentials and no monitor are needed.
func useFakeSyncCluster(t *testing.T, online map[uint64]bool) {
	t.Helper()

	previousClient := nodeRestyClient
	nodeRestyClient = func(_ *model.Node) *resty.Client { return resty.New() }

	previousStatus := nodeStatusSnapshot
	nodeStatusSnapshot = func() analytic.TNodeMap {
		snapshot := analytic.TNodeMap{}
		for id, up := range online {
			snapshot[id] = &analytic.Node{NodeStat: analytic.NodeStat{Status: up}}
		}
		return snapshot
	}

	t.Cleanup(func() {
		nodeRestyClient = previousClient
		nodeStatusSnapshot = previousStatus
	})
}

// setupSyncTestDB wires a database that also knows about nodes.
func setupSyncTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Plugin{}, &model.PluginKV{}, &model.Node{}))
	model.Use(db)
	query.Init(db)
	t.Cleanup(func() { model.Use(nil) })
	return db
}

// newSyncTestManager returns a manager and its syncer, both rooted in a fresh
// directory and database.
func newSyncTestManager(t *testing.T) (*Manager, *Syncer) {
	t.Helper()
	setupSyncTestDB(t)
	m := newManager(t.TempDir())
	m.offline = true
	require.NoError(t, m.discover(context.Background()))
	t.Cleanup(func() { m.Stop(context.Background()) })
	return m, m.Syncer()
}

// webappOnlyManifest describes a plugin that spawns no process.
func webappOnlyManifest(id, version string) *protocol.Manifest {
	return &protocol.Manifest{
		ID:         id,
		Name:       id,
		Version:    version,
		APIVersion: protocol.APIVersion,
		Webapp:     &protocol.ManifestWebapp{BundlePath: "webapp/main.js"},
		SettingsSchema: &protocol.SettingsSchema{
			Settings: []protocol.SettingsField{
				{Key: "endpoint", Type: "text", DisplayName: "Endpoint"},
				{Key: "token", Type: "secret", DisplayName: "Token"},
			},
		},
	}
}

// installSyncTestPlugin packs and installs a process free plugin.
func installSyncTestPlugin(t *testing.T, m *Manager, id, version string, enable bool) {
	t.Helper()
	archive := buildTestPackage(t, webappOnlyManifest(id, version), map[string]string{
		"webapp/main.js": "export default {}",
	})
	_, err := m.Install(context.Background(), archive, InstallOptions{Enable: enable})
	require.NoError(t, err)
}

// addNode stores an enabled node pointing at the fake server.
func addSyncTestNode(t *testing.T, name, url string, acceptPluginSync bool) *model.Node {
	t.Helper()
	node := &model.Node{Name: name, URL: url, Enabled: true, AcceptPluginSync: acceptPluginSync}
	require.NoError(t, model.UseDB().Create(node).Error)
	// The column default is true, so opting out has to be written back.
	require.NoError(t, model.UseDB().Model(node).
		Update("accept_plugin_sync", acceptPluginSync).Error)
	node.AcceptPluginSync = acceptPluginSync
	return node
}

func TestKeepArchiveStoresTheInstalledPackage(t *testing.T) {
	m, _ := newSyncTestManager(t)
	installSyncTestPlugin(t, m, "official.alpha", "1.0.0", false)

	path, ok := m.ArchivePath("official.alpha")
	require.True(t, ok)
	assert.Equal(t, "official.alpha-1.0.0.tar.gz", filepath.Base(path))

	// An upgrade replaces the kept package instead of piling versions up.
	installSyncTestPlugin(t, m, "official.alpha", "1.1.0", false)
	path, ok = m.ArchivePath("official.alpha")
	require.True(t, ok)
	assert.Equal(t, "official.alpha-1.1.0.tar.gz", filepath.Base(path))

	entries, err := os.ReadDir(m.archivesDir())
	require.NoError(t, err)
	assert.Len(t, entries, 1)
}

func TestEnsureArchiveBuildsOneWhenNoneWasKept(t *testing.T) {
	m, _ := newSyncTestManager(t)
	// A plugin dropped into the directory by hand never goes through Install.
	writePluginDir(t, filepath.Join(m.Dir(), "official.copied"), webappOnlyManifest("official.copied", "2.0.0"))
	require.NoError(t, m.discover(context.Background()))

	_, ok := m.ArchivePath("official.copied")
	require.False(t, ok)

	archive, cleanup, err := m.EnsureArchive("official.copied")
	defer cleanup()
	require.NoError(t, err)

	manifest, err := ExtractPackage(archive, filepath.Join(t.TempDir(), "payload"))
	require.NoError(t, err)
	assert.Equal(t, "official.copied", manifest.ID)
	assert.Equal(t, "2.0.0", manifest.Version)
}

func TestSyncPluginUploadsThePackageWhenTheMarketplaceIsUnavailable(t *testing.T) {
	m, syncer := newSyncTestManager(t)
	installSyncTestPlugin(t, m, "official.alpha", "1.0.0", true)

	fake := newFakeSyncNode(t)
	node := addSyncTestNode(t, "node-a", fake.server.URL, true)
	useFakeSyncCluster(t, map[uint64]bool{node.ID: true})

	results, err := syncer.SyncPlugin(context.Background(), "official.alpha", nil)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.True(t, results[0].Success)
	assert.Equal(t, SyncStateInSync, results[0].State)
	assert.Equal(t, []string{"installed"}, results[0].Actions)

	fake.mu.Lock()
	defer fake.mu.Unlock()
	assert.Equal(t, 1, fake.marketCalls)
	assert.Equal(t, 1, fake.uploadCalls)
	assert.Equal(t, "true", fake.uploadEnable)
	assert.NotEmpty(t, fake.uploaded)
}

func TestSyncPluginPrefersTheMarketplace(t *testing.T) {
	m, syncer := newSyncTestManager(t)
	installSyncTestPlugin(t, m, "official.alpha", "1.0.0", true)

	fake := newFakeSyncNode(t)
	fake.marketplaceStatus = http.StatusOK
	node := addSyncTestNode(t, "node-a", fake.server.URL, true)
	useFakeSyncCluster(t, map[uint64]bool{node.ID: true})

	results, err := syncer.SyncPlugin(context.Background(), "official.alpha", nil)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.True(t, results[0].Success)

	fake.mu.Lock()
	defer fake.mu.Unlock()
	assert.Equal(t, 1, fake.marketCalls)
	assert.Equal(t, 0, fake.uploadCalls)
}

func TestSyncPluginAlignsEnabledStateAndPushesSettings(t *testing.T) {
	m, syncer := newSyncTestManager(t)
	installSyncTestPlugin(t, m, "official.alpha", "1.0.0", true)
	require.NoError(t, m.SaveSettings(context.Background(), "official.alpha", map[string]any{
		"endpoint": "https://plugin.example",
		"token":    "s3cret",
	}))
	require.NoError(t, m.setSyncPolicy(context.Background(), "official.alpha",
		model.PluginSyncPolicyAuto, nil, true))

	fake := newFakeSyncNode(t)
	fake.inventory = []Info{{ID: "official.alpha", Version: "1.0.0", Enabled: false}}
	node := addSyncTestNode(t, "node-a", fake.server.URL, true)
	useFakeSyncCluster(t, map[uint64]bool{node.ID: true})

	results, err := syncer.SyncPlugin(context.Background(), "official.alpha", nil)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.True(t, results[0].Success)
	assert.Equal(t, []string{"enabled", "settings"}, results[0].Actions)

	fake.mu.Lock()
	defer fake.mu.Unlock()
	assert.Equal(t, 0, fake.uploadCalls)
	assert.Equal(t, []string{"official.alpha"}, fake.enableCalls)
	require.Len(t, fake.settingsCalls, 1)
	// Secrets are read from the row, not through the redacting API.
	assert.Equal(t, "s3cret", fake.settingsCalls[0]["token"])
}

func TestSyncPluginSkipsAnOptedOutNode(t *testing.T) {
	m, syncer := newSyncTestManager(t)
	installSyncTestPlugin(t, m, "official.alpha", "1.0.0", true)

	fake := newFakeSyncNode(t)
	node := addSyncTestNode(t, "node-a", fake.server.URL, false)
	useFakeSyncCluster(t, map[uint64]bool{node.ID: true})

	results, err := syncer.SyncPlugin(context.Background(), "official.alpha", nil)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, SyncStateOptedOut, results[0].State)

	fake.mu.Lock()
	defer fake.mu.Unlock()
	assert.Equal(t, 0, fake.uploadCalls)
	assert.Equal(t, 0, fake.marketCalls)
}

func TestSyncPluginRefusesANodeSpeakingAnotherAPIVersion(t *testing.T) {
	m, syncer := newSyncTestManager(t)
	installSyncTestPlugin(t, m, "official.alpha", "1.0.0", true)

	fake := newFakeSyncNode(t)
	fake.apiVersions = []int{protocol.APIVersion + 7}
	node := addSyncTestNode(t, "node-a", fake.server.URL, true)
	useFakeSyncCluster(t, map[uint64]bool{node.ID: true})

	results, err := syncer.SyncPlugin(context.Background(), "official.alpha", nil)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.False(t, results[0].Success)
	assert.Equal(t, SyncStateUnsupported, results[0].State)

	fake.mu.Lock()
	defer fake.mu.Unlock()
	assert.Equal(t, 0, fake.uploadCalls)
}

func TestSyncPluginHonoursTheConfiguredNodeSet(t *testing.T) {
	m, syncer := newSyncTestManager(t)
	installSyncTestPlugin(t, m, "official.alpha", "1.0.0", true)

	first := newFakeSyncNode(t)
	second := newFakeSyncNode(t)
	nodeA := addSyncTestNode(t, "node-a", first.server.URL, true)
	nodeB := addSyncTestNode(t, "node-b", second.server.URL, true)
	useFakeSyncCluster(t, map[uint64]bool{nodeA.ID: true, nodeB.ID: true})

	require.NoError(t, m.setSyncPolicy(context.Background(), "official.alpha",
		model.PluginSyncPolicyAuto, []uint64{nodeB.ID}, false))

	results, err := syncer.SyncPlugin(context.Background(), "official.alpha", nil)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "node-b", results[0].Node)

	first.mu.Lock()
	assert.Equal(t, 0, first.uploadCalls)
	first.mu.Unlock()
	second.mu.Lock()
	assert.Equal(t, 1, second.uploadCalls)
	second.mu.Unlock()
}

func TestMatrixReportsEveryStatePerNode(t *testing.T) {
	m, syncer := newSyncTestManager(t)
	installSyncTestPlugin(t, m, "official.alpha", "2.0.0", true)

	inSync := newFakeSyncNode(t)
	inSync.inventory = []Info{{
		ID: "official.alpha", Version: "2.0.0", Enabled: true, Status: StatusRunning,
	}}
	outdated := newFakeSyncNode(t)
	outdated.inventory = []Info{{ID: "official.alpha", Version: "1.0.0", Enabled: true}}
	missing := newFakeSyncNode(t)

	nodeSync := addSyncTestNode(t, "a-in-sync", inSync.server.URL, true)
	nodeOutdated := addSyncTestNode(t, "b-outdated", outdated.server.URL, true)
	nodeMissing := addSyncTestNode(t, "c-missing", missing.server.URL, true)
	nodeOffline := addSyncTestNode(t, "d-offline", "http://127.0.0.1:1", true)
	nodeOptedOut := addSyncTestNode(t, "e-opted-out", inSync.server.URL, false)

	useFakeSyncCluster(t, map[uint64]bool{
		nodeSync.ID: true, nodeOutdated.ID: true, nodeMissing.ID: true,
		nodeOffline.ID: false, nodeOptedOut.ID: true,
	})

	matrix, err := syncer.Matrix(context.Background())
	require.NoError(t, err)
	require.Len(t, matrix.Nodes, 5)
	require.Len(t, matrix.Rows, 1)

	row := matrix.Rows[0]
	assert.Equal(t, "official.alpha", row.PluginID)
	assert.Equal(t, "2.0.0", row.Version)
	assert.Equal(t, model.PluginSyncPolicyManual, row.SyncPolicy)
	require.Len(t, row.Cells, 5)

	states := map[uint64]string{}
	for _, cell := range row.Cells {
		states[cell.NodeID] = cell.State
	}
	assert.Equal(t, SyncStateInSync, states[nodeSync.ID])
	assert.Equal(t, SyncStateOutdated, states[nodeOutdated.ID])
	assert.Equal(t, SyncStateMissing, states[nodeMissing.ID])
	assert.Equal(t, SyncStateOffline, states[nodeOffline.ID])
	assert.Equal(t, SyncStateOptedOut, states[nodeOptedOut.ID])
}

func TestSetPolicyPersistsTheSyncIntent(t *testing.T) {
	m, syncer := newSyncTestManager(t)
	installSyncTestPlugin(t, m, "official.alpha", "1.0.0", false)

	require.NoError(t, syncer.SetPolicy(context.Background(), "official.alpha",
		model.PluginSyncPolicyAuto, []uint64{7, 9}, true))

	info, err := m.Get("official.alpha")
	require.NoError(t, err)
	assert.Equal(t, model.PluginSyncPolicyAuto, info.SyncPolicy)
	assert.Equal(t, []uint64{7, 9}, info.SyncNodeIDs)
	assert.True(t, info.SyncSettings)

	stored, err := query.Plugin.WithContext(context.Background()).
		Where(query.Plugin.PluginID.Eq("official.alpha")).First()
	require.NoError(t, err)
	assert.Equal(t, model.PluginSyncPolicyAuto, stored.SyncPolicy)
	assert.Equal(t, []uint64{7, 9}, stored.SyncNodeIDs)

	// An unknown policy falls back to manual instead of being stored verbatim.
	require.NoError(t, syncer.SetPolicy(context.Background(), "official.alpha", "nonsense", nil, false))
	info, err = m.Get("official.alpha")
	require.NoError(t, err)
	assert.Equal(t, model.PluginSyncPolicyManual, info.SyncPolicy)
}

func TestSyncPluginRejectsAnUnknownPlugin(t *testing.T) {
	_, syncer := newSyncTestManager(t)
	_, err := syncer.SyncPlugin(context.Background(), "official.ghost", nil)
	assert.ErrorIs(t, err, ErrPluginNotFound)
}
