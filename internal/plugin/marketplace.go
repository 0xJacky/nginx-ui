package plugin

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/event"
	"github.com/0xJacky/Nginx-UI/internal/notification"
	"github.com/0xJacky/Nginx-UI/internal/plugin/protocol"
	"github.com/0xJacky/Nginx-UI/internal/version"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/uozi-tech/cosy"
)

// Install progress phases published on the event bus.
const (
	InstallStatusDownloading = "downloading"
	InstallStatusVerifying   = "verifying"
	InstallStatusInstalling  = "installing"
	InstallStatusDone        = "done"
	InstallStatusError       = "error"
)

// EventTypeInstallProgress carries the marketplace install progress to the UI.
const EventTypeInstallProgress = event.Type("plugin_install_progress")

// StageBeta is the catalog entry stage that marks a plugin as beta.
const StageBeta = "beta"

// CatalogSchemaVersion is the only catalog layout this node understands.
const CatalogSchemaVersion = 1

const (
	// catalogTTL is how long a fetched source is reused without refreshing.
	catalogTTL = time.Hour
	// catalogRetryAfter is how long a source that failed is not asked again
	// without refreshing, so an unreachable one does not slow every page.
	catalogRetryAfter = 5 * time.Minute
	// maxCatalogBytes bounds one catalog document.
	maxCatalogBytes = 8 << 20
	// maxReadmeBytes bounds the proxied readme.
	maxReadmeBytes = 512 << 10
	// catalogHTTPTimeout bounds a catalog or readme request.
	catalogHTTPTimeout = 30 * time.Second
	// downloadTimeout bounds one package download.
	downloadTimeout = 15 * time.Minute
	// progressStep is the minimum progress delta worth an event.
	progressStep = 2.0
	// anyPlatform matches every host in a release platform list.
	anyPlatform = "any"
)

// githubHosts are rewritten through the configured GitHub proxy, the same way
// internal/version routes the release API.
var githubHosts = []string{
	"github.com",
	"www.github.com",
	"api.github.com",
	"raw.githubusercontent.com",
	"objects.githubusercontent.com",
	"codeload.github.com",
}

// ReleaseDownload is one package file of a release.
type ReleaseDownload struct {
	URL    string `json:"url"`
	SHA256 string `json:"sha256,omitempty"`
}

// CatalogRelease is one downloadable version of a catalog entry.
type CatalogRelease struct {
	Version           string `json:"version"`
	ReleasedAt        string `json:"released_at,omitempty"`
	APIVersion        int    `json:"api_version"`
	MinNginxUIVersion string `json:"min_nginx_ui_version,omitempty"`
	// Platforms summarises where the release installs: the keys of Downloads
	// plus the platforms the portable package covers. An empty list keeps its
	// original meaning, the portable package runs everywhere.
	Platforms []string `json:"platforms,omitempty"`
	// Downloads maps "<goos>-<goarch>" or "any" to a package built for it.
	Downloads map[string]ReleaseDownload `json:"downloads,omitempty"`
	// DownloadURL and SHA256 describe the portable package, the fallback for
	// every platform Downloads does not name.
	DownloadURL     string `json:"download_url"`
	SHA256          string `json:"sha256,omitempty"`
	ReleaseNotesURL string `json:"release_notes_url,omitempty"`
	Yanked          bool   `json:"yanked,omitempty"`
	// Channel is stable, beta or dev. The catalog may leave it out, the host
	// then fills it from the version, so the UI reads one field.
	Channel  string             `json:"channel,omitempty"`
	Manifest *protocol.Manifest `json:"manifest,omitempty"`
}

// DownloadFor resolves the package a node running platform installs:
// downloads[platform], then downloads["any"], then the portable package when
// the platforms summary lists the platform or "any". key is the downloads key
// that matched, empty for the portable package. ok is false when the release
// ships nothing for the platform.
func (r *CatalogRelease) DownloadFor(platform string) (download ReleaseDownload, key string, ok bool) {
	for _, candidate := range []string{platform, anyPlatform} {
		if download, found := r.Downloads[candidate]; found && download.URL != "" {
			return download, candidate, true
		}
	}
	if r.DownloadURL == "" {
		return ReleaseDownload{}, "", false
	}
	if len(r.Platforms) > 0 && !platformsCover(r.Platforms, platform) {
		return ReleaseDownload{}, "", false
	}
	return ReleaseDownload{URL: r.DownloadURL, SHA256: r.SHA256}, "", true
}

// AvailablePlatforms lists every platform key a release can be installed on,
// sorted, for messages and the UI.
func (r *CatalogRelease) AvailablePlatforms() []string {
	platforms := make([]string, 0, len(r.Downloads)+len(r.Platforms))
	for key, download := range r.Downloads {
		if download.URL != "" {
			platforms = append(platforms, key)
		}
	}
	if r.DownloadURL != "" {
		if len(r.Platforms) == 0 {
			platforms = append(platforms, anyPlatform)
		}
		for _, platform := range r.Platforms {
			if !slices.Contains(platforms, platform) {
				platforms = append(platforms, platform)
			}
		}
	}
	sort.Strings(platforms)
	return slices.Compact(platforms)
}

