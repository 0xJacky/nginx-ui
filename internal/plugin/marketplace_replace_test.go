package plugin

import (
	"context"
	"testing"

	"github.com/0xJacky/Nginx-UI/internal/plugin/protocol"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMarketplaceReplaceSwapsAnUnsignedCopyAtTheSameVersion(t *testing.T) {
	manager := newTestManager(t)
	server := newCatalogServer(t)
	useMarketplace(t, server.catalogURL())
	ctx := context.Background()

	manifest := marketplaceManifest("com.example.alpha", "1.0.0")
	manifest.SettingsSchema = &protocol.SettingsSchema{Settings: []protocol.SettingsField{
		{Key: "endpoint", Type: settingsTypeText, DisplayName: "Endpoint"},
	}}

	// A local unsigned build of the same version is installed first.
	_, err := manager.Install(ctx, buildTestPackage(t, manifest, map[string]string{"webapp/main.js": "export default {}"}),
		InstallOptions{Enable: true})
	require.NoError(t, err)
	require.NoError(t, manager.SaveSettings(ctx, "com.example.alpha", map[string]any{"endpoint": "https://kept.example"}))

	signer := useReleaseKey(t)
	server.publish(t, manifest, signer, nil)

	info, err := manager.Marketplace().Replace(ctx, "com.example.alpha", "")
	require.NoError(t, err)
	assert.Equal(t, TrustOfficial, info.Trust)
	assert.Equal(t, "1.0.0", info.Version)
	assert.True(t, info.Enabled)

	row, err := query.Plugin.WithContext(ctx).Where(query.Plugin.PluginID.Eq("com.example.alpha")).First()
	require.NoError(t, err)
	assert.Equal(t, "https://kept.example", row.Settings["endpoint"])

	// Once official there is nothing more trusted to replace it with.
	_, err = manager.Marketplace().Replace(ctx, "com.example.alpha", "")
	assertPluginError(t, err, ErrReplaceUnavailable)
}

func TestMarketplaceReplaceRefusesAPackageBelowTheCatalogClaim(t *testing.T) {
	manager := newTestManager(t)
	server := newCatalogServer(t)
	useMarketplace(t, server.catalogURL())
	ctx := context.Background()

	manifest := marketplaceManifest("com.example.alpha", "1.0.0")
	_, err := manager.Install(ctx, buildTestPackage(t, manifest, map[string]string{"webapp/main.js": "export default {}"}),
		InstallOptions{})
	require.NoError(t, err)

	// The entry claims official but the package is unsigned.
	server.publish(t, manifest, nil, nil)

	_, err = manager.Marketplace().Replace(ctx, "com.example.alpha", "")
	assertPluginError(t, err, ErrTrustDowngrade)

	_, err = manager.Marketplace().Replace(ctx, "com.example.missing", "")
	assertPluginError(t, err, ErrPluginNotFound)
}

func TestReplaceable(t *testing.T) {
	assert.True(t, replaceable(TrustUnsigned, TrustOfficial))
	assert.True(t, replaceable(TrustCommunity, TrustVerified))
	assert.False(t, replaceable(TrustUnsigned, TrustCommunity))
	assert.False(t, replaceable(TrustOfficial, TrustOfficial))
	assert.False(t, replaceable(TrustOfficial, TrustVerified))
}
