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
	"github.com/0xJacky/Nginx-UI/internal/pkgsign"
	"github.com/0xJacky/Nginx-UI/internal/plugin/protocol"
	"github.com/0xJacky/Nginx-UI/internal/releasesign"
	"github.com/0xJacky/Nginx-UI/internal/version"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/uozi-tech/cosy"
)

// Trust levels a catalog entry can declare.
const (
	TrustOfficial  = "official"
	TrustVerified  = "verified"
	TrustCommunity = "community"
)

// SignedByAuthor marks a release signed with the author key instead of the
// release key pinned in this binary.
const SignedByAuthor = "author"

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

// CatalogSchemaVersion is the only catalog layout this node understands.
const CatalogSchemaVersion = 1

const (
	// catalogTTL is how long a fetched source is reused without refreshing.
	catalogTTL = time.Hour
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
	URL          string `json:"url"`
	SHA256       string `json:"sha256,omitempty"`
	SignatureURL string `json:"signature_url,omitempty"`
}

// signatureURL is the detached signature of the package, which defaults to
// the package URL plus ".minisig".
func (d ReleaseDownload) signatureURL() string {
	if d.SignatureURL != "" || d.URL == "" {
		return d.SignatureURL
	}
	return d.URL + signatureSuffix
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
	// DownloadURL, SHA256 and SignatureURL describe the portable package, the
	// fallback for every platform Downloads does not name.
	DownloadURL     string             `json:"download_url"`
	SHA256          string             `json:"sha256,omitempty"`
	SignatureURL    string             `json:"signature_url,omitempty"`
	SignedBy        string             `json:"signed_by,omitempty"`
	ReleaseNotesURL string             `json:"release_notes_url,omitempty"`
	Yanked          bool               `json:"yanked,omitempty"`
	Manifest        *protocol.Manifest `json:"manifest,omitempty"`
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
	return ReleaseDownload{URL: r.DownloadURL, SHA256: r.SHA256, SignatureURL: r.SignatureURL}, "", true
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
// node computed for it.
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
	Categories      []string          `json:"categories,omitempty"`
	Capabilities    []string          `json:"capabilities,omitempty"`
	License         string            `json:"license,omitempty"`
	Trust           string            `json:"trust,omitempty"`
	Stage           string            `json:"stage,omitempty"`
	Releases        []CatalogRelease  `json:"releases"`

	// Source is the catalog URL this entry was merged from.
	Source string `json:"source"`
	// InstallableRelease is the newest release this node can actually install.
	InstallableRelease *CatalogRelease `json:"installable_release,omitempty"`
	// InstalledVersion is the version currently on this node, if any.
	InstalledVersion string `json:"installed_version,omitempty"`
	UpdateAvailable  bool   `json:"update_available"`
}

// CatalogDocument is the static JSON one source serves.
type CatalogDocument struct {
	SchemaVersion int            `json:"schema_version"`
	UpdatedAt     string         `json:"updated_at,omitempty"`
	Plugins       []CatalogEntry `json:"plugins"`
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
	fetched time.Time
}

// Marketplace fetches the plugin catalogs and installs from them. It is owned
// by the manager, see manager_marketplace.go.
type Marketplace struct {
	manager *Manager

	mu    sync.Mutex
	cache map[string]*sourceCache

	// installMu serialises whole install flows, including their dependencies.
	installMu sync.Mutex
}

func newMarketplace(m *Manager) *Marketplace {
	return &Marketplace{manager: m, cache: map[string]*sourceCache{}}
}

// Sources lists the configured catalog URLs in merge order.
func (mp *Marketplace) Sources() []string {
	return settings.PluginSettings.GetMarketplaceSources()
}

// Catalog fetches every configured source and merges them by id, first source
// wins. Each source is cached for an hour unless refresh is set.
func (mp *Marketplace) Catalog(ctx context.Context, refresh bool) ([]CatalogEntry, error) {
	sources := mp.Sources()
	merged := make([]CatalogEntry, 0, 32)
	seen := make(map[string]struct{}, 32)

	var firstErr error
	for _, source := range sources {
		entries, err := mp.source(ctx, source, refresh)
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
			entry.Trust = effectiveTrust(source, entry.Trust)
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
		if err = checkReadmeURL(entry); err != nil {
			mp.manager.log.Debugf("Skip plugin readme %s: %v", entry.ReadmeURL, err)
		} else if body, err := fetchText(ctx, proxiedURL(entry.ReadmeURL), maxReadmeBytes); err != nil {
			mp.manager.log.Warnf("Plugin readme %s: %v", entry.ReadmeURL, err)
		} else {
			readme = body
		}
	}
	return entry, readme, nil
}

