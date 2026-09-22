package plugin

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0xJacky/Nginx-UI/internal/analytic"
	"github.com/0xJacky/Nginx-UI/internal/version"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// useFakeSyncClusterPlatforms is useFakeSyncCluster with the platform the
// monitor reports for each node.
func useFakeSyncClusterPlatforms(t *testing.T, platforms map[uint64]string) {
	t.Helper()
	online := make(map[uint64]bool, len(platforms))
	for id := range platforms {
		online[id] = true
	}
	useFakeSyncCluster(t, online)

	previous := nodeStatusSnapshot
	nodeStatusSnapshot = func() analytic.TNodeMap {
		snapshot := analytic.TNodeMap{}
		for id, platform := range platforms {
			goos, goarch, _ := strings.Cut(platform, "-")
			node := &analytic.Node{NodeStat: analytic.NodeStat{Status: true}}
			node.NodeRuntimeInfo = version.RuntimeInfo{OS: goos, Arch: goarch}
			snapshot[id] = node
		}
		return snapshot
	}
	t.Cleanup(func() { nodeStatusSnapshot = previous })
}

// installHostPackage installs the per-platform package of this controller.
func installHostPackage(t *testing.T, m *Manager, id, pluginVersion string) {
	t.Helper()
	archive := filepath.Join(t.TempDir(), PackageFileName(id, pluginVersion, HostPlatform()))
	require.NoError(t, os.WriteFile(archive, buildPlatformPackage(t, platformManifest(id, pluginVersion, HostPlatform())), 0o644))
	_, err := m.Install(context.Background(), archive, InstallOptions{Enable: true})
	require.NoError(t, err)
}

func TestSyncPluginPushesALocalPackageForTheNodePlatform(t *testing.T) {
	m, syncer := newSyncTestManager(t)
	useMarketplace(t)
	settings.PluginSettings.MarketplaceEnabled = false
	installHostPackage(t, m, "com.example.native", "1.0.0")

	foreign := foreignPlatform()
	body := buildPlatformPackage(t, platformManifest("com.example.native", "1.0.0", foreign))
	require.NoError(t, os.MkdirAll(m.PackagesDir(), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(m.PackagesDir(),
		PackageFileName("com.example.native", "1.0.0", foreign)), body, 0o644))

	fake := newFakeSyncNode(t)
	// The node names its platform in the plugin spec.
	fake.platform = foreign
	node := addSyncTestNode(t, "node-a", fake.server.URL, true)
	useFakeSyncCluster(t, map[uint64]bool{node.ID: true})

	results, err := syncer.SyncPlugin(context.Background(), "com.example.native", nil)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.True(t, results[0].Success, results[0].Error)
	assert.Equal(t, SyncStateInSync, results[0].State)

	fake.mu.Lock()
	defer fake.mu.Unlock()
	assert.Equal(t, 1, fake.uploadCalls)
	assert.Equal(t, body, fake.uploaded)
}

func TestSyncPluginFetchesTheCatalogPackageForTheNodePlatform(t *testing.T) {
	m, syncer := newSyncTestManager(t)
	server := newCatalogServer(t)
	useMarketplace(t, server.catalogURL())

	foreign := foreignPlatform()
	bodies := server.publishPlatforms(t, "com.example.native", "1.0.0", nil, HostPlatform(), foreign)
	installHostPackage(t, m, "com.example.native", "1.0.0")

	fake := newFakeSyncNode(t)
	node := addSyncTestNode(t, "node-a", fake.server.URL, true)
	// This node is older and only the monitor knows its platform.
	useFakeSyncClusterPlatforms(t, map[uint64]string{node.ID: foreign})

	results, err := syncer.SyncPlugin(context.Background(), "com.example.native", nil)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.True(t, results[0].Success, results[0].Error)

	fake.mu.Lock()
	assert.Equal(t, 1, fake.marketCalls)
	assert.Equal(t, bodies[foreign], fake.uploaded)
	fake.mu.Unlock()

	// The download is kept for the next node of that platform.
	cached := filepath.Join(m.platformCacheDir(), PackageFileName("com.example.native", "1.0.0", foreign))
	assert.FileExists(t, cached)

	// A second run reuses it even when the catalog is gone.
	server.Close()
	m.Marketplace().ClearCache()
	fake.mu.Lock()
	fake.inventory = nil
	fake.mu.Unlock()
	results, err = syncer.SyncPlugin(context.Background(), "com.example.native", nil)
	require.NoError(t, err)
	assert.True(t, results[0].Success, results[0].Error)

	fake.mu.Lock()
	defer fake.mu.Unlock()
	assert.Equal(t, 2, fake.uploadCalls)
	assert.Equal(t, bodies[foreign], fake.uploaded)
}

