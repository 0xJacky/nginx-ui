package plugin

import (
	"context"
	"os"
	"path"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/plugin/jsonrpc"
	"github.com/0xJacky/Nginx-UI/internal/plugin/protocol"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/go-co-op/gocron/v2"
	"github.com/uozi-tech/cosy/logger"
	cSettings "github.com/uozi-tech/cosy/settings"
	"go.uber.org/zap"
)

// Status is the plugin lifecycle state the API exposes. It folds the database
// row, the files on disk and the supervisor state into one value.
type Status string

const (
	// StatusInstalled is a plugin present on disk and in the database, disabled.
	StatusInstalled Status = "installed"
	// StatusStarting is an enabled plugin whose process is coming up.
	StatusStarting Status = "starting"
	// StatusRunning is an enabled plugin that is serving.
	StatusRunning Status = "running"
	// StatusStopped is an idle on_demand plugin or one that finished a run.
	StatusStopped Status = "stopped"
	// StatusError is an enabled plugin the supervisor gave up on.
	StatusError Status = "error"
	// StatusMissing is a database row whose directory disappeared.
	StatusMissing Status = "missing"
	// StatusIncompatible is a plugin built for another protocol version.
	StatusIncompatible Status = "incompatible"
	// StatusNeedsApproval is an enabled plugin whose permission set changed.
	StatusNeedsApproval Status = "needs_approval"
)

const (
	// DataDirName holds the private data directory of every plugin.
	DataDirName = ".data"
	// backupSuffix marks the previous version kept during an upgrade.
	backupSuffix = ".bak"
	// webappRouteDir is the URL segment the webapp static route is mounted on.
	webappRouteDir = "webapp"
	// pagesDirName is both the directory and the URL segment of iframe pages.
	pagesDirName = "pages"
	// defaultWebappDir is used when a manifest declares pages but no bundle.
	defaultWebappDir = "webapp"
	// pluginRoutePrefix is the URL prefix of the plugin static routes.
	pluginRoutePrefix = "plugins"
)

// Info is the plugin as the REST API and the CLI report it.
type Info struct {
	ID                   string                         `json:"id"`
	Name                 string                         `json:"name"`
	Version              string                         `json:"version"`
	Description          string                         `json:"description,omitempty"`
	HomepageURL          string                         `json:"homepage_url,omitempty"`
	IconURL              string                         `json:"icon_url,omitempty"`
	APIVersion           int                            `json:"api_version"`
	MinNginxUIVersion    string                         `json:"min_nginx_ui_version,omitempty"`
	Capabilities         []string                       `json:"capabilities"`
	Permissions          []string                       `json:"permissions"`
	Requires             []protocol.ManifestRequirement `json:"requires"`
	RequiresCapabilities []string                       `json:"requires_capabilities"`
	HasServer            bool                           `json:"has_server"`
	HasWebapp            bool                           `json:"has_webapp"`
	Lifecycle            string                         `json:"lifecycle"`
	Status               Status                         `json:"status"`
	Enabled              bool                           `json:"enabled"`
	LastError            string                         `json:"last_error,omitempty"`
	SettingsSchema       *protocol.SettingsSchema       `json:"settings_schema"`
	SyncPolicy           string                         `json:"sync_policy"`
	SyncNodeIDs          []uint64                       `json:"sync_node_ids"`
	SyncSettings         bool                           `json:"sync_settings"`
	UpdatedAt            time.Time                      `json:"updated_at"`
	DroppedEvents        int64                          `json:"dropped_events"`
	// Trust is derived from the package signature at install time, see
	// signature.go. Signer is the minisign key id, empty when unsigned.
	// Partner is the partner name of a verified package, empty otherwise.
	Trust   string `json:"trust"`
	Signer  string `json:"signer"`
	Partner string `json:"partner"`
	// Transport is how capability calls reach the running process, "stdio" or
	// "grpc". Empty when no process is running.
	Transport string `json:"transport,omitempty"`
	// The access log lines a log.sink plugin accepted, rejected and lost
	// since the host started (spec LOGSINK-11).
	StreamedLogEntries int64 `json:"streamed_log_entries"`
	RejectedLogEntries int64 `json:"rejected_log_entries"`
	DroppedLogEntries  int64 `json:"dropped_log_entries"`
	// Resources are the limits of the plugin process, absent for a plugin
	// without one (spec LIFE-16).
	Resources *ResourceStatus `json:"resources,omitempty"`
	// NameI18n and DescriptionI18n translate Name and Description, keyed by
	// host locale code (spec MAN-40). Name and Description are the fallback.
	NameI18n        map[string]string `json:"name_i18n,omitempty"`
	DescriptionI18n map[string]string `json:"description_i18n,omitempty"`
}

