package plugin

import (
	"archive/tar"
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
// signed when signer is set.
func dropPackage(t *testing.T, m *Manager, id, pluginVersion string, signer *minisign.PrivateKey) string {
	t.Helper()

	archive := buildSignedTestPackage(t, marketplaceManifest(id, pluginVersion),
		map[string]string{"webapp/main.js": "export default {}"}, signer)
	body, err := os.ReadFile(archive)
	require.NoError(t, err)

	require.NoError(t, os.MkdirAll(m.PackagesDir(), 0o755))
	target := filepath.Join(m.PackagesDir(), id+"-"+pluginVersion+packageSuffix)
	require.NoError(t, os.WriteFile(target, body, 0o644))
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
	settings.PluginSettings.DeveloperMode = false

	// An unsigned package stays where it is while developer mode is off.
	unsigned := dropPackage(t, manager, "com.example.offline", "1.0.0", nil)
	require.NoError(t, manager.ScanLocalPackages(context.Background()))
	_, err := manager.Get("com.example.offline")
	assert.ErrorIs(t, err, ErrPluginNotFound)
	assert.FileExists(t, unsigned)

	public, private := newSigningKey(t)
	trustKey(t, public)
	dropPackage(t, manager, "com.example.signed", "1.0.0", &private)
	require.NoError(t, manager.ScanLocalPackages(context.Background()))

	info, err := manager.Get("com.example.signed")
	require.NoError(t, err)
	assert.Equal(t, "1.0.0", info.Version)
	assert.Equal(t, TrustCommunity, info.Trust)
	assert.Equal(t, keyID(&private), info.Signer)
	// Only the archive moves, there is no detached signature any more.
	installed := filepath.Join(manager.PackagesDir(), installedDirName)
	assert.FileExists(t, filepath.Join(installed, "com.example.signed-1.0.0"+packageSuffix))
	assert.NoFileExists(t, filepath.Join(installed, "com.example.signed-1.0.0"+packageSuffix+".minisig"))

	// Developer mode lets the unsigned package in on the next scan.
	settings.PluginSettings.DeveloperMode = true
	require.NoError(t, manager.ScanLocalPackages(context.Background()))
	info, err = manager.Get("com.example.offline")
	require.NoError(t, err)
	assert.Equal(t, TrustUnsigned, info.Trust)
	assert.Empty(t, info.Signer)
	assert.NoFileExists(t, unsigned)
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

func TestPeekPackageReadsWithoutUnpacking(t *testing.T) {
	native := filepath.Join(t.TempDir(), "native.tar.gz")
	require.NoError(t, os.WriteFile(native,
		buildPlatformPackage(t, platformManifest("com.example.native", "1.0.0", foreignPlatform())), 0o644))
	wrapped := writeArchive(t, []tarEntry{
		{header: tar.Header{Name: "cloudflare/", Typeflag: tar.TypeDir, Mode: 0o755}},
		{header: tar.Header{Name: "cloudflare/" + ManifestFileName, Typeflag: tar.TypeReg}, body: manifestJSON(t)},
		{header: tar.Header{Name: "cloudflare/bin/plugin", Typeflag: tar.TypeReg}, body: "binary"},
	})

	// TMPDIR points at a missing directory, so unpacking into a temporary
	// directory would fail.
	missing := filepath.Join(t.TempDir(), "missing")
	t.Setenv("TMPDIR", missing)

	manifest, platforms, err := peekPackage(native)
	require.NoError(t, err)
	assert.Equal(t, "com.example.native", manifest.ID)
	assert.Equal(t, "1.0.0", manifest.Version)
	assert.Equal(t, []string{foreignPlatform()}, platforms)

	// A wrapper directory is stripped the way ExtractPackage strips it.
	manifest, platforms, err = peekPackage(wrapped)
	require.NoError(t, err)
	assert.Equal(t, "official.cloudflare", manifest.ID)
	assert.Equal(t, []string{"linux-amd64"}, platforms)

	assert.NoDirExists(t, missing, "peeking must not unpack the archive")
}

func TestFetchPackageWritesOnlyTheArchive(t *testing.T) {
	manager := newTestManager(t)
	server := newCatalogServer(t)
	useMarketplace(t, server.catalogURL())
	settings.PluginSettings.DeveloperMode = false

	signer := useReleaseKey(t)
	server.publish(t, marketplaceManifest("com.example.alpha", "1.0.0"), signer, nil)

	destination := t.TempDir()
	archive, err := manager.Marketplace().FetchPackage(context.Background(), "com.example.alpha", "", "", destination)
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(destination, "com.example.alpha-1.0.0"+packageSuffix), archive)
	assert.FileExists(t, archive)
	assert.NoFileExists(t, archive+".minisig")

	// The fetched archive carries its signature to the offline node.
	dir := filepath.Join(t.TempDir(), "payload")
	_, err = ExtractPackage(archive, dir)
	require.NoError(t, err)
	trust, err := verifyPackageSignature(dir, "")
	require.NoError(t, err)
	assert.Equal(t, TrustOfficial, trust.Trust)
}

func TestFetchPackageLeavesTheSignatureToTheInstall(t *testing.T) {
	manager := newTestManager(t)
	server := newCatalogServer(t)
	useMarketplace(t, server.catalogURL())
	settings.PluginSettings.DeveloperMode = false

	server.publish(t, marketplaceManifest("com.example.alpha", "1.0.0"), nil, nil)

	// Fetching an unsigned package works, installing it needs developer mode.
	archive, err := manager.Marketplace().FetchPackage(context.Background(), "com.example.alpha", "", "", t.TempDir())
	require.NoError(t, err)
	_, err = manager.Install(context.Background(), archive, InstallOptions{})
	assert.ErrorIs(t, err, ErrUnsignedPackage)
}
