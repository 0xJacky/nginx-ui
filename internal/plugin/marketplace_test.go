package plugin

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"sync/atomic"
	"testing"

	"aead.dev/minisign"
	"github.com/0xJacky/Nginx-UI/internal/plugin/protocol"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// catalogServer serves a static catalog and the packages it points at, which
// is everything the installer talks to.
type catalogServer struct {
	*httptest.Server

	mu       sync.Mutex
	document CatalogDocument
	files    map[string][]byte
	// requests counts the requests for every path besides the catalog.
	requests map[string]int
	hits     atomic.Int64
}

func newCatalogServer(t *testing.T) *catalogServer {
	t.Helper()

	server := &catalogServer{files: map[string][]byte{}, requests: map[string]int{}}
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
		server.requests[r.URL.Path]++
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

// publish builds a package for the manifest, signed when signer is set,
// registers it and adds the release to the catalog. mutate can adjust the
// entry and the release afterwards.
func (cs *catalogServer) publish(t *testing.T, manifest *protocol.Manifest, signer *minisign.PrivateKey,
	mutate func(entry *CatalogEntry, release *CatalogRelease),
) {
	t.Helper()

	archive := buildSignedTestPackage(t, manifest, map[string]string{"webapp/main.js": "export default {}"}, signer)
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

// newSigningKey returns a throwaway minisign key pair.
func newSigningKey(t *testing.T) (minisign.PublicKey, minisign.PrivateKey) {
	t.Helper()
	public, private, err := minisign.GenerateKey(rand.Reader)
	require.NoError(t, err)
	return public, private
}

// useMarketplace points the settings at a test catalog and restores them.
// Developer mode stays on, most tests publish unsigned packages.
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
	settings.PluginSettings.DeveloperMode = true
	settings.PluginSettings.TrustedPublicKeys = nil

	// No test reaches the real partner keyring.
	previousOfficial := officialSource
	officialSource = ""
	t.Cleanup(func() { officialSource = previousOfficial })
}

// encodeKey is the minisign text form of a public key.
func encodeKey(t *testing.T, key minisign.PublicKey) string {
	t.Helper()
	encoded, err := key.MarshalText()
	require.NoError(t, err)
	return string(encoded)
}

// trustKey adds a test public key to the node trust store, which makes the
// packages it signs community trust. The entry carries a publisher name like
// the ones the settings page saves.
func trustKey(t *testing.T, key minisign.PublicKey) {
	t.Helper()
	settings.PluginSettings.TrustedPublicKeys = append(settings.PluginSettings.TrustedPublicKeys, publisherEntry(t, key))
}

// publisherKey is the key line of a public key, the form a trusted publisher
// entry keeps.
func publisherKey(t *testing.T, key minisign.PublicKey) string {
	t.Helper()
	line, _ := settings.ParseTrustedPublisher(encodeKey(t, key))
	return line
}

// publisherEntry is a trusted publisher entry as the settings page saves it:
// the key line followed by a name.
func publisherEntry(t *testing.T, key minisign.PublicKey) string {
	t.Helper()
	return publisherKey(t, key) + " Example Publisher"
}

// useReleaseKey pins a test key as the only release key, so the packages it
// signs are official.
func useReleaseKey(t *testing.T) *minisign.PrivateKey {
	t.Helper()
	public, private := newSigningKey(t)
	encoded := encodeKey(t, public)
	previous := releaseKeys
	releaseKeys = func() []string { return []string{encoded} }
	t.Cleanup(func() { releaseKeys = previous })
	return &private
}

// usePartnerKey returns a key and a keyring that lists it as partner
// "example", so the packages it signs are verified with that keyring.
func usePartnerKey(t *testing.T) (*minisign.PrivateKey, *partnerKeyring) {
	t.Helper()
	public, private := newSigningKey(t)
	return &private, partnerKeyringOf(t, public)
}

// partnerKeyringOf builds a keyring listing keys as partners "example",
// "example2" and so on, without the signature a fetched one carries.
func partnerKeyringOf(t *testing.T, keys ...minisign.PublicKey) *partnerKeyring {
	t.Helper()
	keyring := &partnerKeyring{updatedAt: now(), revoked: map[string]struct{}{}}
	for i, key := range keys {
		name := "example"
		if i > 0 {
			name = fmt.Sprintf("example%d", i+1)
		}
		keyring.partners = append(keyring.partners, partnerCertificate{
			Name:  name,
			KeyID: fmt.Sprintf("%016X", key.ID()),
			Key:   encodeKey(t, key),
		})
	}
	return keyring
}

// keyID is the signer a key leaves on an installed plugin.
func keyID(key *minisign.PrivateKey) string {
	return fmt.Sprintf("%016X", key.ID())
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

func TestMarketplaceCatalogOnlyNormalisesTheDeclaredTrust(t *testing.T) {
	manager := newTestManager(t)
	server := newCatalogServer(t)
	useMarketplace(t, server.catalogURL())
	server.publish(t, marketplaceManifest("com.example.alpha", "1.0.0"), nil, nil)
	server.publish(t, marketplaceManifest("com.example.beta", "1.0.0"), nil, func(entry *CatalogEntry, _ *CatalogRelease) {
		entry.Trust = "partner"
	})

	// The declared level is only shown in the listing, the install derives
	// the real one from the package signature.
	entries, err := manager.Marketplace().Catalog(context.Background(), true)
	require.NoError(t, err)
	assert.Equal(t, TrustOfficial, findEntry(entries, "com.example.alpha", "").Trust)
	// An unknown level is community, so the community pre filter applies.
	assert.Equal(t, TrustCommunity, findEntry(entries, "com.example.beta", "").Trust)

	assert.Equal(t, TrustVerified, effectiveTrust(TrustVerified))
	assert.Equal(t, TrustCommunity, effectiveTrust(TrustCommunity))
	assert.Equal(t, TrustCommunity, effectiveTrust(TrustUnsigned))
	assert.Equal(t, TrustCommunity, effectiveTrust(""))
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

func TestMarketplaceFailedSourceWaitsBeforeTheNextTry(t *testing.T) {
	manager := newTestManager(t)
	var hits atomic.Int32
	failing := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		http.Error(w, "down", http.StatusServiceUnavailable)
	}))
	t.Cleanup(failing.Close)
	working := newCatalogServer(t)
	working.publish(t, marketplaceManifest("com.example.alpha", "1.0.0"), nil, nil)
	useMarketplace(t, failing.URL+"/index.json", working.catalogURL())
	marketplace := manager.Marketplace()
	marketplace.ClearCache()

	for range 3 {
		entries, err := marketplace.Catalog(context.Background(), false)
		require.NoError(t, err, "a working source is enough")
		require.NotNil(t, findEntry(entries, "com.example.alpha", ""))
	}
	assert.Equal(t, int32(1), hits.Load(), "a failed source is not asked again right away")

	_, err := marketplace.Catalog(context.Background(), true)
	require.NoError(t, err)
	assert.Equal(t, int32(2), hits.Load(), "a refresh asks again")

	marketplace.ClearCache()
	_, err = marketplace.Catalog(context.Background(), false)
	require.NoError(t, err)
	assert.Equal(t, int32(3), hits.Load(), "clearing the cache forgets the failure")
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

func TestMarketplaceDetailSkipsAReadmeOnAnotherHost(t *testing.T) {
	manager := newTestManager(t)
	server := newCatalogServer(t)
	useMarketplace(t, server.catalogURL())

	var hits atomic.Int64
	internal := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		_, _ = w.Write([]byte("internal"))
	}))
	t.Cleanup(internal.Close)

	server.publish(t, marketplaceManifest("com.example.alpha", "1.0.0"), nil, func(entry *CatalogEntry, _ *CatalogRelease) {
		entry.ReadmeURL = internal.URL + "/readme.md"
	})

	entry, readme, err := manager.Marketplace().Detail(context.Background(), "com.example.alpha", "")
	require.NoError(t, err)
	require.NotNil(t, entry)
	assert.Empty(t, readme)
	assert.Zero(t, hits.Load(), "the readme host must not be contacted")
}

