package plugin

import (
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/0xJacky/Nginx-UI/internal/plugin/protocol"
	"github.com/0xJacky/Nginx-UI/internal/translation"
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
	// backend, deploy target, blocklist source and discovery provider codes
	// the same way.
	capabilityCodePattern = regexp.MustCompile(`^[a-z0-9-]{2,32}$`)
	// chunkNamePattern is the grammar of a webapp.chunks name.
	chunkNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,31}$`)
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
	protocol.CapabilitySecurityBlocklist,
	protocol.CapabilityUpstreamDiscovery,
	protocol.CapabilityLogSink,
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
	protocol.PermissionLogRead,
	protocol.PermissionLogFiles,
	protocol.PermissionNginxSnippet,
	protocol.PermissionNginxConfigRead,
	protocol.PermissionSitesRead,
	protocol.PermissionCertsRead,
}

// knownLogFormats lists the line formats a log.sink plugin may ask for.
var knownLogFormats = []string{protocol.LogFormatCombined, protocol.LogFormatRaw}

// knownSettingsTypes lists the field types the settings form can render.
var knownSettingsTypes = []string{
	settingsTypeText, "bool", "number", "select", "secret", "textarea", settingsTypeList,
}

const (
	settingsTypeText = "text"
	// settingsTypeList holds a list of single line strings.
	settingsTypeList = "list"
)

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
	if err := validateI18n(m.I18n); err != nil {
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
	if err := validateProcessless(m); err != nil {
		return err
	}
	if err := validateCapabilities(m); err != nil {
		return err
	}
	if err := validatePermissions(m.Permissions); err != nil {
		return err
	}
	if problems := conflictProblems(m); len(problems) > 0 {
		return invalidManifest("%s", problems[0])
	}
	return validateSettingsSchema(m.SettingsSchema)
}

// conflictProblems lists every way the conflicts declaration is invalid,
// empty when it is fine.
func conflictProblems(m *protocol.Manifest) []string {
	var problems []string
	required := make(map[string]struct{}, len(m.Requires))
	for _, requirement := range m.Requires {
		required[requirement.ID] = struct{}{}
	}
	seen := make(map[string]struct{}, len(m.Conflicts))
	for _, id := range m.Conflicts {
		if !IsValidID(id) {
			problems = append(problems, fmt.Sprintf("conflicts entry %q is not a valid plugin id", id))
			continue
		}
		if id == m.ID {
			problems = append(problems, fmt.Sprintf("conflicts entry %q is the plugin itself", id))
		}
		if _, ok := seen[id]; ok {
			problems = append(problems, fmt.Sprintf("conflicts entry %q is listed twice", id))
		}
		seen[id] = struct{}{}
		if _, ok := required[id]; ok {
			problems = append(problems, fmt.Sprintf("conflicts entry %q is also in requires", id))
		}
	}
	return problems
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

// validateI18n checks that every key of the i18n block is a language of the
// host.
func validateI18n(i18n map[string]protocol.ManifestI18n) error {
	for _, locale := range slices.Sorted(maps.Keys(i18n)) {
		if !translation.IsLanguage(locale) {
			return invalidManifest("i18n: %q is not a language of the host (%s)",
				locale, strings.Join(translation.Languages(), ", "))
		}
	}
	return nil
}

// i18nMaps splits the i18n block into one locale map per field and leaves
// out empty translations. A field without any translation yields nil.
func i18nMaps(m *protocol.Manifest) (names, descriptions map[string]string) {
	if m == nil {
		return nil, nil
	}
	for locale, translated := range m.I18n {
		if translated.Name != "" {
			if names == nil {
				names = make(map[string]string, len(m.I18n))
			}
			names[locale] = translated.Name
		}
		if translated.Description != "" {
			if descriptions == nil {
				descriptions = make(map[string]string, len(m.I18n))
			}
			descriptions[locale] = translated.Description
		}
	}
	return names, descriptions
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
	if r := s.Resources; r != nil && (r.MemoryMB < 0 || r.RecommendedMemoryMB < 0 || r.CPUPercent < 0) {
		return invalidManifest("server.resources must not be negative")
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
	if problem := chunksProblem(w); problem != "" {
		return invalidManifest("%s", problem)
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

// chunksProblem describes the first way webapp.chunks is invalid, empty
// when it is fine. Chunk names are visited in sorted order so the message is
// stable.
func chunksProblem(w *protocol.ManifestWebapp) string {
	if len(w.Chunks) == 0 {
		return ""
	}
	if w.BundlePath == "" {
		return "webapp.chunks needs webapp.bundle_path"
	}
	names := make([]string, 0, len(w.Chunks))
	for name := range w.Chunks {
		names = append(names, name)
	}
	sort.Strings(names)

	used := make(map[string]string, len(names))
	for _, name := range names {
		file := w.Chunks[name]
		switch {
		case !chunkNamePattern.MatchString(name):
			return fmt.Sprintf("webapp.chunks name %q must match %s", name, chunkNamePattern)
		case !isSafeRelPath(file):
			return fmt.Sprintf("webapp.chunks[%q] %q must be a relative path inside the plugin", name, file)
		case !strings.HasSuffix(file, ".js"):
			return fmt.Sprintf("webapp.chunks[%q] %q must end in .js", name, file)
		case file == w.BundlePath:
			return fmt.Sprintf("webapp.chunks[%q] must not be the bundle itself", name)
		}
		if other, dup := used[file]; dup {
			return fmt.Sprintf("webapp.chunks[%q] and [%q] name the same file %q", other, name, file)
		}
		used[file] = name
	}
	return ""
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

// validateProcessless rejects what only a process can serve on a manifest
// without a server block.
func validateProcessless(m *protocol.Manifest) error {
	if m.Server != nil {
		return nil
	}
	switch {
	case len(m.Capabilities) > 0:
		return invalidManifest("capabilities need a server block")
	case len(m.Cron) > 0:
		return invalidManifest("cron entries need a server block")
	case len(m.Events) > 0:
		return invalidManifest("events need a server block")
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
	if _, ok := seen[protocol.CapabilitySecurityBlocklist]; ok {
		if err := validateBlocklist(m); err != nil {
			return err
		}
	}
	if _, ok := seen[protocol.CapabilityUpstreamDiscovery]; ok {
		if err := validateDiscovery(m); err != nil {
			return err
		}
	}
	if _, ok := seen[protocol.CapabilityLogSink]; ok && !slices.Contains(m.Permissions, protocol.PermissionLogRead) {
		return invalidManifest("capability log.sink requires the %s permission", protocol.PermissionLogRead)
	}
	return validateLogSink(m.LogSink)
}

// validateLogSink checks the optional log_sink block.
func validateLogSink(l *protocol.ManifestLogSink) error {
	if l == nil {
		return nil
	}
	if !validLogSinkBatchSize(l.BatchSize) {
		return invalidManifest("log_sink.batch_size must be between 0 and %d", protocol.MaxLogSinkBatchSize)
	}
	if !validLogSinkFlushInterval(l.FlushIntervalMS) {
		return invalidManifest("log_sink.flush_interval_ms must be 0 or at least %d", protocol.MinLogSinkFlushIntervalMS)
	}
	seen := make(map[string]struct{}, len(l.Formats))
	for _, format := range l.Formats {
		if !slices.Contains(knownLogFormats, format) {
			return invalidManifest("log_sink.formats: unknown format %q", format)
		}
		if _, dup := seen[format]; dup {
			return invalidManifest("log_sink.formats: %q is listed twice", format)
		}
		seen[format] = struct{}{}
	}
	return nil
}

func validLogSinkBatchSize(size int) bool {
	return size >= 0 && size <= protocol.MaxLogSinkBatchSize
}

func validLogSinkFlushInterval(ms int) bool {
	return ms == 0 || ms >= protocol.MinLogSinkFlushIntervalMS
}

func validateBlocklist(m *protocol.Manifest) error {
	if m.Blocklist == nil || len(m.Blocklist.Sources) == 0 {
		return invalidManifest("capability security.blocklist requires at least one source")
	}
	if !slices.Contains(m.Permissions, protocol.PermissionNetwork) {
		return invalidManifest("capability security.blocklist requires the %s permission", protocol.PermissionNetwork)
	}
	seen := make(map[string]struct{}, len(m.Blocklist.Sources))
	for _, source := range m.Blocklist.Sources {
		if !capabilityCodePattern.MatchString(source.Code) {
			return invalidManifest("blocklist source code %q must match %s", source.Code, capabilityCodePattern)
		}
		if _, dup := seen[source.Code]; dup {
			return invalidManifest("blocklist source code %q is declared twice", source.Code)
		}
		seen[source.Code] = struct{}{}
		if source.Name == "" {
			return invalidManifest("blocklist.sources[%q].name is required", source.Code)
		}
		if !validBlocklistRefresh(source.RefreshSeconds) {
			return invalidManifest("blocklist.sources[%q].refresh_seconds must be 0 or at least %d",
				source.Code, protocol.MinBlocklistRefreshSeconds)
		}
		if err := validateConfigurationSchema(fmt.Sprintf("blocklist.sources[%q]", source.Code), source.Configuration); err != nil {
			return err
		}
	}
	return nil
}

// validBlocklistRefresh reports whether a manifest refresh interval is 0
// (the default) or long enough.
func validBlocklistRefresh(seconds int) bool {
	return seconds == 0 || seconds >= protocol.MinBlocklistRefreshSeconds
}

func validateDiscovery(m *protocol.Manifest) error {
	if m.Discovery == nil || len(m.Discovery.Providers) == 0 {
		return invalidManifest("capability upstream.discovery requires at least one provider")
	}
	if !slices.Contains(m.Permissions, protocol.PermissionNetwork) {
		return invalidManifest("capability upstream.discovery requires the %s permission", protocol.PermissionNetwork)
	}
	seen := make(map[string]struct{}, len(m.Discovery.Providers))
	for _, provider := range m.Discovery.Providers {
		if !capabilityCodePattern.MatchString(provider.Code) {
			return invalidManifest("discovery provider code %q must match %s", provider.Code, capabilityCodePattern)
		}
		if _, dup := seen[provider.Code]; dup {
			return invalidManifest("discovery provider code %q is declared twice", provider.Code)
		}
		seen[provider.Code] = struct{}{}
		if provider.Name == "" {
			return invalidManifest("discovery.providers[%q].name is required", provider.Code)
		}
		if err := validateConfigurationSchema(fmt.Sprintf("discovery.providers[%q]", provider.Code), provider.Configuration); err != nil {
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
		if problems := dns01FormProblems(p); len(problems) > 0 {
			return invalidManifest("%s", problems[0])
		}
	}
	return nil
}

// dns01FormProblems checks the form of a provider shallowly. The form is
// required but may list no fields, keys are unique, groups and units are known,
// method fields are credential fields, method values only target credential
// fields another method lists, no two methods are the same and at most one
// method is recommended.
func dns01FormProblems(p protocol.DNS01Provider) []string {
	where := fmt.Sprintf("dns01 provider %q form", p.Code)
	form := p.Form
	if form == nil {
		return []string{where + " is required"}
	}
	var problems []string

	groups := make(map[string]string, len(form.Fields))
	for i, field := range form.Fields {
		if field.Key == "" {
			problems = append(problems, fmt.Sprintf("%s: fields[%d].key is required", where, i))
			continue
		}
		if _, dup := groups[field.Key]; dup {
			problems = append(problems, fmt.Sprintf("%s: field %q is declared twice", where, field.Key))
			continue
		}
		if field.Label == "" {
			problems = append(problems, fmt.Sprintf("%s: field %q needs a label", where, field.Key))
		}
		switch field.Group {
		case protocol.DNS01FieldGroupCredential, protocol.DNS01FieldGroupSetting:
		default:
			problems = append(problems, fmt.Sprintf("%s: field %q has unknown group %q", where, field.Key, field.Group))
		}
		if field.Unit != "" && field.Unit != protocol.DNS01FieldUnitSeconds {
			problems = append(problems, fmt.Sprintf("%s: field %q has unknown unit %q", where, field.Key, field.Unit))
		}
		groups[field.Key] = field.Group
	}

	if len(form.Methods) == 1 {
		problems = append(problems, fmt.Sprintf("%s: methods must be absent or list at least two ways to sign in", where))
	}
	recommended := 0
	names := make(map[string]struct{}, len(form.Methods))
	signatures := make(map[string]string, len(form.Methods))
	// A fixed value may target a field only when some method lets the user fill it.
	methodFields := make(map[string]struct{})
	for _, method := range form.Methods {
		for _, key := range method.Fields {
			methodFields[key] = struct{}{}
		}
	}
	for i, method := range form.Methods {
		if method.Name == "" {
			problems = append(problems, fmt.Sprintf("%s: methods[%d].name is required", where, i))
		} else if _, dup := names[method.Name]; dup {
			problems = append(problems, fmt.Sprintf("%s: method %q is declared twice", where, method.Name))
		}
		names[method.Name] = struct{}{}
		if method.Recommended {
			recommended++
		}
		for key := range method.Values {
			if key == "" {
				problems = append(problems, fmt.Sprintf("%s: method %q has a value with an empty key", where, method.Name))
				continue
			}
			group, isField := groups[key]
			switch {
			case !isField:
			case group != protocol.DNS01FieldGroupCredential:
				problems = append(problems, fmt.Sprintf("%s: method %q sets value %q that is not a credential field", where, method.Name, key))
			case slices.Contains(method.Fields, key):
				problems = append(problems, fmt.Sprintf("%s: method %q both lists and sets %q", where, method.Name, key))
			default:
				if _, ok := methodFields[key]; !ok {
					problems = append(problems, fmt.Sprintf("%s: method %q sets value %q, a field no method lists", where, method.Name, key))
				}
			}
		}
		sig := methodSignature(method)
		if other, dup := signatures[sig]; dup {
			problems = append(problems, fmt.Sprintf("%s: methods %q and %q use the same fields and values", where, other, method.Name))
		} else {
			signatures[sig] = method.Name
		}
		for _, key := range method.Fields {
			group, ok := groups[key]
			if !ok {
				problems = append(problems, fmt.Sprintf("%s: method %q uses field %q that is not in fields", where, method.Name, key))
			} else if group != protocol.DNS01FieldGroupCredential {
				problems = append(problems, fmt.Sprintf("%s: method %q uses field %q that is not a credential", where, method.Name, key))
			}
		}
	}
	if recommended > 1 {
		problems = append(problems, fmt.Sprintf("%s: %d methods are recommended, at most one may be", where, recommended))
	}
	return problems
}

// methodSignature identifies a method by its field set and fixed values.
func methodSignature(method protocol.DNS01ProviderMethod) string {
	fields := slices.Clone(method.Fields)
	slices.Sort(fields)
	fields = slices.Compact(fields)
	keys := slices.Sorted(maps.Keys(method.Values))
	var b strings.Builder
	for _, key := range fields {
		b.WriteString(strconv.Quote(key))
	}
	b.WriteByte('|')
	for _, key := range keys {
		b.WriteString(strconv.Quote(key))
		b.WriteByte('=')
		b.WriteString(strconv.Quote(method.Values[key]))
	}
	return b.String()
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
		if field.Type == settingsTypeList && field.Default != nil && !isStringList(field.Default) {
			return invalidManifest("settings_schema key %q default must be a list of strings", field.Key)
		}
	}
	return nil
}

// isStringList reports whether a decoded JSON value is an array of strings.
func isStringList(value any) bool {
	switch typed := value.(type) {
	case []string:
		return true
	case []any:
		for _, item := range typed {
			if _, ok := item.(string); !ok {
				return false
			}
		}
		return true
	default:
		return false
	}
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

// unapprovedPermissions lists the permissions the manifest asks for that the
// approved set does not hold. An upgrade that drops a permission asks for
// nothing new.
func unapprovedPermissions(approved []string, m *protocol.Manifest) []string {
	if m == nil {
		return nil
	}
	var missing []string
	for _, permission := range m.Permissions {
		if !slices.Contains(approved, permission) {
			missing = append(missing, permission)
		}
	}
	return missing
}

// approvedSet is the permission set to store once the manifest is approved.
// It is never nil, which marks the plugin as approved before.
func approvedSet(m *protocol.Manifest) []string {
	permissions := []string{}
	if m != nil {
		permissions = append(permissions, m.Permissions...)
	}
	slices.Sort(permissions)
	return permissions
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
