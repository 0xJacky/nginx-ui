package plugin

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"aead.dev/minisign"
	"github.com/0xJacky/Nginx-UI/internal/event"
	"github.com/0xJacky/Nginx-UI/internal/plugin/protocol"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// foreignPlatform is a platform the test binary does not run on.
func foreignPlatform() string {
	if HostPlatform() == "linux-arm64" {
		return "linux-amd64"
	}
	return "linux-arm64"
}

// platformManifest describes a plugin with one native executable per listed
// platform. It is on_demand, so installing it never spawns anything.
func platformManifest(id, pluginVersion string, platforms ...string) *protocol.Manifest {
	executables := make(map[string]string, len(platforms))
	for _, platform := range platforms {
		executables[platform] = "bin/plugin-" + platform
	}
	return &protocol.Manifest{
		ID:         id,
		Name:       id,
		Version:    pluginVersion,
		APIVersion: protocol.APIVersion,
		Server: &protocol.ManifestServer{
			Executables: executables,
			Lifecycle:   protocol.LifecycleOnDemand,
		},
		Webapp: &protocol.ManifestWebapp{BundlePath: "webapp/main.js"},
	}
}

// buildPlatformPackage packs a plugin with every executable it declares.
func buildPlatformPackage(t *testing.T, manifest *protocol.Manifest) []byte {
	t.Helper()
	return buildSignedPlatformPackage(t, manifest, nil)
}

// buildSignedPlatformPackage is buildPlatformPackage, signed when signer is set.
func buildSignedPlatformPackage(t *testing.T, manifest *protocol.Manifest, signer *minisign.PrivateKey) []byte {
	t.Helper()
	files := map[string]string{"webapp/main.js": "export default {}"}
	for platform, rel := range manifest.Server.Executables {
		files[rel] = "#!/bin/sh\n# " + platform + "\nexit 0\n"
	}
	body, err := os.ReadFile(buildSignedTestPackage(t, manifest, files, signer))
	require.NoError(t, err)
	return body
}

// publishPlatforms serves one per-platform package per platform and lists
// them under downloads. The release manifest snapshot carries every platform,
// the portable download_url stays empty.
func (cs *catalogServer) publishPlatforms(t *testing.T, id, pluginVersion string, signer *minisign.PrivateKey,
	platforms ...string,
) map[string][]byte {
	t.Helper()

	bodies := make(map[string][]byte, len(platforms))
	release := CatalogRelease{
		Version:    pluginVersion,
		APIVersion: protocol.APIVersion,
		Platforms:  platforms,
		Downloads:  map[string]ReleaseDownload{},
		Manifest:   platformManifest(id, pluginVersion, platforms...),
	}
	for _, platform := range platforms {
		body := buildSignedPlatformPackage(t, platformManifest(id, pluginVersion, platform), signer)
		name := "/pkg/" + PackageFileName(id, pluginVersion, platform)
		cs.serve(name, body)
		digest := sha256.Sum256(body)
		download := ReleaseDownload{URL: cs.URL + name, SHA256: hex.EncodeToString(digest[:])}
		release.Downloads[platform] = download
		bodies[platform] = body
	}

	cs.mu.Lock()
	defer cs.mu.Unlock()
	for i := range cs.document.Plugins {
		if cs.document.Plugins[i].ID == id {
			cs.document.Plugins[i].Releases = append(cs.document.Plugins[i].Releases, release)
			return bodies
		}
	}
	cs.document.Plugins = append(cs.document.Plugins, CatalogEntry{
		ID:       id,
		Name:     map[string]string{"en": id},
		Trust:    TrustOfficial,
		Releases: []CatalogRelease{release},
	})
	return bodies
}