// CatalogEntry is one plugin as the catalog describes it, plus the state this
// node computed for it. Trust is what the catalog declares and is only shown
// in the listing: the install derives the level from the package signature,
// and AuthorPublicKey is the key that makes a package community trust.
type CatalogEntry struct {
	ID              string            `json:"id"`
	Name            map[string]string `json:"name,omitempty"`
	Description     map[string]string `json:"description,omitempty"`
	Author          string            `json:"author,omitempty"`
	AuthorPublicKey string            `json:"author_public_key,omitempty"`
	HomepageURL     string            `json:"homepage_url,omitempty"`
	RepositoryURL   string            `json:"repository_url,omitempty"`
	ReadmeURL       string            `json:"readme_url,omitempty"`
	IconURL         string            `json:"icon_url,omitempty"`
	// Screenshots are the images the catalog lists, in display order, without
	// the ones this node may not load.
	Screenshots  []CatalogScreenshot `json:"screenshots,omitempty"`
	Categories   []string            `json:"categories,omitempty"`
	Capabilities []string            `json:"capabilities,omitempty"`
	License      string              `json:"license,omitempty"`
	Trust        string              `json:"trust,omitempty"`
	Stage        string              `json:"stage,omitempty"`
	// Channel is computed by this node: the channel of the release it would
	// install, at least beta while the entry stage is beta.
	Channel  string           `json:"channel"`
	Releases []CatalogRelease `json:"releases"`

	// Source is the catalog URL this entry was merged from.
	Source string `json:"source"`
	// InstallableRelease is the newest release this node can actually install.
	InstallableRelease *CatalogRelease `json:"installable_release,omitempty"`
	// InstallableVersions lists the versions this node can install, newest
	// first, without the withdrawn ones. Any of them can be asked for by name.
	InstallableVersions []string `json:"installable_versions"`
	// InstalledVersion is the version currently on this node, if any.
	InstalledVersion string `json:"installed_version,omitempty"`
	UpdateAvailable  bool   `json:"update_available"`
}

// CatalogScreenshot is one image of a catalog entry.
type CatalogScreenshot struct {
	URL string `json:"url"`
	// DarkURL is the same view in the dark theme. Hosts show URL when it is
	// empty.
	DarkURL string            `json:"dark_url,omitempty"`
	Caption map[string]string `json:"caption,omitempty"`
}

// maxCatalogScreenshots is the most screenshots of an entry a node shows.
const maxCatalogScreenshots = 8

// maxCatalogNameLength caps the name a catalog declares, in characters.
const maxCatalogNameLength = 64

// CatalogDocument is the static JSON one source serves.
type CatalogDocument struct {
	SchemaVersion int `json:"schema_version"`
	// Name is the name the catalog goes by, a locale map like an entry name.
	Name map[string]string `json:"name,omitempty"`
	// Icon is the address of an image that stands for the catalog.
	Icon      string         `json:"icon,omitempty"`
	UpdatedAt string         `json:"updated_at,omitempty"`
	Plugins   []CatalogEntry `json:"plugins"`
}

// CatalogInfo is what a catalog declares about itself.
type CatalogInfo struct {
	// Name is a locale map of the catalog name.
	Name map[string]string `json:"catalog_name,omitempty"`
	// Icon is an image a browser of this node may load, empty when the
	// catalog declares none this node accepts.
	Icon string `json:"catalog_icon,omitempty"`
}

// CatalogSource is one configured catalog and what it declares about itself,
// known once it was read.
type CatalogSource struct {
	URL string `json:"url"`
	CatalogInfo
}

// SourceProbe is what reading one catalog found.
type SourceProbe struct {
	// URL is the catalog address that answered, or the one asked for when
	// none did.
	URL       string `json:"url"`
	Reachable bool   `json:"reachable"`
	CatalogInfo
	Plugins int    `json:"plugins"`
	Error   string `json:"error,omitempty"`
}

// CatalogFilter narrows the merged catalog.
type CatalogFilter struct {
	Keyword  string
	Category string
	Source   string
	Refresh  bool
}

// UpdateInfo is one installed plugin the catalog offers a newer version for.
type UpdateInfo struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	InstalledVersion string `json:"installed_version"`
	LatestVersion    string `json:"latest_version"`
	Source           string `json:"source"`
	Trust            string `json:"trust,omitempty"`
	// InstalledYanked marks an installed version the catalog withdrew.
	InstalledYanked    bool            `json:"installed_yanked"`
	PermissionsChanged bool            `json:"permissions_changed"`
	Release            *CatalogRelease `json:"release,omitempty"`
}

// InstallProgress is the payload of EventTypeInstallProgress.
type InstallProgress struct {
	PluginID string  `json:"plugin_id"`
	Status   string  `json:"status"`
	Progress float64 `json:"progress"`
	Message  string  `json:"message"`
	// Platform is the downloads key of the package being installed, "any",
	// or empty for the portable package.
	Platform string `json:"platform,omitempty"`
}

// sourceCache keeps one fetched source for catalogTTL.
type sourceCache struct {
	entries []CatalogEntry
	info    CatalogInfo
	fetched time.Time
}

type sourceFailure struct {
	err    error
	failed time.Time
}

// Marketplace fetches the plugin catalogs and installs from them. It is owned
// by the manager, see manager_marketplace.go.
type Marketplace struct {
	manager *Manager

	mu    sync.Mutex
	cache map[string]*sourceCache
	// failures holds the last error of each source that failed.
	failures map[string]sourceFailure

	// installMu serialises whole install flows, including their dependencies.
	installMu sync.Mutex
}

func newMarketplace(m *Manager) *Marketplace {
	return &Marketplace{manager: m, cache: map[string]*sourceCache{}, failures: map[string]sourceFailure{}}
}

