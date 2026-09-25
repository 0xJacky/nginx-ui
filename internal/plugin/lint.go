package plugin

// This file implements a static linter for a plugin directory or a packaged
// .tar.gz, used by "nginx-ui plugin lint". It mirrors the checks ValidateManifest
// and ExtractPackage apply, but collects every issue instead of stopping at
// the first one, and adds filesystem level checks (missing executables,
// missing docs, package limits) that those functions do not perform.
//
// Every Finding is tagged with the spec requirement it maps to (see
// nginx-ui-plugin-spec/spec/*.md), e.g. "MAN-2" or "PKG-8". A handful of
// checks have no numbered requirement of their own (PATH lookups, README
// section headings, signature verification); those still use the closest
// spec section as the rule id since that is what a reader would look up.

import (
	"archive/tar"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"strings"

	"github.com/0xJacky/Nginx-UI/internal/plugin/protocol"
	"github.com/0xJacky/Nginx-UI/internal/translation"
)

// Level is the severity of one Finding.
type Level string

const (
	LevelError   Level = "error"
	LevelWarning Level = "warning"
)

// Finding is one lint issue.
type Finding struct {
	Level   Level
	Rule    string
	Message string
}

// LintReport collects every Finding Lint produced.
type LintReport struct {
	Findings []Finding
}

// HasErrors reports whether the report contains an error level Finding.
func (r *LintReport) HasErrors() bool {
	for _, f := range r.Findings {
		if f.Level == LevelError {
			return true
		}
	}
	return false
}

func (r *LintReport) add(level Level, rule, format string, args ...any) {
	r.Findings = append(r.Findings, Finding{Level: level, Rule: rule, Message: fmt.Sprintf(format, args...)})
}

// maxWebappBundleSizeForLint mirrors the conformance budget so lint can warn
// about an oversize bundle before conformance ever runs.
const lintReadmeName = "README.md"

// Lint checks a plugin directory or a .tar.gz package against the plugin
// spec. path may be a directory (already extracted) or an archive file, in
// which case it is extracted into a temporary directory that is removed
// before Lint returns.
//
// Lint returns a non-nil error only for a problem with the tool itself (the
// path does not exist, an I/O failure); every problem with the plugin's own
// content is reported as a Finding instead.
func Lint(path string) (*LintReport, error) {
	report := &LintReport{}

	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	dir := path
	archiveName := ""
	if info.IsDir() {
		lintDirLimits(dir, report)
	} else {
		archiveName = filepath.Base(path)
		extracted, ok := lintExtractArchive(path, report)
		if !ok {
			return report, nil
		}
		defer os.RemoveAll(extracted)
		dir = extracted
	}
	lintSums(dir, report)

	manifest, err := LoadManifest(dir)
	if err != nil {
		report.add(LevelError, "MAN-1", "%v", err)
		return report, nil
	}

	lintIdentity(manifest, report)
	lintI18n(manifest.I18n, report)
	if archiveName != "" {
		lintPackageName(archiveName, manifest, report)
	}
	if manifest.Server == nil && manifest.Webapp == nil && manifest.Content == nil {
		report.add(LevelError, "MAN-7", "at least one of server, webapp or content is required")
	}
	lintServer(manifest.Server, dir, report)
	lintWebapp(manifest.Webapp, dir, report)
	lintContent(manifest, dir, report)
	lintProcessless(manifest, report)
	lintCapabilities(manifest, report)
	lintPermissions(manifest.Permissions, report)
	lintRequires(manifest.Requires, report)
	lintSettingsSchema(manifest.SettingsSchema, report)
	lintDocs(dir, report)

	return report, nil
}

// lintIdentity checks the top level scalar fields (MAN-2 through MAN-8).
func lintIdentity(m *protocol.Manifest, report *LintReport) {
	switch {
	case m.ID == "":
		report.add(LevelError, "MAN-2", "id is required")
	case len(m.ID) > maxPluginIDLength:
		report.add(LevelError, "MAN-2", "id must be at most %d characters", maxPluginIDLength)
	case !pluginIDPattern.MatchString(m.ID):
		report.add(LevelError, "MAN-2", "id %q must look like \"vendor.name\" (%s)", m.ID, pluginIDPattern)
	case strings.HasPrefix(m.ID, "com.nginxui."):
		// This CLI has no allowlist of officially maintained plugin ids, so
		// every com.nginxui.* id it sees is treated as unverified.
		report.add(LevelWarning, "NAME-2", "id %q uses the reserved com.nginxui.* namespace; this tool cannot confirm official ownership", m.ID)
	}

	if m.Name == "" {
		report.add(LevelError, "MAN-3", "name is required")
	}

	switch {
	case m.Version == "":
		report.add(LevelError, "MAN-4", "version is required")
	case !semverPattern.MatchString(m.Version):
		report.add(LevelError, "MAN-4", "version %q is not a semantic version", m.Version)
	}

	switch {
	case m.APIVersion == 0:
		report.add(LevelError, "MAN-5", "api_version is required")
	case m.APIVersion != protocol.APIVersion:
		report.add(LevelError, "MAN-5", "api_version %d is not supported by this host (supports %d)", m.APIVersion, protocol.APIVersion)
	}

	if m.IconPath != "" && !isSafeRelPath(m.IconPath) {
		report.add(LevelError, "MAN-8", "icon_path %q must be a safe relative path", m.IconPath)
	}
}