func TestCheckCatalogURL(t *testing.T) {
	useMarketplace(t)
	settings.PluginSettings.AllowInsecureDownloadURL = false

	entry := &CatalogEntry{
		Source: "https://catalog.example/index.json",
		InstallableRelease: &CatalogRelease{
			Downloads: map[string]ReleaseDownload{HostPlatform(): {URL: "https://cdn.example/pkg.tar.gz"}},
		},
	}
	allowed := []string{
		"https://catalog.example/readme.md",
		"https://CATALOG.example/docs/README.md",
		"https://cdn.example/readme.md",
		"https://raw.githubusercontent.com/example/plugin/main/README.md",
		"https://github.com/example/plugin/raw/main/README.md",
	}
	for _, raw := range allowed {
		entry.ReadmeURL = raw
		assert.NoError(t, checkCatalogURL(entry, entry.ReadmeURL), raw)
	}

	refused := []string{
		"http://catalog.example/readme.md",
		"https://169.254.169.254/latest/meta-data/",
		"https://localhost/readme.md",
		"https://catalog.example:8443/readme.md",
		"https://raw.githubusercontent.com:8443/readme.md",
		"https://catalog.example.evil.test/readme.md",
		"file:///etc/passwd",
		"ftp://catalog.example/readme.md",
		"https:///readme.md",
		"://catalog.example/readme.md",
	}
	for _, raw := range refused {
		entry.ReadmeURL = raw
		assert.Error(t, checkCatalogURL(entry, entry.ReadmeURL), raw)
	}

	// Plain http passes once insecure downloads are allowed.
	settings.PluginSettings.AllowInsecureDownloadURL = true
	entry.ReadmeURL = "http://catalog.example/readme.md"
	assert.NoError(t, checkCatalogURL(entry, entry.ReadmeURL))

	// The portable package host counts as well.
	entry.InstallableRelease = &CatalogRelease{DownloadURL: "https://portable.example/pkg.tar.gz"}
	entry.ReadmeURL = "https://portable.example/readme.md"
	assert.NoError(t, checkCatalogURL(entry, entry.ReadmeURL))
	entry.ReadmeURL = "https://cdn.example/readme.md"
	assert.Error(t, checkCatalogURL(entry, entry.ReadmeURL))
}