// Sources lists the configured catalog URLs in merge order.
func (mp *Marketplace) Sources() []string {
	return settings.PluginSettings.GetMarketplaceSources()
}

// SourceList lists the configured catalogs in merge order with the name each
// declared when it was last read.
func (mp *Marketplace) SourceList() []CatalogSource {
	urls := mp.Sources()
	list := make([]CatalogSource, 0, len(urls))
	mp.mu.Lock()
	defer mp.mu.Unlock()
	for _, rawURL := range urls {
		source := CatalogSource{URL: rawURL}
		if cached, ok := mp.cache[rawURL]; ok {
			source.CatalogInfo = cached.info
		}
		list = append(list, source)
	}
	return list
}

// Probe reads one catalog now and reports whether it answered, what it
// declares and how many plugins it lists. An address without a path is a
// site, so the usual catalog paths on it are tried in turn. A configured
// source keeps what was read in its cache.
func (mp *Marketplace) Probe(ctx context.Context, rawURL string) SourceProbe {
	var firstErr error
	for _, candidate := range catalogCandidates(rawURL) {
		entries, info, err := fetchCatalog(ctx, candidate)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		if slices.Contains(mp.Sources(), candidate) {
			mp.mu.Lock()
			mp.cache[candidate] = &sourceCache{entries: entries, info: info, fetched: time.Now()}
			delete(mp.failures, candidate)
			mp.mu.Unlock()
		}
		return SourceProbe{URL: candidate, Reachable: true, CatalogInfo: info, Plugins: len(entries)}
	}
	return SourceProbe{URL: rawURL, Error: firstErr.Error()}
}

// catalogPaths are where a site serves its catalog, in the order they are
// tried.
var catalogPaths = []string{"/v1/index.json", "/index.json"}

// catalogCandidates lists the addresses to read for a source address: the
// address itself, or the catalog paths on it when it names only a site.
func catalogCandidates(rawURL string) []string {
	parsed, err := url.Parse(rawURL)
	if err != nil || (parsed.Path != "" && parsed.Path != "/") || parsed.RawQuery != "" || parsed.Fragment != "" {
		return []string{rawURL}
	}
	base := strings.TrimSuffix(rawURL, "/")
	candidates := make([]string, 0, len(catalogPaths))
	for _, path := range catalogPaths {
		candidates = append(candidates, base+path)
	}
	return candidates
}

// Catalog fetches every configured source and merges them by id, first source
// wins. Each source is cached for an hour unless refresh is set, which also
// refreshes the partner keyring of the official source.
func (mp *Marketplace) Catalog(ctx context.Context, refresh bool) ([]CatalogEntry, error) {
	if refresh {
		mp.manager.refreshPartners(ctx)
	}
	sources := mp.Sources()

	// The sources are read at the same time, so a slow one does not hold up
	// the others, and merged in their order.
	type result struct {
		entries []CatalogEntry
		err     error
	}
	results := make([]result, len(sources))
	var wg sync.WaitGroup
	for i, source := range sources {
		wg.Go(func() {
			entries, err := mp.source(ctx, source, refresh)
			results[i] = result{entries: entries, err: err}
		})
	}
	wg.Wait()

	merged := make([]CatalogEntry, 0, 32)
	seen := make(map[string]struct{}, 32)

	var firstErr error
	for i, source := range sources {
		entries, err := results[i].entries, results[i].err
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			mp.manager.log.Warnf("Plugin marketplace source %s: %v", source, err)
			continue
		}
		for _, entry := range entries {
			if _, duplicate := seen[entry.ID]; duplicate {
				continue
			}
			seen[entry.ID] = struct{}{}
			entry.Source = source
			entry.Trust = effectiveTrust(entry.Trust)
			mp.decorate(&entry)
			merged = append(merged, entry)
		}
	}

	// Only a total outage is an error, a single broken mirror is not.
	if len(merged) == 0 && firstErr != nil {
		return nil, firstErr
	}
	sort.Slice(merged, func(i, j int) bool { return merged[i].ID < merged[j].ID })
	return merged, nil
}

// Search returns the merged catalog narrowed by the filter.
func (mp *Marketplace) Search(ctx context.Context, filter CatalogFilter) ([]CatalogEntry, error) {
	entries, err := mp.Catalog(ctx, filter.Refresh)
	if err != nil {
		return nil, err
	}

	keyword := strings.ToLower(strings.TrimSpace(filter.Keyword))
	result := make([]CatalogEntry, 0, len(entries))
	for _, entry := range entries {
		if filter.Source != "" && entry.Source != filter.Source {
			continue
		}
		if filter.Category != "" && !slices.Contains(entry.Categories, filter.Category) {
			continue
		}
		if keyword != "" && !entryMatches(&entry, keyword) {
			continue
		}
		result = append(result, entry)
	}
	return result, nil
}

// Detail returns one entry together with its rendered readme source. An empty
// source takes the first catalog that offers the plugin.
func (mp *Marketplace) Detail(ctx context.Context, id, source string) (*CatalogEntry, string, error) {
	entries, err := mp.Catalog(ctx, false)
	if err != nil {
		return nil, "", err
	}
	entry := findEntry(entries, id, source)
	if entry == nil {
		return nil, "", ErrMarketplaceNotFound
	}

	readme := ""
	if entry.ReadmeURL != "" {
		if err = checkCatalogURL(entry, entry.ReadmeURL); err != nil {
			mp.manager.log.Debugf("Skip plugin readme %s: %v", entry.ReadmeURL, err)
		} else if body, err := fetchText(ctx, proxiedURL(entry.ReadmeURL), maxReadmeBytes); err != nil {
			mp.manager.log.Warnf("Plugin readme %s: %v", entry.ReadmeURL, err)
		} else {
			readme = body
		}
	}
	return entry, readme, nil
}