// checkReadmeURL decides whether the host may fetch the readme of an entry.
// The URL comes from the catalog, so it must use https, or http when insecure
// downloads are allowed, and live on the catalog source host, the host of the
// package this node installs, or GitHub.
func checkReadmeURL(entry *CatalogEntry) error {
	parsed, err := url.Parse(entry.ReadmeURL)
	if err != nil || parsed.Host == "" {
		return fmt.Errorf("%q is not a valid url", entry.ReadmeURL)
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
	allowed := []string{entry.Source}
	if release := entry.InstallableRelease; release != nil {
		download, _, ok := release.DownloadFor(HostPlatform())
		if !ok {
			download.URL = release.DownloadURL
		}
		allowed = append(allowed, download.URL)
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
	current, err := mp.manager.Get(id)
	if err != nil {
		return nil, err
	}
	return mp.Install(ctx, id, wantVersion, "", InstallOptions{
		Enable:             current.Enabled,
		ApprovePermissions: approve,
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

	if err = mp.installRequirements(ctx, entries, release, seen); err != nil {
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
	if _, err = mp.verifySignature(ctx, entry, release, download, archive); err != nil {
		return nil, err
	}

	progress(InstallStatusInstalling, 95)
	// The package must be what the catalog promised, otherwise a compromised
	// mirror could swap a well known id for something else. The install
	// checks it on the one extraction it needs anyway.
	opts.ExpectedID = entry.ID
	opts.ExpectedVersion = release.Version
	return mp.manager.Install(ctx, archive, opts)
}

// installRequirements installs every dependency the manifest declares that is
// not already satisfied on this node.
func (mp *Marketplace) installRequirements(ctx context.Context, entries []CatalogEntry,
	release *CatalogRelease, seen map[string]bool,
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
		candidate := pickRelease(dependency, requirement.Version, HostPlatform())
		if candidate == nil {
			return cosy.WrapErrorWithParams(ErrDependencyMissing, requirement.ID+"@"+requirement.Version)
		}
		if _, err := mp.installEntry(ctx, entries, dependency.ID, candidate.Version, dependency.Source,
			InstallOptions{Enable: true, ApprovePermissions: true}, seen); err != nil {
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
		if release := pickRelease(entry, "", platform); release != nil {
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
// resolved to.
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

// verifySignature enforces the signature policy on the downloaded package: the
// official source always requires one, a custom source only when
// RequireSignature is set. It returns the verified signature, empty when an
// unsigned package was accepted.
func (mp *Marketplace) verifySignature(ctx context.Context, entry *CatalogEntry, release *CatalogRelease,
	download ReleaseDownload, archive string,
) (string, error) {
	required := isOfficialSource(entry.Source) || settings.PluginSettings.RequireSignature
	signature, err := fetchText(ctx, proxiedURL(download.signatureURL()), maxReadmeBytes)
	if err != nil || strings.TrimSpace(signature) == "" {
		if required {
			return "", ErrSignatureMissing
		}
		mp.manager.log.Warnf("[plugin:%s] accepting %s without a signature", entry.ID, release.Version)
		return "", nil
	}

	if _, err = pkgsign.VerifyFile(archive, []byte(signature), mp.trustedKeys(entry, release)); err != nil {
		return "", cosy.WrapErrorWithParams(ErrSignatureInvalid, err.Error())
	}
	return signature, nil
}

// trustedKeys is the release key set accepted for one entry. A community
// plugin may additionally be signed with the author key the catalog pins.
func (mp *Marketplace) trustedKeys(entry *CatalogEntry, release *CatalogRelease) []string {
	keys := releasesign.TrustedPublicKeys()
	keys = append(keys, settings.PluginSettings.TrustedPublicKeys...)
	if release.SignedBy == SignedByAuthor && entry.Trust == TrustCommunity && entry.AuthorPublicKey != "" {
		keys = append(keys, entry.AuthorPublicKey)
	}
	return keys
}

// permissionsChanged reports whether a release asks for more than the user
// approved for the installed version.
func (mp *Marketplace) permissionsChanged(id string, release *CatalogRelease) bool {
	if release == nil || release.Manifest == nil {
		// Without a manifest snapshot the change cannot be ruled out.
		return true
	}
	approved, ok := mp.manager.approvedPermissionsHash(id)
	if !ok {
		return false
	}
	if approved == "" {
		return len(release.Manifest.Permissions) > 0
	}
	return approved != PermissionsHash(release.Manifest)
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
	}

	entries, err := fetchCatalog(ctx, proxiedURL(rawURL))
	if err != nil {
		return nil, err
	}

	mp.mu.Lock()
	mp.cache[rawURL] = &sourceCache{entries: entries, fetched: time.Now()}
	mp.mu.Unlock()
	return entries, nil
}

// ClearCache drops every cached source, which the tests and the settings
// endpoint rely on.
func (mp *Marketplace) ClearCache() {
	mp.mu.Lock()
	mp.cache = map[string]*sourceCache{}
	mp.mu.Unlock()
}

// decorate computes the node specific fields of one entry.
func (mp *Marketplace) decorate(entry *CatalogEntry) {
	entry.InstallableRelease = pickRelease(entry, "", HostPlatform())
	if info, err := mp.manager.Get(entry.ID); err == nil {
		entry.InstalledVersion = info.Version
	}
	if entry.InstallableRelease != nil && entry.InstalledVersion != "" {
		entry.UpdateAvailable = CompareVersions(entry.InstalledVersion, entry.InstallableRelease.Version) < 0
	}
}

// runMaintenance is the daily job: refresh the catalog, tell the user about
// pending updates and apply the automatic ones.
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
		// Only vetted plugins update themselves, and only while the permission
		// set the user approved still covers the new version.
		if update.Trust != TrustOfficial && update.Trust != TrustVerified {
			continue
		}
		if update.PermissionsChanged {
			continue
		}
		if _, err = mp.Update(ctx, update.ID, update.LatestVersion, false); err != nil {
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

// fetchCatalog downloads and validates one catalog document.
func fetchCatalog(ctx context.Context, rawURL string) ([]CatalogEntry, error) {
	body, err := fetchText(ctx, rawURL, maxCatalogBytes)
	if err != nil {
		return nil, err
	}

	var document CatalogDocument
	if err = json.Unmarshal([]byte(body), &document); err != nil {
		return nil, cosy.WrapErrorWithParams(ErrCatalogInvalid, err.Error())
	}
	if document.SchemaVersion != CatalogSchemaVersion {
		return nil, cosy.WrapErrorWithParams(ErrCatalogInvalid,
			fmt.Sprintf("schema_version %d is not supported", document.SchemaVersion))
	}

	entries := make([]CatalogEntry, 0, len(document.Plugins))
	for _, entry := range document.Plugins {
		// A malformed id would end up as a directory name, drop it early.
		if !IsValidID(entry.ID) {
			continue
		}
		entries = append(entries, entry)
	}
	return entries, nil
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

// isOfficialSource reports whether a source URL is the catalog shipped with
// this binary, which always requires signed packages.
func isOfficialSource(source string) bool {
	return strings.EqualFold(strings.TrimSpace(source), settings.DefaultPluginMarketplaceSource)
}

// effectiveTrust is the trust level this node grants an entry. The level is a
// claim about who published the plugin and the release signature is its
// proof, so a custom source whose packages are not held to the signature
// policy cannot vouch for more than community. An unknown level is community
// as well, which keeps the community policy gate in front of it.
func effectiveTrust(source, claimed string) string {
	switch claimed {
	case TrustOfficial, TrustVerified:
	default:
		return TrustCommunity
	}
	if isOfficialSource(source) || settings.PluginSettings.RequireSignature {
		return claimed
	}
	return TrustCommunity
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
// optionally restricted to a semver range.
func pickRelease(entry *CatalogEntry, versionRange, platform string) *CatalogRelease {
	var best *CatalogRelease
	for i := range entry.Releases {
		release := &entry.Releases[i]
		if release.Yanked || !releaseRunsOn(release, platform) {
			continue
		}
		if versionRange != "" && !VersionSatisfies(release.Version, versionRange) {
			continue
		}
		if best == nil || CompareVersions(release.Version, best.Version) > 0 {
			best = release
		}
	}
	return best
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
