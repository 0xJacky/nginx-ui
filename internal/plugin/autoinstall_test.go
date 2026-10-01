package plugin

import (
	"context"
	"testing"

	"aead.dev/minisign"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/0xJacky/Nginx-UI/internal/plugin/protocol"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/0xJacky/Nginx-UI/settings"
)

// newCertAwareManager wires a manager to a database that also holds the
// certificate table the auto install consults.
func newCertAwareManager(t *testing.T) *Manager {
	t.Helper()

	db := setupPluginTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.Cert{}))
	useDeveloperMode(t, true)

	m := newManager(t.TempDir())
	t.Cleanup(func() { m.Stop(context.Background()) })
	return m
}

// addDNS01Cert stores a certificate that needs the DNS-01 challenge.
func addDNS01Cert(t *testing.T) {
	t.Helper()
	require.NoError(t, query.Cert.WithContext(context.Background()).Create(&model.Cert{
		Name:            "example.com",
		ChallengeMethod: model.CertChallengeMethodDNS01,
	}))
}

// dns01Manifest is the official plugin, with the setting the deprecated core
// resolver list is carried over into.
func dns01Manifest(pluginVersion string) *protocol.Manifest {
	manifest := marketplaceManifest(OfficialDNS01PluginID, pluginVersion)
	manifest.SettingsSchema = &protocol.SettingsSchema{
		Settings: []protocol.SettingsField{
			{Key: recursiveNameserversKey, Type: "text", DisplayName: "Recursive nameservers"},
		},
	}
	return manifest
}

// useOfficialMarketplace points the settings at a test catalog with developer
// mode off and returns a key whose packages are official.
func useOfficialMarketplace(t *testing.T, source string) *minisign.PrivateKey {
	t.Helper()
	useMarketplace(t, source)
	settings.PluginSettings.DeveloperMode = false
	return useReleaseKey(t)
}

func TestEnsureDNS01PluginInstallsFromTheMarketplace(t *testing.T) {
	manager := newCertAwareManager(t)
	server := newCatalogServer(t)
	signer := useOfficialMarketplace(t, server.catalogURL())

	previous := settings.CertSettings.RecursiveNameservers
	settings.CertSettings.RecursiveNameservers = []string{"8.8.8.8:53", "1.1.1.1:53"}
	t.Cleanup(func() { settings.CertSettings.RecursiveNameservers = previous })

	addDNS01Cert(t)
	server.publish(t, dns01Manifest("1.0.0"), signer, nil)

	manager.EnsureDNS01Plugin(context.Background())

	info, err := manager.Get(OfficialDNS01PluginID)
	require.NoError(t, err)
	assert.Equal(t, "1.0.0", info.Version)
	assert.True(t, info.Enabled)
	assert.Equal(t, TrustOfficial, info.Trust)

	// The deprecated core resolver list moved into the plugin settings.
	_, values, err := manager.Settings(OfficialDNS01PluginID)
	require.NoError(t, err)
	assert.Equal(t, "8.8.8.8:53,1.1.1.1:53", values[recursiveNameserversKey])
}

func TestEnsureDNS01PluginPrefersALocalPackage(t *testing.T) {
	manager := newCertAwareManager(t)
	signer := useOfficialMarketplace(t, "http://127.0.0.1:1/index.json")

	addDNS01Cert(t)
	archive := dropPackage(t, manager, OfficialDNS01PluginID, "1.0.0", signer)

	manager.EnsureDNS01Plugin(context.Background())

	info, err := manager.Get(OfficialDNS01PluginID)
	require.NoError(t, err)
	assert.True(t, info.Enabled)
	assert.NoFileExists(t, archive)
}

func TestEnsureDNS01PluginSkipsWhenNoCertificateNeedsIt(t *testing.T) {
	manager := newCertAwareManager(t)
	server := newCatalogServer(t)
	useMarketplace(t, server.catalogURL())
	server.publish(t, dns01Manifest("1.0.0"), nil, nil)

	manager.EnsureDNS01Plugin(context.Background())

	_, err := manager.Get(OfficialDNS01PluginID)
	assert.ErrorIs(t, err, ErrPluginNotFound)
}

func TestEnsureDNS01PluginRefusesACommunityImpostor(t *testing.T) {
	manager := newCertAwareManager(t)
	server := newCatalogServer(t)
	signer := useOfficialMarketplace(t, server.catalogURL())

	addDNS01Cert(t)
	server.publish(t, dns01Manifest("1.0.0"), signer, func(entry *CatalogEntry, _ *CatalogRelease) {
		entry.Trust = TrustCommunity
	})

	manager.EnsureDNS01Plugin(context.Background())

	_, err := manager.Get(OfficialDNS01PluginID)
	assert.ErrorIs(t, err, ErrPluginNotFound)
}