// lintI18n checks that every key of the i18n block is a language of the
// host (MAN-40).
func lintI18n(i18n map[string]protocol.ManifestI18n, report *LintReport) {
	for _, locale := range slices.Sorted(maps.Keys(i18n)) {
		if !translation.IsLanguage(locale) {
			report.add(LevelError, "MAN-40", "i18n: %q is not a language of the host (%s)",
				locale, strings.Join(translation.Languages(), ", "))
		}
	}
}

// lintServer checks the server block (MAN-9 through MAN-14) and, unlike
// ValidateManifest, also verifies every declared executable actually exists.
func lintServer(s *protocol.ManifestServer, dir string, report *LintReport) {
	if s == nil {
		return
	}

	switch s.Lifecycle {
	case "", protocol.LifecycleResident, protocol.LifecycleOnDemand:
	default:
		report.add(LevelError, "MAN-10", "server.lifecycle %q must be %q or %q", s.Lifecycle, protocol.LifecycleResident, protocol.LifecycleOnDemand)
	}
	if s.IdleTimeoutSeconds < 0 {
		report.add(LevelError, "MAN-11", "server.idle_timeout_seconds must not be negative")
	}
	if r := s.Resources; r != nil {
		if r.MemoryMB < 0 {
			report.add(LevelError, "MAN-39", "server.resources.memory_mb must not be negative")
		}
		if r.CPUPercent < 0 {
			report.add(LevelError, "MAN-39", "server.resources.cpu_percent must not be negative")
		}
	}
	if len(s.Executables) == 0 && len(s.Command) == 0 {
		report.add(LevelError, "MAN-9", "server needs executables or command")
	}

	platforms := make([]string, 0, len(s.Executables))
	for platform := range s.Executables {
		platforms = append(platforms, platform)
	}
	sort.Strings(platforms)
	for _, platform := range platforms {
		rel := s.Executables[platform]
		if !isSafeRelPath(rel) {
			report.add(LevelError, "MAN-12", "server.executables[%q] %q must be a safe relative path", platform, rel)
			continue
		}
		lintExecutableFile(filepath.Join(dir, filepath.FromSlash(rel)), platform, report)
	}

	if len(s.Command) == 0 {
		return
	}
	switch {
	case s.Command[0] == "":
		report.add(LevelError, "MAN-13", "server.command[0] is empty")
	case hasPathSeparator(s.Command[0]):
		if !isSafeRelPath(s.Command[0]) {
			report.add(LevelError, "MAN-12", "server.command[0] %q must be a safe relative path", s.Command[0])
		} else {
			lintExecutableFile(filepath.Join(dir, filepath.FromSlash(s.Command[0])), "command", report)
		}
	default:
		if _, err := exec.LookPath(s.Command[0]); err != nil {
			report.add(LevelWarning, "MAN-12", "server.command[0] %q was not found on PATH: %v", s.Command[0], err)
		}
	}
}

// lintPackageName checks an archive file name against its manifest. A name
// that follows neither package form (an upload saved under another name) is
// not checked at all. PKG-1 wants the id and version in the name to match the
// manifest, PKG-12 wants a per-platform package to declare exactly the
// platform its name carries, so the platform a catalog serves it for is the
// one it runs on. PKG-9 still requires every declared executable to exist.
func lintPackageName(name string, m *protocol.Manifest, report *LintReport) {
	parsed, ok := ParsePackageFileName(name)
	if !ok {
		return
	}
	if parsed.ID != m.ID || parsed.Version != m.Version {
		report.add(LevelWarning, "PKG-1", "file name %s does not match the manifest id %q and version %q", name, m.ID, m.Version)
	}
	if parsed.Platform == "" {
		return
	}

	declared := []string{}
	if m.Server != nil {
		for platform := range m.Server.Executables {
			declared = append(declared, platform)
		}
	}
	sort.Strings(declared)
	if len(declared) != 1 || declared[0] != parsed.Platform {
		report.add(LevelError, "PKG-12", "file name %s targets %s, but server.executables declares [%s]; a per-platform package must declare exactly its own platform",
			name, parsed.Platform, strings.Join(declared, ", "))
	}
}