// WebappEntry tells the browser runtime what to load for one plugin. The URLs
// are relative to the application root, see webappURL.
type WebappEntry struct {
	ID        string                  `json:"id"`
	Version   string                  `json:"version"`
	BundleURL string                  `json:"bundle_url"`
	StyleURL  string                  `json:"style_url,omitempty"`
	Shared    map[string]string       `json:"shared,omitempty"`
	Pages     []protocol.ManifestPage `json:"pages,omitempty"`
}

// DNS01ProviderEntry is one provider offered by one enabled plugin.
type DNS01ProviderEntry struct {
	PluginID string                 `json:"plugin_id"`
	Provider protocol.DNS01Provider `json:"provider"`
}

// Spec advertises what this host supports to the UI and to package tooling.
type Spec struct {
	APIVersions      []int    `json:"api_versions"`
	WebappAPIVersion int      `json:"webapp_api_version"`
	Capabilities     []string `json:"capabilities"`
	Transports       []string `json:"transports"`
	// Platform is the "<goos>-<goarch>" key a package must cover to run here.
	Platform string `json:"platform,omitempty"`
}

// InstallOptions controls one install or upgrade.
type InstallOptions struct {
	// Enable marks the plugin as wanted right after the files are in place.
	Enable bool
	// ApprovePermissions records the manifest permission set as approved.
	ApprovePermissions bool
	// ExpectedID and ExpectedVersion, when set, refuse a package whose
	// manifest names another plugin or version. The marketplace passes what
	// the catalog promised, so a swapped package is caught by the one
	// extraction the install needs anyway.
	ExpectedID      string
	ExpectedVersion string
	// AuthorPublicKey is the key of the catalog entry the package came from,
	// a signature by it makes the package community trust.
	AuthorPublicKey string
	// MinTrust refuses a package whose derived trust ranks below it.
	MinTrust string
}

// InspectResult describes a package without installing it.
type InspectResult struct {
	Manifest           *protocol.Manifest             `json:"manifest"`
	Permissions        []string                       `json:"permissions"`
	PermissionsChanged bool                           `json:"permissions_changed"`
	InstalledVersion   string                         `json:"installed_version,omitempty"`
	RequiresMissing    []protocol.ManifestRequirement `json:"requires_missing"`
	// Platforms lists the "<goos>-<goarch>" keys whose executable ships in the
	// package, plus "any" when the plugin needs no executable of its own.
	Platforms []string `json:"platforms"`
	// HostPlatform is the "<goos>-<goarch>" key of this node.
	HostPlatform string `json:"host_platform"`
	// PlatformSupported reports whether the package runs on this node.
	PlatformSupported bool `json:"platform_supported"`
	// NameI18n and DescriptionI18n are the non-empty translations of the
	// manifest i18n block, keyed by host locale code (spec MAN-40).
	NameI18n        map[string]string `json:"name_i18n,omitempty"`
	DescriptionI18n map[string]string `json:"description_i18n,omitempty"`
	// UploadID names the kept upload an install can reuse, set by the API layer.
	UploadID string `json:"upload_id,omitempty"`
	// Trust and Signer are derived from the embedded signature with the keys
	// of this node, an upload has no catalog author key. Partner is the
	// partner name of a verified package, empty otherwise.
	Trust   string `json:"trust"`
	Signer  string `json:"signer"`
	Partner string `json:"partner"`
}

// entry is one plugin the manager knows about. A missing plugin has a row but
// no manifest, a plugin discovered before its row was created has neither.
type entry struct {
	id       string
	dir      string
	dataDir  string
	manifest *protocol.Manifest
	row      *model.Plugin

	supervisor *Supervisor
	state      State
	lastErr    string

	// events is the bounded per plugin delivery queue, see events.go.
	events   chan protocol.EventNotification
	stop     chan struct{}
	drained  chan struct{}
	dropped  atomic.Int64
	cronJobs map[string]gocron.Job

	// logSink streams the access log to a log.sink plugin, see logsink.go.
	logSink     *logSink
	logCounters logSinkCounters
}

