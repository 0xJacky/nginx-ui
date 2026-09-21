package plugin

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"sync/atomic"
	"testing"

	"aead.dev/minisign"
	"github.com/0xJacky/Nginx-UI/internal/plugin/protocol"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// catalogServer serves a static catalog, the packages it points at and their
// detached signatures, which is everything the installer talks to.
type catalogServer struct {
	*httptest.Server

	mu       sync.Mutex
	document CatalogDocument
	files    map[string][]byte
	hits     atomic.Int64
}

func newCatalogServer(t *testing.T) *catalogServer {
	t.Helper()

	server := &catalogServer{files: map[string][]byte{}}
	server.document = CatalogDocument{SchemaVersion: CatalogSchemaVersion, UpdatedAt: "2026-01-01T00:00:00Z"}

	mux := http.NewServeMux()
	mux.HandleFunc("/index.json", func(w http.ResponseWriter, _ *http.Request) {
		server.hits.Add(1)
		server.mu.Lock()
		defer server.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(server.document)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		server.mu.Lock()
		body, ok := server.files[r.URL.Path]
		server.mu.Unlock()
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Length", itoa(len(body)))
		_, _ = w.Write(body)
	})

	server.Server = httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	digits := make([]byte, 0, 20)
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}

// catalogURL is the source URL the settings point at.
func (cs *catalogServer) catalogURL() string { return cs.URL + "/index.json" }

// serve registers one file under the server root.
func (cs *catalogServer) serve(path string, body []byte) {
	cs.mu.Lock()
	cs.files[path] = body
	cs.mu.Unlock()
}

// publish builds a package for the manifest, registers it and adds the release
// to the catalog. mutate can adjust the entry and the release afterwards.
func (cs *catalogServer) publish(t *testing.T, manifest *protocol.Manifest, signer *minisign.PrivateKey,
	mutate func(entry *CatalogEntry, release *CatalogRelease),
) {
	t.Helper()

	archive := buildTestPackage(t, manifest, map[string]string{"webapp/main.js": "export default {}"})
	body, err := os.ReadFile(archive)
	require.NoError(t, err)

	name := "/pkg/" + manifest.ID + "-" + manifest.Version + ".tar.gz"
	cs.serve(name, body)

	digest := sha256.Sum256(body)
	release := CatalogRelease{
		Version:     manifest.Version,
		ReleasedAt:  "2026-01-01T00:00:00Z",
		APIVersion:  manifest.APIVersion,
		Platforms:   []string{anyPlatform},
		DownloadURL: cs.URL + name,
		SHA256:      hex.EncodeToString(digest[:]),
		Manifest:    manifest,
	}
	if signer != nil {
		cs.serve(name+".minisig", signPackage(t, body, *signer))
		release.SignatureURL = release.DownloadURL + ".minisig"
	}

	cs.mu.Lock()
	defer cs.mu.Unlock()

	index := -1
	for i := range cs.document.Plugins {
		if cs.document.Plugins[i].ID == manifest.ID {
			index = i
			break
		}
	}
	if index < 0 {
		cs.document.Plugins = append(cs.document.Plugins, CatalogEntry{
			ID:           manifest.ID,
			Name:         map[string]string{"en": manifest.Name, "zh_CN": manifest.Name + " 中文"},
			Description:  map[string]string{"en": manifest.Description},
			Author:       "example",
			Categories:   []string{"dns01"},
			Capabilities: manifest.Capabilities,
			Trust:        TrustOfficial,
			Stage:        "production",
		})
		index = len(cs.document.Plugins) - 1
	}
	entry := &cs.document.Plugins[index]
	entry.Releases = append(entry.Releases, release)
	if mutate != nil {
		mutate(entry, &entry.Releases[len(entry.Releases)-1])
	}
}