// checkCatalogURL decides whether the host may fetch, or let a browser load,
// a URL of an entry, such as its readme or a screenshot. The URL comes from
// the catalog, so it must use https, or http when insecure downloads are
// allowed, and live on the catalog source host, the host of the package this
// node installs, or GitHub.
func checkCatalogURL(entry *CatalogEntry, raw string) error {
	allowed := []string{entry.Source}
	if release := entry.InstallableRelease; release != nil {
		download, _, ok := release.DownloadFor(HostPlatform())
		if !ok {
			download.URL = release.DownloadURL
		}
		allowed = append(allowed, download.URL)
	}
	return checkURLOnHosts(raw, allowed...)
}

// checkURLOnHosts accepts an https URL, or http when insecure downloads are
// allowed, on GitHub or on the host of one of the allowed URLs.
func checkURLOnHosts(raw string, allowed ...string) error {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" {
		return fmt.Errorf("%q is not a valid url", raw)
	}
	switch parsed.Scheme {
	case "https":
	case "http":
		if !settings.PluginSettings.AllowInsecureDownloadURL {
			return errors.New("plain http is not allowed")
		}
	default:
		return fmt.Errorf("scheme %q is not allowed", parsed.Scheme)
	}

	host := strings.ToLower(parsed.Host)
	if slices.Contains(githubHosts, host) {
		return nil
	}
	for _, raw := range allowed {
		if other, err := url.Parse(raw); err == nil && other.Host != "" && strings.ToLower(other.Host) == host {
			return nil
		}
	}
	return fmt.Errorf("host %s is not the catalog, package or GitHub host", parsed.Host)
}

// Install downloads, verifies and installs one catalog release together with
// the dependencies its manifest declares.
func (mp *Marketplace) Install(ctx context.Context, id, wantVersion, source string, opts InstallOptions) (*Info, error) {
	if !settings.PluginSettings.Enabled {
		return nil, ErrPluginsDisabled
	}
	if !settings.PluginSettings.MarketplaceEnabled {
		return nil, ErrMarketplaceDisabled
	}
	if !IsValidID(id) {
		return nil, ErrPluginNotFound
	}

	mp.installMu.Lock()
	defer mp.installMu.Unlock()

	entries, err := mp.Catalog(ctx, false)
	if err != nil {
		return nil, err
	}

	info, err := mp.installEntry(ctx, entries, id, wantVersion, source, opts, map[string]bool{})
	if err != nil {
		publishInstallProgress(id, InstallStatusError, 100, err.Error())
		return nil, err
	}
	publishInstallProgress(id, InstallStatusDone, 100, "")
	return info, nil
}

// Updates lists the installed plugins the catalog offers a newer release for.
func (mp *Marketplace) Updates(ctx context.Context) ([]UpdateInfo, error) {
	entries, err := mp.Catalog(ctx, false)
	if err != nil {
		return nil, err
	}

	installed := mp.manager.List()
	byID := make(map[string]*CatalogEntry, len(entries))
	for i := range entries {
		byID[entries[i].ID] = &entries[i]
	}

	updates := make([]UpdateInfo, 0, len(installed))
	for _, item := range installed {
		entry, ok := byID[item.ID]
		if !ok || entry.InstallableRelease == nil {
			continue
		}
		if CompareVersions(item.Version, entry.InstallableRelease.Version) >= 0 {
			continue
		}
		updates = append(updates, UpdateInfo{
			ID:                 item.ID,
			Name:               localizedName(entry, item.ID),
			InstalledVersion:   item.Version,
			LatestVersion:      entry.InstallableRelease.Version,
			Source:             entry.Source,
			Trust:              entry.Trust,
			InstalledYanked:    isVersionYanked(entry, item.Version),
			PermissionsChanged: mp.permissionsChanged(item.ID, entry.InstallableRelease),
			Release:            entry.InstallableRelease,
		})
	}
	return updates, nil
}

// Update upgrades one installed plugin, keeping its enabled state and its
// stored settings.
func (mp *Marketplace) Update(ctx context.Context, id, wantVersion string, approve bool) (*Info, error) {
	return mp.update(ctx, id, wantVersion, approve, "")
}

// update is Update with a floor for the trust of the new package, which the
// automatic flows set.
func (mp *Marketplace) update(ctx context.Context, id, wantVersion string, approve bool, minTrust string) (*Info, error) {
	current, err := mp.manager.Get(id)
	if err != nil {
		return nil, err
	}
	return mp.Install(ctx, id, wantVersion, "", InstallOptions{
		Enable:             current.Enabled,
		ApprovePermissions: approve,
		MinTrust:           minTrust,
	})
}

