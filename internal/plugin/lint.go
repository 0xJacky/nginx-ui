package plugin

// This file implements a static linter for a plugin directory or a packaged
// .tar.gz, used by "nginx-ui plugin lint". It mirrors the checks ValidateManifest
// and ExtractPackage apply, but collects every issue instead of stopping at
// the first one, and adds filesystem level checks (missing executables,
// missing docs, package limits) that those functions do not perform.
//
// Every Finding carries the name of the rule it breaks, e.g. RuleManifestID
// or RulePackageDocs. The plugin development guide explains every rule, see
// RulesURL.

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

	"github.com/0xJacky/Nginx-UI/internal/pkgsign"
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
	lintSums(dir, lintPartnerCertificate(dir, report), report)

	manifest, err := LoadManifest(dir)
	if err != nil {
		report.add(LevelError, RuleManifestJSON, "%v", err)
		return report, nil
	}

	lintIdentity(manifest, report)
	lintI18n(manifest.I18n, report)
	if archiveName != "" {
		lintPackageName(archiveName, manifest, report)
	}
	if manifest.Server == nil && manifest.Webapp == nil && manifest.Content == nil {
		report.add(LevelError, RuleManifestParts, "at least one of server, webapp or content is required")
	}
	lintServer(manifest.Server, dir, report)
	lintWebapp(manifest.Webapp, dir, report)
	lintContent(manifest, dir, report)
	lintProcessless(manifest, report)
	lintCapabilities(manifest, report)
	lintEvents(manifest, report)
	lintPermissions(manifest.Permissions, report)
	lintRequires(manifest.Requires, report)
	lintConflicts(manifest, report)
	lintSettingsSchema(manifest.SettingsSchema, report)
	lintDocs(dir, report)

	return report, nil
}

// lintIdentity checks the top level scalar fields.
func lintIdentity(m *protocol.Manifest, report *LintReport) {
	switch {
	case m.ID == "":
		report.add(LevelError, RuleManifestID, "id is required")
	case len(m.ID) > maxPluginIDLength:
		report.add(LevelError, RuleManifestID, "id must be at most %d characters", maxPluginIDLength)
	case !pluginIDPattern.MatchString(m.ID):
		report.add(LevelError, RuleManifestID, "id %q must look like \"vendor.name\" (%s)", m.ID, pluginIDPattern)
	case IsReservedID(m.ID):
		// This CLI has no allowlist of officially maintained plugin ids, so
		// every com.nginxui.* id it sees is treated as unverified.
		report.add(LevelWarning, RuleReservedNamespace, "id %q uses the reserved com.nginxui.* namespace; this tool cannot confirm official ownership", m.ID)
	}

	if m.Name == "" {
		report.add(LevelError, RuleManifestName, "name is required")
	}

	switch {
	case m.Version == "":
		report.add(LevelError, RuleManifestVersion, "version is required")
	case !semverPattern.MatchString(m.Version):
		report.add(LevelError, RuleManifestVersion, "version %q is not a semantic version", m.Version)
	}

	switch {
	case m.APIVersion == 0:
		report.add(LevelError, RuleManifestAPIVersion, "api_version is required")
	case m.APIVersion != protocol.APIVersion:
		report.add(LevelError, RuleManifestAPIVersion, "api_version %d is not supported by this host (supports %d)", m.APIVersion, protocol.APIVersion)
	}

	if m.IconPath != "" && !isSafeRelPath(m.IconPath) {
		report.add(LevelError, RuleManifestIcon, "icon_path %q must be a safe relative path", m.IconPath)
	}
}

// lintI18n checks that every key of the i18n block is a language of the
// host.
func lintI18n(i18n map[string]protocol.ManifestI18n, report *LintReport) {
	for _, locale := range slices.Sorted(maps.Keys(i18n)) {
		if !translation.IsLanguage(locale) {
			report.add(LevelError, RuleManifestI18n, "i18n: %q is not a language of the host (%s)",
				locale, strings.Join(translation.Languages(), ", "))
		}
	}
}