// marketplaceManifest describes a plugin that spawns no process, so the
// marketplace tests never start a supervisor.
func marketplaceManifest(id, pluginVersion string) *protocol.Manifest {
	return &protocol.Manifest{
		ID:          id,
		Name:        id,
		Version:     pluginVersion,
		Description: "A test plugin",
		APIVersion:  protocol.APIVersion,
		Webapp:      &protocol.ManifestWebapp{BundlePath: "webapp/main.js"},
	}
}

// signPackage produces the prehashed signature pkgsign verifies. The reader
// has to see the whole archive before the signature is taken.
func signPackage(t *testing.T, body []byte, key minisign.PrivateKey) []byte {
	t.Helper()
	reader := minisign.NewReader(bytes.NewReader(body))
	_, err := io.Copy(io.Discard, reader)
	require.NoError(t, err)
	return reader.Sign(key)
}

// newSigningKey returns a throwaway minisign key pair.
func newSigningKey(t *testing.T) (minisign.PublicKey, minisign.PrivateKey) {
	t.Helper()
	public, private, err := minisign.GenerateKey(rand.Reader)
	require.NoError(t, err)
	return public, private
}

// useMarketplace points the settings at a test catalog and restores them.
func useMarketplace(t *testing.T, sources ...string) {
	t.Helper()

	previous := *settings.PluginSettings
	t.Cleanup(func() { *settings.PluginSettings = previous })

	settings.PluginSettings.Enabled = true
	settings.PluginSettings.MarketplaceEnabled = true
	settings.PluginSettings.MarketplaceSources = sources
	settings.PluginSettings.AllowCommunityPlugins = true
	// httptest only speaks plain http.
	settings.PluginSettings.AllowInsecureDownloadURL = true
	settings.PluginSettings.RequireSignature = false
	settings.PluginSettings.TrustedPublicKeys = nil
}

// trustKey adds a test public key to the node trust store.
func trustKey(t *testing.T, key minisign.PublicKey) {
	t.Helper()
	encoded, err := key.MarshalText()
	require.NoError(t, err)
	settings.PluginSettings.TrustedPublicKeys = append(settings.PluginSettings.TrustedPublicKeys, string(encoded))
}

func TestMarketplaceCatalogComputesInstallableState(t *testing.T) {
	manager := newTestManager(t)
	server := newCatalogServer(t)
	useMarketplace(t, server.catalogURL())

	server.publish(t, marketplaceManifest("com.example.alpha", "1.0.0"), nil, nil)
	server.publish(t, marketplaceManifest("com.example.alpha", "1.1.0"), nil, nil)
	server.publish(t, marketplaceManifest("com.example.beta", "2.0.0"), nil, func(_ *CatalogEntry, release *CatalogRelease) {
		release.Yanked = true
	})
	server.publish(t, marketplaceManifest("com.example.gamma", "1.0.0"), nil, func(_ *CatalogEntry, release *CatalogRelease) {
		release.Platforms = []string{"plan9-386"}
	})

	entries, err := manager.Marketplace().Catalog(context.Background(), true)
	require.NoError(t, err)
	require.Len(t, entries, 3)

	alpha := findEntry(entries, "com.example.alpha", "")
	require.NotNil(t, alpha)
	require.NotNil(t, alpha.InstallableRelease)
	assert.Equal(t, "1.1.0", alpha.InstallableRelease.Version)
	assert.Equal(t, server.catalogURL(), alpha.Source)
	assert.False(t, alpha.UpdateAvailable)

	// A yanked or foreign platform release is never installable.
	assert.Nil(t, findEntry(entries, "com.example.beta", "").InstallableRelease)
	assert.Nil(t, findEntry(entries, "com.example.gamma", "").InstallableRelease)
}