// installEntry resolves one entry, installs whatever it requires and then the
// entry itself. seen keeps a dependency cycle from recursing forever.
func (mp *Marketplace) installEntry(ctx context.Context, entries []CatalogEntry,
	id, wantVersion, source string, opts InstallOptions, seen map[string]bool,
) (*Info, error) {
	if seen[id] {
		return nil, ErrDependencyCycle
	}
	seen[id] = true

	entry := findEntry(entries, id, source)
	if entry == nil {
		return nil, ErrMarketplaceNotFound
	}
	platform := HostPlatform()
	release, err := mp.resolveRelease(entry, wantVersion, platform)
	if err != nil {
		return nil, err
	}
	download, key, ok := release.DownloadFor(platform)
	if !ok {
		return nil, ErrPlatformUnsupported
	}
	if err = mp.checkPolicy(entry, download.URL); err != nil {
		return nil, err
	}

	if err = mp.installRequirements(ctx, entries, release, opts.MinTrust, seen); err != nil {
		return nil, err
	}

	staging, err := os.MkdirTemp("", "nginx-ui-plugin-fetch-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(staging)

	archive := filepath.Join(staging, PackageFileName(entry.ID, release.Version, key))
	progress := func(status string, percent float64) {
		publishPlatformProgress(entry.ID, status, percent, key)
	}
	if err = mp.download(ctx, proxiedURL(download.URL), archive, progress); err != nil {
		return nil, err
	}

	progress(InstallStatusVerifying, 90)
	if err = verifyDigest(archive, download.SHA256); err != nil {
		return nil, err
	}

	progress(InstallStatusInstalling, 95)
	// The package must be what the catalog promised, otherwise a compromised
	// mirror could swap a well known id for something else. The install
	// checks it, and the embedded signature, on the one extraction it needs.
	opts.ExpectedID = entry.ID
	opts.ExpectedVersion = release.Version
	opts.AuthorPublicKey = entry.AuthorPublicKey
	opts.Channel = release.Channel
	return mp.manager.Install(ctx, archive, opts)
}

// installRequirements installs every dependency the manifest declares that is
// not already satisfied on this node, held to the same trust floor.
func (mp *Marketplace) installRequirements(ctx context.Context, entries []CatalogEntry,
	release *CatalogRelease, minTrust string, seen map[string]bool,
) error {
	if release.Manifest == nil {
		return nil
	}
	for _, requirement := range release.Manifest.Requires {
		if mp.requirementSatisfied(requirement) {
			continue
		}
		dependency := findEntry(entries, requirement.ID, "")
		if dependency == nil {
			return cosy.WrapErrorWithParams(ErrDependencyMissing, requirement.ID)
		}
		candidate := pickRelease(dependency, requirement.Version, HostPlatform(), mp.followedChannel(dependency.ID))
		if candidate == nil {
			return cosy.WrapErrorWithParams(ErrDependencyMissing, requirement.ID+"@"+requirement.Version)
		}
		if _, err := mp.installEntry(ctx, entries, dependency.ID, candidate.Version, dependency.Source,
			InstallOptions{Enable: true, ApprovePermissions: true, MinTrust: minTrust}, seen); err != nil {
			return err
		}
	}
	return nil
}

// requirementSatisfied reports whether the installed inventory already covers
// one dependency.
func (mp *Marketplace) requirementSatisfied(requirement protocol.ManifestRequirement) bool {
	info, err := mp.manager.Get(requirement.ID)
	if err != nil || info.Status == StatusMissing {
		return false
	}
	return requirement.Version == "" || VersionSatisfies(info.Version, requirement.Version)
}

// resolveRelease picks the requested version, or the newest one that installs
// on platform.
func (mp *Marketplace) resolveRelease(entry *CatalogEntry, wantVersion, platform string) (*CatalogRelease, error) {
	if wantVersion == "" {
		if platform == HostPlatform() {
			if entry.InstallableRelease == nil {
				return nil, ErrPlatformUnsupported
			}
			return entry.InstallableRelease, nil
		}
		if release := pickRelease(entry, "", platform, mp.followedChannel(entry.ID)); release != nil {
			return release, nil
		}
		return nil, ErrPlatformUnsupported
	}
	for i := range entry.Releases {
		release := &entry.Releases[i]
		if release.Version != wantVersion {
			continue
		}
		if release.Yanked {
			return nil, ErrReleaseYanked
		}
		if !releaseRunsOn(release, platform) {
			return nil, ErrPlatformUnsupported
		}
		return release, nil
	}
	return nil, ErrReleaseNotFound
}

// checkPolicy applies the node wide install policy to the package a release
// resolved to. The community gate is only a pre filter on the catalog claim,
// the install applies it again to the trust the signature proves.
func (mp *Marketplace) checkPolicy(entry *CatalogEntry, downloadURL string) error {
	if entry.Trust == TrustCommunity && !settings.PluginSettings.AllowCommunityPlugins {
		return ErrCommunityNotAllowed
	}
	parsed, err := url.Parse(downloadURL)
	if err != nil || parsed.Host == "" {
		return cosy.WrapErrorWithParams(ErrCatalogInvalid, "download url is not a valid url")
	}
	if parsed.Scheme != "https" && !settings.PluginSettings.AllowInsecureDownloadURL {
		return ErrInsecureURL
	}
	return nil
}

// download streams a package to disk. progress, when set, receives the
// downloading phase so the caller can publish it.
func (mp *Marketplace) download(ctx context.Context, rawURL, target string, progress func(status string, percent float64)) error {
	if progress == nil {
		progress = func(string, float64) {}
	}
	progress(InstallStatusDownloading, 0)

	client, err := newHTTPClient(downloadTimeout)
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return err
	}
	response, err := client.Do(request)
	if err != nil {
		return cosy.WrapErrorWithParams(ErrSourceUnavailable, err.Error())
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return cosy.WrapErrorWithParams(ErrSourceUnavailable, response.Status)
	}

	file, err := os.Create(target)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := &progressWriter{
		writer:   file,
		total:    response.ContentLength,
		progress: progress,
	}
	if _, err = io.Copy(writer, io.LimitReader(response.Body, int64(MaxPackageSize)+1)); err != nil {
		return err
	}
	if writer.written > int64(MaxPackageSize) {
		return ErrPackageTooLarge
	}
	return nil
}

