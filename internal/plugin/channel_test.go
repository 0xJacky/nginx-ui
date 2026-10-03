package plugin

import (
	"context"
	"testing"

	"github.com/0xJacky/Nginx-UI/internal/plugin/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInferChannel(t *testing.T) {
	assert.Equal(t, ChannelStable, InferChannel("1.0.0"))
	assert.Equal(t, ChannelStable, InferChannel("v1.0.0+build.5"))
	assert.Equal(t, ChannelStable, InferChannel("not a version"))
	assert.Equal(t, ChannelStable, InferChannel(""))

	for _, version := range []string{"1.0.0-beta.1", "v1.9.0-rc.1+build.5", "1.0.0-pre", "0.3.0-x.1"} {
		assert.Equal(t, ChannelBeta, InferChannel(version), version)
	}
	for _, version := range []string{"1.0.0-alpha", "1.0.0-DEV.3", "1.0.0-nightly.20260930", "1.0.0-snapshot", "1.0.0-canary.2", "1.0.0-preview.1"} {
		assert.Equal(t, ChannelDev, InferChannel(version), version)
	}
	// Only the first identifier decides.
	assert.Equal(t, ChannelBeta, InferChannel("1.0.0-beta.alpha"))
	assert.Equal(t, ChannelBeta, InferChannel("1.0.0-alphabet"))
}

func TestChannelOrder(t *testing.T) {
	assert.Less(t, ChannelRank(ChannelStable), ChannelRank(ChannelBeta))
	assert.Less(t, ChannelRank(ChannelBeta), ChannelRank(ChannelDev))
	assert.Equal(t, ChannelStable, NormalizeChannel(""))
	assert.Equal(t, ChannelStable, NormalizeChannel("nightly"))
	assert.Equal(t, ChannelBeta, lessStableChannel(ChannelStable, ChannelBeta))
	assert.Equal(t, ChannelDev, lessStableChannel(ChannelDev, ChannelBeta))
	assert.Equal(t, ChannelBeta, lessStableChannel("", ChannelBeta))
}

func channelTestEntry() *CatalogEntry {
	release := func(version, channel string) CatalogRelease {
		return CatalogRelease{
			Version: version, APIVersion: protocol.APIVersion, Channel: channel,
			Platforms: []string{anyPlatform}, DownloadURL: "https://example.com/" + version,
		}
	}
	return &CatalogEntry{ID: "com.example.alpha", Releases: []CatalogRelease{
		release("1.0.0", ""),
		release("1.1.0", ""),
		release("1.2.0", ChannelBeta),
		release("1.9.0-rc.1", ""),
		release("2.0.0-nightly.3", ""),
	}}
}

func TestPickReleaseByFollowedChannel(t *testing.T) {
	host := HostPlatform()
	entry := channelTestEntry()

	pick := func(followed string) string {
		release := pickRelease(entry, "", host, followed)
		if release == nil {
			return ""
		}
		return release.Version
	}

	// Not installed: the newest stable one.
	assert.Equal(t, "1.1.0", pick(""))
	assert.Equal(t, "1.1.0", pick(ChannelStable))
	assert.Equal(t, "1.9.0-rc.1", pick(ChannelBeta))
	assert.Equal(t, "2.0.0-nightly.3", pick(ChannelDev))

	// A range that only a beta release satisfies installs it when nothing is
	// installed, and offers nothing to a stable installation.
	assert.Equal(t, "1.2.0", pickRelease(entry, ">=1.2.0 <1.9.0-rc.1", host, "").Version)
	assert.Nil(t, pickRelease(entry, ">=1.2.0 <1.9.0-rc.1", host, ChannelStable))
}