func TestMarketplaceCatalogMergesSourcesFirstWins(t *testing.T) {
	manager := newTestManager(t)
	primary := newCatalogServer(t)
	secondary := newCatalogServer(t)
	useMarketplace(t, primary.catalogURL(), secondary.catalogURL())

	primary.publish(t, marketplaceManifest("com.example.shared", "1.0.0"), nil, nil)
	secondary.publish(t, marketplaceManifest("com.example.shared", "9.9.9"), nil, nil)
	secondary.publish(t, marketplaceManifest("com.example.only", "1.0.0"), nil, nil)

	entries, err := manager.Marketplace().Catalog(context.Background(), true)
	require.NoError(t, err)
	require.Len(t, entries, 2)

	shared := findEntry(entries, "com.example.shared", "")
	require.NotNil(t, shared)
	assert.Equal(t, primary.catalogURL(), shared.Source)
	assert.Equal(t, "1.0.0", shared.InstallableRelease.Version)
	assert.Equal(t, secondary.catalogURL(), findEntry(entries, "com.example.only", "").Source)
}

func TestMarketplaceCatalogCachesEverySource(t *testing.T) {
	manager := newTestManager(t)
	server := newCatalogServer(t)
	useMarketplace(t, server.catalogURL())
	server.publish(t, marketplaceManifest("com.example.alpha", "1.0.0"), nil, nil)

	marketplace := manager.Marketplace()
	_, err := marketplace.Catalog(context.Background(), false)
	require.NoError(t, err)
	_, err = marketplace.Catalog(context.Background(), false)
	require.NoError(t, err)
	assert.Equal(t, int64(1), server.hits.Load())

	_, err = marketplace.Catalog(context.Background(), true)
	require.NoError(t, err)
	assert.Equal(t, int64(2), server.hits.Load())

	marketplace.ClearCache()
	_, err = marketplace.Catalog(context.Background(), false)
	require.NoError(t, err)
	assert.Equal(t, int64(3), server.hits.Load())
}

func TestMarketplaceUnavailableSourceIsReported(t *testing.T) {
	manager := newTestManager(t)
	useMarketplace(t, "http://127.0.0.1:1/index.json")

	_, err := manager.Marketplace().Catalog(context.Background(), true)
	assertPluginError(t, err, ErrSourceUnavailable)
}

func TestMarketplaceSearchFiltersKeywordAndCategory(t *testing.T) {
	manager := newTestManager(t)
	server := newCatalogServer(t)
	useMarketplace(t, server.catalogURL())

	server.publish(t, marketplaceManifest("com.example.alpha", "1.0.0"), nil, nil)
	server.publish(t, marketplaceManifest("com.example.beta", "1.0.0"), nil, func(entry *CatalogEntry, _ *CatalogRelease) {
		entry.Categories = []string{"metrics"}
		entry.Name = map[string]string{"zh_CN": "指标面板"}
	})

	marketplace := manager.Marketplace()
	ctx := context.Background()

	byCategory, err := marketplace.Search(ctx, CatalogFilter{Category: "metrics", Refresh: true})
	require.NoError(t, err)
	require.Len(t, byCategory, 1)
	assert.Equal(t, "com.example.beta", byCategory[0].ID)

	// The keyword filter looks at every language.
	byKeyword, err := marketplace.Search(ctx, CatalogFilter{Keyword: "指标"})
	require.NoError(t, err)
	require.Len(t, byKeyword, 1)
	assert.Equal(t, "com.example.beta", byKeyword[0].ID)

	bySource, err := marketplace.Search(ctx, CatalogFilter{Source: "https://example.invalid/index.json"})
	require.NoError(t, err)
	assert.Empty(t, bySource)
}

func TestMarketplaceDetailProxiesReadme(t *testing.T) {
	manager := newTestManager(t)
	server := newCatalogServer(t)
	useMarketplace(t, server.catalogURL())

	server.publish(t, marketplaceManifest("com.example.alpha", "1.0.0"), nil, func(entry *CatalogEntry, _ *CatalogRelease) {
		entry.ReadmeURL = server.URL + "/readme.md"
	})
	server.serve("/readme.md", []byte("# Alpha\n"))

	entry, readme, err := manager.Marketplace().Detail(context.Background(), "com.example.alpha", "")
	require.NoError(t, err)
	require.NotNil(t, entry)
	assert.Equal(t, "# Alpha\n", readme)

	_, _, err = manager.Marketplace().Detail(context.Background(), "com.example.missing", "")
	assert.ErrorIs(t, err, ErrMarketplaceNotFound)
}