func TestMarketplaceInstallTrustsTheReleaseKey(t *testing.T) {
	manager := newTestManager(t)
	server := newCatalogServer(t)
	useMarketplace(t, server.catalogURL())
	settings.PluginSettings.DeveloperMode = false

	signer := useReleaseKey(t)
	server.publish(t, marketplaceManifest("com.example.alpha", "1.0.0"), signer, nil)

	info, err := manager.Marketplace().Install(context.Background(), "com.example.alpha", "", "",
		InstallOptions{Enable: true, ApprovePermissions: true})
	require.NoError(t, err)
	assert.Equal(t, "com.example.alpha", info.ID)
	assert.Equal(t, "1.0.0", info.Version)
	assert.True(t, info.Enabled)
	assert.Equal(t, TrustOfficial, info.Trust)
	assert.Equal(t, keyID(signer), info.Signer)

	// The catalog now reports the installed state.
	entries, err := manager.Marketplace().Catalog(context.Background(), true)
	require.NoError(t, err)
	assert.Equal(t, "1.0.0", findEntry(entries, "com.example.alpha", "").InstalledVersion)
}

func TestMarketplaceInstallDerivesTheTrustFromTheSigner(t *testing.T) {
	manager := newTestManager(t)
	server := newCatalogServer(t)
	useMarketplace(t, server.catalogURL())
	settings.PluginSettings.DeveloperMode = false

	partner, keyring := usePartnerKey(t)
	manager.setPartnerKeyring(keyring)
	userPublic, user := newSigningKey(t)
	trustKey(t, userPublic)
	authorPublic, author := newSigningKey(t)

	// Every entry claims official, the signature decides.
	server.publish(t, marketplaceManifest("com.example.partner", "1.0.0"), partner, nil)
	server.publish(t, marketplaceManifest("com.example.user", "1.0.0"), &user, nil)
	server.publish(t, marketplaceManifest("com.example.author", "1.0.0"), &author,
		func(entry *CatalogEntry, _ *CatalogRelease) {
			entry.AuthorPublicKey = encodeKey(t, authorPublic)
		})

	for id, want := range map[string]struct {
		trust   string
		signer  string
		partner string
	}{
		"com.example.partner": {TrustVerified, keyID(partner), "example"},
		"com.example.user":    {TrustCommunity, keyID(&user), ""},
		"com.example.author":  {TrustCommunity, keyID(&author), ""},
	} {
		info, err := manager.Marketplace().Install(context.Background(), id, "", "", InstallOptions{})
		require.NoError(t, err, id)
		assert.Equal(t, want.trust, info.Trust, id)
		assert.Equal(t, want.signer, info.Signer, id)
		assert.Equal(t, want.partner, info.Partner, id)

		// The trust is stored with the row, not recomputed.
		row, err := query.Plugin.WithContext(context.Background()).Where(query.Plugin.PluginID.Eq(id)).First()
		require.NoError(t, err)
		assert.Equal(t, want.trust, row.Trust, id)
		assert.Equal(t, want.signer, row.Signer, id)
		assert.Equal(t, want.partner, row.Partner, id)
	}
}

