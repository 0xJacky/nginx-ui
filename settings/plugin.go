package settings

// DefaultPluginMarketplaceSource is the official static catalog.
const DefaultPluginMarketplaceSource = "https://plugins.nginxui.com/v1/index.json"

// DefaultPluginCgroupRoot is the usual mount point of the cgroup v2 hierarchy.
const DefaultPluginCgroupRoot = "/sys/fs/cgroup"

// Plugin holds the plugin system settings.
type Plugin struct {
	// Enabled is the master switch of the plugin system.
	Enabled bool `json:"enabled"`
	// Dir overrides the plugin directory. Empty means {conf_dir}/plugins.
	Dir string `json:"dir" protected:"true"`
	// DefaultSyncPolicy is applied to newly installed plugins: manual or auto.
	DefaultSyncPolicy string `json:"default_sync_policy"`

	MarketplaceEnabled bool `json:"marketplace_enabled"`
	// MarketplaceSources lists catalog URLs, merged in order.
	MarketplaceSources []string `json:"marketplace_sources" ini:",,allowshadow"`
	// TrustedPublicKeys are extra minisign public keys accepted for package signatures.
	TrustedPublicKeys []string `json:"trusted_public_keys" ini:",,allowshadow"`
	// DeveloperMode permits installing packages without a known signature, on every install path.
	DeveloperMode bool `json:"developer_mode"`
	// AllowCommunityPlugins permits installing community trust level plugins after confirmation.
	AllowCommunityPlugins bool `json:"allow_community_plugins"`
	// AllowInsecureDownloadURL permits plain http download URLs.
	AllowInsecureDownloadURL bool `json:"allow_insecure_download_url"`
	// AllowUploads permits installing packages uploaded through the UI or CLI.
	AllowUploads bool `json:"allow_uploads"`
	// AutoUpdate upgrades official and verified plugins whose permission set did not change.
	AutoUpdate bool `json:"auto_update"`

	// MemoryLimitMB caps the memory of every plugin process in MiB. 0 or less
	// means unlimited. A smaller server.resources.memory_mb hint of a manifest wins.
	MemoryLimitMB int `json:"memory_limit_mb"`
	// CPUPercent caps the CPU time of every plugin process in percent of one core,
	// 100 being one core. 0 or less means unlimited. A smaller manifest hint wins.
	CPUPercent int `json:"cpu_percent"`
	// CgroupRoot is the cgroup v2 mount point the limits are enforced under, on
	// Linux only. Each plugin gets <CgroupRoot>/nginx-ui/plugins/<id>.
	CgroupRoot string `json:"cgroup_root" protected:"true"`
}

var PluginSettings = &Plugin{
	Enabled:               true,
	Dir:                   "",
	DefaultSyncPolicy:     "manual",
	MarketplaceEnabled:    true,
	MarketplaceSources:    []string{},
	TrustedPublicKeys:     []string{},
	AllowCommunityPlugins: true,
	AllowUploads:          true,
	CgroupRoot:            DefaultPluginCgroupRoot,
}

// GetCgroupRoot returns the configured cgroup root or the default one.
func (p *Plugin) GetCgroupRoot() string {
	if p.CgroupRoot == "" {
		return DefaultPluginCgroupRoot
	}
	return p.CgroupRoot
}

// GetMarketplaceSources returns the configured sources or the official default.
func (p *Plugin) GetMarketplaceSources() []string {
	if len(p.MarketplaceSources) == 0 {
		return []string{DefaultPluginMarketplaceSource}
	}
	return append([]string(nil), p.MarketplaceSources...)
}

// GetDefaultSyncPolicy normalizes the default sync policy.
func (p *Plugin) GetDefaultSyncPolicy() string {
	if p.DefaultSyncPolicy == "auto" {
		return "auto"
	}
	return "manual"
}