// permissionsChanged reports whether a release asks for more than the user
// approved for the installed version.
func (mp *Marketplace) permissionsChanged(id string, release *CatalogRelease) bool {
	if release == nil || release.Manifest == nil {
		// Without a manifest snapshot the change cannot be ruled out.
		return true
	}
	approved, ok := mp.manager.approvedPermissions(id)
	if !ok {
		return false
	}
	return len(unapprovedPermissions(approved, release.Manifest)) > 0
}

// source returns one catalog source, using the memory cache unless refresh.
func (mp *Marketplace) source(ctx context.Context, rawURL string, refresh bool) ([]CatalogEntry, error) {
	if !refresh {
		mp.mu.Lock()
		cached, ok := mp.cache[rawURL]
		mp.mu.Unlock()
		if ok && time.Since(cached.fetched) < catalogTTL {
			return cached.entries, nil
		}
		mp.mu.Lock()
		failure, failed := mp.failures[rawURL]
		mp.mu.Unlock()
		if failed && time.Since(failure.failed) < catalogRetryAfter {
			return nil, failure.err
		}
	}

	entries, info, err := fetchCatalog(ctx, rawURL)
	if err != nil {
		// A cancelled request says nothing about the source.
		if ctx.Err() == nil {
			mp.mu.Lock()
			mp.failures[rawURL] = sourceFailure{err: err, failed: time.Now()}
			mp.mu.Unlock()
		}
		return nil, err
	}

	mp.mu.Lock()
	mp.cache[rawURL] = &sourceCache{entries: entries, info: info, fetched: time.Now()}
	delete(mp.failures, rawURL)
	mp.mu.Unlock()
	return entries, nil
}

// ClearCache drops every cached source, which the tests and the settings
// endpoint rely on.
func (mp *Marketplace) ClearCache() {
	mp.mu.Lock()
	mp.cache = map[string]*sourceCache{}
	mp.failures = map[string]sourceFailure{}
	mp.mu.Unlock()
}

// decorate computes the node specific fields of one entry.
func (mp *Marketplace) decorate(entry *CatalogEntry) {
	entry.InstalledVersion = mp.installedVersion(entry.ID)
	entry.InstallableRelease = pickRelease(entry, "", HostPlatform(), mp.followedChannel(entry.ID))
	entry.InstallableVersions = installableVersions(entry, HostPlatform())
	entry.Channel = ChannelStable
	if entry.Stage == StageBeta {
		entry.Channel = ChannelBeta
	}
	if entry.InstallableRelease != nil {
		entry.Channel = lessStableChannel(entry.Channel, channelOfRelease(entry.InstallableRelease))
	}
	if entry.InstallableRelease != nil && entry.InstalledVersion != "" {
		entry.UpdateAvailable = CompareVersions(entry.InstalledVersion, entry.InstallableRelease.Version) < 0
	}
	entry.Screenshots = loadableScreenshots(entry)
	if entry.IconURL != "" && checkCatalogURL(entry, entry.IconURL) != nil {
		entry.IconURL = ""
	}
}

// loadableScreenshots keeps the screenshots a browser of this node may load:
// the ones checkCatalogURL accepts, at most maxCatalogScreenshots. A dark
// variant it does not accept is dropped and the screenshot keeps its light
// image. The entry may be shared with the source cache, so the list is a new
// one.
func loadableScreenshots(entry *CatalogEntry) []CatalogScreenshot {
	var kept []CatalogScreenshot
	for _, shot := range entry.Screenshots {
		if len(kept) == maxCatalogScreenshots {
			break
		}
		if err := checkCatalogURL(entry, shot.URL); err != nil {
			continue
		}
		if shot.DarkURL != "" && checkCatalogURL(entry, shot.DarkURL) != nil {
			shot.DarkURL = ""
		}
		kept = append(kept, shot)
	}
	return kept
}

// installedVersion is the version of the plugin on this node, empty when it
// is not installed.
func (mp *Marketplace) installedVersion(id string) string {
	if info, err := mp.manager.Get(id); err == nil {
		return info.Version
	}
	return ""
}

// followedChannel is the channel an installed plugin takes updates from: the
// less stable of the one the person chose and the one of the release it runs.
// Empty when the plugin is not installed.
func (mp *Marketplace) followedChannel(id string) string {
	info, err := mp.manager.Get(id)
	if err != nil {
		return ""
	}
	return info.EffectiveChannel
}