// lintServer checks the server block and, unlike
// ValidateManifest, also verifies every declared executable actually exists.
func lintServer(s *protocol.ManifestServer, dir string, report *LintReport) {
	if s == nil {
		return
	}

	switch s.Lifecycle {
	case "", protocol.LifecycleResident, protocol.LifecycleOnDemand:
	default:
		report.add(LevelError, RuleServerLifecycle, "server.lifecycle %q must be %q or %q", s.Lifecycle, protocol.LifecycleResident, protocol.LifecycleOnDemand)
	}
	if s.IdleTimeoutSeconds < 0 {
		report.add(LevelError, RuleServerIdleTimeout, "server.idle_timeout_seconds must not be negative")
	}
	if r := s.Resources; r != nil {
		if r.MemoryMB < 0 {
			report.add(LevelError, RuleServerResources, "server.resources.memory_mb must not be negative")
		}
		if r.RecommendedMemoryMB < 0 {
			report.add(LevelError, RuleServerResources, "server.resources.recommended_memory_mb must not be negative")
		}
		if r.CPUPercent < 0 {
			report.add(LevelError, RuleServerResources, "server.resources.cpu_percent must not be negative")
		}
	}
	if len(s.Executables) == 0 && len(s.Command) == 0 {
		report.add(LevelError, RuleServerEntry, "server needs executables or command")
	}

	platforms := make([]string, 0, len(s.Executables))
	for platform := range s.Executables {
		platforms = append(platforms, platform)
	}
	sort.Strings(platforms)
	for _, platform := range platforms {
		rel := s.Executables[platform]
		if !isSafeRelPath(rel) {
			report.add(LevelError, RuleServerPaths, "server.executables[%q] %q must be a safe relative path", platform, rel)
			continue
		}
		lintExecutableFile(filepath.Join(dir, filepath.FromSlash(rel)), platform, report)
	}

	if len(s.Command) == 0 {
		return
	}
	switch {
	case s.Command[0] == "":
		report.add(LevelError, RuleServerCommand, "server.command[0] is empty")
	case hasPathSeparator(s.Command[0]):
		if !isSafeRelPath(s.Command[0]) {
			report.add(LevelError, RuleServerPaths, "server.command[0] %q must be a safe relative path", s.Command[0])
		} else {
			lintExecutableFile(filepath.Join(dir, filepath.FromSlash(s.Command[0])), "command", report)
		}
	default:
		if _, err := exec.LookPath(s.Command[0]); err != nil {
			report.add(LevelWarning, RuleServerPaths, "server.command[0] %q was not found on PATH: %v", s.Command[0], err)
		}
	}
}

// lintPackageName checks an archive file name against its manifest. A name
// that follows neither package form (an upload saved under another name) is
// not checked at all. The id and version in the name have to match the
// manifest, and a per-platform package has to declare exactly the platform
// its name carries, so the platform a catalog serves it for is the one it
// runs on. Every declared executable still has to exist.
func lintPackageName(name string, m *protocol.Manifest, report *LintReport) {
	parsed, ok := ParsePackageFileName(name)
	if !ok {
		return
	}
	if parsed.ID != m.ID || parsed.Version != m.Version {
		report.add(LevelWarning, RulePackageFileName, "file name %s does not match the manifest id %q and version %q", name, m.ID, m.Version)
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
		report.add(LevelError, RulePackagePlatform, "file name %s targets %s, but server.executables declares [%s]; a per-platform package must declare exactly its own platform",
			name, parsed.Platform, strings.Join(declared, ", "))
	}
}

