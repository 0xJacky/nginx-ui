package protocol

// Manifest is the parsed plugin.json. Validation lives in internal/plugin.
type Manifest struct {
	ID                string `json:"id"`
	Name              string `json:"name"`
	Version           string `json:"version"`
	Description       string `json:"description,omitempty"`
	HomepageURL       string `json:"homepage_url,omitempty"`
	IconPath          string `json:"icon_path,omitempty"`
	APIVersion        int    `json:"api_version"`
	MinNginxUIVersion string `json:"min_nginx_ui_version,omitempty"`

	Server  *ManifestServer  `json:"server,omitempty"`
	Webapp  *ManifestWebapp  `json:"webapp,omitempty"`
	Content *ManifestContent `json:"content,omitempty"`

	Capabilities         []string              `json:"capabilities,omitempty"`
	Permissions          []string              `json:"permissions,omitempty"`
	Requires             []ManifestRequirement `json:"requires,omitempty"`
	RequiresCapabilities []string              `json:"requires_capabilities,omitempty"`
	Events               []string              `json:"events,omitempty"`
	Cron                 []ManifestCron        `json:"cron,omitempty"`
	NetworkHosts         []string              `json:"network_hosts,omitempty"`

	DNS01          *ManifestDNS01  `json:"dns01,omitempty"`
	HTTP           *ManifestHTTP   `json:"http,omitempty"`
	SettingsSchema *SettingsSchema `json:"settings_schema,omitempty"`
}

// ManifestServer describes how to start the plugin process.
type ManifestServer struct {
	// Executables maps "<goos>-<goarch>" to a path relative to the plugin directory.
	Executables map[string]string `json:"executables,omitempty"`
	// Command is the fallback argv for interpreted plugins.
	Command []string `json:"command,omitempty"`
	// Lifecycle is "resident" (default) or "on_demand".
	Lifecycle          string `json:"lifecycle,omitempty"`
	IdleTimeoutSeconds int    `json:"idle_timeout_seconds,omitempty"`
}

// ManifestWebapp describes the optional browser bundle.
type ManifestWebapp struct {
	BundlePath string `json:"bundle_path,omitempty"`
	StylePath  string `json:"style_path,omitempty"`
	// Shared maps a shared runtime library to the semver range the bundle was built against.
	Shared map[string]string `json:"shared,omitempty"`
	Pages  []ManifestPage    `json:"pages,omitempty"`
}

// ManifestPage is a zero-build iframe page.
type ManifestPage struct {
	Path  string            `json:"path"`
	Title map[string]string `json:"title"`
	Icon  string            `json:"icon,omitempty"`
	File  string            `json:"file"`
}

// ManifestContent declares process-less contributions.
type ManifestContent struct {
	Templates string `json:"templates,omitempty"`
	Locales   string `json:"locales,omitempty"`
}

// ManifestRequirement is a hard dependency on another plugin.
type ManifestRequirement struct {
	ID      string `json:"id"`
	Version string `json:"version,omitempty"` // semver range
}

// ManifestCron is a host-scheduled task.
type ManifestCron struct {
	ID       string `json:"id"`
	Schedule string `json:"schedule"`
	Method   string `json:"method"`
}

// ManifestDNS01 is the metadata block for the dns01 capability.
type ManifestDNS01 struct {
	Providers []DNS01Provider `json:"providers"`
}

// DNS01Provider describes one vendor a plugin can solve DNS-01 for.
type DNS01Provider struct {
	Name                      string               `json:"name"`
	Code                      string               `json:"code"`
	Configuration             *DNS01ProviderConfig `json:"configuration,omitempty"`
	Links                     *DNS01ProviderLinks  `json:"links,omitempty"`
	PropagationTimeoutSeconds int                  `json:"propagation_timeout_seconds,omitempty"`
	PollingIntervalSeconds    int                  `json:"polling_interval_seconds,omitempty"`
}

// DNS01ProviderConfig lists credential and additional fields with help text.
type DNS01ProviderConfig struct {
	Credentials map[string]string `json:"credentials,omitempty"`
	Additional  map[string]string `json:"additional,omitempty"`
}

// DNS01ProviderLinks points at vendor documentation.
type DNS01ProviderLinks struct {
	API      string `json:"api,omitempty"`
	GoClient string `json:"go_client,omitempty"`
}

// ManifestHTTP is the metadata block for the http capability.
type ManifestHTTP struct {
	// Listen is "unix" (reverse proxy to a socket) or "rpc" (http.handle fallback).
	Listen string `json:"listen"`
}

// SettingsSchema drives the auto-rendered settings form.
type SettingsSchema struct {
	Header   string          `json:"header,omitempty"`
	Footer   string          `json:"footer,omitempty"`
	Settings []SettingsField `json:"settings"`
}

// SettingsField is one entry of SettingsSchema.
type SettingsField struct {
	Key         string           `json:"key"`
	Type        string           `json:"type"` // text | bool | number | select | secret | textarea
	DisplayName string           `json:"display_name"`
	HelpText    string           `json:"help_text,omitempty"`
	Default     any              `json:"default,omitempty"`
	Options     []SettingsOption `json:"options,omitempty"`
	Required    bool             `json:"required,omitempty"`
}

// SettingsOption is a choice for a select field.
type SettingsOption struct {
	Value string `json:"value"`
	Label string `json:"label"`
}