func TestSyncPluginKeepsTheOwnPackageForTheSamePlatform(t *testing.T) {
	m, syncer := newSyncTestManager(t)
	useMarketplace(t)
	settings.PluginSettings.MarketplaceEnabled = false
	installHostPackage(t, m, "com.example.native", "1.0.0")

	fake := newFakeSyncNode(t)
	fake.platform = HostPlatform()
	node := addSyncTestNode(t, "node-a", fake.server.URL, true)
	useFakeSyncCluster(t, map[uint64]bool{node.ID: true})

	results, err := syncer.SyncPlugin(context.Background(), "com.example.native", nil)
	require.NoError(t, err)
	assert.True(t, results[0].Success, results[0].Error)

	own, ok := m.ArchivePath("com.example.native")
	require.True(t, ok)
	body, err := os.ReadFile(own)
	require.NoError(t, err)

	fake.mu.Lock()
	defer fake.mu.Unlock()
	assert.Equal(t, body, fake.uploaded)
}

func TestSyncPluginReportsAnUnsupportedPlatform(t *testing.T) {
	m, syncer := newSyncTestManager(t)
	server := newCatalogServer(t)
	useMarketplace(t, server.catalogURL())

	// The catalog only has a build for the controller.
	server.publishPlatforms(t, "com.example.native", "1.0.0", nil, HostPlatform())
	installHostPackage(t, m, "com.example.native", "1.0.0")

	foreign := foreignPlatform()
	fake := newFakeSyncNode(t)
	fake.platform = foreign
	node := addSyncTestNode(t, "node-a", fake.server.URL, true)
	useFakeSyncCluster(t, map[uint64]bool{node.ID: true})

	results, err := syncer.SyncPlugin(context.Background(), "com.example.native", nil)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.False(t, results[0].Success)
	assert.Equal(t, SyncStateUnsupportedPlatform, results[0].State)
	assert.Contains(t, results[0].Error, foreign)

	fake.mu.Lock()
	assert.Equal(t, 0, fake.uploadCalls)
	fake.mu.Unlock()

	// The matrix agrees before anything is pushed.
	matrix, err := syncer.Matrix(context.Background())
	require.NoError(t, err)
	require.Len(t, matrix.Rows, 1)
	require.Len(t, matrix.Rows[0].Cells, 1)
	assert.Equal(t, SyncStateUnsupportedPlatform, matrix.Rows[0].Cells[0].State)
	assert.Equal(t, foreign, matrix.Nodes[0].OS+"-"+matrix.Nodes[0].Arch)
}

func TestMatrixCountsACatalogBuildAsSupported(t *testing.T) {
	m, syncer := newSyncTestManager(t)
	server := newCatalogServer(t)
	useMarketplace(t, server.catalogURL())

	foreign := foreignPlatform()
	server.publishPlatforms(t, "com.example.native", "1.0.0", nil, HostPlatform(), foreign)
	installHostPackage(t, m, "com.example.native", "1.0.0")

	fake := newFakeSyncNode(t)
	node := addSyncTestNode(t, "node-a", fake.server.URL, true)
	useFakeSyncClusterPlatforms(t, map[uint64]string{node.ID: foreign})

	matrix, err := syncer.Matrix(context.Background())
	require.NoError(t, err)
	require.Len(t, matrix.Rows, 1)
	assert.Equal(t, SyncStateMissing, matrix.Rows[0].Cells[0].State)
}