func TestParsePackageFileName(t *testing.T) {
	for _, testCase := range []struct {
		name string
		want PackageName
		ok   bool
	}{
		{"com.nginxui.dns01-1.0.0.tar.gz", PackageName{ID: "com.nginxui.dns01", Version: "1.0.0"}, true},
		{"com.nginxui.dns01-1.0.0-linux-amd64.tar.gz",
			PackageName{ID: "com.nginxui.dns01", Version: "1.0.0", Platform: "linux-amd64"}, true},
		{"com.nginxui.dns01-1.2.0-rc.1-windows-arm64.tar.gz",
			PackageName{ID: "com.nginxui.dns01", Version: "1.2.0-rc.1", Platform: "windows-arm64"}, true},
		{"com.nginxui.dns01-1.2.0-rc.1.tar.gz", PackageName{ID: "com.nginxui.dns01", Version: "1.2.0-rc.1"}, true},
		// Hyphens inside the id do not confuse the split.
		{"io.github.my-org.dns-2-0.3.1-darwin-arm64.tar.gz",
			PackageName{ID: "io.github.my-org.dns-2", Version: "0.3.1", Platform: "darwin-arm64"}, true},
		// An unknown platform is read as a prerelease of the version.
		{"com.example.alpha-1.0.0-beos-ppc.tar.gz", PackageName{ID: "com.example.alpha", Version: "1.0.0-beos-ppc"}, true},
		{"/some/dir/com.example.alpha-1.0.0.tar.gz", PackageName{ID: "com.example.alpha", Version: "1.0.0"}, true},
		{"package.tar.gz", PackageName{}, false},
		{"com.example.alpha.tar.gz", PackageName{}, false},
		{"com.example.alpha-1.0.0.zip", PackageName{}, false},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			got, ok := ParsePackageFileName(testCase.name)
			assert.Equal(t, testCase.ok, ok)
			assert.Equal(t, testCase.want, got)
		})
	}

	assert.Equal(t, "com.example.alpha-1.0.0.tar.gz", PackageFileName("com.example.alpha", "1.0.0", ""))
	assert.Equal(t, "com.example.alpha-1.0.0.tar.gz", PackageFileName("com.example.alpha", "1.0.0", anyPlatform))
	assert.Equal(t, "com.example.alpha-1.0.0-linux-amd64.tar.gz",
		PackageFileName("com.example.alpha", "1.0.0", "linux-amd64"))

	assert.True(t, IsValidPlatform("linux-amd64"))
	assert.True(t, IsValidPlatform(HostPlatform()))
	assert.False(t, IsValidPlatform(anyPlatform))
	assert.False(t, IsValidPlatform("linux"))
	assert.False(t, IsValidPlatform("beos-ppc"))
}

func TestReleaseDownloadForSelectsByPlatform(t *testing.T) {
	const (
		linux    = "linux-amd64"
		darwin   = "darwin-arm64"
		windows  = "windows-amd64"
		portable = "https://example.com/pkg.tar.gz"
	)
	perPlatform := CatalogRelease{
		Platforms: []string{linux, darwin, windows},
		Downloads: map[string]ReleaseDownload{
			linux:  {URL: "https://example.com/linux.tar.gz", SHA256: "aa"},
			darwin: {URL: "https://example.com/darwin.tar.gz", SHA256: "bb"},
			// An entry without a url is ignored.
			"freebsd-amd64": {},
		},
		DownloadURL: portable,
		SHA256:      "cc",
	}

	// The platform entry wins.
	download, key, ok := perPlatform.DownloadFor(linux)
	require.True(t, ok)
	assert.Equal(t, linux, key)
	assert.Equal(t, "https://example.com/linux.tar.gz", download.URL)
	assert.Equal(t, "aa", download.SHA256)

	download, _, ok = perPlatform.DownloadFor(darwin)
	require.True(t, ok)
	assert.Equal(t, "bb", download.SHA256)

	// A platform only the summary lists falls back to the portable package.
	download, key, ok = perPlatform.DownloadFor(windows)
	require.True(t, ok)
	assert.Empty(t, key)
	assert.Equal(t, portable, download.URL)
	assert.Equal(t, "cc", download.SHA256)

	// Neither a download nor the summary: not installable.
	_, _, ok = perPlatform.DownloadFor("freebsd-amd64")
	assert.False(t, ok)
	_, _, ok = perPlatform.DownloadFor("linux-arm64")
	assert.False(t, ok)

	// "any" in downloads serves every platform without an own entry.
	withAny := CatalogRelease{Downloads: map[string]ReleaseDownload{
		linux:       {URL: "https://example.com/linux.tar.gz"},
		anyPlatform: {URL: "https://example.com/any.tar.gz"},
	}}
	download, key, ok = withAny.DownloadFor("plan9-386")
	require.True(t, ok)
	assert.Equal(t, anyPlatform, key)
	assert.Equal(t, "https://example.com/any.tar.gz", download.URL)

	// The schema 1 layout keeps working: no downloads, an empty platform list
	// means the portable package runs everywhere.
	legacy := CatalogRelease{DownloadURL: portable}
	_, key, ok = legacy.DownloadFor("plan9-386")
	assert.True(t, ok)
	assert.Empty(t, key)
	legacyAny := CatalogRelease{DownloadURL: portable, Platforms: []string{anyPlatform}}
	_, _, ok = legacyAny.DownloadFor(darwin)
	assert.True(t, ok)

	// Without a portable url only the downloads count.
	onlyDownloads := CatalogRelease{Platforms: []string{linux, windows},
		Downloads: map[string]ReleaseDownload{linux: {URL: "https://example.com/linux.tar.gz"}}}
	_, _, ok = onlyDownloads.DownloadFor(windows)
	assert.False(t, ok)
	_, _, ok = (&CatalogRelease{}).DownloadFor(linux)
	assert.False(t, ok)

	assert.Equal(t, []string{darwin, linux, windows}, perPlatform.AvailablePlatforms())
	assert.Equal(t, []string{anyPlatform, linux}, withAny.AvailablePlatforms())
	assert.Equal(t, []string{anyPlatform}, legacy.AvailablePlatforms())
	assert.Equal(t, []string{linux}, onlyDownloads.AvailablePlatforms())
}