func TestMarketplaceInstallVerifiesSignature(t *testing.T) {
	manager := newTestManager(t)
	server := newCatalogServer(t)
	useMarketplace(t, server.catalogURL())

	public, private := newSigningKey(t)
	trustKey(t, public)
	settings.PluginSettings.RequireSignature = true
	server.publish(t, marketplaceManifest("com.example.alpha", "1.0.0"), &private, nil)

	info, err := manager.Marketplace().Install(context.Background(), "com.example.alpha", "", "",
		InstallOptions{Enable: true, ApprovePermissions: true})
	require.NoError(t, err)
	assert.Equal(t, "com.example.alpha", info.ID)
	assert.Equal(t, "1.0.0", info.Version)
	assert.True(t, info.Enabled)

	// The catalog now reports the installed state.
	entries, err := manager.Marketplace().Catalog(context.Background(), true)
	require.NoError(t, err)
	assert.Equal(t, "1.0.0", findEntry(entries, "com.example.alpha", "").InstalledVersion)
}

func TestMarketplaceInstallRejectsUnsignedWhenRequired(t *testing.T) {
	manager := newTestManager(t)
	server := newCatalogServer(t)
	useMarketplace(t, server.catalogURL())
	settings.PluginSettings.RequireSignature = true

	server.publish(t, marketplaceManifest("com.example.alpha", "1.0.0"), nil, nil)

	_, err := manager.Marketplace().Install(context.Background(), "com.example.alpha", "", "", InstallOptions{})
	assert.ErrorIs(t, err, ErrSignatureMissing)

	// The same package installs once the node accepts unsigned sources.
	settings.PluginSettings.RequireSignature = false
	_, err = manager.Marketplace().Install(context.Background(), "com.example.alpha", "", "", InstallOptions{})
	require.NoError(t, err)
}

