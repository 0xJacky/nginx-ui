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

// useBundledDir points the settings at a fresh bundled package directory with
// an unreachable marketplace and returns the directory and an official key.
func useBundledDir(t *testing.T) (string, *minisign.PrivateKey) {
	t.Helper()
	signer := useOfficialMarketplace(t, "http://127.0.0.1:1/index.json")
	dir := t.TempDir()
	settings.PluginSettings.BundledDir = dir
	return dir, signer
}

// bundlePackage writes a signed package into the bundled directory.
func bundlePackage(t *testing.T, dir, id, pluginVersion string, signer *minisign.PrivateKey) string {
	t.Helper()
	archive := buildSignedTestPackage(t, marketplaceManifest(id, pluginVersion),
		map[string]string{"webapp/main.js": "export default {}"}, signer)
	body, err := os.ReadFile(archive)
	require.NoError(t, err)
	target := filepath.Join(dir, id+"-"+pluginVersion+packageSuffix)
	require.NoError(t, os.WriteFile(target, body, 0o644))
	return target
}

func TestInstallBundledPackagesInstallsAndEnablesOnce(t *testing.T) {
	manager := newCertAwareManager(t)
	dir, signer := useBundledDir(t)
	archive := bundlePackage(t, dir, OfficialDNS01PluginID, "1.0.0", signer)

	require.NoError(t, manager.InstallBundledPackages(context.Background()))

	info, err := manager.Get(OfficialDNS01PluginID)
	require.NoError(t, err)
	assert.Equal(t, "1.0.0", info.Version)
	assert.True(t, info.Enabled)
	assert.Equal(t, TrustOfficial, info.Trust)
	// The bundled directory is read-only, the archive stays where it is.
	assert.FileExists(t, archive)
	assert.Equal(t, []string{OfficialDNS01PluginID}, manager.readBundledState().Installed)
}

func TestInstallBundledPackagesKeepsAnUninstall(t *testing.T) {
	manager := newCertAwareManager(t)
	dir, signer := useBundledDir(t)
	bundlePackage(t, dir, OfficialDNS01PluginID, "1.0.0", signer)
	ctx := context.Background()

	require.NoError(t, manager.InstallBundledPackages(ctx))
	require.NoError(t, manager.Uninstall(ctx, OfficialDNS01PluginID, false))

	// The next boot does not bring it back, not even with a newer bundle.
	bundlePackage(t, dir, OfficialDNS01PluginID, "1.1.0", signer)
	require.NoError(t, manager.InstallBundledPackages(ctx))

	_, err := manager.Get(OfficialDNS01PluginID)
	assert.ErrorIs(t, err, ErrPluginNotFound)
}

func TestInstallBundledPackagesUpgradesAndKeepsItDisabled(t *testing.T) {
	manager := newCertAwareManager(t)
	dir, signer := useBundledDir(t)
	bundlePackage(t, dir, OfficialDNS01PluginID, "1.0.0", signer)
	ctx := context.Background()

	require.NoError(t, manager.InstallBundledPackages(ctx))
	_, err := manager.Disable(ctx, OfficialDNS01PluginID)
	require.NoError(t, err)

	bundlePackage(t, dir, OfficialDNS01PluginID, "1.1.0", signer)
	require.NoError(t, manager.InstallBundledPackages(ctx))

	info, err := manager.Get(OfficialDNS01PluginID)
	require.NoError(t, err)
	assert.Equal(t, "1.1.0", info.Version)
	assert.False(t, info.Enabled)
}

func TestInstallBundledPackagesRefusesANonOfficialSignature(t *testing.T) {
	manager := newCertAwareManager(t)
	dir, _ := useBundledDir(t)
	public, impostor := newSigningKey(t)
	trustKey(t, public)
	bundlePackage(t, dir, OfficialDNS01PluginID, "1.0.0", &impostor)

	require.NoError(t, manager.InstallBundledPackages(context.Background()))

	_, err := manager.Get(OfficialDNS01PluginID)
	assert.ErrorIs(t, err, ErrPluginNotFound)
	// Nothing was installed, so a later boot may try again.
	assert.Empty(t, manager.readBundledState().Installed)
}

func TestInstallBundledPackagesWithoutADirectory(t *testing.T) {
	manager := newCertAwareManager(t)
	useBundledDir(t)
	settings.PluginSettings.BundledDir = ""

	require.NoError(t, manager.InstallBundledPackages(context.Background()))
	assert.NoFileExists(t, filepath.Join(manager.Dir(), bundledStateFile))
}

func TestEnsureDNS01PluginReinstallsTheBundledPackage(t *testing.T) {
	manager := newCertAwareManager(t)
	dir, signer := useBundledDir(t)
	bundlePackage(t, dir, OfficialDNS01PluginID, "1.0.0", signer)
	ctx := context.Background()

	require.NoError(t, manager.InstallBundledPackages(ctx))
	require.NoError(t, manager.Uninstall(ctx, OfficialDNS01PluginID, false))

	// A certificate still renews through DNS-01, so the plugin comes back from
	// the bundle even though the marketplace is unreachable.
	addDNS01Cert(t)
	manager.EnsureDNS01Plugin(ctx)

	info, err := manager.Get(OfficialDNS01PluginID)
	require.NoError(t, err)
	assert.True(t, info.Enabled)
}

func TestEnsureDNS01PluginAndRenewRenewsOnceThePluginArrives(t *testing.T) {
	manager := newCertAwareManager(t)
	dir, signer := useBundledDir(t)
	bundlePackage(t, dir, OfficialDNS01PluginID, "1.0.0", signer)
	addDNS01Cert(t)

	renewals := 0
	previous := renewAfterDNS01Install
	renewAfterDNS01Install = func() { renewals++ }
	t.Cleanup(func() { renewAfterDNS01Install = previous })

	ctx := context.Background()
	manager.ensureDNS01PluginAndRenew(ctx)
	assert.Equal(t, 1, renewals, "the certificates that waited for the plugin are renewed")

	// Already there: nothing new to renew for.
	manager.ensureDNS01PluginAndRenew(ctx)
	assert.Equal(t, 1, renewals)
}