func TestPickReleaseHonoursThePlatform(t *testing.T) {
	host, foreign := HostPlatform(), foreignPlatform()
	entry := &CatalogEntry{Releases: []CatalogRelease{
		{Version: "1.0.0", APIVersion: protocol.APIVersion,
			Downloads: map[string]ReleaseDownload{host: {URL: "https://example.com/1-host"}}},
		// The newest release has no build for this node.
		{Version: "1.1.0", APIVersion: protocol.APIVersion,
			Downloads: map[string]ReleaseDownload{foreign: {URL: "https://example.com/2-foreign"}}},
	}}

	assert.Equal(t, "1.0.0", pickRelease(entry, "", host, "").Version)
	assert.Equal(t, "1.1.0", pickRelease(entry, "", foreign, "").Version)
	assert.Nil(t, pickRelease(entry, "", "plan9-386", ""))
}

func TestPackagePlatforms(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "bin"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "bin", "linux"), []byte("x"), 0o755))

	manifest := &protocol.Manifest{Server: &protocol.ManifestServer{Executables: map[string]string{
		"linux-amd64":  "bin/linux",
		"darwin-arm64": "bin/darwin",
	}}}
	// Only the platform whose file ships counts.
	assert.Equal(t, []string{"linux-amd64"}, packagePlatforms(manifest, dir))

	manifest.Server.Command = []string{"python3", "main.py"}
	assert.Equal(t, []string{"linux-amd64", anyPlatform}, packagePlatforms(manifest, dir))

	assert.Equal(t, []string{anyPlatform}, packagePlatforms(&protocol.Manifest{}, dir))
	assert.True(t, platformsCover([]string{anyPlatform}, "plan9-386"))
	assert.False(t, platformsCover([]string{"linux-amd64"}, "plan9-386"))
}

func TestMarketplaceInstallPicksTheHostDownload(t *testing.T) {
	manager := newTestManager(t)
	server := newCatalogServer(t)
	useMarketplace(t, server.catalogURL())
	settings.PluginSettings.DeveloperMode = false
	signer := useOfficialKey(t)

	host, foreign := HostPlatform(), foreignPlatform()
	server.publishPlatforms(t, "com.example.native", "1.0.0", signer, host, foreign)

	var (
		mu        sync.Mutex
		platforms []string
	)
	unsubscribe := event.Subscribe(func(published event.Event) {
		progress, ok := published.Data.(InstallProgress)
		if !ok || published.Type != EventTypeInstallProgress || progress.Status == InstallStatusDone {
			return
		}
		mu.Lock()
		platforms = append(platforms, progress.Platform)
		mu.Unlock()
	})
	t.Cleanup(unsubscribe)

	entries, err := manager.Marketplace().Catalog(context.Background(), true)
	require.NoError(t, err)
	entry := findEntry(entries, "com.example.native", "")
	require.NotNil(t, entry.InstallableRelease)
	assert.Equal(t, "1.0.0", entry.InstallableRelease.Version)

	info, err := manager.Marketplace().Install(context.Background(), "com.example.native", "", "", InstallOptions{})
	require.NoError(t, err)
	assert.Equal(t, "1.0.0", info.Version)
	assert.Equal(t, TrustOfficial, info.Trust)

	// The installed copy is the host package: its manifest names the host
	// only and its executable is there.
	manifest, ok := manager.Manifest("com.example.native")
	require.True(t, ok)
	assert.Equal(t, map[string]string{host: "bin/plugin-" + host}, manifest.Server.Executables)
	assert.Equal(t, []string{host}, manager.installedPlatforms("com.example.native"))

	mu.Lock()
	defer mu.Unlock()
	require.NotEmpty(t, platforms)
	for _, platform := range platforms {
		assert.Equal(t, host, platform)
	}
}