func TestMarketplaceInstallRejectsUnsignedWithoutDeveloperMode(t *testing.T) {
	manager := newTestManager(t)
	server := newCatalogServer(t)
	useMarketplace(t, server.catalogURL())
	settings.PluginSettings.DeveloperMode = false

	server.publish(t, marketplaceManifest("com.example.alpha", "1.0.0"), nil, nil)

	_, err := manager.Marketplace().Install(context.Background(), "com.example.alpha", "", "", InstallOptions{})
	assert.ErrorIs(t, err, ErrUnsignedPackage)
	_, err = manager.Get("com.example.alpha")
	assert.ErrorIs(t, err, ErrPluginNotFound)

	// The same package installs in developer mode, as unsigned.
	settings.PluginSettings.DeveloperMode = true
	info, err := manager.Marketplace().Install(context.Background(), "com.example.alpha", "", "", InstallOptions{})
	require.NoError(t, err)
	assert.Equal(t, TrustUnsigned, info.Trust)
	assert.Empty(t, info.Signer)
}

func TestMarketplaceInstallCountsAnUnknownSignerAsUnsigned(t *testing.T) {
	manager := newTestManager(t)
	server := newCatalogServer(t)
	useMarketplace(t, server.catalogURL())
	settings.PluginSettings.DeveloperMode = false

	// The key that signed the package is known nowhere.
	_, private := newSigningKey(t)
	server.publish(t, marketplaceManifest("com.example.alpha", "1.0.0"), &private, nil)

	_, err := manager.Marketplace().Install(context.Background(), "com.example.alpha", "", "", InstallOptions{})
	assert.ErrorIs(t, err, ErrUnsignedPackage)

	settings.PluginSettings.DeveloperMode = true
	info, err := manager.Marketplace().Install(context.Background(), "com.example.alpha", "", "", InstallOptions{})
	require.NoError(t, err)
	assert.Equal(t, TrustUnsigned, info.Trust)
	assert.Empty(t, info.Signer)
}