func TestPickReleaseFallsBackToTheMostStableAvailable(t *testing.T) {
	host := HostPlatform()
	betaOnly := &CatalogEntry{Releases: []CatalogRelease{
		{Version: "0.1.0-beta.1", APIVersion: protocol.APIVersion, Platforms: []string{anyPlatform}, DownloadURL: "https://example.com/1"},
		{Version: "0.1.0-beta.2", APIVersion: protocol.APIVersion, Platforms: []string{anyPlatform}, DownloadURL: "https://example.com/2"},
		{Version: "0.2.0-dev.1", APIVersion: protocol.APIVersion, Platforms: []string{anyPlatform}, DownloadURL: "https://example.com/3"},
	}}
	// The beta wins over the newer dev release for a first install.
	assert.Equal(t, "0.1.0-beta.2", pickRelease(betaOnly, "", host, "").Version)
	assert.Equal(t, "0.1.0-beta.2", pickRelease(betaOnly, "", host, ChannelBeta).Version)
	assert.Nil(t, pickRelease(betaOnly, "", host, ChannelStable))

	devOnly := &CatalogEntry{Releases: betaOnly.Releases[2:]}
	assert.Equal(t, "0.2.0-dev.1", pickRelease(devOnly, "", host, "").Version)
}

func TestPickReleaseSkipsYankedAndForeignPlatformReleases(t *testing.T) {
	host := HostPlatform()
	entry := channelTestEntry()
	entry.Releases[1].Yanked = true
	entry.Releases[4].Platforms = []string{"plan9-386"}
	assert.Equal(t, "1.0.0", pickRelease(entry, "", host, ChannelStable).Version)
	assert.Equal(t, "1.9.0-rc.1", pickRelease(entry, "", host, ChannelDev).Version)
	assert.Equal(t, []string{"1.9.0-rc.1", "1.2.0", "1.0.0"}, installableVersions(entry, host))
}

func TestMarketplaceChannels(t *testing.T) {
	manager := newTestManager(t)
	server := newCatalogServer(t)
	useMarketplace(t, server.catalogURL())

	server.publish(t, marketplaceManifest("com.example.alpha", "1.0.0"), nil, nil)
	server.publish(t, marketplaceManifest("com.example.alpha", "1.1.0-beta.1"), nil, nil)
	server.publish(t, marketplaceManifest("com.example.alpha", "1.1.0-nightly.1"), nil, nil)
	// A plain version the catalog puts on the beta channel.
	server.publish(t, marketplaceManifest("com.example.gamma", "0.9.0"), nil, func(_ *CatalogEntry, release *CatalogRelease) {
		release.Channel = ChannelBeta
	})
	server.publish(t, marketplaceManifest("com.example.delta", "1.0.0"), nil, nil)

	ctx := context.Background()
	marketplace := manager.Marketplace()
	entries, err := marketplace.Catalog(ctx, true)
	require.NoError(t, err)

	alpha := findEntry(entries, "com.example.alpha", "")
	require.NotNil(t, alpha)
	assert.Equal(t, "1.0.0", alpha.InstallableRelease.Version)
	assert.Equal(t, ChannelStable, alpha.Channel)
	assert.Equal(t, ChannelStable, alpha.Releases[0].Channel)
	assert.Equal(t, ChannelBeta, alpha.Releases[1].Channel)
	assert.Equal(t, ChannelDev, alpha.Releases[2].Channel)
	assert.Equal(t, []string{"1.1.0-nightly.1", "1.1.0-beta.1", "1.0.0"}, alpha.InstallableVersions)

	assert.Equal(t, ChannelBeta, findEntry(entries, "com.example.gamma", "").Channel)
	assert.Equal(t, ChannelStable, findEntry(entries, "com.example.delta", "").Channel)

	// A new installation follows stable and is not offered a prerelease.
	info, err := marketplace.Install(ctx, "com.example.alpha", "", "", InstallOptions{Enable: true, ApprovePermissions: true})
	require.NoError(t, err)
	assert.Equal(t, ChannelStable, info.Channel)
	assert.Equal(t, ChannelStable, info.FollowedChannel)
	updates, err := marketplace.Updates(ctx)
	require.NoError(t, err)
	assert.Empty(t, updates)

	// Following beta offers the beta release but not the dev one.
	info, err = manager.SetChannel(ctx, "com.example.alpha", ChannelBeta)
	require.NoError(t, err)
	assert.Equal(t, ChannelBeta, info.FollowedChannel)
	updates, err = marketplace.Updates(ctx)
	require.NoError(t, err)
	require.Len(t, updates, 1)
	assert.Equal(t, "1.1.0-beta.1", updates[0].LatestVersion)

	_, err = manager.SetChannel(ctx, "com.example.alpha", "nightly")
	assert.ErrorIs(t, err, ErrChannelInvalid)
	_, err = manager.SetChannel(ctx, "com.example.missing", ChannelStable)
	assert.ErrorIs(t, err, ErrPluginNotFound)

	// Installing a dev release by name raises the channel updates come from
	// while it runs, the chosen channel stays.
	info, err = marketplace.Update(ctx, "com.example.alpha", "1.1.0-nightly.1", false)
	require.NoError(t, err)
	assert.Equal(t, ChannelDev, info.Channel)
	assert.Equal(t, ChannelBeta, info.FollowedChannel)
	assert.Equal(t, ChannelDev, info.EffectiveChannel)

	// Back on a stable release the chosen channel applies again.
	info, err = marketplace.Update(ctx, "com.example.alpha", "1.0.0", false)
	require.NoError(t, err)
	assert.Equal(t, "1.0.0", info.Version)
	assert.Equal(t, ChannelStable, info.Channel)
	assert.Equal(t, ChannelBeta, info.FollowedChannel)
	assert.Equal(t, ChannelBeta, info.EffectiveChannel)
}

