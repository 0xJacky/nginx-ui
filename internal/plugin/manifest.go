package plugin

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"sort"
	"strings"

	"github.com/0xJacky/Nginx-UI/internal/plugin/protocol"
)

// ManifestFileName is the manifest every plugin package carries at its root.
const ManifestFileName = "plugin.json"

// maxPluginIDLength bounds the manifest id so it stays usable as a directory
// name and as a database key.
const maxPluginIDLength = 64

var (
	// pluginIDPattern requires a dotted namespace, e.g. "official.cloudflare".
	pluginIDPattern = regexp.MustCompile(`^[a-z0-9]+(\.[a-z0-9-]+)+$`)
	// dns01CodePattern bounds provider codes so they are safe in URLs and forms.
	dns01CodePattern = regexp.MustCompile(`^[a-z0-9-]{2,32}$`)
	// capabilityCodePattern bounds notify channel, probe kind, storage
	// backend and deploy target codes the same way.
	capabilityCodePattern = regexp.MustCompile(`^[a-z0-9-]{2,32}$`)
	// mcpToolNamePattern keeps the published tool name within the MCP limits,
	// see MCPToolName.
	mcpToolNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,47}$`)
	// semverPattern is the official semantic versioning 2.0.0 grammar.
	semverPattern = regexp.MustCompile(`^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)` +
		`(?:-((?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*)(?:\.(?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*))*))?` +
		`(?:\+([0-9a-zA-Z-]+(?:\.[0-9a-zA-Z-]+)*))?$`)
)

// knownCapabilities lists the capability names the host can serve.
var knownCapabilities = []string{
	protocol.CapabilityDNS01,
	protocol.CapabilityHTTP,
	protocol.CapabilityNotify,
	protocol.CapabilityProbe,
	protocol.CapabilityMCP,
	protocol.CapabilityStorage,
	protocol.CapabilityCertDeploy,
}

// knownPermissions lists the fixed permission names. Credential permissions
// carry a kind suffix and are checked separately.
var knownPermissions = []string{
	protocol.PermissionKV,
	protocol.PermissionNetwork,
	protocol.PermissionCron,
	protocol.PermissionNotify,
	protocol.PermissionMetricsRead,
	protocol.PermissionCoreAPI,
	protocol.PermissionMCP,
	protocol.PermissionCertDeploy,
}

// knownSettingsTypes lists the field types the settings form can render.
var knownSettingsTypes = []string{"text", "bool", "number", "select", "secret", "textarea"}

// knownHTTPListenModes lists the transports of the http capability.
var knownHTTPListenModes = []string{"unix", "rpc"}

// knownConfigurationFieldTypes lists the field types of a capability entry
// form. Empty means text.
var knownConfigurationFieldTypes = []string{
	"",
	protocol.ConfigurationFieldText,
	protocol.ConfigurationFieldTextarea,
	protocol.ConfigurationFieldNumber,
	protocol.ConfigurationFieldBool,
}

// invalidManifest wraps ErrManifestInvalid with the concrete reason so the API
// layer keeps the cosy error code while the user still sees what is wrong.
func invalidManifest(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrManifestInvalid, fmt.Sprintf(format, args...))
}

// IsValidID reports whether id is a well formed plugin id. Route handlers use
// it before a plugin id ever reaches the filesystem.
func IsValidID(id string) bool {
	return id != "" && len(id) <= maxPluginIDLength && pluginIDPattern.MatchString(id)
}

// LoadManifest reads and decodes plugin.json from a plugin directory. It does
// not validate the content, call ValidateManifest for that.
func LoadManifest(dir string) (*protocol.Manifest, error) {
	data, err := os.ReadFile(filepath.Join(dir, ManifestFileName))
	if err != nil {
		return nil, invalidManifest("read %s: %v", ManifestFileName, err)
	}
	var m protocol.Manifest
	if err = json.Unmarshal(data, &m); err != nil {
		return nil, invalidManifest("decode %s: %v", ManifestFileName, err)
	}
	return &m, nil
}

// IsCompatible reports whether the manifest targets the protocol version this
// host implements.
func IsCompatible(m *protocol.Manifest) bool {
	return m != nil && m.APIVersion == protocol.APIVersion
}

// ValidateManifest checks every field the host relies on. Every failure wraps
// ErrManifestInvalid.
func ValidateManifest(m *protocol.Manifest) error {
	if m == nil {
		return invalidManifest("manifest is empty")
	}
	if err := validateIdentity(m); err != nil {
		return err
	}
	if m.Server == nil && m.Webapp == nil && m.Content == nil {
		return invalidManifest("at least one of server, webapp or content is required")
	}
	if err := validateServer(m.Server); err != nil {
		return err
	}
	if err := validateWebapp(m.Webapp); err != nil {
		return err
	}
	if err := validateContent(m.Content); err != nil {
		return err
	}
	if err := validateCapabilities(m); err != nil {
		return err
	}
	if err := validatePermissions(m.Permissions); err != nil {
		return err
	}
	return validateSettingsSchema(m.SettingsSchema)
}

func validateIdentity(m *protocol.Manifest) error {
	switch {
	case m.ID == "":
		return invalidManifest("id is required")
	case len(m.ID) > maxPluginIDLength:
		return invalidManifest("id must be at most %d characters", maxPluginIDLength)
	case !pluginIDPattern.MatchString(m.ID):
		return invalidManifest("id %q must look like \"vendor.name\"", m.ID)
	case m.Name == "":
		return invalidManifest("name is required")
	case m.Version == "":
		return invalidManifest("version is required")
	case !semverPattern.MatchString(m.Version):
		return invalidManifest("version %q is not a semantic version", m.Version)
	case m.APIVersion == 0:
		return invalidManifest("api_version is required")
	}
	if m.IconPath != "" && !isSafeRelPath(m.IconPath) {
		return invalidManifest("icon_path %q must be a relative path inside the plugin", m.IconPath)
	}
	return nil
}

func validateServer(s *protocol.ManifestServer) error {
	if s == nil {
		return nil
	}
	switch s.Lifecycle {
	case "", protocol.LifecycleResident, protocol.LifecycleOnDemand:
	default:
		return invalidManifest("server.lifecycle %q must be %q or %q",
			s.Lifecycle, protocol.LifecycleResident, protocol.LifecycleOnDemand)
	}
	if s.IdleTimeoutSeconds < 0 {
		return invalidManifest("server.idle_timeout_seconds must not be negative")
	}
	for platform, rel := range s.Executables {
		if !isSafeRelPath(rel) {
			return invalidManifest("server.executables[%q] %q must be a relative path inside the plugin", platform, rel)
		}
	}
	if len(s.Command) > 0 && s.Command[0] == "" {
		return invalidManifest("server.command[0] is empty")
	}
	if len(s.Command) > 0 && hasPathSeparator(s.Command[0]) && !isSafeRelPath(s.Command[0]) {
		return invalidManifest("server.command[0] %q must be a relative path inside the plugin", s.Command[0])
	}
	if len(s.Executables) == 0 && len(s.Command) == 0 {
		return invalidManifest("server needs executables or command")
	}
	return nil
}

func validateWebapp(w *protocol.ManifestWebapp) error {
	if w == nil {
		return nil
	}
	if w.BundlePath != "" && !isSafeRelPath(w.BundlePath) {
		return invalidManifest("webapp.bundle_path %q must be a relative path inside the plugin", w.BundlePath)
	}
	if w.StylePath != "" && !isSafeRelPath(w.StylePath) {
		return invalidManifest("webapp.style_path %q must be a relative path inside the plugin", w.StylePath)
	}
	for _, p := range w.Pages {
		if p.Path == "" {
			return invalidManifest("webapp.pages[].path is required")
		}
		if !isSafeRelPath(p.File) {
			return invalidManifest("webapp.pages[%q].file %q must be a relative path inside the plugin", p.Path, p.File)
		}
	}
	return nil
}

func validateContent(c *protocol.ManifestContent) error {
	if c == nil {
		return nil
	}
	if c.Templates != "" && !isSafeRelPath(c.Templates) {
		return invalidManifest("content.templates %q must be a relative path inside the plugin", c.Templates)
	}
	if c.Locales != "" && !isSafeRelPath(c.Locales) {
		return invalidManifest("content.locales %q must be a relative path inside the plugin", c.Locales)
	}
	return nil
}

func validateCapabilities(m *protocol.Manifest) error {
	seen := make(map[string]struct{}, len(m.Capabilities))
	for _, capability := range m.Capabilities {
		if !slices.Contains(knownCapabilities, capability) {
			return invalidManifest("unknown capability %q", capability)
		}
		if _, dup := seen[capability]; dup {
			return invalidManifest("capability %q is declared twice", capability)
		}
		seen[capability] = struct{}{}
	}
	if _, ok := seen[protocol.CapabilityDNS01]; ok {
		if err := validateDNS01(m.DNS01); err != nil {
			return err
		}
	}
	if _, ok := seen[protocol.CapabilityHTTP]; ok {
		if m.HTTP == nil {
			return invalidManifest("capability http requires the http block")
		}
		if !slices.Contains(knownHTTPListenModes, m.HTTP.Listen) {
			return invalidManifest("http.listen %q must be one of %s", m.HTTP.Listen, strings.Join(knownHTTPListenModes, ", "))
		}
	}
	if _, ok := seen[protocol.CapabilityNotify]; ok {
		if err := validateNotify(m.Notify); err != nil {
			return err
		}
	}
	if _, ok := seen[protocol.CapabilityProbe]; ok {
		if err := validateProbe(m.Probe); err != nil {
			return err
		}
	}
	if _, ok := seen[protocol.CapabilityMCP]; ok {
		if err := validateMCP(m); err != nil {
			return err
		}
	}
	if _, ok := seen[protocol.CapabilityStorage]; ok {
		if err := validateStorage(m.Storage); err != nil {
			return err
		}
	}
	if _, ok := seen[protocol.CapabilityCertDeploy]; ok {
		if err := validateDeploy(m); err != nil {
			return err
		}
	}
	return nil
}

func validateStorage(s *protocol.ManifestStorage) error {
	if s == nil || len(s.Backends) == 0 {
		return invalidManifest("capability storage requires at least one backend")
	}
	seen := make(map[string]struct{}, len(s.Backends))
	for _, b := range s.Backends {
		if !capabilityCodePattern.MatchString(b.Code) {
			return invalidManifest("storage backend code %q must match %s", b.Code, capabilityCodePattern)
		}
		if _, dup := seen[b.Code]; dup {
			return invalidManifest("storage backend code %q is declared twice", b.Code)
		}
		seen[b.Code] = struct{}{}
		if b.Name == "" {
			return invalidManifest("storage.backends[%q].name is required", b.Code)
		}
		if err := validateConfigurationSchema(fmt.Sprintf("storage.backends[%q]", b.Code), b.Configuration); err != nil {
			return err
		}
	}
	return nil
}

func validateDeploy(m *protocol.Manifest) error {
	if m.Deploy == nil || len(m.Deploy.Targets) == 0 {
		return invalidManifest("capability cert.deploy requires at least one target")
	}
	if !slices.Contains(m.Permissions, protocol.PermissionCertDeploy) {
		return invalidManifest("capability cert.deploy requires the %s permission", protocol.PermissionCertDeploy)
	}
	seen := make(map[string]struct{}, len(m.Deploy.Targets))
	for _, t := range m.Deploy.Targets {
		if !capabilityCodePattern.MatchString(t.Code) {
			return invalidManifest("deploy target code %q must match %s", t.Code, capabilityCodePattern)
		}
		if _, dup := seen[t.Code]; dup {
			return invalidManifest("deploy target code %q is declared twice", t.Code)
		}
		seen[t.Code] = struct{}{}
		if t.Name == "" {
			return invalidManifest("deploy.targets[%q].name is required", t.Code)
		}
		if err := validateConfigurationSchema(fmt.Sprintf("deploy.targets[%q]", t.Code), t.Configuration); err != nil {
			return err
		}
	}
	return nil
}

func validateNotify(n *protocol.ManifestNotify) error {
	if n == nil || len(n.Channels) == 0 {
		return invalidManifest("capability notify requires at least one channel")
	}
	seen := make(map[string]struct{}, len(n.Channels))
	for _, c := range n.Channels {
		if !capabilityCodePattern.MatchString(c.Code) {
			return invalidManifest("notify channel code %q must match %s", c.Code, capabilityCodePattern)
		}
		if _, dup := seen[c.Code]; dup {
			return invalidManifest("notify channel code %q is declared twice", c.Code)
		}
		seen[c.Code] = struct{}{}
		if c.Name == "" {
			return invalidManifest("notify.channels[%q].name is required", c.Code)
		}
		if err := validateConfigurationSchema(fmt.Sprintf("notify.channels[%q]", c.Code), c.Configuration); err != nil {
			return err
		}
	}
	return nil
}

func validateProbe(p *protocol.ManifestProbe) error {
	if p == nil || len(p.Kinds) == 0 {
		return invalidManifest("capability probe requires at least one kind")
	}
	seen := make(map[string]struct{}, len(p.Kinds))
	for _, k := range p.Kinds {
		if !capabilityCodePattern.MatchString(k.Code) {
			return invalidManifest("probe kind code %q must match %s", k.Code, capabilityCodePattern)
		}
		if _, dup := seen[k.Code]; dup {
			return invalidManifest("probe kind code %q is declared twice", k.Code)
		}
		seen[k.Code] = struct{}{}
		if k.Name == "" {
			return invalidManifest("probe.kinds[%q].name is required", k.Code)
		}
		if err := validateConfigurationSchema(fmt.Sprintf("probe.kinds[%q]", k.Code), k.Configuration); err != nil {
			return err
		}
	}
	return nil
}

// validateConfigurationSchema checks the form of a capability entry. where
// names the owner in the error.
func validateConfigurationSchema(where string, schema *protocol.ConfigurationSchema) error {
	if schema == nil {
		return nil
	}
	seen := make(map[string]struct{}, len(schema.Fields))
	for _, field := range schema.Fields {
		if field.Key == "" {
			return invalidManifest("%s.configuration field key is required", where)
		}
		if _, dup := seen[field.Key]; dup {
			return invalidManifest("%s.configuration key %q is declared twice", where, field.Key)
		}
		seen[field.Key] = struct{}{}
		if !slices.Contains(knownConfigurationFieldTypes, field.Type) {
			return invalidManifest("%s.configuration key %q has unknown type %q", where, field.Key, field.Type)
		}
		if field.DisplayName == "" {
			return invalidManifest("%s.configuration key %q needs a display_name", where, field.Key)
		}
	}
	return nil
}

func validateMCP(m *protocol.Manifest) error {
	if m.MCP == nil || len(m.MCP.Tools) == 0 {
		return invalidManifest("capability mcp requires at least one tool")
	}
	if !slices.Contains(m.Permissions, protocol.PermissionMCP) {
		return invalidManifest("capability mcp requires the %s permission", protocol.PermissionMCP)
	}
	seen := make(map[string]struct{}, len(m.MCP.Tools))
	for _, tool := range m.MCP.Tools {
		if !mcpToolNamePattern.MatchString(tool.Name) {
			return invalidManifest("mcp tool name %q must match %s", tool.Name, mcpToolNamePattern)
		}
		if _, dup := seen[tool.Name]; dup {
			return invalidManifest("mcp tool %q is declared twice", tool.Name)
		}
		seen[tool.Name] = struct{}{}
		if tool.Description == "" {
			return invalidManifest("mcp tool %q needs a description", tool.Name)
		}
		if !isObjectSchema(tool.InputSchema) {
			return invalidManifest("mcp tool %q input_schema must have type object", tool.Name)
		}
	}
	return nil
}

// isObjectSchema reports whether an MCP input schema describes an object. An
// absent schema means a tool without arguments.
func isObjectSchema(schema map[string]any) bool {
	if schema == nil {
		return true
	}
	kind, ok := schema["type"].(string)
	return ok && kind == "object"
}

func validateDNS01(d *protocol.ManifestDNS01) error {
	if d == nil || len(d.Providers) == 0 {
		return invalidManifest("capability dns01 requires at least one provider")
	}
	seen := make(map[string]struct{}, len(d.Providers))
	for _, p := range d.Providers {
		if p.Name == "" {
			return invalidManifest("dns01.providers[%q].name is required", p.Code)
		}
		if !dns01CodePattern.MatchString(p.Code) {
			return invalidManifest("dns01 provider code %q must match %s", p.Code, dns01CodePattern)
		}
		if _, dup := seen[p.Code]; dup {
			return invalidManifest("dns01 provider code %q is declared twice", p.Code)
		}
		seen[p.Code] = struct{}{}
	}
	return nil
}

func validatePermissions(permissions []string) error {
	for _, permission := range permissions {
		if slices.Contains(knownPermissions, permission) {
			continue
		}
		if kind, ok := strings.CutPrefix(permission, protocol.PermissionCredentialsReadPrefix); ok && kind != "" {
			continue
		}
		return invalidManifest("unknown permission %q", permission)
	}
	return nil
}

func validateSettingsSchema(schema *protocol.SettingsSchema) error {
	if schema == nil {
		return nil
	}
	seen := make(map[string]struct{}, len(schema.Settings))
	for _, field := range schema.Settings {
		if field.Key == "" {
			return invalidManifest("settings_schema field key is required")
		}
		if _, dup := seen[field.Key]; dup {
			return invalidManifest("settings_schema key %q is declared twice", field.Key)
		}
		seen[field.Key] = struct{}{}
		if !slices.Contains(knownSettingsTypes, field.Type) {
			return invalidManifest("settings_schema key %q has unknown type %q", field.Key, field.Type)
		}
		if field.Type == "select" && len(field.Options) == 0 {
			return invalidManifest("settings_schema key %q needs options", field.Key)
		}
	}
	return nil
}

// ResolveExecutable builds the argv used to spawn the plugin. The per platform
// executable wins, the interpreted command is the fallback.
func ResolveExecutable(m *protocol.Manifest, dir string) ([]string, error) {
	if m == nil || m.Server == nil {
		return nil, ErrNoExecutableForPlatform
	}
	platform := runtime.GOOS + "-" + runtime.GOARCH
	if rel := m.Server.Executables[platform]; rel != "" {
		if !isSafeRelPath(rel) {
			return nil, invalidManifest("server.executables[%q] %q must be a relative path inside the plugin", platform, rel)
		}
		abs, err := absoluteIn(dir, rel)
		if err != nil {
			return nil, err
		}
		return []string{abs}, nil
	}

	if len(m.Server.Command) == 0 {
		return nil, ErrNoExecutableForPlatform
	}
	argv := slices.Clone(m.Server.Command)
	if hasPathSeparator(argv[0]) {
		if !isSafeRelPath(argv[0]) {
			return nil, invalidManifest("server.command[0] %q must be a relative path inside the plugin", argv[0])
		}
		abs, err := absoluteIn(dir, argv[0])
		if err != nil {
			return nil, err
		}
		argv[0] = abs
	}
	return argv, nil
}

// PermissionsHash fingerprints the permission set so an upgrade that asks for
// more can be detected and re-approved.
func PermissionsHash(m *protocol.Manifest) string {
	var permissions []string
	if m != nil {
		permissions = slices.Clone(m.Permissions)
	}
	sort.Strings(permissions)
	sum := sha256.Sum256([]byte(strings.Join(permissions, "\n")))
	return hex.EncodeToString(sum[:])
}

// absoluteIn joins a manifest relative path onto the plugin directory.
func absoluteIn(dir, rel string) (string, error) {
	abs, err := filepath.Abs(filepath.Join(dir, filepath.FromSlash(rel)))
	if err != nil {
		return "", invalidManifest("resolve %q: %v", rel, err)
	}
	return abs, nil
}

// isSafeRelPath reports whether p is a clean, slash separated relative path
// that cannot escape the plugin directory.
func isSafeRelPath(p string) bool {
	if p == "" || strings.ContainsRune(p, '\\') || strings.ContainsRune(p, 0) {
		return false
	}
	if path.IsAbs(p) || filepath.IsAbs(p) {
		return false
	}
	// Reject Windows drive and share prefixes such as "C:dir".
	if len(p) > 1 && p[1] == ':' {
		return false
	}
	if path.Clean(p) != p {
		return false
	}
	return p != ".." && !strings.HasPrefix(p, "../") && p != "."
}

func hasPathSeparator(p string) bool {
	return strings.ContainsAny(p, `/\`)
}