func TestMarketplaceInstallRefusesAReleaseWithoutAHostBuild(t *testing.T) {
	manager := newTestManager(t)
	server := newCatalogServer(t)
	useMarketplace(t, server.catalogURL())

	server.publishPlatforms(t, "com.example.native", "1.0.0", nil, foreignPlatform())

	entries, err := manager.Marketplace().Catalog(context.Background(), true)
	require.NoError(t, err)
	assert.Nil(t, findEntry(entries, "com.example.native", "").InstallableRelease)

	_, err = manager.Marketplace().Install(context.Background(), "com.example.native", "", "", InstallOptions{})
	assert.ErrorIs(t, err, ErrPlatformUnsupported)
	_, err = manager.Marketplace().Install(context.Background(), "com.example.native", "1.0.0", "", InstallOptions{})
	assert.ErrorIs(t, err, ErrPlatformUnsupported)
}

func TestMarketplaceInstallVerifiesTheSelectedDownload(t *testing.T) {
	manager := newTestManager(t)
	server := newCatalogServer(t)
	useMarketplace(t, server.catalogURL())

	host := HostPlatform()
	server.publishPlatforms(t, "com.example.native", "1.0.0", nil, host, foreignPlatform())
	server.mu.Lock()
	release := &server.document.Plugins[0].Releases[0]
	download := release.Downloads[host]
	download.SHA256 = "00000000000000000000000000000000000000000000000000000000deadbeef"
	release.Downloads[host] = download
	server.mu.Unlock()

	_, err := manager.Marketplace().Install(context.Background(), "com.example.native", "", "", InstallOptions{})
	assert.ErrorIs(t, err, ErrDigestMismatch)
}

func TestFetchPackageDownloadsTheRequestedPlatform(t *testing.T) {
	manager := newTestManager(t)
	server := newCatalogServer(t)
	useMarketplace(t, server.catalogURL())
	signer := useOfficialKey(t)

	host, foreign := HostPlatform(), foreignPlatform()
	bodies := server.publishPlatforms(t, "com.example.native", "1.0.0", signer, host, foreign)
	marketplace := manager.Marketplace()
	destination := t.TempDir()

	archive, err := marketplace.FetchPackage(context.Background(), "com.example.native", "", foreign, destination)
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(destination, PackageFileName("com.example.native", "1.0.0", foreign)), archive)
	// The package is carried as served, its signature is inside.
	assert.NoFileExists(t, archive+".minisig")
	body, err := os.ReadFile(archive)
	require.NoError(t, err)
	assert.Equal(t, bodies[foreign], body)

	// The default is the platform of this node.
	archive, err = marketplace.FetchPackage(context.Background(), "com.example.native", "", "", destination)
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(destination, PackageFileName("com.example.native", "1.0.0", host)), archive)

	_, err = marketplace.FetchPackage(context.Background(), "com.example.native", "", "plan9-386", destination)
	assert.ErrorIs(t, err, ErrPlatformUnsupported)
	_, err = marketplace.FetchPackage(context.Background(), "com.example.native", "", "not-a-platform", destination)
	assertPluginError(t, err, ErrPlatformPackageMissing)

	version, targets, err := marketplace.FetchTargets(context.Background(), "com.example.native", "")
	require.NoError(t, err)
	assert.Equal(t, "1.0.0", version)
	assert.ElementsMatch(t, []string{host, foreign}, targets)
}

func TestLocalPackagesSkipForeignPlatforms(t *testing.T) {
	manager := newTestManager(t)
	useMarketplace(t)

	host, foreign := HostPlatform(), foreignPlatform()
	require.NoError(t, os.MkdirAll(manager.PackagesDir(), 0o755))
	drop := func(manifest *protocol.Manifest, platform string) string {
		target := filepath.Join(manager.PackagesDir(), PackageFileName(manifest.ID, manifest.Version, platform))
		require.NoError(t, os.WriteFile(target, buildPlatformPackage(t, manifest), 0o644))
		return target
	}
	// The foreign build is newer, it must still not win on this node.
	hostPackage := drop(platformManifest("com.example.native", "1.0.0", host), host)
	foreignPackage := drop(platformManifest("com.example.native", "2.0.0", foreign), foreign)
	// A portable name does not help a package without a host build.
	portable := drop(platformManifest("com.example.other", "1.0.0", foreign), "")

	info, err := manager.InstallLocalPackage(context.Background(), "com.example.native", InstallOptions{})
	require.NoError(t, err)
	assert.Equal(t, "1.0.0", info.Version)
	assert.NoFileExists(t, hostPackage)

	require.NoError(t, manager.ScanLocalPackages(context.Background()))
	_, err = manager.Get("com.example.other")
	assert.ErrorIs(t, err, ErrPluginNotFound)

	// Both stay in place for cluster sync.
	assert.FileExists(t, foreignPackage)
	assert.FileExists(t, portable)
}