func updateVersions(t *testing.T, marketplace *Marketplace) []string {
	t.Helper()
	marketplace.ClearCache()
	updates, err := marketplace.Updates(context.Background())
	require.NoError(t, err)
	versions := make([]string, 0, len(updates))
	for _, update := range updates {
		versions = append(versions, update.LatestVersion)
	}
	return versions
}

func TestPluginWithOnlyBetaReleasesFallsBackToStable(t *testing.T) {
	manager := newTestManager(t)
	server := newCatalogServer(t)
	useMarketplace(t, server.catalogURL())
	ctx := context.Background()
	marketplace := manager.Marketplace()

	server.publish(t, marketplaceManifest("com.example.alpha", "0.9.0-beta.1"), nil, nil)
	server.publish(t, marketplaceManifest("com.example.alpha", "0.9.0-beta.2"), nil, nil)

	// Nothing stable exists, so the newest beta installs.
	info, err := marketplace.Install(ctx, "com.example.alpha", "", "", InstallOptions{Enable: true, ApprovePermissions: true})
	require.NoError(t, err)
	assert.Equal(t, "0.9.0-beta.2", info.Version)
	assert.Equal(t, ChannelBeta, info.Channel)
	assert.Equal(t, ChannelStable, info.FollowedChannel)
	assert.Equal(t, ChannelBeta, info.EffectiveChannel)
	assert.Empty(t, updateVersions(t, marketplace))

	// Newer betas keep coming while the installed release is beta.
	server.publish(t, marketplaceManifest("com.example.alpha", "0.9.0-beta.3"), nil, nil)
	assert.Equal(t, []string{"0.9.0-beta.3"}, updateVersions(t, marketplace))
	_, err = marketplace.Update(ctx, "com.example.alpha", "", false)
	require.NoError(t, err)

	// The stable release is offered as soon as it is out.
	server.publish(t, marketplaceManifest("com.example.alpha", "1.0.0"), nil, nil)
	assert.Equal(t, []string{"1.0.0"}, updateVersions(t, marketplace))
	info, err = marketplace.Update(ctx, "com.example.alpha", "", false)
	require.NoError(t, err)
	assert.Equal(t, "1.0.0", info.Version)

	// Running a stable release, the effective channel is the chosen one again.
	assert.Equal(t, ChannelStable, info.Channel)
	assert.Equal(t, ChannelStable, info.FollowedChannel)
	assert.Equal(t, ChannelStable, info.EffectiveChannel)
	server.publish(t, marketplaceManifest("com.example.alpha", "1.1.0-beta.1"), nil, nil)
	assert.Empty(t, updateVersions(t, marketplace))

	// Someone who chose beta keeps getting betas after the stable release.
	_, err = manager.SetChannel(ctx, "com.example.alpha", ChannelBeta)
	require.NoError(t, err)
	assert.Equal(t, []string{"1.1.0-beta.1"}, updateVersions(t, marketplace))
}