func TestMarketplaceInstallRejectsForeignSignature(t *testing.T) {
	manager := newTestManager(t)
	server := newCatalogServer(t)
	useMarketplace(t, server.catalogURL())

	_, private := newSigningKey(t)
	// The key that signed the package is never added to the trust store.
	settings.PluginSettings.RequireSignature = true
	server.publish(t, marketplaceManifest("com.example.alpha", "1.0.0"), &private, nil)

	_, err := manager.Marketplace().Install(context.Background(), "com.example.alpha", "", "", InstallOptions{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "signature")
}

func TestMarketplaceInstallRejectsDigestMismatch(t *testing.T) {
	manager := newTestManager(t)
	server := newCatalogServer(t)
	useMarketplace(t, server.catalogURL())

	server.publish(t, marketplaceManifest("com.example.alpha", "1.0.0"), nil, func(_ *CatalogEntry, release *CatalogRelease) {
		release.SHA256 = "00000000000000000000000000000000000000000000000000000000deadbeef"
	})

	_, err := manager.Marketplace().Install(context.Background(), "com.example.alpha", "", "", InstallOptions{})
	assert.ErrorIs(t, err, ErrDigestMismatch)
}

func TestMarketplaceInstallRejectsYankedAndUnknownRelease(t *testing.T) {
	manager := newTestManager(t)
	server := newCatalogServer(t)
	useMarketplace(t, server.catalogURL())

	server.publish(t, marketplaceManifest("com.example.alpha", "1.0.0"), nil, func(_ *CatalogEntry, release *CatalogRelease) {
		release.Yanked = true
	})

	marketplace := manager.Marketplace()
	_, err := marketplace.Install(context.Background(), "com.example.alpha", "1.0.0", "", InstallOptions{})
	assert.ErrorIs(t, err, ErrReleaseYanked)

	_, err = marketplace.Install(context.Background(), "com.example.alpha", "2.0.0", "", InstallOptions{})
	assert.ErrorIs(t, err, ErrReleaseNotFound)

	// With every release yanked there is nothing installable left.
	_, err = marketplace.Install(context.Background(), "com.example.alpha", "", "", InstallOptions{})
	assert.ErrorIs(t, err, ErrPlatformUnsupported)
}

func TestMarketplaceInstallHonoursPolicy(t *testing.T) {
	manager := newTestManager(t)
	server := newCatalogServer(t)
	useMarketplace(t, server.catalogURL())

	server.publish(t, marketplaceManifest("com.example.alpha", "1.0.0"), nil, func(entry *CatalogEntry, _ *CatalogRelease) {
		entry.Trust = TrustCommunity
	})
	marketplace := manager.Marketplace()

	settings.PluginSettings.AllowCommunityPlugins = false
	_, err := marketplace.Install(context.Background(), "com.example.alpha", "", "", InstallOptions{})
	assert.ErrorIs(t, err, ErrCommunityNotAllowed)

	settings.PluginSettings.AllowCommunityPlugins = true
	settings.PluginSettings.AllowInsecureDownloadURL = false
	_, err = marketplace.Install(context.Background(), "com.example.alpha", "", "", InstallOptions{})
	assert.ErrorIs(t, err, ErrInsecureURL)

	settings.PluginSettings.MarketplaceEnabled = false
	_, err = marketplace.Install(context.Background(), "com.example.alpha", "", "", InstallOptions{})
	assert.ErrorIs(t, err, ErrMarketplaceDisabled)
}

func TestMarketplaceInstallRejectsIDMismatch(t *testing.T) {
	manager := newTestManager(t)
	server := newCatalogServer(t)
	useMarketplace(t, server.catalogURL())

	// The package ships another id than the catalog advertises.
	manifest := marketplaceManifest("com.example.other", "1.0.0")
	archive := buildTestPackage(t, manifest, map[string]string{"webapp/main.js": "export default {}"})
	body, err := os.ReadFile(archive)
	require.NoError(t, err)
	server.serve("/pkg/swapped.tar.gz", body)

	digest := sha256.Sum256(body)
	server.mu.Lock()
	server.document.Plugins = append(server.document.Plugins, CatalogEntry{
		ID:    "com.example.alpha",
		Name:  map[string]string{"en": "Alpha"},
		Trust: TrustOfficial,
		Releases: []CatalogRelease{{
			Version:     "1.0.0",
			APIVersion:  protocol.APIVersion,
			Platforms:   []string{anyPlatform},
			DownloadURL: server.URL + "/pkg/swapped.tar.gz",
			SHA256:      hex.EncodeToString(digest[:]),
		}},
	})
	server.mu.Unlock()

	_, err = manager.Marketplace().Install(context.Background(), "com.example.alpha", "", "", InstallOptions{})
	assert.ErrorIs(t, err, ErrPluginIDMismatch)
}

func TestMarketplaceInstallsDependenciesFirst(t *testing.T) {
	manager := newTestManager(t)
	server := newCatalogServer(t)
	useMarketplace(t, server.catalogURL())

	base := marketplaceManifest("com.example.base", "1.2.0")
	dependent := marketplaceManifest("com.example.dependent", "1.0.0")
	dependent.Requires = []protocol.ManifestRequirement{{ID: "com.example.base", Version: "^1.0.0"}}

	server.publish(t, base, nil, nil)
	server.publish(t, dependent, nil, nil)

	_, err := manager.Marketplace().Install(context.Background(), "com.example.dependent", "", "",
		InstallOptions{Enable: true, ApprovePermissions: true})
	require.NoError(t, err)

	installed, err := manager.Get("com.example.base")
	require.NoError(t, err)
	assert.Equal(t, "1.2.0", installed.Version)
}

func TestMarketplaceInstallReportsMissingDependency(t *testing.T) {
	manager := newTestManager(t)
	server := newCatalogServer(t)
	useMarketplace(t, server.catalogURL())

	dependent := marketplaceManifest("com.example.dependent", "1.0.0")
	dependent.Requires = []protocol.ManifestRequirement{{ID: "com.example.base"}}
	server.publish(t, dependent, nil, nil)

	_, err := manager.Marketplace().Install(context.Background(), "com.example.dependent", "", "", InstallOptions{})
	assertPluginError(t, err, ErrDependencyMissing)
}

func TestMarketplaceUpdatesAndUpdate(t *testing.T) {
	manager := newTestManager(t)
	server := newCatalogServer(t)
	useMarketplace(t, server.catalogURL())

	server.publish(t, marketplaceManifest("com.example.alpha", "1.0.0"), nil, nil)
	marketplace := manager.Marketplace()
	ctx := context.Background()

	_, err := marketplace.Install(ctx, "com.example.alpha", "", "", InstallOptions{Enable: true, ApprovePermissions: true})
	require.NoError(t, err)

	updates, err := marketplace.Updates(ctx)
	require.NoError(t, err)
	assert.Empty(t, updates)

	// A newer release shows up and the installed one is withdrawn.
	server.publish(t, marketplaceManifest("com.example.alpha", "1.1.0"), nil, func(entry *CatalogEntry, _ *CatalogRelease) {
		entry.Releases[0].Yanked = true
	})
	marketplace.ClearCache()

	updates, err = marketplace.Updates(ctx)
	require.NoError(t, err)
	require.Len(t, updates, 1)
	assert.Equal(t, "1.0.0", updates[0].InstalledVersion)
	assert.Equal(t, "1.1.0", updates[0].LatestVersion)
	assert.True(t, updates[0].InstalledYanked)
	assert.False(t, updates[0].PermissionsChanged)

	info, err := marketplace.Update(ctx, "com.example.alpha", "", false)
	require.NoError(t, err)
	assert.Equal(t, "1.1.0", info.Version)
	// The upgrade keeps the plugin switched on.
	assert.True(t, info.Enabled)
}

func TestMarketplaceUpdatesFlagsPermissionChanges(t *testing.T) {
	manager := newTestManager(t)
	server := newCatalogServer(t)
	useMarketplace(t, server.catalogURL())

	server.publish(t, marketplaceManifest("com.example.alpha", "1.0.0"), nil, nil)
	ctx := context.Background()
	marketplace := manager.Marketplace()
	_, err := marketplace.Install(ctx, "com.example.alpha", "", "", InstallOptions{Enable: true, ApprovePermissions: true})
	require.NoError(t, err)

	next := marketplaceManifest("com.example.alpha", "1.1.0")
	next.Permissions = []string{protocol.PermissionKV, protocol.PermissionNetwork}
	server.publish(t, next, nil, nil)
	marketplace.ClearCache()

	updates, err := marketplace.Updates(ctx)
	require.NoError(t, err)
	require.Len(t, updates, 1)
	assert.True(t, updates[0].PermissionsChanged)
}

func TestReleaseRunsHere(t *testing.T) {
	assert.True(t, releaseRunsHere(&CatalogRelease{APIVersion: protocol.APIVersion}))
	assert.True(t, releaseRunsHere(&CatalogRelease{APIVersion: protocol.APIVersion, Platforms: []string{anyPlatform}}))
	assert.False(t, releaseRunsHere(&CatalogRelease{APIVersion: protocol.APIVersion + 1}))
	assert.False(t, releaseRunsHere(&CatalogRelease{APIVersion: protocol.APIVersion, Platforms: []string{"plan9-386"}}))
}

func TestProxiedURLOnlyRewritesGithub(t *testing.T) {
	previous := settings.HTTPSettings.GithubProxy
	settings.HTTPSettings.GithubProxy = "https://proxy.example.com/"
	t.Cleanup(func() { settings.HTTPSettings.GithubProxy = previous })

	assert.Equal(t, "https://proxy.example.com/https://raw.githubusercontent.com/a/b/index.json",
		proxiedURL("https://raw.githubusercontent.com/a/b/index.json"))
	assert.Equal(t, "https://mirror.example.org/index.json", proxiedURL("https://mirror.example.org/index.json"))
	assert.Equal(t, "", proxiedURL(""))
}
