package plugin

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"aead.dev/minisign"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/0xJacky/Nginx-UI/settings"
)

// dropPackage copies a built archive into the offline package directory,
// optionally with a detached signature.
func dropPackage(t *testing.T, m *Manager, id, pluginVersion string, signer *minisign.PrivateKey) string {
	t.Helper()

	archive := buildTestPackage(t, marketplaceManifest(id, pluginVersion),
		map[string]string{"webapp/main.js": "export default {}"})
	body, err := os.ReadFile(archive)
	require.NoError(t, err)

	require.NoError(t, os.MkdirAll(m.PackagesDir(), 0o755))
	target := filepath.Join(m.PackagesDir(), id+"-"+pluginVersion+packageSuffix)
	require.NoError(t, os.WriteFile(target, body, 0o644))

	if signer != nil {
		require.NoError(t, os.WriteFile(target+signatureSuffix, signPackage(t, body, *signer), 0o644))
	}
	return target
}

func TestScanLocalPackagesInstallsAndArchives(t *testing.T) {
	manager := newTestManager(t)
	useMarketplace(t)

	archive := dropPackage(t, manager, "com.example.offline", "1.0.0", nil)
	require.NoError(t, manager.ScanLocalPackages(context.Background()))

	info, err := manager.Get("com.example.offline")
	require.NoError(t, err)
	assert.Equal(t, "1.0.0", info.Version)

	// The archive moved out of the scan path so the next boot ignores it.
	assert.NoFileExists(t, archive)
	assert.FileExists(t, filepath.Join(manager.PackagesDir(), installedDirName, filepath.Base(archive)))
}

func TestScanLocalPackagesUpgradesOnlyNewerVersions(t *testing.T) {
	manager := newTestManager(t)
	useMarketplace(t)

	dropPackage(t, manager, "com.example.offline", "1.0.0", nil)
	require.NoError(t, manager.ScanLocalPackages(context.Background()))

	dropPackage(t, manager, "com.example.offline", "0.9.0", nil)
	dropPackage(t, manager, "com.example.offline", "1.1.0", nil)
	require.NoError(t, manager.ScanLocalPackages(context.Background()))

	info, err := manager.Get("com.example.offline")
	require.NoError(t, err)
	assert.Equal(t, "1.1.0", info.Version)
}

func TestScanLocalPackagesVerifiesSignatures(t *testing.T) {
	manager := newTestManager(t)
	useMarketplace(t)
	settings.PluginSettings.RequireSignature = true

	dropPackage(t, manager, "com.example.offline", "1.0.0", nil)
	require.NoError(t, manager.ScanLocalPackages(context.Background()))
	_, err := manager.Get("com.example.offline")
	assert.ErrorIs(t, err, ErrPluginNotFound)

	public, private := newSigningKey(t)
	trustKey(t, public)
	dropPackage(t, manager, "com.example.signed", "1.0.0", &private)
	require.NoError(t, manager.ScanLocalPackages(context.Background()))

	info, err := manager.Get("com.example.signed")
	require.NoError(t, err)
	assert.Equal(t, "1.0.0", info.Version)
	assert.FileExists(t, filepath.Join(manager.PackagesDir(), installedDirName,
		"com.example.signed-1.0.0"+packageSuffix+signatureSuffix))
}

func TestInstallLocalPackagePicksTheNewestBuild(t *testing.T) {
	manager := newTestManager(t)
	useMarketplace(t)

	dropPackage(t, manager, "com.example.offline", "1.0.0", nil)
	dropPackage(t, manager, "com.example.offline", "2.0.0", nil)

	info, err := manager.InstallLocalPackage(context.Background(), "com.example.offline",
		InstallOptions{Enable: true, ApprovePermissions: true})
	require.NoError(t, err)
	assert.Equal(t, "2.0.0", info.Version)
	assert.True(t, info.Enabled)

	_, err = manager.InstallLocalPackage(context.Background(), "com.example.absent", InstallOptions{})
	assert.ErrorIs(t, err, ErrLocalPackageNotFound)
}

func TestFetchPackageWritesArchiveAndSignature(t *testing.T) {
	manager := newTestManager(t)
	server := newCatalogServer(t)
	useMarketplace(t, server.catalogURL())

	public, private := newSigningKey(t)
	trustKey(t, public)
	settings.PluginSettings.RequireSignature = true
	server.publish(t, marketplaceManifest("com.example.alpha", "1.0.0"), &private, nil)

	destination := t.TempDir()
	archive, err := manager.Marketplace().FetchPackage(context.Background(), "com.example.alpha", "", destination)
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(destination, "com.example.alpha-1.0.0"+packageSuffix), archive)
	assert.FileExists(t, archive)
	assert.FileExists(t, archive+signatureSuffix)

	// The fetched archive is a valid package for an offline install.
	manifest, err := peekPackageManifest(archive)
	require.NoError(t, err)
	assert.Equal(t, "1.0.0", manifest.Version)
}

func TestFetchPackageRefusesAnUnsignedRelease(t *testing.T) {
	manager := newTestManager(t)
	server := newCatalogServer(t)
	useMarketplace(t, server.catalogURL())
	settings.PluginSettings.RequireSignature = true

	server.publish(t, marketplaceManifest("com.example.alpha", "1.0.0"), nil, nil)

	_, err := manager.Marketplace().FetchPackage(context.Background(), "com.example.alpha", "", t.TempDir())
	assert.ErrorIs(t, err, ErrSignatureMissing)
}
