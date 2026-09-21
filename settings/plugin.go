package settings

// DefaultPluginMarketplaceSource is the official static catalog.
const DefaultPluginMarketplaceSource = "https://raw.githubusercontent.com/0xJacky/nginx-ui-plugins/main/v1/index.json"

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
	// RequireSignature rejects unsigned packages from custom sources. Official sources always require it.
	RequireSignature bool `json:"require_signature"`
	// AllowCommunityPlugins permits installing community trust level plugins after confirmation.
	AllowCommunityPlugins bool `json:"allow_community_plugins"`
	// AllowInsecureDownloadURL permits plain http download URLs.
	AllowInsecureDownloadURL bool `json:"allow_insecure_download_url"`
	// AllowUploads permits installing packages uploaded through the UI or CLI.
	AllowUploads bool `json:"allow_uploads"`
	// AutoUpdate upgrades official and verified plugins whose permission set did not change.
	AutoUpdate bool `json:"auto_update"`
}

var PluginSettings = &Plugin{
	Enabled:               true,
	Dir:                   "",
	DefaultSyncPolicy:     "manual",
	MarketplaceEnabled:    true,
	MarketplaceSources:    []string{},
	TrustedPublicKeys:     []string{},
	RequireSignature:      true,
	AllowCommunityPlugins: true,
	AllowUploads:          true,
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