// runMaintenance is the daily job: refresh the catalog and the partner
// keyring, tell the user about pending updates and apply the automatic ones.
func (mp *Marketplace) runMaintenance(ctx context.Context) {
	if !settings.PluginSettings.Enabled || !settings.PluginSettings.MarketplaceEnabled {
		return
	}
	if _, err := mp.Catalog(ctx, true); err != nil {
		mp.manager.log.Warnf("Plugin catalog refresh: %v", err)
		return
	}

	updates, err := mp.Updates(ctx)
	if err != nil {
		mp.manager.log.Warnf("Plugin updates: %v", err)
		return
	}
	if len(updates) == 0 {
		return
	}

	names := make([]string, 0, len(updates))
	for _, update := range updates {
		names = append(names, fmt.Sprintf("%s %s -> %s", update.Name, update.InstalledVersion, update.LatestVersion))
	}
	notification.Info("Plugin updates available", strings.Join(names, ", "), map[string]any{"updates": updates})

	if !settings.PluginSettings.AutoUpdate {
		return
	}
	for _, update := range updates {
		// Only plugins installed from an official or verified signature update
		// themselves, never to a package signed with less, and only while the
		// permission set the user approved still covers the new version.
		installed, err := mp.manager.Get(update.ID)
		if err != nil || (installed.Trust != TrustOfficial && installed.Trust != TrustVerified) {
			continue
		}
		if update.PermissionsChanged {
			continue
		}
		if _, err = mp.update(ctx, update.ID, update.LatestVersion, false, installed.Trust); err != nil {
			mp.manager.log.Warnf("[plugin:%s] auto update: %v", update.ID, err)
		}
	}
}

// progressWriter reports download progress, throttled so a fast link does
// not flood the event bus.
type progressWriter struct {
	writer   io.Writer
	total    int64
	written  int64
	reported float64
	progress func(status string, percent float64)
}

func (p *progressWriter) Write(chunk []byte) (int, error) {
	n, err := p.writer.Write(chunk)
	p.written += int64(n)
	if p.total <= 0 {
		return n, err
	}
	// The download is the first 85% of the install, the rest is verification.
	percent := float64(p.written) / float64(p.total) * 85
	if percent-p.reported >= progressStep || p.written >= p.total {
		p.reported = percent
		p.progress(InstallStatusDownloading, percent)
	}
	return n, err
}

// publishInstallProgress pushes one progress frame to the websocket clients.
func publishInstallProgress(pluginID, status string, progress float64, message string) {
	event.Publish(event.Event{
		Type: EventTypeInstallProgress,
		Data: InstallProgress{
			PluginID: pluginID,
			Status:   status,
			Progress: progress,
			Message:  message,
		},
	})
}

// publishPlatformProgress is publishInstallProgress for the phases that know
// which package of the release is being installed.
func publishPlatformProgress(pluginID, status string, progress float64, platform string) {
	event.Publish(event.Event{
		Type: EventTypeInstallProgress,
		Data: InstallProgress{
			PluginID: pluginID,
			Status:   status,
			Progress: progress,
			Platform: platform,
		},
	})
}

// fetchCatalog downloads and validates the catalog document of one source.
func fetchCatalog(ctx context.Context, source string) ([]CatalogEntry, CatalogInfo, error) {
	body, err := fetchText(ctx, proxiedURL(source), maxCatalogBytes)
	if err != nil {
		return nil, CatalogInfo{}, err
	}

	var document CatalogDocument
	if err = json.Unmarshal([]byte(body), &document); err != nil {
		return nil, CatalogInfo{}, cosy.WrapErrorWithParams(ErrCatalogInvalid, err.Error())
	}
	if document.SchemaVersion != CatalogSchemaVersion {
		return nil, CatalogInfo{}, cosy.WrapErrorWithParams(ErrCatalogInvalid,
			fmt.Sprintf("schema_version %d is not supported", document.SchemaVersion))
	}

	entries := make([]CatalogEntry, 0, len(document.Plugins))
	for _, entry := range document.Plugins {
		// A malformed id would end up as a directory name, drop it early.
		if !IsValidID(entry.ID) {
			continue
		}
		for i := range entry.Releases {
			entry.Releases[i].Channel = channelOfRelease(&entry.Releases[i])
		}
		entries = append(entries, entry)
	}
	info := CatalogInfo{Name: catalogName(document.Name)}
	if document.Icon != "" && checkURLOnHosts(document.Icon, source) == nil {
		info.Icon = document.Icon
	}
	return entries, info, nil
}

// catalogName keeps the non-empty names of a catalog, short enough to show.
func catalogName(raw map[string]string) map[string]string {
	name := make(map[string]string, len(raw))
	for locale, value := range raw {
		value = strings.Join(strings.Fields(value), " ")
		if locale == "" || value == "" {
			continue
		}
		if runes := []rune(value); len(runes) > maxCatalogNameLength {
			value = string(runes[:maxCatalogNameLength])
		}
		name[locale] = value
	}
	if len(name) == 0 {
		return nil
	}
	return name
}

// fetchText reads a remote document with a hard size cap.
func fetchText(ctx context.Context, rawURL string, limit int64) (string, error) {
	if rawURL == "" {
		return "", ErrSourceUnavailable
	}
	client, err := newHTTPClient(catalogHTTPTimeout)
	if err != nil {
		return "", err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", cosy.WrapErrorWithParams(ErrSourceUnavailable, err.Error())
	}
	response, err := client.Do(request)
	if err != nil {
		return "", cosy.WrapErrorWithParams(ErrSourceUnavailable, err.Error())
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", cosy.WrapErrorWithParams(ErrSourceUnavailable, response.Status)
	}

	body, err := io.ReadAll(io.LimitReader(response.Body, limit))
	if err != nil {
		return "", cosy.WrapErrorWithParams(ErrSourceUnavailable, err.Error())
	}
	return string(body), nil
}