func TestMarketplaceInstallRejectsATamperedPackage(t *testing.T) {
	manager := newTestManager(t)
	server := newCatalogServer(t)
	useMarketplace(t, server.catalogURL())

	signer := useReleaseKey(t)
	server.publish(t, marketplaceManifest("com.example.alpha", "1.0.0"), signer, nil)

	// A mirror swaps a file and fixes the catalog digest, the embedded
	// signature still catches it, developer mode or not.
	name := "/pkg/com.example.alpha-1.0.0.tar.gz"
	server.mu.Lock()
	original := server.files[name]
	server.mu.Unlock()
	tampered := rewritePackageBytes(t, original, func(entry *tarEntry) {
		if entry.header.Name == "webapp/main.js" {
			entry.body = "alert(1)"
		}
	})
	server.serve(name, tampered)
	digest := sha256.Sum256(tampered)
	server.mu.Lock()
	server.document.Plugins[0].Releases[0].SHA256 = hex.EncodeToString(digest[:])
	server.mu.Unlock()

	_, err := manager.Marketplace().Install(context.Background(), "com.example.alpha", "", "", InstallOptions{})
	assertPluginError(t, err, ErrSignatureInvalid)
	_, err = manager.Get("com.example.alpha")
	assert.ErrorIs(t, err, ErrPluginNotFound)
}