// lintExecutableFile checks that a resolved executable path exists and, on
// Unix, warns when it is not already marked executable. PKG-9 requires the
// host to fix the bit up itself, so a missing bit is a warning, not an error.
func lintExecutableFile(path, platform string, report *LintReport) {
	info, err := os.Stat(path)
	switch {
	case os.IsNotExist(err):
		report.add(LevelError, "PKG-9", "executable for %s is missing: %s", platform, path)
		return
	case err != nil:
		report.add(LevelError, "PKG-9", "cannot stat the executable for %s: %v", platform, err)
		return
	case !info.Mode().IsRegular():
		report.add(LevelError, "PKG-9", "executable for %s is not a regular file: %s", platform, path)
		return
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0o111 == 0 {
		report.add(LevelWarning, "PKG-9", "executable for %s does not have the executable bit set: %s", platform, path)
	}
}

// lintWebapp checks the webapp block (MAN-15/MAN-16) and that every file it
// points at exists.
func lintWebapp(w *protocol.ManifestWebapp, dir string, report *LintReport) {
	if w == nil {
		return
	}
	if w.BundlePath != "" {
		if !isSafeRelPath(w.BundlePath) {
			report.add(LevelError, "MAN-15", "webapp.bundle_path %q must be a safe relative path", w.BundlePath)
		} else {
			lintFileExists(filepath.Join(dir, filepath.FromSlash(w.BundlePath)), "MAN-15", "webapp.bundle_path", report)
		}
	}
	if w.StylePath != "" {
		if !isSafeRelPath(w.StylePath) {
			report.add(LevelError, "MAN-15", "webapp.style_path %q must be a safe relative path", w.StylePath)
		} else {
			lintFileExists(filepath.Join(dir, filepath.FromSlash(w.StylePath)), "MAN-15", "webapp.style_path", report)
		}
	}
	for _, p := range w.Pages {
		if p.Path == "" {
			report.add(LevelError, "MAN-16", "webapp.pages[].path is required")
		}
		if !isSafeRelPath(p.File) {
			report.add(LevelError, "MAN-16", "webapp.pages[%q].file %q must be a safe relative path", p.Path, p.File)
			continue
		}
		lintFileExists(filepath.Join(dir, filepath.FromSlash(p.File)), "MAN-16", fmt.Sprintf("webapp.pages[%q].file", p.Path), report)
	}
}

func lintFileExists(path, rule, what string, report *LintReport) {
	if info, err := os.Stat(path); err != nil {
		report.add(LevelError, rule, "%s points at a missing file: %s", what, path)
	} else if info.IsDir() {
		report.add(LevelError, rule, "%s points at a directory, not a file: %s", what, path)
	}
}

// lintContent checks the content block (MAN-18) and the templates and
// translation files it points at (CONTENT-2, CONTENT-3, CONTENT-6 and
// CONTENT-7).
func lintContent(m *protocol.Manifest, dir string, report *LintReport) {
	c := m.Content
	if c == nil {
		return
	}
	if c.Templates != "" && !isSafeRelPath(c.Templates) {
		report.add(LevelError, "MAN-18", "content.templates %q must be a safe relative path", c.Templates)
	}
	if c.Locales != "" && !isSafeRelPath(c.Locales) {
		report.add(LevelError, "MAN-18", "content.locales %q must be a safe relative path", c.Locales)
	}
	for _, problem := range CheckContent(m, dir) {
		report.add(problem.Level, problem.Rule, "%s", problem.String())
	}
}

// lintProcessless checks that a manifest without a server block declares
// nothing only a process can serve (CONTENT-1).
func lintProcessless(m *protocol.Manifest, report *LintReport) {
	if m.Server != nil {
		return
	}
	if len(m.Capabilities) > 0 {
		report.add(LevelError, "CONTENT-1", "capabilities %v need a server block, a plugin without one has no process", m.Capabilities)
	}
	if len(m.Cron) > 0 {
		report.add(LevelError, "CONTENT-1", "cron entries need a server block, a plugin without one has no process")
	}
	if len(m.Events) > 0 {
		report.add(LevelError, "CONTENT-1", "events need a server block, a plugin without one has no process")
	}
}

// lintCapabilities checks capabilities (MAN-19/MAN-20) and their blocks.
func lintCapabilities(m *protocol.Manifest, report *LintReport) {
	seen := make(map[string]struct{}, len(m.Capabilities))
	for _, capability := range m.Capabilities {
		if !slices.Contains(knownCapabilities, capability) {
			report.add(LevelError, "MAN-19", "unknown capability %q", capability)
			continue
		}
		if _, dup := seen[capability]; dup {
			report.add(LevelError, "MAN-20", "capability %q is declared twice", capability)
		}
		seen[capability] = struct{}{}
	}

	if _, ok := seen[protocol.CapabilityDNS01]; ok {
		lintDNS01(m.DNS01, report)
	}
	if _, ok := seen[protocol.CapabilityHTTP]; ok {
		if m.HTTP == nil {
			report.add(LevelError, "MAN-22", "capability http requires the http block")
		} else if !slices.Contains(knownHTTPListenModes, m.HTTP.Listen) {
			report.add(LevelError, "MAN-22", "http.listen %q must be one of %s", m.HTTP.Listen, strings.Join(knownHTTPListenModes, ", "))
		}
	}
	if _, ok := seen[protocol.CapabilityNotify]; ok {
		lintNotify(m.Notify, report)
		lintNetworkPermission(m, protocol.CapabilityNotify, report)
	}
	if _, ok := seen[protocol.CapabilityProbe]; ok {
		lintProbe(m.Probe, report)
		lintNetworkPermission(m, protocol.CapabilityProbe, report)
	}
	_, hasMCP := seen[protocol.CapabilityMCP]
	if hasMCP {
		lintMCP(m, report)
	} else if slices.Contains(m.Permissions, protocol.PermissionMCP) {
		report.add(LevelWarning, "SEC-5", "permission %q is requested but capability %q is not declared, so it grants nothing", protocol.PermissionMCP, protocol.CapabilityMCP)
	}
	if _, ok := seen[protocol.CapabilityStorage]; ok {
		lintStorage(m.Storage, report)
		lintNetworkPermission(m, protocol.CapabilityStorage, report)
	}
	if _, ok := seen[protocol.CapabilityCertDeploy]; ok {
		lintDeploy(m, report)
		lintNetworkPermission(m, protocol.CapabilityCertDeploy, report)
	} else if slices.Contains(m.Permissions, protocol.PermissionCertDeploy) {
		report.add(LevelWarning, "SEC-5", "permission %q is requested but capability %q is not declared, so it grants nothing", protocol.PermissionCertDeploy, protocol.CapabilityCertDeploy)
	}
	if _, ok := seen[protocol.CapabilitySecurityBlocklist]; ok {
		lintBlocklist(m, report)
	}
	if _, ok := seen[protocol.CapabilityUpstreamDiscovery]; ok {
		lintDiscovery(m, report)
	}
	_, hasLogSink := seen[protocol.CapabilityLogSink]
	if hasLogSink && !slices.Contains(m.Permissions, protocol.PermissionLogRead) {
		report.add(LevelError, "MAN-38", "capability log.sink requires the %q permission", protocol.PermissionLogRead)
	}
	if !hasLogSink && slices.Contains(m.Permissions, protocol.PermissionLogRead) {
		report.add(LevelWarning, "SEC-5", "permission %q is requested but capability %q is not declared, so it grants nothing", protocol.PermissionLogRead, protocol.CapabilityLogSink)
	}
	if !hasLogSink && m.LogSink != nil {
		report.add(LevelWarning, "LOGSINK-1", "the log_sink block has no effect without capability %q", protocol.CapabilityLogSink)
	}
	lintLogSink(m.LogSink, report)
}

// lintLogSink checks the optional log_sink block (LOGSINK-2, LOGSINK-3).
func lintLogSink(l *protocol.ManifestLogSink, report *LintReport) {
	if l == nil {
		return
	}
	if !validLogSinkBatchSize(l.BatchSize) {
		report.add(LevelError, "LOGSINK-2", "log_sink.batch_size %d must be between 0 and %d", l.BatchSize, protocol.MaxLogSinkBatchSize)
	}
	if !validLogSinkFlushInterval(l.FlushIntervalMS) {
		report.add(LevelError, "LOGSINK-2", "log_sink.flush_interval_ms %d must be 0 or at least %d", l.FlushIntervalMS, protocol.MinLogSinkFlushIntervalMS)
	}
	seen := make(map[string]struct{}, len(l.Formats))
	for _, format := range l.Formats {
		if !slices.Contains(knownLogFormats, format) {
			report.add(LevelError, "LOGSINK-3", "log_sink.formats: unknown format %q, want one of %s", format, strings.Join(knownLogFormats, ", "))
		} else if _, dup := seen[format]; dup {
			report.add(LevelError, "LOGSINK-3", "log_sink.formats: %q is listed twice", format)
		}
		seen[format] = struct{}{}
	}
}

// lintBlocklist checks blocklist.sources and the network permission
// (MAN-36, BLOCKLIST-2 and BLOCKLIST-3).
func lintBlocklist(m *protocol.Manifest, report *LintReport) {
	if !slices.Contains(m.Permissions, protocol.PermissionNetwork) {
		report.add(LevelError, "MAN-36", "capability security.blocklist requires the %q permission", protocol.PermissionNetwork)
	}
	if m.Blocklist == nil || len(m.Blocklist.Sources) == 0 {
		report.add(LevelError, "MAN-36", "capability security.blocklist requires at least one source")
		return
	}
	seen := make(map[string]struct{}, len(m.Blocklist.Sources))
	for _, source := range m.Blocklist.Sources {
		if !capabilityCodePattern.MatchString(source.Code) {
			report.add(LevelError, "BLOCKLIST-2", "blocklist source code %q must match %s", source.Code, capabilityCodePattern)
		} else if _, dup := seen[source.Code]; dup {
			report.add(LevelError, "BLOCKLIST-2", "blocklist source code %q is declared twice", source.Code)
		}
		seen[source.Code] = struct{}{}
		if source.Name == "" {
			report.add(LevelError, "BLOCKLIST-3", "blocklist source %q is missing a name", source.Code)
		}
		if !validBlocklistRefresh(source.RefreshSeconds) {
			report.add(LevelError, "BLOCKLIST-3", "blocklist source %q refresh_seconds %d must be 0 or at least %d",
				source.Code, source.RefreshSeconds, protocol.MinBlocklistRefreshSeconds)
		}
		lintConfigurationSchema(fmt.Sprintf("blocklist source %q", source.Code), "BLOCKLIST-3", source.Configuration, report)
	}
}

// lintDiscovery checks discovery.providers and the network permission
// (MAN-37, DISCOVERY-2 and DISCOVERY-3).
func lintDiscovery(m *protocol.Manifest, report *LintReport) {
	if !slices.Contains(m.Permissions, protocol.PermissionNetwork) {
		report.add(LevelError, "MAN-37", "capability upstream.discovery requires the %q permission", protocol.PermissionNetwork)
	}
	if m.Discovery == nil || len(m.Discovery.Providers) == 0 {
		report.add(LevelError, "MAN-37", "capability upstream.discovery requires at least one provider")
		return
	}
	seen := make(map[string]struct{}, len(m.Discovery.Providers))
	for _, provider := range m.Discovery.Providers {
		if !capabilityCodePattern.MatchString(provider.Code) {
			report.add(LevelError, "DISCOVERY-2", "discovery provider code %q must match %s", provider.Code, capabilityCodePattern)
		} else if _, dup := seen[provider.Code]; dup {
			report.add(LevelError, "DISCOVERY-2", "discovery provider code %q is declared twice", provider.Code)
		}
		seen[provider.Code] = struct{}{}
		if provider.Name == "" {
			report.add(LevelError, "DISCOVERY-3", "discovery provider %q is missing a name", provider.Code)
		}
		lintConfigurationSchema(fmt.Sprintf("discovery provider %q", provider.Code), "DISCOVERY-3", provider.Configuration, report)
	}
}

// lintStorage checks storage.backends (MAN-34, STORAGE-2 and STORAGE-3).
func lintStorage(s *protocol.ManifestStorage, report *LintReport) {
	if s == nil || len(s.Backends) == 0 {
		report.add(LevelError, "MAN-34", "capability storage requires at least one backend")
		return
	}
	seen := make(map[string]struct{}, len(s.Backends))
	for _, b := range s.Backends {
		if !capabilityCodePattern.MatchString(b.Code) {
			report.add(LevelError, "STORAGE-2", "storage backend code %q must match %s", b.Code, capabilityCodePattern)
		} else if _, dup := seen[b.Code]; dup {
			report.add(LevelError, "STORAGE-2", "storage backend code %q is declared twice", b.Code)
		}
		seen[b.Code] = struct{}{}
		if b.Name == "" {
			report.add(LevelError, "STORAGE-3", "storage backend %q is missing a name", b.Code)
		}
		lintConfigurationSchema(fmt.Sprintf("storage backend %q", b.Code), "STORAGE-3", b.Configuration, report)
	}
}

// lintDeploy checks deploy.targets and the cert.deploy permission (MAN-35,
// DEPLOY-2 and DEPLOY-3).
func lintDeploy(m *protocol.Manifest, report *LintReport) {
	if !slices.Contains(m.Permissions, protocol.PermissionCertDeploy) {
		report.add(LevelError, "MAN-35", "capability cert.deploy requires the %q permission", protocol.PermissionCertDeploy)
	}
	if m.Deploy == nil || len(m.Deploy.Targets) == 0 {
		report.add(LevelError, "MAN-35", "capability cert.deploy requires at least one target")
		return
	}
	seen := make(map[string]struct{}, len(m.Deploy.Targets))
	for _, t := range m.Deploy.Targets {
		if !capabilityCodePattern.MatchString(t.Code) {
			report.add(LevelError, "DEPLOY-2", "deploy target code %q must match %s", t.Code, capabilityCodePattern)
		} else if _, dup := seen[t.Code]; dup {
			report.add(LevelError, "DEPLOY-2", "deploy target code %q is declared twice", t.Code)
		}
		seen[t.Code] = struct{}{}
		if t.Name == "" {
			report.add(LevelError, "DEPLOY-3", "deploy target %q is missing a name", t.Code)
		}
		lintConfigurationSchema(fmt.Sprintf("deploy target %q", t.Code), "DEPLOY-3", t.Configuration, report)
	}
}

// lintNotify checks notify.channels (MAN-31, NOTIFY-2 through NOTIFY-4).
func lintNotify(n *protocol.ManifestNotify, report *LintReport) {
	if n == nil || len(n.Channels) == 0 {
		report.add(LevelError, "MAN-31", "capability notify requires at least one channel")
		return
	}
	seen := make(map[string]struct{}, len(n.Channels))
	for _, c := range n.Channels {
		if !capabilityCodePattern.MatchString(c.Code) {
			report.add(LevelError, "NOTIFY-2", "notify channel code %q must match %s", c.Code, capabilityCodePattern)
		} else if _, dup := seen[c.Code]; dup {
			report.add(LevelError, "NOTIFY-2", "notify channel code %q is declared twice", c.Code)
		}
		seen[c.Code] = struct{}{}
		if c.Name == "" {
			report.add(LevelError, "NOTIFY-3", "notify channel %q is missing a name", c.Code)
		}
		lintConfigurationSchema(fmt.Sprintf("notify channel %q", c.Code), "NOTIFY-4", c.Configuration, report)
	}
}

// lintProbe checks probe.kinds (MAN-32, PROBE-2 and PROBE-3).
func lintProbe(p *protocol.ManifestProbe, report *LintReport) {
	if p == nil || len(p.Kinds) == 0 {
		report.add(LevelError, "MAN-32", "capability probe requires at least one kind")
		return
	}
	seen := make(map[string]struct{}, len(p.Kinds))
	for _, k := range p.Kinds {
		if !capabilityCodePattern.MatchString(k.Code) {
			report.add(LevelError, "PROBE-2", "probe kind code %q must match %s", k.Code, capabilityCodePattern)
		} else if _, dup := seen[k.Code]; dup {
			report.add(LevelError, "PROBE-2", "probe kind code %q is declared twice", k.Code)
		}
		seen[k.Code] = struct{}{}
		if k.Name == "" {
			report.add(LevelError, "PROBE-3", "probe kind %q is missing a name", k.Code)
		}
		lintConfigurationSchema(fmt.Sprintf("probe kind %q", k.Code), "PROBE-3", k.Configuration, report)
	}
}

// lintConfigurationSchema checks the form of a capability entry (NOTIFY-4,
// which PROBE-3, STORAGE-3, DEPLOY-3, BLOCKLIST-3 and DISCOVERY-3 refer to).
func lintConfigurationSchema(owner, rule string, schema *protocol.ConfigurationSchema, report *LintReport) {
	if schema == nil {
		return
	}
	seen := make(map[string]struct{}, len(schema.Fields))
	for _, field := range schema.Fields {
		if field.Key == "" {
			report.add(LevelError, rule, "%s has a configuration field without a key", owner)
			continue
		}
		if _, dup := seen[field.Key]; dup {
			report.add(LevelError, rule, "%s declares the configuration key %q twice", owner, field.Key)
		}
		seen[field.Key] = struct{}{}
		if !slices.Contains(knownConfigurationFieldTypes, field.Type) {
			report.add(LevelError, rule, "%s configuration key %q has unknown type %q", owner, field.Key, field.Type)
		}
		if field.DisplayName == "" {
			report.add(LevelError, rule, "%s configuration key %q needs a display_name", owner, field.Key)
		}
	}
}

// lintNetworkPermission warns when a capability that talks to the outside
// world does not tell the person installing it so (SEC-3).
func lintNetworkPermission(m *protocol.Manifest, capability string, report *LintReport) {
	if !slices.Contains(m.Permissions, protocol.PermissionNetwork) {
		report.add(LevelWarning, "SEC-3", "capability %s usually reaches a vendor or a target, but permission %q is not requested", capability, protocol.PermissionNetwork)
	}
}

// lintMCP checks mcp.tools and the mcp permission (MAN-33, MCP-2, MCP-3).
func lintMCP(m *protocol.Manifest, report *LintReport) {
	if !slices.Contains(m.Permissions, protocol.PermissionMCP) {
		report.add(LevelError, "MAN-33", "capability mcp requires the %q permission", protocol.PermissionMCP)
	}
	if m.MCP == nil || len(m.MCP.Tools) == 0 {
		report.add(LevelError, "MAN-33", "capability mcp requires at least one tool")
		return
	}
	seen := make(map[string]struct{}, len(m.MCP.Tools))
	for _, tool := range m.MCP.Tools {
		if !mcpToolNamePattern.MatchString(tool.Name) {
			report.add(LevelError, "MCP-2", "mcp tool name %q must match %s", tool.Name, mcpToolNamePattern)
		} else if _, dup := seen[tool.Name]; dup {
			report.add(LevelError, "MCP-2", "mcp tool %q is declared twice", tool.Name)
		}
		seen[tool.Name] = struct{}{}
		if tool.Description == "" {
			report.add(LevelError, "MCP-2", "mcp tool %q needs a description", tool.Name)
		}
		if !isObjectSchema(tool.InputSchema) {
			report.add(LevelError, "MCP-3", "mcp tool %q input_schema must have type \"object\"", tool.Name)
		}
	}
}

// lintDNS01 checks dns01.providers (MAN-21, DNS01-1 through DNS01-3).
func lintDNS01(d *protocol.ManifestDNS01, report *LintReport) {
	if d == nil || len(d.Providers) == 0 {
		report.add(LevelError, "MAN-21", "capability dns01 requires at least one provider")
		return
	}
	seen := make(map[string]struct{}, len(d.Providers))
	for _, p := range d.Providers {
		if p.Name == "" {
			report.add(LevelError, "DNS01-3", "dns01 provider %q is missing a name", p.Code)
		}
		if !dns01CodePattern.MatchString(p.Code) {
			report.add(LevelError, "DNS01-2", "dns01 provider code %q must match %s", p.Code, dns01CodePattern)
		} else if _, dup := seen[p.Code]; dup {
			report.add(LevelError, "DNS01-2", "dns01 provider code %q is declared twice", p.Code)
		}
		seen[p.Code] = struct{}{}

		hasField := p.Configuration != nil && (len(p.Configuration.Credentials) > 0 || len(p.Configuration.Additional) > 0)
		if !hasField {
			report.add(LevelWarning, "DNS01-1", "dns01 provider %q declares no credential or additional field for the person installing it to fill in", p.Code)
		}
	}
}

// lintPermissions checks permissions (MAN-23).
func lintPermissions(permissions []string, report *LintReport) {
	seen := make(map[string]struct{}, len(permissions))
	for _, permission := range permissions {
		if _, dup := seen[permission]; dup {
			report.add(LevelWarning, "MAN-23", "permission %q is declared twice", permission)
		}
		seen[permission] = struct{}{}

		if slices.Contains(knownPermissions, permission) {
			continue
		}
		if kind, ok := strings.CutPrefix(permission, protocol.PermissionCredentialsReadPrefix); ok && kind != "" {
			continue
		}
		report.add(LevelError, "MAN-23", "unknown permission %q", permission)
	}
}

// lintRequires checks that every dependency id is well formed (MAN-24).
func lintRequires(requires []protocol.ManifestRequirement, report *LintReport) {
	for _, r := range requires {
		if !IsValidID(r.ID) {
			report.add(LevelError, "MAN-24", "requires[].id %q is not a valid plugin id", r.ID)
		}
	}
}

// lintSettingsSchema checks settings_schema (MAN-27 through MAN-29) and, as
// an extension ValidateManifest does not perform, that a default value's
// JSON type is compatible with the field's declared type.
func lintSettingsSchema(schema *protocol.SettingsSchema, report *LintReport) {
	if schema == nil {
		return
	}
	seen := make(map[string]struct{}, len(schema.Settings))
	for _, field := range schema.Settings {
		if field.Key == "" {
			report.add(LevelError, "MAN-27", "settings_schema field key is required")
			continue
		}
		if _, dup := seen[field.Key]; dup {
			report.add(LevelError, "MAN-27", "settings_schema key %q is declared twice", field.Key)
		}
		seen[field.Key] = struct{}{}

		if !slices.Contains(knownSettingsTypes, field.Type) {
			report.add(LevelError, "MAN-28", "settings_schema key %q has unknown type %q", field.Key, field.Type)
			continue
		}
		if field.Type == "select" && len(field.Options) == 0 {
			report.add(LevelError, "MAN-29", "settings_schema key %q needs options", field.Key)
		}
		if field.Default != nil && !settingsDefaultMatchesType(field.Type, field.Default) {
			report.add(LevelWarning, "MAN-28", "settings_schema key %q default %v does not look like a %q value", field.Key, field.Default, field.Type)
		}
	}
}

// settingsDefaultMatchesType checks the JSON type json.Unmarshal would have
// produced for "default" against the field's declared type.
func settingsDefaultMatchesType(fieldType string, value any) bool {
	switch fieldType {
	case "bool":
		_, ok := value.(bool)
		return ok
	case "number":
		switch value.(type) {
		case float64, int, int64:
			return true
		}
		return false
	default: // text, secret, textarea, select
		_, ok := value.(string)
		return ok
	}
}

// lintDocs checks PKG-8 (README.md/LICENSE/CHANGELOG.md presence) and warns
// when README.md is missing the sections every plugin author should fill in.
func lintDocs(dir string, report *LintReport) {
	lintRequiredDoc(dir, lintReadmeName, LevelError, report)
	lintRequiredDoc(dir, "LICENSE", LevelWarning, report)
	lintRequiredDoc(dir, "CHANGELOG.md", LevelWarning, report)

	data, err := os.ReadFile(filepath.Join(dir, lintReadmeName))
	if err != nil {
		return
	}
	text := string(data)
	for _, section := range []string{"Permissions", "Supported platforms"} {
		if !strings.Contains(strings.ToLower(text), strings.ToLower(section)) {
			report.add(LevelWarning, "PKG-8", "%s is missing a %q section", lintReadmeName, section)
		}
	}
}

func lintRequiredDoc(dir, name string, level Level, report *LintReport) {
	if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
		report.add(level, "PKG-8", "%s is missing from the package root", name)
	}
}