func TestInstallRefusesAPackageForAnotherPlatform(t *testing.T) {
	manager := newTestManager(t)
	useMarketplace(t)

	archive := filepath.Join(t.TempDir(), "package.tar.gz")
	require.NoError(t, os.WriteFile(archive,
		buildPlatformPackage(t, platformManifest("com.example.native", "1.0.0", foreignPlatform())), 0o644))

	inspected, err := manager.Inspect(archive)
	require.NoError(t, err)
	assert.Equal(t, []string{foreignPlatform()}, inspected.Platforms)
	assert.Equal(t, HostPlatform(), inspected.HostPlatform)
	assert.False(t, inspected.PlatformSupported)

	_, err = manager.Install(context.Background(), archive, InstallOptions{})
	assert.ErrorIs(t, err, ErrNoExecutableForPlatform)

	// A package without a server runs anywhere.
	portable := buildTestPackage(t, marketplaceManifest("com.example.web", "1.0.0"),
		map[string]string{"webapp/main.js": "export default {}"})
	inspected, err = manager.Inspect(portable)
	require.NoError(t, err)
	assert.Equal(t, []string{anyPlatform}, inspected.Platforms)
	assert.True(t, inspected.PlatformSupported)
}

func TestLintPerPlatformPackageMustDeclareItsPlatform(t *testing.T) {
	host, foreign := HostPlatform(), foreignPlatform()
	lintArchive := func(t *testing.T, manifest *protocol.Manifest, name string) *LintReport {
		t.Helper()
		dir := writeLintFixture(t, lintFixture{manifest: manifest})
		archive := filepath.Join(t.TempDir(), name)
		require.NoError(t, BuildPackage(dir, archive))
		report, err := Lint(archive)
		require.NoError(t, err)
		return report
	}

	base := goodManifest()
	base.Server.Executables = map[string]string{host: "bin/plugin-" + host}
	name := PackageFileName(base.ID, base.Version, host)
	report := lintArchive(t, base, name)
	assert.Empty(t, report.Findings, "%+v", report.Findings)

	// A per-platform package that carries every platform.
	both := goodManifest()
	both.Server.Executables = map[string]string{host: "bin/plugin-" + host, foreign: "bin/plugin-" + foreign}
	report = lintArchive(t, both, name)
	assertHasFinding(t, report, LevelError, RulePackagePlatform)

	// A per-platform name over the wrong platform.
	report = lintArchive(t, base, PackageFileName(base.ID, base.Version, foreign))
	assertHasFinding(t, report, LevelError, RulePackagePlatform)

	// The portable form is free to list several platforms.
	report = lintArchive(t, both, PackageFileName(both.ID, both.Version, ""))
	assert.Empty(t, report.Findings, "%+v", report.Findings)

	// A name for another id or version is a warning.
	report = lintArchive(t, base, PackageFileName(base.ID, "9.9.9", host))
	assertHasFinding(t, report, LevelWarning, RulePackageFileName)
	assert.False(t, report.HasErrors(), "%+v", report.Findings)
}

func TestEnsureDNS01PluginInstallsTheHostDownload(t *testing.T) {
	manager := newCertAwareManager(t)
	server := newCatalogServer(t)
	signer := useOfficialMarketplace(t, server.catalogURL())

	addDNS01Cert(t)
	host := HostPlatform()
	server.publishPlatforms(t, OfficialDNS01PluginID, "1.0.0", signer, foreignPlatform(), host)

	manager.EnsureDNS01Plugin(context.Background())

	info, err := manager.Get(OfficialDNS01PluginID)
	require.NoError(t, err)
	assert.Equal(t, "1.0.0", info.Version)
	assert.Equal(t, []string{host}, manager.installedPlatforms(OfficialDNS01PluginID))
}