func TestMarketplaceUpdatesAutomaticallyOnlyToTheSameTrust(t *testing.T) {
	manager := newTestManager(t)
	server := newCatalogServer(t)
	useMarketplace(t, server.catalogURL())
	settings.PluginSettings.AutoUpdate = true

	official := useReleaseKey(t)
	userPublic, user := newSigningKey(t)
	trustKey(t, userPublic)
	ctx := context.Background()
	marketplace := manager.Marketplace()

	server.publish(t, marketplaceManifest("com.example.alpha", "1.0.0"), official, nil)
	server.publish(t, marketplaceManifest("com.example.dev", "1.0.0"), nil, nil)
	for _, id := range []string{"com.example.alpha", "com.example.dev"} {
		_, err := marketplace.Install(ctx, id, "", "", InstallOptions{Enable: true, ApprovePermissions: true})
		require.NoError(t, err)
	}

	// A community signed update of an official plugin is refused, and an
	// unsigned plugin never updates itself.
	server.publish(t, marketplaceManifest("com.example.alpha", "1.1.0"), &user, nil)
	server.publish(t, marketplaceManifest("com.example.dev", "1.1.0"), official, nil)
	marketplace.runMaintenance(ctx)

	alpha, err := manager.Get("com.example.alpha")
	require.NoError(t, err)
	assert.Equal(t, "1.0.0", alpha.Version)
	dev, err := manager.Get("com.example.dev")
	require.NoError(t, err)
	assert.Equal(t, "1.0.0", dev.Version)

	// An official update goes through.
	server.publish(t, marketplaceManifest("com.example.alpha", "1.2.0"), official, nil)
	marketplace.runMaintenance(ctx)

	alpha, err = manager.Get("com.example.alpha")
	require.NoError(t, err)
	assert.Equal(t, "1.2.0", alpha.Version)
	assert.Equal(t, TrustOfficial, alpha.Trust)
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
	releaseRunsHere := func(release *CatalogRelease) bool { return releaseRunsOn(release, HostPlatform()) }
	const portable = "https://example.com/pkg.tar.gz"
	assert.True(t, releaseRunsHere(&CatalogRelease{APIVersion: protocol.APIVersion, DownloadURL: portable}))
	assert.True(t, releaseRunsHere(&CatalogRelease{APIVersion: protocol.APIVersion, DownloadURL: portable,
		Platforms: []string{anyPlatform}}))
	assert.False(t, releaseRunsHere(&CatalogRelease{APIVersion: protocol.APIVersion + 1, DownloadURL: portable}))
	assert.False(t, releaseRunsHere(&CatalogRelease{APIVersion: protocol.APIVersion, DownloadURL: portable,
		Platforms: []string{"plan9-386"}}))
	// A release without any package cannot be installed anywhere.
	assert.False(t, releaseRunsHere(&CatalogRelease{APIVersion: protocol.APIVersion}))
	// A per-platform download is enough on its own.
	assert.True(t, releaseRunsHere(&CatalogRelease{APIVersion: protocol.APIVersion,
		Downloads: map[string]ReleaseDownload{HostPlatform(): {URL: portable}}}))
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

func TestScreenshotsKeepTheLoadableOnes(t *testing.T) {
	previous := settings.PluginSettings.AllowInsecureDownloadURL
	settings.PluginSettings.AllowInsecureDownloadURL = false
	t.Cleanup(func() { settings.PluginSettings.AllowInsecureDownloadURL = previous })

	shared := []CatalogScreenshot{
		{URL: "https://raw.githubusercontent.com/example/plugin/v1/docs/list.png", Caption: map[string]string{"en": "List"}},
		{URL: "https://tracker.example/pixel.png"},
		{URL: "http://catalog.example/plain.png"},
		{URL: "https://catalog.example/shots/dashboard.webp"},
	}
	for i := 0; i < 10; i++ {
		shared = append(shared, CatalogScreenshot{URL: fmt.Sprintf("https://catalog.example/shots/%d.png", i)})
	}
	entry := &CatalogEntry{Source: "https://catalog.example/v1/index.json", Screenshots: shared}

	kept := loadableScreenshots(entry)
	require.Len(t, kept, maxCatalogScreenshots)
	assert.Equal(t, "List", kept[0].Caption["en"])
	assert.Equal(t, "https://catalog.example/shots/dashboard.webp", kept[1].URL)
	for _, shot := range kept {
		assert.NotContains(t, shot.URL, "tracker.example")
		assert.NotContains(t, shot.URL, "http://")
	}
	assert.Len(t, entry.Screenshots, len(shared), "the entry of the cache keeps its list")
}

func TestScreenshotsDropADarkVariantThatCannotLoad(t *testing.T) {
	previous := settings.PluginSettings.AllowInsecureDownloadURL
	settings.PluginSettings.AllowInsecureDownloadURL = false
	t.Cleanup(func() { settings.PluginSettings.AllowInsecureDownloadURL = previous })

	shared := []CatalogScreenshot{
		{URL: "https://catalog.example/shots/list.png", DarkURL: "https://catalog.example/shots/list-dark.png"},
		{URL: "https://catalog.example/shots/chart.png", DarkURL: "https://tracker.example/chart-dark.png"},
		{URL: "https://tracker.example/only.png", DarkURL: "https://catalog.example/shots/only-dark.png"},
	}
	entry := &CatalogEntry{Source: "https://catalog.example/v1/index.json", Screenshots: shared}

	kept := loadableScreenshots(entry)
	require.Len(t, kept, 2, "a screenshot without a loadable light image is dropped")
	assert.Equal(t, "https://catalog.example/shots/list-dark.png", kept[0].DarkURL)
	assert.Equal(t, "https://catalog.example/shots/chart.png", kept[1].URL)
	assert.Empty(t, kept[1].DarkURL, "the light image stands in for a dark one that cannot load")
	assert.Equal(t, "https://tracker.example/chart-dark.png", shared[1].DarkURL, "the entry of the cache keeps its list")
}

func TestDecorateKeepsOnlyLoadableIcons(t *testing.T) {
	mp := newTestManager(t).Marketplace()
	entry := &CatalogEntry{
		ID:      "com.example.icon",
		Source:  "https://catalog.example/index.json",
		IconURL: "https://catalog.example/icon.png",
	}
	mp.decorate(entry)
	assert.Equal(t, "https://catalog.example/icon.png", entry.IconURL)

	entry.IconURL = "https://tracker.example/pixel.png"
	mp.decorate(entry)
	assert.Empty(t, entry.IconURL)
}

func TestCatalogCandidatesOfASite(t *testing.T) {
	assert.Equal(t, []string{
		"https://plugins.example/v1/index.json",
		"https://plugins.example/index.json",
	}, catalogCandidates("https://plugins.example"))
	assert.Equal(t, []string{
		"https://plugins.example/v1/index.json",
		"https://plugins.example/index.json",
	}, catalogCandidates("https://plugins.example/"))
	assert.Equal(t, []string{"https://plugins.example/catalog.json"}, catalogCandidates("https://plugins.example/catalog.json"))
	assert.Equal(t, []string{"https://plugins.example/?v=1"}, catalogCandidates("https://plugins.example/?v=1"))
}