// lintExecutableFile checks that a resolved executable path exists and, on
// Unix, warns when it is not already marked executable. The host sets the
// bit itself on install, so a missing bit is a warning, not an error.
func lintExecutableFile(path, platform string, report *LintReport) {
	info, err := os.Stat(path)
	switch {
	case os.IsNotExist(err):
		report.add(LevelError, RulePackageExecutables, "executable for %s is missing: %s", platform, path)
		return
	case err != nil:
		report.add(LevelError, RulePackageExecutables, "cannot stat the executable for %s: %v", platform, err)
		return
	case !info.Mode().IsRegular():
		report.add(LevelError, RulePackageExecutables, "executable for %s is not a regular file: %s", platform, path)
		return
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0o111 == 0 {
		report.add(LevelWarning, RulePackageExecutables, "executable for %s does not have the executable bit set: %s", platform, path)
	}
}

// lintWebapp checks the webapp block and that every file it
// points at exists.
func lintWebapp(w *protocol.ManifestWebapp, dir string, report *LintReport) {
	if w == nil {
		return
	}
	if w.BundlePath != "" {
		if !isSafeRelPath(w.BundlePath) {
			report.add(LevelError, RuleWebappPaths, "webapp.bundle_path %q must be a safe relative path", w.BundlePath)
		} else {
			lintFileExists(filepath.Join(dir, filepath.FromSlash(w.BundlePath)), RuleWebappPaths, "webapp.bundle_path", report)
		}
	}
	if w.StylePath != "" {
		if !isSafeRelPath(w.StylePath) {
			report.add(LevelError, RuleWebappPaths, "webapp.style_path %q must be a safe relative path", w.StylePath)
		} else {
			lintFileExists(filepath.Join(dir, filepath.FromSlash(w.StylePath)), RuleWebappPaths, "webapp.style_path", report)
		}
	}
	lintChunks(w, dir, report)
	for _, p := range w.Pages {
		if p.Path == "" {
			report.add(LevelError, RuleWebappPages, "webapp.pages[].path is required")
		}
		if !isSafeRelPath(p.File) {
			report.add(LevelError, RuleWebappPages, "webapp.pages[%q].file %q must be a safe relative path", p.Path, p.File)
			continue
		}
		lintFileExists(filepath.Join(dir, filepath.FromSlash(p.File)), RuleWebappPages, fmt.Sprintf("webapp.pages[%q].file", p.Path), report)
	}
}

// lintChunks checks webapp.chunks: the declaration itself and
// that every chunk file is in the package and not empty. A chunk outside the
// directory of the bundle is not reachable through the webapp route of this
// host, which is reported as a warning.
func lintChunks(w *protocol.ManifestWebapp, dir string, report *LintReport) {
	if len(w.Chunks) == 0 {
		return
	}
	if problem := chunksProblem(w); problem != "" {
		report.add(LevelError, RuleWebappChunks, "%s", problem)
	}

	names := make([]string, 0, len(w.Chunks))
	for name := range w.Chunks {
		names = append(names, name)
	}
	sort.Strings(names)

	root := webappDir(&protocol.Manifest{Webapp: w})
	for _, name := range names {
		file := w.Chunks[name]
		if !isSafeRelPath(file) {
			continue
		}
		what := fmt.Sprintf("webapp.chunks[%q]", name)
		full := filepath.Join(dir, filepath.FromSlash(file))
		if info, err := os.Stat(full); err == nil && info.Mode().IsRegular() && info.Size() == 0 {
			report.add(LevelError, RuleWebappChunkFiles, "%s points at an empty file: %s", what, file)
		} else {
			lintFileExists(full, RuleWebappChunks, what, report)
		}
		if w.BundlePath != "" && root != "" && !strings.HasPrefix(file, root+"/") {
			report.add(LevelWarning, RuleWebappChunkFiles, "%s is outside %s, the directory this host serves the bundle from, so it cannot be loaded", what, root)
		}
	}
}

// lintEvents checks the events array against the permissions that gate an
// event.
func lintEvents(m *protocol.Manifest, report *LintReport) {
	for _, eventType := range m.Events {
		if permission := eventPermission(eventType); permission != "" && !slices.Contains(m.Permissions, permission) {
			report.add(LevelWarning, RuleEventPermission, "event %q is listed without the %q permission, so it is never delivered", eventType, permission)
		}
	}
}

func lintFileExists(path, rule, what string, report *LintReport) {
	if info, err := os.Stat(path); err != nil {
		report.add(LevelError, rule, "%s points at a missing file: %s", what, path)
	} else if info.IsDir() {
		report.add(LevelError, rule, "%s points at a directory, not a file: %s", what, path)
	}
}

// lintContent checks the content block and the templates and translation
// files it points at.
func lintContent(m *protocol.Manifest, dir string, report *LintReport) {
	c := m.Content
	if c == nil {
		return
	}
	if c.Templates != "" && !isSafeRelPath(c.Templates) {
		report.add(LevelError, RuleContentPaths, "content.templates %q must be a safe relative path", c.Templates)
	}
	if c.Locales != "" && !isSafeRelPath(c.Locales) {
		report.add(LevelError, RuleContentPaths, "content.locales %q must be a safe relative path", c.Locales)
	}
	for _, problem := range CheckContent(m, dir) {
		report.add(problem.Level, problem.Rule, "%s", problem.String())
	}
}

// lintProcessless checks that a manifest without a server block declares
// nothing only a process can serve.
func lintProcessless(m *protocol.Manifest, report *LintReport) {
	if m.Server != nil {
		return
	}
	if len(m.Capabilities) > 0 {
		report.add(LevelError, RuleContentProcessLess, "capabilities %v need a server block, a plugin without one has no process", m.Capabilities)
	}
	if len(m.Cron) > 0 {
		report.add(LevelError, RuleContentProcessLess, "cron entries need a server block, a plugin without one has no process")
	}
	if len(m.Events) > 0 {
		report.add(LevelError, RuleContentProcessLess, "events need a server block, a plugin without one has no process")
	}
}

// lintCapabilities checks capabilities and their blocks.
func lintCapabilities(m *protocol.Manifest, report *LintReport) {
	seen := make(map[string]struct{}, len(m.Capabilities))
	for _, capability := range m.Capabilities {
		if !slices.Contains(knownCapabilities, capability) {
			report.add(LevelError, RuleCapabilityName, "unknown capability %q", capability)
			continue
		}
		if _, dup := seen[capability]; dup {
			report.add(LevelError, RuleCapabilityDuplicate, "capability %q is declared twice", capability)
		}
		seen[capability] = struct{}{}
	}

	if _, ok := seen[protocol.CapabilityDNS01]; ok {
		lintDNS01(m.DNS01, report)
	}
	if _, ok := seen[protocol.CapabilityHTTP]; ok {
		if m.HTTP == nil {
			report.add(LevelError, RuleHTTPBlock, "capability http requires the http block")
		} else if !slices.Contains(knownHTTPListenModes, m.HTTP.Listen) {
			report.add(LevelError, RuleHTTPBlock, "http.listen %q must be one of %s", m.HTTP.Listen, strings.Join(knownHTTPListenModes, ", "))
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
		report.add(LevelWarning, RuleUnusedPermission, "permission %q is requested but capability %q is not declared, so it grants nothing", protocol.PermissionMCP, protocol.CapabilityMCP)
	}
	if _, ok := seen[protocol.CapabilityStorage]; ok {
		lintStorage(m.Storage, report)
		lintNetworkPermission(m, protocol.CapabilityStorage, report)
	}
	if _, ok := seen[protocol.CapabilityCertDeploy]; ok {
		lintDeploy(m, report)
		lintNetworkPermission(m, protocol.CapabilityCertDeploy, report)
	} else if slices.Contains(m.Permissions, protocol.PermissionCertDeploy) {
		report.add(LevelWarning, RuleUnusedPermission, "permission %q is requested but capability %q is not declared, so it grants nothing", protocol.PermissionCertDeploy, protocol.CapabilityCertDeploy)
	}
	if _, ok := seen[protocol.CapabilitySecurityBlocklist]; ok {
		lintBlocklist(m, report)
	}
	if _, ok := seen[protocol.CapabilityUpstreamDiscovery]; ok {
		lintDiscovery(m, report)
	}
	_, hasLogSink := seen[protocol.CapabilityLogSink]
	if hasLogSink && !slices.Contains(m.Permissions, protocol.PermissionLogRead) {
		report.add(LevelError, RuleLogSinkPermission, "capability log.sink requires the %q permission", protocol.PermissionLogRead)
	}
	if !hasLogSink && slices.Contains(m.Permissions, protocol.PermissionLogRead) {
		report.add(LevelWarning, RuleUnusedPermission, "permission %q is requested but capability %q is not declared, so it grants nothing", protocol.PermissionLogRead, protocol.CapabilityLogSink)
	}
	if !hasLogSink && m.LogSink != nil {
		report.add(LevelWarning, RuleLogSinkBlock, "the log_sink block has no effect without capability %q", protocol.CapabilityLogSink)
	}
	lintLogSink(m.LogSink, report)
}

// lintLogSink checks the optional log_sink block.
func lintLogSink(l *protocol.ManifestLogSink, report *LintReport) {
	if l == nil {
		return
	}
	if !validLogSinkBatchSize(l.BatchSize) {
		report.add(LevelError, RuleLogSinkBatch, "log_sink.batch_size %d must be between 0 and %d", l.BatchSize, protocol.MaxLogSinkBatchSize)
	}
	if !validLogSinkFlushInterval(l.FlushIntervalMS) {
		report.add(LevelError, RuleLogSinkBatch, "log_sink.flush_interval_ms %d must be 0 or at least %d", l.FlushIntervalMS, protocol.MinLogSinkFlushIntervalMS)
	}
	seen := make(map[string]struct{}, len(l.Formats))
	for _, format := range l.Formats {
		if !slices.Contains(knownLogFormats, format) {
			report.add(LevelError, RuleLogSinkFormats, "log_sink.formats: unknown format %q, want one of %s", format, strings.Join(knownLogFormats, ", "))
		} else if _, dup := seen[format]; dup {
			report.add(LevelError, RuleLogSinkFormats, "log_sink.formats: %q is listed twice", format)
		}
		seen[format] = struct{}{}
	}
}

// lintBlocklist checks blocklist.sources, their forms and the network
// permission.
func lintBlocklist(m *protocol.Manifest, report *LintReport) {
	if !slices.Contains(m.Permissions, protocol.PermissionNetwork) {
		report.add(LevelError, RuleBlocklistBlock, "capability security.blocklist requires the %q permission", protocol.PermissionNetwork)
	}
	if m.Blocklist == nil || len(m.Blocklist.Sources) == 0 {
		report.add(LevelError, RuleBlocklistBlock, "capability security.blocklist requires at least one source")
		return
	}
	seen := make(map[string]struct{}, len(m.Blocklist.Sources))
	for _, source := range m.Blocklist.Sources {
		if !capabilityCodePattern.MatchString(source.Code) {
			report.add(LevelError, RuleBlocklistCode, "blocklist source code %q must match %s", source.Code, capabilityCodePattern)
		} else if _, dup := seen[source.Code]; dup {
			report.add(LevelError, RuleBlocklistCode, "blocklist source code %q is declared twice", source.Code)
		}
		seen[source.Code] = struct{}{}
		if source.Name == "" {
			report.add(LevelError, RuleBlocklistSource, "blocklist source %q is missing a name", source.Code)
		}
		if !validBlocklistRefresh(source.RefreshSeconds) {
			report.add(LevelError, RuleBlocklistSource, "blocklist source %q refresh_seconds %d must be 0 or at least %d",
				source.Code, source.RefreshSeconds, protocol.MinBlocklistRefreshSeconds)
		}
		lintConfigurationSchema(fmt.Sprintf("blocklist source %q", source.Code), RuleBlocklistSource, source.Configuration, report)
	}
}

// lintDiscovery checks discovery.providers, their forms and the network
// permission.
func lintDiscovery(m *protocol.Manifest, report *LintReport) {
	if !slices.Contains(m.Permissions, protocol.PermissionNetwork) {
		report.add(LevelError, RuleDiscoveryBlock, "capability upstream.discovery requires the %q permission", protocol.PermissionNetwork)
	}
	if m.Discovery == nil || len(m.Discovery.Providers) == 0 {
		report.add(LevelError, RuleDiscoveryBlock, "capability upstream.discovery requires at least one provider")
		return
	}
	seen := make(map[string]struct{}, len(m.Discovery.Providers))
	for _, provider := range m.Discovery.Providers {
		if !capabilityCodePattern.MatchString(provider.Code) {
			report.add(LevelError, RuleDiscoveryCode, "discovery provider code %q must match %s", provider.Code, capabilityCodePattern)
		} else if _, dup := seen[provider.Code]; dup {
			report.add(LevelError, RuleDiscoveryCode, "discovery provider code %q is declared twice", provider.Code)
		}
		seen[provider.Code] = struct{}{}
		if provider.Name == "" {
			report.add(LevelError, RuleDiscoveryProvider, "discovery provider %q is missing a name", provider.Code)
		}
		lintConfigurationSchema(fmt.Sprintf("discovery provider %q", provider.Code), RuleDiscoveryProvider, provider.Configuration, report)
	}
}

// lintStorage checks storage.backends and their forms.
func lintStorage(s *protocol.ManifestStorage, report *LintReport) {
	if s == nil || len(s.Backends) == 0 {
		report.add(LevelError, RuleStorageBlock, "capability storage requires at least one backend")
		return
	}
	seen := make(map[string]struct{}, len(s.Backends))
	for _, b := range s.Backends {
		if !capabilityCodePattern.MatchString(b.Code) {
			report.add(LevelError, RuleStorageCode, "storage backend code %q must match %s", b.Code, capabilityCodePattern)
		} else if _, dup := seen[b.Code]; dup {
			report.add(LevelError, RuleStorageCode, "storage backend code %q is declared twice", b.Code)
		}
		seen[b.Code] = struct{}{}
		if b.Name == "" {
			report.add(LevelError, RuleStorageBackend, "storage backend %q is missing a name", b.Code)
		}
		lintConfigurationSchema(fmt.Sprintf("storage backend %q", b.Code), RuleStorageBackend, b.Configuration, report)
	}
}

// lintDeploy checks deploy.targets and the cert.deploy permission.
func lintDeploy(m *protocol.Manifest, report *LintReport) {
	if !slices.Contains(m.Permissions, protocol.PermissionCertDeploy) {
		report.add(LevelError, RuleDeployBlock, "capability cert.deploy requires the %q permission", protocol.PermissionCertDeploy)
	}
	if m.Deploy == nil || len(m.Deploy.Targets) == 0 {
		report.add(LevelError, RuleDeployBlock, "capability cert.deploy requires at least one target")
		return
	}
	seen := make(map[string]struct{}, len(m.Deploy.Targets))
	for _, t := range m.Deploy.Targets {
		if !capabilityCodePattern.MatchString(t.Code) {
			report.add(LevelError, RuleDeployCode, "deploy target code %q must match %s", t.Code, capabilityCodePattern)
		} else if _, dup := seen[t.Code]; dup {
			report.add(LevelError, RuleDeployCode, "deploy target code %q is declared twice", t.Code)
		}
		seen[t.Code] = struct{}{}
		if t.Name == "" {
			report.add(LevelError, RuleDeployTarget, "deploy target %q is missing a name", t.Code)
		}
		lintConfigurationSchema(fmt.Sprintf("deploy target %q", t.Code), RuleDeployTarget, t.Configuration, report)
	}
}

// lintNotify checks notify.channels and their forms.
func lintNotify(n *protocol.ManifestNotify, report *LintReport) {
	if n == nil || len(n.Channels) == 0 {
		report.add(LevelError, RuleNotifyBlock, "capability notify requires at least one channel")
		return
	}
	seen := make(map[string]struct{}, len(n.Channels))
	for _, c := range n.Channels {
		if !capabilityCodePattern.MatchString(c.Code) {
			report.add(LevelError, RuleNotifyCode, "notify channel code %q must match %s", c.Code, capabilityCodePattern)
		} else if _, dup := seen[c.Code]; dup {
			report.add(LevelError, RuleNotifyCode, "notify channel code %q is declared twice", c.Code)
		}
		seen[c.Code] = struct{}{}
		if c.Name == "" {
			report.add(LevelError, RuleNotifyName, "notify channel %q is missing a name", c.Code)
		}
		lintConfigurationSchema(fmt.Sprintf("notify channel %q", c.Code), RuleConfigurationFields, c.Configuration, report)
	}
}

// lintProbe checks probe.kinds and their forms.
func lintProbe(p *protocol.ManifestProbe, report *LintReport) {
	if p == nil || len(p.Kinds) == 0 {
		report.add(LevelError, RuleProbeBlock, "capability probe requires at least one kind")
		return
	}
	seen := make(map[string]struct{}, len(p.Kinds))
	for _, k := range p.Kinds {
		if !capabilityCodePattern.MatchString(k.Code) {
			report.add(LevelError, RuleProbeCode, "probe kind code %q must match %s", k.Code, capabilityCodePattern)
		} else if _, dup := seen[k.Code]; dup {
			report.add(LevelError, RuleProbeCode, "probe kind code %q is declared twice", k.Code)
		}
		seen[k.Code] = struct{}{}
		if k.Name == "" {
			report.add(LevelError, RuleProbeKind, "probe kind %q is missing a name", k.Code)
		}
		lintConfigurationSchema(fmt.Sprintf("probe kind %q", k.Code), RuleProbeKind, k.Configuration, report)
	}
}

// lintConfigurationSchema checks the configuration form of a capability
// entry, which every capability with such a form shares.
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
// world does not tell the person installing it so.
func lintNetworkPermission(m *protocol.Manifest, capability string, report *LintReport) {
	if !slices.Contains(m.Permissions, protocol.PermissionNetwork) {
		report.add(LevelWarning, RuleNetworkPermission, "capability %s usually reaches a vendor or a target, but permission %q is not requested", capability, protocol.PermissionNetwork)
	}
}

// lintMCP checks mcp.tools and the mcp permission.
func lintMCP(m *protocol.Manifest, report *LintReport) {
	if !slices.Contains(m.Permissions, protocol.PermissionMCP) {
		report.add(LevelError, RuleMCPBlock, "capability mcp requires the %q permission", protocol.PermissionMCP)
	}
	if m.MCP == nil || len(m.MCP.Tools) == 0 {
		report.add(LevelError, RuleMCPBlock, "capability mcp requires at least one tool")
		return
	}
	seen := make(map[string]struct{}, len(m.MCP.Tools))
	for _, tool := range m.MCP.Tools {
		if !mcpToolNamePattern.MatchString(tool.Name) {
			report.add(LevelError, RuleMCPTool, "mcp tool name %q must match %s", tool.Name, mcpToolNamePattern)
		} else if _, dup := seen[tool.Name]; dup {
			report.add(LevelError, RuleMCPTool, "mcp tool %q is declared twice", tool.Name)
		}
		seen[tool.Name] = struct{}{}
		if tool.Description == "" {
			report.add(LevelError, RuleMCPTool, "mcp tool %q needs a description", tool.Name)
		}
		if !isObjectSchema(tool.InputSchema) {
			report.add(LevelError, RuleMCPInputSchema, "mcp tool %q input_schema must have type \"object\"", tool.Name)
		}
	}
}

// lintDNS01 checks dns01.providers and their credential forms.
func lintDNS01(d *protocol.ManifestDNS01, report *LintReport) {
	if d == nil || len(d.Providers) == 0 {
		report.add(LevelError, RuleDNS01Block, "capability dns01 requires at least one provider")
		return
	}
	seen := make(map[string]struct{}, len(d.Providers))
	for _, p := range d.Providers {
		if p.Name == "" {
			report.add(LevelError, RuleDNS01Name, "dns01 provider %q is missing a name", p.Code)
		}
		if !dns01CodePattern.MatchString(p.Code) {
			report.add(LevelError, RuleDNS01Code, "dns01 provider code %q must match %s", p.Code, dns01CodePattern)
		} else if _, dup := seen[p.Code]; dup {
			report.add(LevelError, RuleDNS01Code, "dns01 provider code %q is declared twice", p.Code)
		}
		seen[p.Code] = struct{}{}

		for _, problem := range dns01FormProblems(p) {
			report.add(LevelError, RuleDNS01Form, "%s", problem)
		}
	}
}

// lintPermissions checks permissions.
func lintPermissions(permissions []string, report *LintReport) {
	seen := make(map[string]struct{}, len(permissions))
	for _, permission := range permissions {
		if _, dup := seen[permission]; dup {
			report.add(LevelWarning, RuleManifestPermissions, "permission %q is declared twice", permission)
		}
		seen[permission] = struct{}{}

		if slices.Contains(knownPermissions, permission) {
			continue
		}
		if kind, ok := strings.CutPrefix(permission, protocol.PermissionCredentialsReadPrefix); ok && kind != "" {
			continue
		}
		report.add(LevelError, RuleManifestPermissions, "unknown permission %q", permission)
	}
}

// lintRequires checks that every dependency id is well formed.
func lintRequires(requires []protocol.ManifestRequirement, report *LintReport) {
	for _, r := range requires {
		if !IsValidID(r.ID) {
			report.add(LevelError, RuleManifestRequires, "requires[].id %q is not a valid plugin id", r.ID)
		}
	}
}

// lintConflicts checks the conflicts declaration.
func lintConflicts(m *protocol.Manifest, report *LintReport) {
	for _, problem := range conflictProblems(m) {
		report.add(LevelError, RuleManifestConflicts, "%s", problem)
	}
}

// lintSettingsSchema checks settings_schema and, as
// an extension ValidateManifest does not perform, that a default value's
// JSON type is compatible with the field's declared type.
func lintSettingsSchema(schema *protocol.SettingsSchema, report *LintReport) {
	if schema == nil {
		return
	}
	seen := make(map[string]struct{}, len(schema.Settings))
	for _, field := range schema.Settings {
		if field.Key == "" {
			report.add(LevelError, RuleSettingsKey, "settings_schema field key is required")
			continue
		}
		if _, dup := seen[field.Key]; dup {
			report.add(LevelError, RuleSettingsKey, "settings_schema key %q is declared twice", field.Key)
		}
		seen[field.Key] = struct{}{}

		if !slices.Contains(knownSettingsTypes, field.Type) {
			report.add(LevelError, RuleSettingsType, "settings_schema key %q has unknown type %q", field.Key, field.Type)
			continue
		}
		if field.Type == "select" && len(field.Options) == 0 {
			report.add(LevelError, RuleSettingsOptions, "settings_schema key %q needs options", field.Key)
		}
		if field.Type == settingsTypeList && field.Default != nil && !isStringList(field.Default) {
			report.add(LevelError, RuleSettingsType, "settings_schema key %q default must be a list of strings", field.Key)
			continue
		}
		if field.Default != nil && !settingsDefaultMatchesType(field.Type, field.Default) {
			report.add(LevelWarning, RuleSettingsType, "settings_schema key %q default %v does not look like a %q value", field.Key, field.Default, field.Type)
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
	case settingsTypeList:
		return isStringList(value)
	default: // text, secret, textarea, select
		_, ok := value.(string)
		return ok
	}
}

// lintDocs checks that README.md, LICENSE and CHANGELOG.md are present and warns
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
			report.add(LevelWarning, RulePackageDocs, "%s is missing a %q section", lintReadmeName, section)
		}
	}
}