// lintDirLimits walks a plain plugin directory and enforces the same limits
// walkPackage enforces on an archive (PKG-4, PKG-6, PKG-7), since a
// directory input skips packaging entirely.
func lintDirLimits(dir string, report *LintReport) {
	var (
		files int
		total int64
	)
	_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == dir {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			rel, _ := filepath.Rel(dir, path)
			report.add(LevelError, "PKG-4", "entry %q is a symlink", rel)
		}
		files++
		if info.Mode().IsRegular() {
			total += info.Size()
		}
		return nil
	})
	if files > MaxPackageFiles {
		report.add(LevelError, "PKG-6", "the directory contains %d entries, more than the %d limit", files, MaxPackageFiles)
	}
	if total > MaxPackageSize {
		report.add(LevelError, "PKG-7", "the directory payload is %d bytes, more than the %d byte limit", total, MaxPackageSize)
	}
}

// lintExtractArchive extracts path into a fresh temporary directory for
// linting. Unlike ExtractPackage it does not validate or load the manifest
// itself (Lint wants to inspect the manifest and report every issue, not
// abort on the first one), but it reuses the same package walker so PKG-2
// through PKG-7 are enforced identically. Any violation becomes a Finding
// instead of a hard failure; ok is false when nothing could be extracted at
// all, in which case the caller has nothing left to lint.
func lintExtractArchive(archivePath string, report *LintReport) (dir string, ok bool) {
	tempDir, err := os.MkdirTemp("", "nginx-ui-plugin-lint-*")
	if err != nil {
		report.add(LevelError, "PKG-2", "cannot create a temporary directory: %v", err)
		return "", false
	}

	prefix, err := packagePrefix(archivePath)
	if err != nil {
		report.add(LevelError, packageErrorRule(err), "%v", err)
		_ = os.RemoveAll(tempDir)
		return "", false
	}
	root, err := filepath.Abs(tempDir)
	if err != nil {
		report.add(LevelError, "PKG-2", "%v", err)
		_ = os.RemoveAll(tempDir)
		return "", false
	}

	err = walkPackage(archivePath, func(header *tar.Header, name string, reader io.Reader) error {
		rel, inside := strings.CutPrefix(name, prefix)
		if !inside || rel == "" {
			return nil
		}
		target := filepath.Join(root, filepath.FromSlash(rel))
		if !isInside(root, target) {
			return invalidPackage("entry %q escapes the destination", header.Name)
		}
		switch header.Typeflag {
		case tar.TypeDir:
			return os.MkdirAll(target, 0o755)
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			// Keep the packed mode so PKG-9 can report a missing executable bit.
			return writePackageFile(target, header, reader, packedMode(header))
		default:
			return invalidPackage("entry %q has an unsupported type", header.Name)
		}
	})
	if err != nil {
		report.add(LevelError, packageErrorRule(err), "%v", err)
		_ = os.RemoveAll(tempDir)
		return "", false
	}
	return tempDir, true
}