// newHTTPClient reuses the proxy aware client of the updater and adds a
// timeout, so a stalled mirror cannot pin a goroutine forever.
func newHTTPClient(timeout time.Duration) (*http.Client, error) {
	base, err := version.NewHTTPClient()
	if err != nil {
		return nil, err
	}
	client := *base
	client.Timeout = timeout
	return &client, nil
}

// proxiedURL routes GitHub hosted files through the configured proxy, exactly
// like the core updater does for the release API.
func proxiedURL(rawURL string) string {
	if rawURL == "" {
		return ""
	}
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Scheme == "" {
		return rawURL
	}
	if !slices.Contains(githubHosts, strings.ToLower(parsed.Hostname())) {
		return rawURL
	}
	return version.GetUrl(rawURL)
}

// effectiveTrust normalises the trust level a catalog entry declares. It is
// only shown in the listing, the install derives the level from the package
// signature. An unknown level is community, which keeps the community pre
// filter in front of it.
func effectiveTrust(claimed string) string {
	switch claimed {
	case TrustOfficial, TrustVerified, TrustCommunity:
		return claimed
	default:
		return TrustCommunity
	}
}

// verifyDigest checks the sha256 the catalog promised, when it promised one.
func verifyDigest(path, expected string) error {
	expected = strings.TrimSpace(strings.ToLower(expected))
	if expected == "" {
		return nil
	}
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	digest := sha256.New()
	if _, err = io.Copy(digest, file); err != nil {
		return err
	}
	if hex.EncodeToString(digest.Sum(nil)) != expected {
		return ErrDigestMismatch
	}
	return nil
}

// findEntry looks one entry up, optionally pinned to a source.
func findEntry(entries []CatalogEntry, id, source string) *CatalogEntry {
	for i := range entries {
		if entries[i].ID != id {
			continue
		}
		if source != "" && entries[i].Source != source {
			continue
		}
		return &entries[i]
	}
	return nil
}

// pickRelease returns the newest release that installs on platform,
// optionally restricted to a semver range. followed is the channel of an
// installed plugin, and only releases on it or a more stable one qualify. It
// is empty when the plugin is not installed: the newest stable release is
// chosen, else the newest beta, else the newest dev one.
func pickRelease(entry *CatalogEntry, versionRange, platform, followed string) *CatalogRelease {
	var byRank [3]*CatalogRelease
	for i := range entry.Releases {
		release := &entry.Releases[i]
		if release.Yanked || !releaseRunsOn(release, platform) {
			continue
		}
		if versionRange != "" && !VersionSatisfies(release.Version, versionRange) {
			continue
		}
		rank := ChannelRank(channelOfRelease(release))
		if byRank[rank] == nil || CompareVersions(release.Version, byRank[rank].Version) > 0 {
			byRank[rank] = release
		}
	}

	if followed == "" {
		for _, release := range byRank {
			if release != nil {
				return release
			}
		}
		return nil
	}
	var best *CatalogRelease
	for rank, release := range byRank {
		if rank > ChannelRank(followed) || release == nil {
			continue
		}
		if best == nil || CompareVersions(release.Version, best.Version) > 0 {
			best = release
		}
	}
	return best
}

// installableVersions lists the versions of an entry that install on
// platform, newest first, without the withdrawn ones.
func installableVersions(entry *CatalogEntry, platform string) []string {
	versions := make([]string, 0, len(entry.Releases))
	for i := range entry.Releases {
		if release := &entry.Releases[i]; !release.Yanked && releaseRunsOn(release, platform) {
			versions = append(versions, release.Version)
		}
	}
	sort.SliceStable(versions, func(a, b int) bool { return CompareVersions(versions[a], versions[b]) > 0 })
	return versions
}

// releaseRunsOn reports whether a node running this nginx-ui build on
// platform can install a release: the api version matches, the host is recent
// enough and the release ships a package for the platform.
func releaseRunsOn(release *CatalogRelease, platform string) bool {
	if release.APIVersion != protocol.APIVersion {
		return false
	}
	if release.MinNginxUIVersion != "" {
		current := version.GetVersionInfo().Version
		// A development build carries no version, do not block it.
		if current != "" && CompareVersions(current, release.MinNginxUIVersion) < 0 {
			return false
		}
	}
	_, _, ok := release.DownloadFor(platform)
	return ok
}

// isVersionYanked reports whether the catalog withdrew a version.
func isVersionYanked(entry *CatalogEntry, wantVersion string) bool {
	for i := range entry.Releases {
		if entry.Releases[i].Version == wantVersion {
			return entry.Releases[i].Yanked
		}
	}
	return false
}

// entryMatches implements the keyword filter over the id and every localized
// name and description.
func entryMatches(entry *CatalogEntry, keyword string) bool {
	if strings.Contains(strings.ToLower(entry.ID), keyword) {
		return true
	}
	if strings.Contains(strings.ToLower(entry.Author), keyword) {
		return true
	}
	for _, values := range []map[string]string{entry.Name, entry.Description} {
		for _, value := range values {
			if strings.Contains(strings.ToLower(value), keyword) {
				return true
			}
		}
	}
	return false
}

// localizedName prefers English and falls back to any language, then the id.
func localizedName(entry *CatalogEntry, fallback string) string {
	if name, ok := entry.Name["en"]; ok && name != "" {
		return name
	}
	for _, name := range entry.Name {
		if name != "" {
			return name
		}
	}
	return fallback
}
