package plugin

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
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

// CatalogRelease is one downloadable version of a catalog entry.
type CatalogRelease struct {
	Version           string             `json:"version"`
	ReleasedAt        string             `json:"released_at,omitempty"`
	APIVersion        int                `json:"api_version"`
	MinNginxUIVersion string             `json:"min_nginx_ui_version,omitempty"`
	Platforms         []string           `json:"platforms,omitempty"`
	DownloadURL       string             `json:"download_url"`
	SHA256            string             `json:"sha256,omitempty"`
	SignatureURL      string             `json:"signature_url,omitempty"`
	SignedBy          string             `json:"signed_by,omitempty"`
	ReleaseNotesURL   string             `json:"release_notes_url,omitempty"`
	Yanked            bool               `json:"yanked,omitempty"`
	Manifest          *protocol.Manifest `json:"manifest,omitempty"`
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
		body, err := fetchText(ctx, proxiedURL(entry.ReadmeURL), maxReadmeBytes)
		if err != nil {
			mp.manager.log.Warnf("Plugin readme %s: %v", entry.ReadmeURL, err)
		} else {
			readme = body
		}
	}
	return entry, readme, nil
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
	release, err := mp.resolveRelease(entry, wantVersion)
	if err != nil {
		return nil, err
	}
	if err = mp.checkPolicy(entry, release); err != nil {
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

	archive := filepath.Join(staging, entry.ID+".tar.gz")
	if err = mp.download(ctx, entry.ID, proxiedURL(release.DownloadURL), archive); err != nil {
		return nil, err
	}

	publishInstallProgress(entry.ID, InstallStatusVerifying, 90, "")
	if err = verifyDigest(archive, release.SHA256); err != nil {
		return nil, err
	}
	if err = mp.verifySignature(ctx, entry, release, archive); err != nil {
		return nil, err
	}

	// The package must be what the catalog promised, otherwise a compromised
	// mirror could swap a well known id for something else.
	inspected, err := mp.manager.Inspect(archive)
	if err != nil {
		return nil, err
	}
	if inspected.Manifest.ID != entry.ID {
		return nil, ErrPluginIDMismatch
	}
	if inspected.Manifest.Version != release.Version {
		return nil, cosy.WrapErrorWithParams(ErrCatalogInvalid,
			fmt.Sprintf("%s ships %s, the catalog promised %s", entry.ID, inspected.Manifest.Version, release.Version))
	}

	publishInstallProgress(entry.ID, InstallStatusInstalling, 95, "")
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
		candidate := pickRelease(dependency, requirement.Version)
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

// resolveRelease picks the requested version, or the newest installable one.
func (mp *Marketplace) resolveRelease(entry *CatalogEntry, wantVersion string) (*CatalogRelease, error) {
	if wantVersion == "" {
		if entry.InstallableRelease == nil {
			return nil, ErrPlatformUnsupported
		}
		return entry.InstallableRelease, nil
	}
	for i := range entry.Releases {
		release := &entry.Releases[i]
		if release.Version != wantVersion {
			continue
		}
		if release.Yanked {
			return nil, ErrReleaseYanked
		}
		if !releaseRunsHere(release) {
			return nil, ErrPlatformUnsupported
		}
		return release, nil
	}
	return nil, ErrReleaseNotFound
}

// checkPolicy applies the node wide install policy to one release.
func (mp *Marketplace) checkPolicy(entry *CatalogEntry, release *CatalogRelease) error {
	if entry.Trust == TrustCommunity && !settings.PluginSettings.AllowCommunityPlugins {
		return ErrCommunityNotAllowed
	}
	parsed, err := url.Parse(release.DownloadURL)
	if err != nil || parsed.Host == "" {
		return cosy.WrapErrorWithParams(ErrCatalogInvalid, "download_url is not a valid url")
	}
	if parsed.Scheme != "https" && !settings.PluginSettings.AllowInsecureDownloadURL {
		return ErrInsecureURL
	}
	return nil
}

// download streams a package to disk, publishing progress on the event bus.
func (mp *Marketplace) download(ctx context.Context, pluginID, rawURL, target string) error {
	publishInstallProgress(pluginID, InstallStatusDownloading, 0, "")

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
		pluginID: pluginID,
	}
	if _, err = io.Copy(writer, io.LimitReader(response.Body, int64(MaxPackageSize)+1)); err != nil {
		return err
	}
	if writer.written > int64(MaxPackageSize) {
		return ErrPackageTooLarge
	}
	return nil
}

// verifySignature enforces the signature policy: the official source always
// requires one, a custom source only when RequireSignature is set.
func (mp *Marketplace) verifySignature(ctx context.Context, entry *CatalogEntry, release *CatalogRelease, archive string) error {
	signatureURL := release.SignatureURL
	if signatureURL == "" && release.DownloadURL != "" {
		signatureURL = release.DownloadURL + ".minisig"
	}

	required := isOfficialSource(entry.Source) || settings.PluginSettings.RequireSignature
	signature, err := fetchText(ctx, proxiedURL(signatureURL), maxReadmeBytes)
	if err != nil || strings.TrimSpace(signature) == "" {
		if required {
			return ErrSignatureMissing
		}
		mp.manager.log.Warnf("[plugin:%s] installing %s without a signature", entry.ID, release.Version)
		return nil
	}

	if _, err = pkgsign.VerifyFile(archive, []byte(signature), mp.trustedKeys(entry, release)); err != nil {
		return cosy.WrapErrorWithParams(ErrSignatureInvalid, err.Error())
	}
	return nil
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
	entry.InstallableRelease = pickRelease(entry, "")
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

// progressWriter publishes download progress, throttled so a fast link does
// not flood the event bus.
type progressWriter struct {
	writer   io.Writer
	total    int64
	written  int64
	reported float64
	pluginID string
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
		publishInstallProgress(p.pluginID, InstallStatusDownloading, percent, "")
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

// pickRelease returns the newest installable release, optionally restricted to
// a semver range.
func pickRelease(entry *CatalogEntry, versionRange string) *CatalogRelease {
	var best *CatalogRelease
	for i := range entry.Releases {
		release := &entry.Releases[i]
		if release.Yanked || !releaseRunsHere(release) {
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

// releaseRunsHere reports whether this node can install a release at all.
func releaseRunsHere(release *CatalogRelease) bool {
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
	if len(release.Platforms) == 0 {
		return true
	}
	platform := runtime.GOOS + "-" + runtime.GOARCH
	return slices.Contains(release.Platforms, platform) || slices.Contains(release.Platforms, anyPlatform)
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