func TestStableFollowerWhoTriedABetaGoesBackToStable(t *testing.T) {
	manager := newTestManager(t)
	server := newCatalogServer(t)
	useMarketplace(t, server.catalogURL())
	ctx := context.Background()
	marketplace := manager.Marketplace()

	server.publish(t, marketplaceManifest("com.example.alpha", "1.0.0"), nil, nil)
	server.publish(t, marketplaceManifest("com.example.alpha", "1.1.0-beta.1"), nil, nil)
	_, err := marketplace.Install(ctx, "com.example.alpha", "", "", InstallOptions{Enable: true, ApprovePermissions: true})
	require.NoError(t, err)

	info, err := marketplace.Update(ctx, "com.example.alpha", "1.1.0-beta.1", false)
	require.NoError(t, err)
	assert.Equal(t, ChannelStable, info.FollowedChannel)
	assert.Equal(t, ChannelBeta, info.EffectiveChannel)

	// Until the stable release is out, newer betas are offered.
	server.publish(t, marketplaceManifest("com.example.alpha", "1.1.0-beta.2"), nil, nil)
	assert.Equal(t, []string{"1.1.0-beta.2"}, updateVersions(t, marketplace))
	server.publish(t, marketplaceManifest("com.example.alpha", "1.1.0"), nil, nil)
	assert.Equal(t, []string{"1.1.0"}, updateVersions(t, marketplace))

	info, err = marketplace.Update(ctx, "com.example.alpha", "", false)
	require.NoError(t, err)
	assert.Equal(t, "1.1.0", info.Version)
	assert.Equal(t, ChannelStable, info.EffectiveChannel)
	server.publish(t, marketplaceManifest("com.example.alpha", "1.2.0-beta.1"), nil, nil)
	assert.Empty(t, updateVersions(t, marketplace))
}

func TestMarketplaceDowngradeIsOnlyOnRequest(t *testing.T) {
	manager := newTestManager(t)
	server := newCatalogServer(t)
	useMarketplace(t, server.catalogURL())

	server.publish(t, marketplaceManifest("com.example.alpha", "1.0.0"), nil, nil)
	server.publish(t, marketplaceManifest("com.example.alpha", "1.1.0"), nil, nil)
	server.publish(t, marketplaceManifest("com.example.alpha", "1.2.0"), nil, func(_ *CatalogEntry, release *CatalogRelease) {
		release.Yanked = true
	})

	ctx := context.Background()
	marketplace := manager.Marketplace()
	info, err := marketplace.Install(ctx, "com.example.alpha", "", "", InstallOptions{Enable: true, ApprovePermissions: true})
	require.NoError(t, err)
	assert.Equal(t, "1.1.0", info.Version)

	// The withdrawn release cannot be asked for, an older one can.
	_, err = marketplace.Update(ctx, "com.example.alpha", "1.2.0", false)
	assert.ErrorIs(t, err, ErrReleaseYanked)
	info, err = marketplace.Update(ctx, "com.example.alpha", "1.0.0", false)
	require.NoError(t, err)
	assert.Equal(t, "1.0.0", info.Version)
	assert.Equal(t, "1.0.0", manager.Marketplace().installedVersion("com.example.alpha"))

	// The newer release is offered again, which the automatic flows would
	// install, but they never step back to an older one.
	updates, err := marketplace.Updates(ctx)
	require.NoError(t, err)
	require.Len(t, updates, 1)
	assert.Equal(t, "1.1.0", updates[0].LatestVersion)

	_, err = marketplace.Update(ctx, "com.example.alpha", "1.1.0", false)
	require.NoError(t, err)
	server.publish(t, marketplaceManifest("com.example.alpha", "1.0.5"), nil, nil)
	marketplace.ClearCache()
	updates, err = marketplace.Updates(ctx)
	require.NoError(t, err)
	assert.Empty(t, updates)
}