func TestEnsureDNS01PluginRefusesACommunitySignedImpostor(t *testing.T) {
	manager := newCertAwareManager(t)
	server := newCatalogServer(t)
	useOfficialMarketplace(t, server.catalogURL())
	public, impostor := newSigningKey(t)
	trustKey(t, public)

	// Both the local package and the catalog entry claim to be the official
	// plugin, but a community key signed them.
	addDNS01Cert(t)
	local := dropPackage(t, manager, OfficialDNS01PluginID, "1.0.0", &impostor)
	server.publish(t, dns01Manifest("1.0.0"), &impostor, nil)

	opts := InstallOptions{Enable: true, ApprovePermissions: true, MinTrust: TrustOfficial}
	_, err := manager.InstallLocalPackage(context.Background(), OfficialDNS01PluginID, opts)
	assertPluginError(t, err, ErrTrustDowngrade)
	_, err = manager.Marketplace().Install(context.Background(), OfficialDNS01PluginID, "", "", opts)
	assertPluginError(t, err, ErrTrustDowngrade)

	manager.EnsureDNS01Plugin(context.Background())

	_, err = manager.Get(OfficialDNS01PluginID)
	assert.ErrorIs(t, err, ErrPluginNotFound)
	assert.FileExists(t, local)
}

func TestEnsureDNS01PluginIgnoresAnOfficialClaimOfAnUnsignedPackage(t *testing.T) {
	manager := newCertAwareManager(t)
	server := newCatalogServer(t)
	// Even in developer mode the automatic install wants a release key.
	useMarketplace(t, server.catalogURL())

	addDNS01Cert(t)
	server.publish(t, dns01Manifest("1.0.0"), nil, nil)

	manager.EnsureDNS01Plugin(context.Background())

	_, err := manager.Get(OfficialDNS01PluginID)
	assert.ErrorIs(t, err, ErrPluginNotFound)
}

func TestInstallSeedsTheOfficialDNS01PluginWithTheCoreResolvers(t *testing.T) {
	manager := newCertAwareManager(t)
	ctx := context.Background()
	require.NoError(t, manager.LoadOffline(ctx))

	previous := settings.CertSettings.RecursiveNameservers
	settings.CertSettings.RecursiveNameservers = []string{"8.8.8.8:53"}
	t.Cleanup(func() { settings.CertSettings.RecursiveNameservers = previous })

	// Every way a package arrives goes through Install, an upload included.
	files := map[string]string{"webapp/main.js": "export default {}"}
	_, err := manager.Install(ctx, buildTestPackage(t, dns01Manifest("1.0.0"), files), InstallOptions{Enable: true})
	require.NoError(t, err)
	_, values, err := manager.Settings(OfficialDNS01PluginID)
	require.NoError(t, err)
	assert.Equal(t, "8.8.8.8:53", values[recursiveNameserversKey])

	// The value the user set afterwards survives an upgrade.
	require.NoError(t, manager.SaveSettings(ctx, OfficialDNS01PluginID, map[string]any{recursiveNameserversKey: "1.1.1.1:53"}))
	_, err = manager.Install(ctx, buildTestPackage(t, dns01Manifest("1.1.0"), files), InstallOptions{Enable: true})
	require.NoError(t, err)
	_, values, err = manager.Settings(OfficialDNS01PluginID)
	require.NoError(t, err)
	assert.Equal(t, "1.1.1.1:53", values[recursiveNameserversKey])
}

func TestRepairIncompatiblePluginsUpgradesOfficialOnes(t *testing.T) {
	manager := newCertAwareManager(t)
	server := newCatalogServer(t)
	signer := useOfficialMarketplace(t, server.catalogURL())
	public, community := newSigningKey(t)
	trustKey(t, public)

	// A core upgrade left plugins built for another protocol version behind.
	for _, id := range []string{"com.nginxui.alpha", "com.nginxui.beta"} {
		stale := marketplaceManifest(id, "1.0.0")
		stale.APIVersion = protocol.APIVersion + 1
		writePluginDir(t, manager.Dir()+"/"+stale.ID, stale)
	}
	manager.offline = true
	require.NoError(t, manager.discover(context.Background()))

	current, err := manager.Get("com.nginxui.alpha")
	require.NoError(t, err)
	require.Equal(t, StatusIncompatible, current.Status)

	// Both entries claim official, only one package is signed like it.
	server.publish(t, marketplaceManifest("com.nginxui.alpha", "1.1.0"), signer, nil)
	server.publish(t, marketplaceManifest("com.nginxui.beta", "1.1.0"), &community, nil)
	manager.repairIncompatiblePlugins(context.Background())

	repaired, err := manager.Get("com.nginxui.alpha")
	require.NoError(t, err)
	assert.Equal(t, "1.1.0", repaired.Version)
	assert.NotEqual(t, StatusIncompatible, repaired.Status)
	assert.Equal(t, TrustOfficial, repaired.Trust)

	skipped, err := manager.Get("com.nginxui.beta")
	require.NoError(t, err)
	assert.Equal(t, StatusIncompatible, skipped.Status)
}