// packageErrorRule maps a package walker error onto the spec rule it
// violates, using the sentinel error where one exists and the message
// otherwise.
func packageErrorRule(err error) string {
	switch {
	case errors.Is(err, ErrPackageTooLarge):
		if strings.Contains(err.Error(), "entries") {
			return "PKG-6"
		}
		return "PKG-7"
	case strings.Contains(err.Error(), "is a link"):
		return "PKG-4"
	case strings.Contains(err.Error(), "escapes"):
		return "PKG-5"
	case strings.Contains(err.Error(), ManifestFileName):
		return "PKG-2"
	default:
		return "PKG-3"
	}
}

// lintSums checks the embedded signature files: plugin.sums has to follow
// PKG-19 and match the files (PKG-21) whether or not the package is signed,
// both files belong together (PKG-20), and a signature the keys pinned in
// this binary do not verify is only a warning (SEC-18), since a community
// key is named by a catalog entry or an operator.
func lintSums(dir string, report *LintReport) {
	sums, err := readRootFile(dir, SumsFileName)
	if err != nil {
		report.add(LevelError, "PKG-19", "read %s: %v", SumsFileName, err)
		return
	}
	signature, err := readRootFile(dir, SumsSignatureFileName)
	if err != nil {
		report.add(LevelError, "PKG-20", "read %s: %v", SumsSignatureFileName, err)
		return
	}

	switch {
	case sums == nil && signature == nil:
		return
	case sums == nil:
		report.add(LevelWarning, "PKG-20", "%s is present without %s, the package is unsigned", SumsSignatureFileName, SumsFileName)
		return
	case signature == nil:
		report.add(LevelWarning, "PKG-20", "%s is present without %s, the package is unsigned", SumsFileName, SumsSignatureFileName)
	}

	if listed, err := parseSums(sums); err != nil {
		report.add(LevelError, "PKG-19", "%v", err)
	} else if err = matchSums(dir, listed); err != nil {
		report.add(LevelError, "PKG-21", "%v", err)
	}
	if signature == nil {
		return
	}

	trust, err := signatureTrust(sums, signature, pinnedTiers())
	switch {
	case err != nil:
		report.add(LevelError, "PKG-21", "%s: %v", SumsSignatureFileName, err)
	case trust.Trust == TrustUnsigned:
		report.add(LevelWarning, "SEC-18", "%s does not verify with the release or partner keys pinned in this binary (expected for a community plugin)", SumsSignatureFileName)
	}
}