// Manager owns every installed plugin: the files, the database rows, the
// supervised processes and the host side of the plugin API.
type Manager struct {
	// opMu serialises the mutating operations. It is never held while a
	// supervisor callback can run, so a state change cannot deadlock it.
	opMu sync.Mutex
	// mu guards entries and the mutable fields of every entry.
	mu      sync.RWMutex
	entries map[string]*entry

	dir     string
	started bool
	// offline keeps the manager from spawning processes and background
	// workers, which is what the command line tools need.
	offline bool
	// handshakeTimeout is handed to every supervisor; zero keeps the default.
	handshakeTimeout time.Duration
	ctx              context.Context

	backend     *hostBackend
	scheduler   gocron.Scheduler
	unsubscribe func()
	log         *zap.SugaredLogger

	// logMu guards the log sink list and the feed subscription.
	logMu          sync.Mutex
	logFeed        LogFeed
	logUnsubscribe func()
	logSinkList    []*logSink
	// logSinks is the list the feed goroutine reads without a lock.
	logSinks atomic.Pointer[[]*logSink]

	// partners is the signed partner keyring, see partners.go.
	partners partnerStore
}

var (
	managerOnce sync.Once
	manager     *Manager
)

// GetManager returns the process wide plugin manager.
func GetManager() *Manager {
	managerOnce.Do(func() { manager = newManager("") })
	return manager
}

// Init discovers the installed plugins and brings the enabled ones up. It
// returns as soon as discovery is done so the boot sequence is not blocked by
// plugin handshakes, the processes start in the background.
func Init(ctx context.Context) {
	GetManager().Start(ctx)
	GetManager().StartMarketplace(ctx)
}

// newManager builds a manager rooted at dir. An empty dir is resolved from the
// settings the first time the manager is started.
func newManager(dir string) *Manager {
	m := &Manager{
		entries: map[string]*entry{},
		dir:     dir,
		log:     logger.GetLogger(),
	}
	m.backend = &hostBackend{manager: m}
	return m
}

// Dir is the directory holding one subdirectory per installed plugin.
func (m *Manager) Dir() string {
	m.mu.RLock()
	dir := m.dir
	m.mu.RUnlock()
	if dir != "" {
		return dir
	}
	return DefaultDir()
}

// DefaultDir resolves the configured plugin directory, falling back to the
// directory holding the nginx-ui configuration file.
func DefaultDir() string {
	if settings.PluginSettings.Dir != "" {
		return settings.PluginSettings.Dir
	}
	return filepath.Join(path.Dir(cSettings.ConfPath), "plugins")
}

// DataDir is the private writable directory handed to one plugin.
func (m *Manager) DataDir(id string) string {
	return filepath.Join(m.Dir(), DataDirName, id)
}

// Start performs discovery synchronously and then brings the enabled plugins
// up in the background.
func (m *Manager) Start(ctx context.Context) {
	if !settings.PluginSettings.Enabled {
		m.log.Info("Plugin system is disabled, skipping plugin discovery")
		return
	}

	m.opMu.Lock()
	if m.started {
		m.opMu.Unlock()
		return
	}
	m.started = true
	m.mu.Lock()
	if m.dir == "" {
		m.dir = DefaultDir()
	}
	m.ctx = ctx
	m.mu.Unlock()

	if err := os.MkdirAll(m.Dir(), 0o755); err != nil {
		m.started = false
		m.opMu.Unlock()
		m.log.Errorf("Create plugin directory: %v", err)
		return
	}
	if err := os.MkdirAll(filepath.Join(m.Dir(), DataDirName), 0o700); err != nil {
		m.log.Errorf("Create plugin data directory: %v", err)
	}
	m.loadPartnerCache()

	if err := m.discover(ctx); err != nil {
		m.log.Errorf("Plugin discovery: %v", err)
	}
	m.startScheduler()
	m.subscribeEvents()
	m.opMu.Unlock()

	go m.startEnabled(ctx)
	m.StartSync(ctx)
}

