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

	DNS01          *ManifestDNS01   `json:"dns01,omitempty"`
	HTTP           *ManifestHTTP    `json:"http,omitempty"`
	SettingsSchema *SettingsSchema  `json:"settings_schema,omitempty"`
	Notify         *ManifestNotify  `json:"notify,omitempty"`
	Probe          *ManifestProbe   `json:"probe,omitempty"`
	MCP            *ManifestMCP     `json:"mcp,omitempty"`
	Storage        *ManifestStorage `json:"storage,omitempty"`
	Deploy         *ManifestDeploy  `json:"deploy,omitempty"`
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

// ManifestNotify is the metadata block for the notify capability.
type ManifestNotify struct {
	Channels []NotifyChannel `json:"channels"`
}

// NotifyChannel describes one vendor channel a plugin delivers notifications through.
type NotifyChannel struct {
	// Code is shared across every installed notify plugin.
	Code          string               `json:"code"`
	Name          string               `json:"name"`
	Configuration *ConfigurationSchema `json:"configuration,omitempty"`
}

// ManifestProbe is the metadata block for the probe capability.
type ManifestProbe struct {
	Kinds []ProbeKind `json:"kinds"`
}

// ProbeKind describes one way a plugin can check the health of a target.
type ProbeKind struct {
	// Code is shared across every installed probe plugin.
	Code          string               `json:"code"`
	Name          string               `json:"name"`
	Configuration *ConfigurationSchema `json:"configuration,omitempty"`
}

// ManifestStorage is the metadata block for the storage capability.
type ManifestStorage struct {
	Backends []StorageBackend `json:"backends"`
}

// StorageBackend describes one place a plugin can keep host files.
type StorageBackend struct {
	// Code is shared across every installed storage plugin.
	Code          string               `json:"code"`
	Name          string               `json:"name"`
	Configuration *ConfigurationSchema `json:"configuration,omitempty"`
}

// ManifestDeploy is the metadata block for the cert.deploy capability.
type ManifestDeploy struct {
	Targets []DeployTarget `json:"targets"`
}

// DeployTarget describes one kind of external target a plugin can push
// certificates to.
type DeployTarget struct {
	// Code is shared across every installed cert.deploy plugin.
	Code          string               `json:"code"`
	Name          string               `json:"name"`
	Configuration *ConfigurationSchema `json:"configuration,omitempty"`
}

// ConfigurationSchema drives the form of a notify channel, a probe kind, a
// storage backend or a deploy target. The values travel as a map of strings.
type ConfigurationSchema struct {
	Fields []ConfigurationField `json:"fields,omitempty"`
}

// ConfigurationField is one entry of ConfigurationSchema.
type ConfigurationField struct {
	Key string `json:"key"`
	// Type is one of the ConfigurationFieldType values, empty means text.
	Type        string `json:"type,omitempty"`
	DisplayName string `json:"display_name"`
	HelpText    string `json:"help_text,omitempty"`
	Required    bool   `json:"required,omitempty"`
	// Secret marks a credential: masked in forms and never logged.
	Secret bool `json:"secret,omitempty"`
}

// Values of ConfigurationField.Type.
const (
	ConfigurationFieldText     = "text"
	ConfigurationFieldTextarea = "textarea"
	ConfigurationFieldNumber   = "number"
	ConfigurationFieldBool     = "bool"
)

// ManifestMCP is the metadata block for the mcp capability.
type ManifestMCP struct {
	Tools []MCPTool `json:"tools"`
}

// MCPTool describes one Model Context Protocol tool a plugin serves.
type MCPTool struct {
	// Name is unique within the plugin. The host publishes it with a prefix
	// derived from the plugin id.
	Name        string `json:"name"`
	Description string `json:"description"`
	// InputSchema is the JSON Schema of the arguments object.
	InputSchema map[string]any `json:"input_schema,omitempty"`
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