func lintRequiredDoc(dir, name string, level Level, report *LintReport) {
	if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
		report.add(level, RulePackageDocs, "%s is missing from the package root", name)
	}
}

// lintDirLimits walks a plain plugin directory and enforces the same limits
// walkPackage enforces on an archive, since a
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
			report.add(LevelError, RulePackageLinks, "entry %q is a symlink", rel)
		}
		files++
		if info.Mode().IsRegular() {
			total += info.Size()
		}
		return nil
	})
	if files > MaxPackageFiles {
		report.add(LevelError, RulePackageEntries, "the directory contains %d entries, more than the %d limit", files, MaxPackageFiles)
	}
	if total > MaxPackageSize {
		report.add(LevelError, RulePackageSize, "the directory payload is %d bytes, more than the %d byte limit", total, MaxPackageSize)
	}
}

// lintExtractArchive extracts path into a fresh temporary directory for
// linting. Unlike ExtractPackage it does not validate or load the manifest
// itself (Lint wants to inspect the manifest and report every issue, not
// abort on the first one), but it reuses the same package walker so the
// layout, path and size checks are enforced identically. Any violation becomes a Finding
// instead of a hard failure; ok is false when nothing could be extracted at
// all, in which case the caller has nothing left to lint.
func lintExtractArchive(archivePath string, report *LintReport) (dir string, ok bool) {
	tempDir, err := os.MkdirTemp("", "nginx-ui-plugin-lint-*")
	if err != nil {
		report.add(LevelError, RulePackageLayout, "cannot create a temporary directory: %v", err)
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
		report.add(LevelError, RulePackageLayout, "%v", err)
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
			// Keep the packed mode so a missing executable bit can be reported.
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

// packageErrorRule maps a package walker error onto the check rule it
// violates, using the sentinel error where one exists and the message
// otherwise.
func packageErrorRule(err error) string {
	switch {
	case errors.Is(err, ErrPackageTooLarge):
		if strings.Contains(err.Error(), "entries") {
			return RulePackageEntries
		}
		return RulePackageSize
	case strings.Contains(err.Error(), "is a link"):
		return RulePackageLinks
	case strings.Contains(err.Error(), "escapes"):
		return RulePackageEscape
	case strings.Contains(err.Error(), ManifestFileName):
		return RulePackageLayout
	default:
		return RulePackagePaths
	}
}

// lintPartnerCertificate checks the partner certificate files: both belong
// together, an official plugin key verifies them and the trusted comment
// parses, and a certificate with an expiry has not expired by the UTC date of
// the linter clock. It returns the certificate when it is valid.
// The linter reads no keyring, so revocations are not checked.
func lintPartnerCertificate(dir string, report *LintReport) *partnerCertificate {
	certificate, err := readPartnerCertificate(dir, nil)
	switch {
	case errors.Is(err, errPartnerIncomplete):
		report.add(LevelWarning, RulePartnerFiles, "%v, the certificate is ignored", err)
	case errors.Is(err, errPartnerExpired):
		report.add(LevelWarning, RulePartnerCertificate, "%v, the certificate is ignored", err)
	case err != nil:
		report.add(LevelWarning, RulePartnerComment, "the partner certificate does not verify and is ignored: %v", err)
	}
	return certificate
}

// lintSums checks the embedded signature files: plugin.sums has to follow
// its format and match the files whether or not the package is signed, both
// files belong together, a valid partner certificate has to name the key
// that signed plugin.sums, and a signature neither the official plugin keys
// pinned in this binary nor the certificate key verify is only a warning,
// since a community key is named by a catalog entry or an operator.
func lintSums(dir string, certificate *partnerCertificate, report *LintReport) {
	sums, err := readRootFile(dir, SumsFileName)
	if err != nil {
		report.add(LevelError, RuleSignatureSums, "read %s: %v", SumsFileName, err)
		return
	}
	signature, err := readRootFile(dir, SumsSignatureFileName)
	if err != nil {
		report.add(LevelError, RuleSignatureFiles, "read %s: %v", SumsSignatureFileName, err)
		return
	}

	if certificate != nil && signature == nil {
		report.add(LevelWarning, RulePartnerCertificate, "%s is missing, the key %s the partner certificate names signs nothing", SumsSignatureFileName, certificate.KeyID)
	}

	switch {
	case sums == nil && signature == nil:
		return
	case sums == nil:
		report.add(LevelWarning, RuleSignatureFiles, "%s is present without %s, the package is unsigned", SumsSignatureFileName, SumsFileName)
		return
	case signature == nil:
		report.add(LevelWarning, RuleSignatureFiles, "%s is present without %s, the package is unsigned", SumsFileName, SumsSignatureFileName)
	}

	if listed, err := parseSums(sums); err != nil {
		report.add(LevelError, RuleSignatureSums, "%v", err)
	} else if err = matchSums(dir, listed); err != nil {
		report.add(LevelError, RuleSignatureMismatch, "%v", err)
	}
	if signature == nil {
		return
	}

	var partners []partnerCertificate
	if certificate != nil {
		partners = append(partners, *certificate)
		if keyID, err := pkgsign.KeyID(signature); err == nil && fmt.Sprintf("%016X", keyID) != certificate.KeyID {
			report.add(LevelWarning, RulePartnerCertificate, "%s is signed by key %016X, not by the key %s the partner certificate names",
				SumsSignatureFileName, keyID, certificate.KeyID)
		}
	}
	trust, err := signatureTrust(sums, signature, partnerTiers(partners, nil))
	switch {
	case err != nil:
		report.add(LevelError, RuleSignatureMismatch, "%s: %v", SumsSignatureFileName, err)
	case trust.Trust == TrustUnsigned:
		report.add(LevelWarning, RuleSignatureSigner, "%s does not verify with the official plugin keys pinned in this binary or a partner certificate key (expected for a community plugin)", SumsSignatureFileName)
	}
}