// Stop shuts every plugin and every background worker down. The manager can be
// started again afterwards, which is what the tests rely on.
func (m *Manager) Stop(ctx context.Context) {
	m.opMu.Lock()
	defer m.opMu.Unlock()

	if m.unsubscribe != nil {
		m.unsubscribe()
		m.unsubscribe = nil
	}
	if m.scheduler != nil {
		_ = m.scheduler.Shutdown()
		m.scheduler = nil
	}

	m.StopSync()
	for _, item := range m.snapshot() {
		m.stopEntry(ctx, item)
		m.stopEventPump(item)
	}
	m.started = false
}

// snapshot copies the entry pointers so callers can work without the lock.
func (m *Manager) snapshot() []*entry {
	m.mu.RLock()
	defer m.mu.RUnlock()
	items := make([]*entry, 0, len(m.entries))
	for _, item := range m.entries {
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].id < items[j].id })
	return items
}

func (m *Manager) lookup(id string) (*entry, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	item, ok := m.entries[id]
	return item, ok
}

// discover aligns the plugin directory with the plugins table. It is the only
// place that creates rows for plugins that were dropped into the directory.
func (m *Manager) discover(ctx context.Context) error {
	onDisk, err := m.scanDirectory()
	if err != nil {
		return err
	}

	rows, err := query.Plugin.WithContext(ctx).Find()
	if err != nil {
		return err
	}
	byID := make(map[string]*model.Plugin, len(rows))
	for _, row := range rows {
		byID[row.PluginID] = row
	}

	entries := make(map[string]*entry, len(onDisk)+len(rows))
	for id, manifest := range onDisk {
		row := byID[id]
		switch {
		case row == nil:
			row = &model.Plugin{
				PluginID:   id,
				Version:    manifest.Version,
				Enabled:    false,
				SyncPolicy: settings.PluginSettings.GetDefaultSyncPolicy(),
			}
			if err = query.Plugin.WithContext(ctx).Create(row); err != nil {
				m.log.Errorf("Register plugin %s: %v", id, err)
				continue
			}
		case row.Version != manifest.Version:
			// The files were replaced out of band, keep the row in sync.
			row.Version = manifest.Version
			if err = m.saveRow(ctx, row); err != nil {
				m.log.Errorf("Update plugin %s version: %v", id, err)
			}
		}
		entries[id] = m.newEntry(id, manifest, row)
	}

	for id, row := range byID {
		if _, ok := entries[id]; ok {
			continue
		}
		// The row outlived its directory, report it instead of dropping it.
		entries[id] = m.newEntry(id, nil, row)
	}

	m.mu.Lock()
	m.entries = entries
	m.mu.Unlock()

	if !m.offline {
		for _, item := range m.snapshot() {
			m.startEventPump(item)
		}
	}
	return nil
}

// LoadOffline discovers the installed plugins without starting any process or
// background worker. The command line tools use it so an install never leaves
// a plugin process behind when the command exits.
func (m *Manager) LoadOffline(ctx context.Context) error {
	m.opMu.Lock()
	defer m.opMu.Unlock()

	m.mu.Lock()
	m.offline = true
	if m.dir == "" {
		m.dir = DefaultDir()
	}
	m.ctx = ctx
	m.mu.Unlock()

	if err := os.MkdirAll(m.Dir(), 0o755); err != nil {
		return err
	}
	m.loadPartnerCache()
	return m.discover(ctx)
}

// scanDirectory reads every <pluginsDir>/<id>/plugin.json.
func (m *Manager) scanDirectory() (map[string]*protocol.Manifest, error) {
	dir := m.Dir()
	items, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]*protocol.Manifest{}, nil
		}
		return nil, err
	}

	found := make(map[string]*protocol.Manifest, len(items))
	for _, item := range items {
		name := item.Name()
		// Hidden directories hold host data, .bak is an interrupted upgrade.
		if !item.IsDir() || strings.HasPrefix(name, ".") || name == PackagesDirName || strings.HasSuffix(name, backupSuffix) {
			continue
		}
		manifest, err := LoadManifest(filepath.Join(dir, name))
		if err != nil {
			m.log.Warnf("Skip plugin directory %s: %v", name, err)
			continue
		}
		if err = ValidateManifest(manifest); err != nil {
			m.log.Warnf("Skip plugin directory %s: %v", name, err)
			continue
		}
		if manifest.ID != name {
			// The directory name is the plugin id everywhere else, so a
			// mismatch would make the static routes and the data directory
			// point at the wrong plugin.
			m.log.Warnf("Skip plugin directory %s: %v (%s)", name, ErrPluginIDMismatch, manifest.ID)
			continue
		}
		found[manifest.ID] = manifest
	}
	return found, nil
}