func releasesOf(versions ...string) *CatalogEntry {
	entry := &CatalogEntry{}
	for _, version := range versions {
		entry.Releases = append(entry.Releases, CatalogRelease{
			Version: version, APIVersion: protocol.APIVersion,
			Platforms: []string{anyPlatform}, DownloadURL: "https://example.com/" + version,
		})
	}
	return entry
}

// offered is the version an installation on installed would be updated to,
// empty when nothing newer qualifies.
func offered(entry *CatalogEntry, followed, installed string) string {
	release := pickRelease(entry, "", HostPlatform(), followed)
	if release == nil || CompareVersions(installed, release.Version) >= 0 {
		return ""
	}
	return release.Version
}

func TestBetaFollowersAlsoGetNewerStableReleases(t *testing.T) {
	// One release line: the stable release that ends a beta series is offered
	// to the people following beta, then the next beta.
	entry := releasesOf("1.0.0", "1.1.0-beta.1", "1.1.0-beta.3")
	assert.Equal(t, "1.1.0-beta.3", offered(entry, ChannelBeta, "1.0.0"))
	assert.Equal(t, "", offered(entry, ChannelBeta, "1.1.0-beta.3"))

	entry = releasesOf("1.0.0", "1.1.0-beta.1", "1.1.0-beta.3", "1.1.0")
	assert.Equal(t, "1.1.0", offered(entry, ChannelBeta, "1.1.0-beta.3"))
	assert.Equal(t, "1.1.0", offered(entry, ChannelDev, "1.1.0-beta.3"))
	assert.Equal(t, "1.1.0", offered(entry, ChannelStable, "1.0.0"))

	entry = releasesOf("1.0.0", "1.1.0-beta.3", "1.1.0", "1.2.0-beta.1")
	assert.Equal(t, "1.2.0-beta.1", offered(entry, ChannelBeta, "1.1.0"))
	// A stable follower stays on the stable release.
	assert.Equal(t, "", offered(entry, ChannelStable, "1.1.0"))
}

func TestBetaFollowersAreNotOfferedOlderStableReleases(t *testing.T) {
	// 1.0.1 is older than 1.1.0-beta.2 by precedence, so it is no update.
	entry := releasesOf("1.0.0", "1.0.1", "1.1.0-beta.2")
	assert.Equal(t, "", offered(entry, ChannelBeta, "1.1.0-beta.2"))
	assert.Equal(t, "1.1.0-beta.2", pickRelease(entry, "", HostPlatform(), ChannelBeta).Version)
	// A maintenance release of the old line reaches the stable followers.
	assert.Equal(t, "1.0.1", offered(entry, ChannelStable, "1.0.0"))
}

func TestPrereleaseNumbersOrderNumerically(t *testing.T) {
	assert.Equal(t, 1, CompareVersions("1.1.0-beta.10", "1.1.0-beta.9"))
	assert.Equal(t, 1, CompareVersions("1.1.0", "1.1.0-beta.10"))
	// Without the dot the identifiers compare as text.
	assert.Equal(t, -1, CompareVersions("1.1.0-beta10", "1.1.0-beta9"))

	entry := releasesOf("1.1.0-beta.9", "1.1.0-beta.10", "1.1.0-beta.2")
	assert.Equal(t, "1.1.0-beta.10", pickRelease(entry, "", HostPlatform(), ChannelBeta).Version)
	assert.Equal(t, "1.1.0-beta.10", offered(entry, ChannelBeta, "1.1.0-beta.9"))
}
