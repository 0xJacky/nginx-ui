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

// useSignedMarketplace points the settings at a test catalog whose packages
// must be signed, which is what lets a custom source vouch for its entries.
func useSignedMarketplace(t *testing.T, source string) *minisign.PrivateKey {
	t.Helper()
	useMarketplace(t, source)
	public, private := newSigningKey(t)
	trustKey(t, public)
	settings.PluginSettings.RequireSignature = true
	return &private
}

func TestEnsureDNS01PluginInstallsFromTheMarketplace(t *testing.T) {
	manager := newCertAwareManager(t)
	server := newCatalogServer(t)
	signer := useSignedMarketplace(t, server.catalogURL())

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

	// The deprecated core resolver list moved into the plugin settings.
	_, values, err := manager.Settings(OfficialDNS01PluginID)
	require.NoError(t, err)
	assert.Equal(t, "8.8.8.8:53,1.1.1.1:53", values[recursiveNameserversKey])
}

func TestEnsureDNS01PluginPrefersALocalPackage(t *testing.T) {
	manager := newCertAwareManager(t)
	useMarketplace(t, "http://127.0.0.1:1/index.json")

	addDNS01Cert(t)
	archive := dropPackage(t, manager, OfficialDNS01PluginID, "1.0.0", nil)

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
	signer := useSignedMarketplace(t, server.catalogURL())

	addDNS01Cert(t)
	server.publish(t, dns01Manifest("1.0.0"), signer, func(entry *CatalogEntry, _ *CatalogRelease) {
		entry.Trust = TrustCommunity
	})

	manager.EnsureDNS01Plugin(context.Background())

	_, err := manager.Get(OfficialDNS01PluginID)
	assert.ErrorIs(t, err, ErrPluginNotFound)
}

func TestEnsureDNS01PluginIgnoresAnOfficialClaimFromAnUnsignedSource(t *testing.T) {
	manager := newCertAwareManager(t)
	server := newCatalogServer(t)
	// Without the signature policy the source cannot vouch for anything,
	// whatever trust level its entries claim.
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
	signer := useSignedMarketplace(t, server.catalogURL())

	// A core upgrade left a plugin built for another protocol version behind.
	stale := marketplaceManifest("com.example.alpha", "1.0.0")
	stale.APIVersion = protocol.APIVersion + 1
	writePluginDir(t, manager.Dir()+"/"+stale.ID, stale)
	manager.offline = true
	require.NoError(t, manager.discover(context.Background()))

	current, err := manager.Get("com.example.alpha")
	require.NoError(t, err)
	require.Equal(t, StatusIncompatible, current.Status)

	server.publish(t, marketplaceManifest("com.example.alpha", "1.1.0"), signer, nil)
	manager.repairIncompatiblePlugins(context.Background())

	repaired, err := manager.Get("com.example.alpha")
	require.NoError(t, err)
	assert.Equal(t, "1.1.0", repaired.Version)
	assert.NotEqual(t, StatusIncompatible, repaired.Status)
}