func (m *Manager) newEntry(id string, manifest *protocol.Manifest, row *model.Plugin) *entry {
	return &entry{
		id:       id,
		dir:      filepath.Join(m.Dir(), id),
		dataDir:  m.DataDir(id),
		manifest: manifest,
		row:      row,
		state:    StateStopped,
		lastErr:  row.LastError,
		cronJobs: map[string]gocron.Job{},
	}
}

// List returns every known plugin, ordered by id.
func (m *Manager) List() []Info {
	items := m.snapshot()
	infos := make([]Info, 0, len(items))
	for _, item := range items {
		infos = append(infos, m.infoOf(item))
	}
	return infos
}

// Get returns one plugin.
func (m *Manager) Get(id string) (*Info, error) {
	item, ok := m.lookup(id)
	if !ok {
		return nil, ErrPluginNotFound
	}
	info := m.infoOf(item)
	return &info, nil
}

// Manifest returns the parsed manifest of an installed plugin.
func (m *Manager) Manifest(pluginID string) (*protocol.Manifest, bool) {
	item, ok := m.lookup(pluginID)
	if !ok {
		return nil, false
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	if item.manifest == nil {
		return nil, false
	}
	return item.manifest, true
}

// Spec advertises the protocol versions and capabilities of this host.
func (m *Manager) Spec() Spec {
	return Spec{
		APIVersions:      []int{protocol.APIVersion},
		WebappAPIVersion: protocol.APIVersion,
		Capabilities:     slices.Clone(knownCapabilities),
		Transports:       []string{protocol.TransportStdio, protocol.TransportGRPC},
		Platform:         HostPlatform(),
	}
}

// infoOf renders one entry for the API.
func (m *Manager) infoOf(item *entry) Info {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.infoLocked(item)
}

func (m *Manager) infoLocked(item *entry) Info {
	info := Info{
		ID:                   item.id,
		Requires:             []protocol.ManifestRequirement{},
		Capabilities:         []string{},
		Permissions:          []string{},
		RequiresCapabilities: []string{},
		SyncNodeIDs:          []uint64{},
		Status:               statusOf(item),
		Trust:                TrustUnsigned,
		DroppedEvents:        item.dropped.Load(),
		StreamedLogEntries:   item.logCounters.streamed.Load(),
		RejectedLogEntries:   item.logCounters.rejected.Load(),
		DroppedLogEntries:    item.logCounters.dropped.Load(),
	}

	if row := item.row; row != nil {
		info.Version = row.Version
		info.Enabled = row.Enabled
		info.SyncPolicy = row.SyncPolicy
		info.SyncSettings = row.SyncSettings
		info.UpdatedAt = row.UpdatedAt
		if row.Trust != "" {
			info.Trust = row.Trust
		}
		info.Signer = row.Signer
		info.Partner = row.Partner
		if len(row.SyncNodeIDs) > 0 {
			info.SyncNodeIDs = row.SyncNodeIDs
		}
	}
	if info.SyncPolicy == "" {
		info.SyncPolicy = model.PluginSyncPolicyManual
	}
	info.LastError = item.lastErr
	if item.supervisor != nil {
		info.Transport = item.supervisor.Transport()
	}

	manifest := item.manifest
	if manifest == nil {
		info.Name = item.id
		return info
	}

	if manifest.Server != nil {
		var resources ResourceStatus
		if item.supervisor != nil {
			resources = item.supervisor.Resources()
		} else {
			resources = resourceStatus(EffectiveResources(hostResourceLimits(), manifest), false)
		}
		info.Resources = &resources
	}

	info.Name = manifest.Name
	info.Version = manifest.Version
	info.Description = manifest.Description
	info.NameI18n, info.DescriptionI18n = i18nMaps(manifest)
	info.HomepageURL = manifest.HomepageURL
	info.APIVersion = manifest.APIVersion
	info.MinNginxUIVersion = manifest.MinNginxUIVersion
	info.IconURL = iconURL(manifest)
	info.HasServer = manifest.Server != nil
	info.HasWebapp = manifest.Webapp != nil
	info.Lifecycle = lifecycleOf(manifest)
	info.SettingsSchema = manifest.SettingsSchema
	if len(manifest.Capabilities) > 0 {
		info.Capabilities = manifest.Capabilities
	}
	if len(manifest.Permissions) > 0 {
		info.Permissions = manifest.Permissions
	}
	if len(manifest.Requires) > 0 {
		info.Requires = manifest.Requires
	}
	if len(manifest.RequiresCapabilities) > 0 {
		info.RequiresCapabilities = manifest.RequiresCapabilities
	}
	return info
}

// statusOf folds the row, the files and the process state into one status. The
// caller must hold at least the read lock.
func statusOf(item *entry) Status {
	if item.manifest == nil {
		return StatusMissing
	}
	if !IsCompatible(item.manifest) {
		return StatusIncompatible
	}
	if item.row == nil {
		return StatusInstalled
	}
	if !item.row.Enabled {
		// An upgrade that asks for more permissions than the user approved
		// leaves the plugin down until the new set is confirmed.
		if item.row.ApprovedPermissionsHash != "" && !isApproved(item) {
			return StatusNeedsApproval
		}
		return StatusInstalled
	}
	if !isApproved(item) {
		return StatusNeedsApproval
	}
	if item.manifest.Server == nil {
		// A webapp or content only plugin has nothing to supervise.
		return StatusRunning
	}
	switch item.state {
	case StateStarting:
		return StatusStarting
	case StateRunning:
		return StatusRunning
	case StateError:
		return StatusError
	default:
		return StatusStopped
	}
}

// isApproved reports whether the stored approval covers the manifest. A plugin
// that asks for nothing never needs an approval step.
func isApproved(item *entry) bool {
	if item.manifest == nil || item.row == nil {
		return false
	}
	if len(item.manifest.Permissions) == 0 {
		return true
	}
	return item.row.ApprovedPermissionsHash == PermissionsHash(item.manifest)
}

func lifecycleOf(manifest *protocol.Manifest) string {
	if manifest == nil || manifest.Server == nil {
		return protocol.LifecycleResident
	}
	if manifest.Server.Lifecycle != "" {
		return manifest.Server.Lifecycle
	}
	return protocol.LifecycleResident
}

// webappDir is the plugin relative directory the webapp static route serves.
func webappDir(manifest *protocol.Manifest) string {
	if manifest == nil || manifest.Webapp == nil || manifest.Webapp.BundlePath == "" {
		return defaultWebappDir
	}
	dir := path.Dir(manifest.Webapp.BundlePath)
	if dir == "." {
		return ""
	}
	return dir
}

// webappURL maps a manifest relative path onto the webapp static route. It
// returns an empty string when the file is not under the served directory.
// The URL is relative to the application root, like the API base the browser
// uses, so it resolves under a sub path deployment as well.
func webappURL(id, manifestPath string, manifest *protocol.Manifest) string {
	if manifestPath == "" {
		return ""
	}
	root := webappDir(manifest)
	rel := manifestPath
	if root != "" {
		var inside bool
		rel, inside = strings.CutPrefix(manifestPath, root+"/")
		if !inside {
			return ""
		}
	}
	return path.Join(pluginRoutePrefix, id, webappRouteDir, rel)
}

// iconURL exposes the manifest icon only when it is reachable through one of
// the static routes.
func iconURL(manifest *protocol.Manifest) string {
	if manifest == nil || manifest.IconPath == "" {
		return ""
	}
	if rel, inside := strings.CutPrefix(manifest.IconPath, pagesDirName+"/"); inside {
		return path.Join(pluginRoutePrefix, manifest.ID, pagesDirName, rel)
	}
	return webappURL(manifest.ID, manifest.IconPath, manifest)
}

// WebappEntries lists the browser bundles of every enabled plugin whose files
// are actually there.
func (m *Manager) WebappEntries() []WebappEntry {
	items := m.snapshot()
	m.mu.RLock()
	defer m.mu.RUnlock()

	entries := make([]WebappEntry, 0, len(items))
	for _, item := range items {
		manifest := item.manifest
		if manifest == nil || manifest.Webapp == nil || item.row == nil || !item.row.Enabled {
			continue
		}
		if !IsCompatible(manifest) {
			continue
		}

		bundleURL := ""
		if manifest.Webapp.BundlePath != "" {
			if _, err := os.Stat(filepath.Join(item.dir, filepath.FromSlash(manifest.Webapp.BundlePath))); err == nil {
				bundleURL = webappURL(item.id, manifest.Webapp.BundlePath, manifest)
			}
		}
		if bundleURL == "" && len(manifest.Webapp.Pages) == 0 {
			continue
		}

		entries = append(entries, WebappEntry{
			ID:        item.id,
			Version:   manifest.Version,
			BundleURL: bundleURL,
			StyleURL:  webappURL(item.id, manifest.Webapp.StylePath, manifest),
			Shared:    manifest.Webapp.Shared,
			Pages:     manifest.Webapp.Pages,
		})
	}
	return entries
}

// StaticRoot resolves the directory one static route serves for a plugin. Only
// enabled plugins are served.
func (m *Manager) StaticRoot(id, kind string) (string, error) {
	item, ok := m.lookup(id)
	if !ok {
		return "", ErrPluginNotFound
	}

	m.mu.RLock()
	defer m.mu.RUnlock()
	if item.manifest == nil || item.row == nil || !item.row.Enabled {
		return "", ErrPluginNotFound
	}
	switch kind {
	case webappRouteDir:
		return filepath.Join(item.dir, filepath.FromSlash(webappDir(item.manifest))), nil
	case pagesDirName:
		return filepath.Join(item.dir, pagesDirName), nil
	default:
		return "", ErrPluginNotFound
	}
}

// Logs returns the buffered stderr tail of a plugin.
func (m *Manager) Logs(id string) []LogLine {
	item, ok := m.lookup(id)
	if !ok {
		return nil
	}
	m.mu.RLock()
	supervisor := item.supervisor
	m.mu.RUnlock()
	if supervisor == nil {
		return nil
	}
	return supervisor.Logs()
}

// Settings returns the schema of a plugin and the stored values with every
// secret replaced by the redaction placeholder.
func (m *Manager) Settings(id string) (*protocol.SettingsSchema, map[string]any, error) {
	item, ok := m.lookup(id)
	if !ok {
		return nil, nil, ErrPluginNotFound
	}

	m.mu.RLock()
	defer m.mu.RUnlock()
	if item.manifest == nil {
		return nil, nil, ErrPluginNotFound
	}
	schema := item.manifest.SettingsSchema
	values := mergedSettings(item.manifest, item.row)
	for _, field := range schemaFields(schema) {
		if field.Type != "secret" {
			continue
		}
		if value, ok := values[field.Key]; ok && value != nil && value != "" {
			values[field.Key] = settings.RedactedSensitiveValue
		}
	}
	return schema, values, nil
}

// SaveSettings validates the submitted values against the schema, keeps the
// stored secret when the placeholder comes back and pushes the result into a
// running plugin.
func (m *Manager) SaveSettings(ctx context.Context, id string, values map[string]any) error {
	if !settings.PluginSettings.Enabled {
		return ErrPluginsDisabled
	}

	m.opMu.Lock()
	item, ok := m.lookup(id)
	if !ok {
		m.opMu.Unlock()
		return ErrPluginNotFound
	}

	m.mu.RLock()
	manifest := item.manifest
	row := item.row
	stored := map[string]any{}
	if row != nil {
		for key, value := range row.Settings {
			stored[key] = value
		}
	}
	m.mu.RUnlock()

	if manifest == nil || row == nil {
		m.opMu.Unlock()
		return ErrPluginNotFound
	}

	next, err := normalizeSettings(manifest.SettingsSchema, values, stored)
	if err != nil {
		m.opMu.Unlock()
		return err
	}

	m.mu.Lock()
	row.Settings = next
	m.mu.Unlock()
	if err = m.saveRow(ctx, row); err != nil {
		m.opMu.Unlock()
		return err
	}

	m.mu.RLock()
	supervisor := item.supervisor
	merged := mergedSettings(manifest, row)
	m.mu.RUnlock()
	m.opMu.Unlock()

	if supervisor != nil {
		if err = supervisor.Configure(ctx, merged); err != nil {
			m.log.Warnf("[plugin:%s] push settings: %v", id, err)
		}
	}
	return nil
}

// schemaFields is a nil safe accessor for the schema field list.
func schemaFields(schema *protocol.SettingsSchema) []protocol.SettingsField {
	if schema == nil {
		return nil
	}
	return schema.Settings
}

// mergedSettings layers the stored values over the schema defaults.
func mergedSettings(manifest *protocol.Manifest, row *model.Plugin) map[string]any {
	merged := map[string]any{}
	if manifest != nil {
		for _, field := range schemaFields(manifest.SettingsSchema) {
			if field.Default != nil {
				merged[field.Key] = field.Default
			}
		}
	}
	if row != nil {
		for key, value := range row.Settings {
			merged[key] = value
		}
	}
	return merged
}

// OwnerOf resolves which enabled plugin serves a capability code: a dns01
// provider, a notify channel, a probe kind, a storage backend, a deploy
// target kind, a blocklist source kind or a discovery provider. When several
// plugins declare the same code the lowest plugin id wins, except that the
// official plugin always wins a dns01 code.
func (m *Manager) OwnerOf(capability, code string) (string, bool) {
	switch capability {
	case protocol.CapabilityDNS01, protocol.CapabilityNotify, protocol.CapabilityProbe,
		protocol.CapabilityStorage, protocol.CapabilityCertDeploy,
		protocol.CapabilitySecurityBlocklist, protocol.CapabilityUpstreamDiscovery:
	default:
		return "", false
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	owner := ""
	for _, item := range m.entries {
		if !enabledWithCapabilityLocked(item, capability) || !slices.Contains(declaredCodes(item.manifest, capability), code) {
			continue
		}
		if capability == protocol.CapabilityDNS01 && item.id == OfficialDNS01PluginID {
			return item.id, true
		}
		if owner == "" || item.id < owner {
			owner = item.id
		}
	}
	return owner, owner != ""
}

// OfficialDNS01PluginID is preferred when several plugins claim one code.
const OfficialDNS01PluginID = "com.nginxui.dns01"

// EnabledWithCapability lists the enabled plugins declaring a capability.
func (m *Manager) EnabledWithCapability(capability string) []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ids := make([]string, 0, len(m.entries))
	for _, item := range m.entries {
		if enabledWithCapabilityLocked(item, capability) {
			ids = append(ids, item.id)
		}
	}
	sort.Strings(ids)
	return ids
}

// DNS01Providers lists every provider offered by an enabled dns01 plugin.
func (m *Manager) DNS01Providers() []DNS01ProviderEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()

	providers := make([]DNS01ProviderEntry, 0, len(m.entries))
	for _, item := range m.entries {
		if !enabledWithCapabilityLocked(item, protocol.CapabilityDNS01) || item.manifest.DNS01 == nil {
			continue
		}
		for _, provider := range item.manifest.DNS01.Providers {
			providers = append(providers, DNS01ProviderEntry{PluginID: item.id, Provider: provider})
		}
	}
	sort.Slice(providers, func(i, j int) bool {
		if providers[i].PluginID == providers[j].PluginID {
			return providers[i].Provider.Code < providers[j].Provider.Code
		}
		return providers[i].PluginID < providers[j].PluginID
	})
	return providers
}

// enabledWithCapabilityLocked reports whether the entry is usable right now.
// The caller must hold at least the read lock.
func enabledWithCapabilityLocked(item *entry, capability string) bool {
	if item.manifest == nil || item.row == nil || !item.row.Enabled {
		return false
	}
	if !IsCompatible(item.manifest) || !isApproved(item) {
		return false
	}
	return slices.Contains(item.manifest.Capabilities, capability)
}

// Acquire hands out a client for a plugin, starting an on_demand one. The
// returned release function must always be called.
func (m *Manager) Acquire(ctx context.Context, pluginID string) (jsonrpc.Caller, func(), error) {
	noop := func() {}
	if !settings.PluginSettings.Enabled {
		return nil, noop, ErrPluginsDisabled
	}

	item, ok := m.lookup(pluginID)
	if !ok {
		return nil, noop, ErrPluginNotFound
	}

	m.mu.RLock()
	supervisor := item.supervisor
	enabled := item.row != nil && item.row.Enabled
	m.mu.RUnlock()

	if !enabled {
		return nil, noop, ErrPluginNotRunning
	}
	if supervisor == nil {
		return nil, noop, ErrPluginNotRunning
	}
	return supervisor.Acquire(ctx)
}
