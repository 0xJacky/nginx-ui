package site

import (
	"crypto/sha256"
	"fmt"
	"net/http"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"

	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/internal/nodeauth"
	"github.com/0xJacky/Nginx-UI/internal/notification"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/tufanbarisyildirim/gonginx/config"
	"github.com/tufanbarisyildirim/gonginx/dumper"
	"github.com/tufanbarisyildirim/gonginx/parser"
	"github.com/uozi-tech/cosy/logger"
	cSettings "github.com/uozi-tech/cosy/settings"
)

const MaintenanceSuffix = "_nginx_ui_maintenance"

const (
	maintenanceSiteHeaderKey           = "X-Maintenance-Site"
	maintenanceStartTimeHeaderKey      = "X-Maintenance-Start-Time"
	maintenanceEndTimeHeaderKey        = "X-Maintenance-End-Time"
	maintenanceContactHeaderKey        = "X-Maintenance-Contact"
	maintenanceAdditionalInfoHeaderKey = "X-Maintenance-Additional-Information"
)

type MaintenancePayload struct {
	StartTime           string `json:"start_time"`
	EndTime             string `json:"end_time"`
	Contact             string `json:"contact"`
	AdditionInformation string `json:"additioninfomation"`
}

var baseMaintenanceServerDirectives = map[string]struct{}{
	"listen":      {},
	"server_name": {},
	"http2":       {},
}

const (
	maintenanceMaxIncludeDepth       = 5
	maintenanceMaxWildcardMatches    = 32
	maintenanceIncludeDebugLogPrefix = "maintenance include expansion"
)

// certbotNginxTLSOptionsPath is the well-known certbot-managed SSL options snippet.
// It is the only path outside the nginx configuration directory that is allowed
// to be expanded into the maintenance configuration, because certbot installs it
// at a fixed location and its contents are known-safe TLS hardening directives.
// The value is cleaned at init time so OS-specific separators do not break the
// equality check against `filepath.Clean`-normalized include paths.
var certbotNginxTLSOptionsPath = filepath.Clean("/etc/letsencrypt/options-ssl-nginx.conf")

type maintenanceIncludeExpander struct {
	confDir string
	visited map[string]struct{}
}

func sanitizeMaintenanceHeaderValue(value string) string {
	value = strings.TrimSpace(value)
	value = strings.ReplaceAll(value, "\r", " ")
	value = strings.ReplaceAll(value, "\n", " ")
	return value
}

func normalizeMaintenancePayload(payload *MaintenancePayload) MaintenancePayload {
	if payload == nil {
		return MaintenancePayload{}
	}

	return MaintenancePayload{
		StartTime:           sanitizeMaintenanceHeaderValue(payload.StartTime),
		EndTime:             sanitizeMaintenanceHeaderValue(payload.EndTime),
		Contact:             sanitizeMaintenanceHeaderValue(payload.Contact),
		AdditionInformation: sanitizeMaintenanceHeaderValue(payload.AdditionInformation),
	}
}

// EnableMaintenance enables maintenance mode for a site
func EnableMaintenance(name string) (err error) {
	return EnableMaintenanceWithPayload(name, nil)
}

// EnableMaintenanceWithPayload enables maintenance mode for a site and injects
// custom metadata used by maintenance pages.
func EnableMaintenanceWithPayload(name string, payload *MaintenancePayload) (err error) {
	normalizedPayload := normalizeMaintenancePayload(payload)

	if err = validateSiteName(name); err != nil {
		return err
	}

	// Check if the site exists in sites-available
	configFilePath, err := ResolveAvailablePath(name)
	if err != nil {
		return err
	}
	if _, err = nginx.Stat(configFilePath); err != nil {
		return
	}

	// Path for the maintenance configuration file
	maintenanceConfigPath, err := ResolveEnabledMaintenancePath(name)
	if err != nil {
		return err
	}

	// Path for original configuration in sites-enabled
	originalEnabledPath, err := resolveEnabledSymlinkPath(name)
	if err != nil {
		return err
	}

	// Check if the site is already in maintenance mode
	maintenanceExists, err := nginx.Exists(maintenanceConfigPath)
	if err != nil {
		return err
	}
	if maintenanceExists {
		return
	}

	// Remote namespaces have no local Nginx to switch into maintenance mode, so
	// the request is only dispatched to the member nodes.
	if IsRemoteDeploy(name) {
		go syncEnableMaintenance(name, normalizedPayload)

		return
	}

	// Read the original configuration file
	content, err := nginx.ReadFile(configFilePath)
	if err != nil {
		return
	}

	// Parse the nginx configuration
	p := parser.NewStringParser(string(content), parser.WithSkipValidDirectivesErr())
	conf, err := p.Parse()
	if err != nil {
		return fmt.Errorf("failed to parse nginx configuration: %s", err)
	}

	// Create new maintenance configuration
	maintenanceConfig := createMaintenanceConfigWithPayload(conf, filepath.Dir(configFilePath), name, normalizedPayload)

	// Write maintenance configuration to file
	err = nginx.WriteFile(maintenanceConfigPath, []byte(maintenanceConfig), 0644)
	if err != nil {
		return
	}

	// Remove the original symlink from sites-enabled if it exists
	originalWasEnabled, err := nginx.Exists(originalEnabledPath)
	if err != nil {
		_ = nginx.Remove(maintenanceConfigPath)
		return err
	}
	if originalWasEnabled {
		err = nginx.Remove(originalEnabledPath)
		if err != nil {
			// If we couldn't remove the original, remove the maintenance file and return the error
			_ = nginx.Remove(maintenanceConfigPath)
			return
		}
	}

	// revertMaintenance drops the maintenance config and re-enables the original
	// site, so a failed switch never leaves the site without any enabled config.
	revertMaintenance := func() {
		_ = nginx.Remove(maintenanceConfigPath)
		if !originalWasEnabled {
			return
		}
		if symlinkErr := nginx.Symlink(configFilePath, originalEnabledPath); symlinkErr != nil {
			logger.Error("Failed to restore site after maintenance rollback", symlinkErr)
		}
	}

	// Test nginx config, if not pass, then restore original configuration
	res := nginx.Control(nginx.TestConfig)
	if res.IsError() {
		// Configuration error, cleanup and revert
		revertMaintenance()
		return res.GetError()
	}

	// Reload nginx
	res = nginx.Control(nginx.Reload)
	if res.IsError() {
		revertMaintenance()
		nginx.Control(nginx.Reload)
		return res.GetError()
	}

	// Synchronize with other nodes
	go syncEnableMaintenance(name, normalizedPayload)

	return nil
}

// DisableMaintenance disables maintenance mode for a site
func DisableMaintenance(name string) (err error) {
	if err = validateSiteName(name); err != nil {
		return err
	}

	// Remote namespaces have no local maintenance configuration to restore.
	if IsRemoteDeploy(name) {
		go syncDisableMaintenance(name)

		return
	}

	// Check if the site is in maintenance mode
	maintenanceConfigPath, err := ResolveEnabledMaintenancePath(name)
	if err != nil {
		return err
	}
	if _, err = nginx.Stat(maintenanceConfigPath); err != nil {
		return
	}

	// Original configuration paths
	configFilePath, err := ResolveAvailablePath(name)
	if err != nil {
		return err
	}

	enabledConfigFilePath, err := resolveEnabledSymlinkPath(name)
	if err != nil {
		return err
	}

	// Check if the original configuration exists
	_, err = nginx.Stat(configFilePath)
	if err != nil {
		return
	}

	// Create symlink to original configuration
	err = nginx.Symlink(configFilePath, enabledConfigFilePath)
	if err != nil {
		return
	}

	// Keep the generated maintenance config so it can be restored on rollback
	maintenanceContent, err := nginx.ReadFile(maintenanceConfigPath)
	if err != nil {
		_ = nginx.Remove(enabledConfigFilePath)
		return
	}

	// Remove maintenance configuration
	err = nginx.Remove(maintenanceConfigPath)
	if err != nil {
		// If we couldn't remove the maintenance file, remove the new symlink and return the error
		_ = nginx.Remove(enabledConfigFilePath)
		return
	}

	// Test nginx config, if not pass, then revert
	res := nginx.Control(nginx.TestConfig)
	if res.IsError() {
		// Configuration error, cleanup and revert
		_ = nginx.Remove(enabledConfigFilePath)
		_ = nginx.WriteFile(maintenanceConfigPath, maintenanceContent, 0644)
		return res.GetError()
	}

	// Reload nginx
	res = nginx.Control(nginx.Reload)
	if res.IsError() {
		// Reload failed after the config already tested clean: restore the
		// maintenance file and drop the new symlink so Nginx keeps serving
		// what it actually reloaded, then reload again to reconcile.
		_ = nginx.Remove(enabledConfigFilePath)
		_ = nginx.WriteFile(maintenanceConfigPath, maintenanceContent, 0644)
		nginx.Control(nginx.Reload)
		return res.GetError()
	}

	// Synchronize with other nodes
	go syncDisableMaintenance(name)

	return nil
}

// createMaintenanceConfig creates a maintenance configuration based on the original config.
// baseDir is the directory used to resolve relative include directives; pass "" to fall back
// to the nginx configuration directory. siteName is forwarded to the maintenance page so it
// can render a site specific template; pass "" to skip it.
func createMaintenanceConfig(conf *config.Config, baseDir string, siteName string) string {
	return createMaintenanceConfigWithPayload(conf, baseDir, siteName, MaintenancePayload{})
}

func createMaintenanceConfigWithPayload(conf *config.Config, baseDir string, siteName string, payload MaintenancePayload) string {
	nginxUIPort := cSettings.ServerSettings.Port
	schema := "http"
	if cSettings.ServerSettings.EnableHTTPS {
		schema = "https"
	}
	maintenanceHost := settings.NginxSettings.GetMaintenanceHost(schema, nginxUIPort)
	if bypassIP := settings.NginxSettings.GetMaintenanceBypassIP(); bypassIP != "" {
		if content, ok := createBypassMaintenanceConfig(conf, siteName, payload, bypassIP, maintenanceHost, schema, nginxUIPort); ok {
			return content
		}
	}

	// Create new configuration
	ngxConfig := nginx.NewNgxConfig("")

	// Find all server blocks in the original configuration
	serverBlocks := findServerBlocks(conf.Block)
	includeBaseDir := baseDir
	if includeBaseDir == "" {
		includeBaseDir = nginx.GetConfPath()
	}

	// Create maintenance mode configuration for each server block
	for _, server := range serverBlocks {
		ngxServer := nginx.NewNgxServer()

		// Preserve server identity and TLS handshake settings from the original site.
		for _, directive := range extractMaintenanceServerDirectives(server, includeBaseDir) {
			ngxDirective := &nginx.NgxDirective{
				Directive: directive.GetName(),
				Params:    strings.Join(extractParams(directive), " "),
				Raw:       dumpMaintenanceDirective(directive),
			}
			ngxServer.Directives = append(ngxServer.Directives, ngxDirective)
		}

		// Add acme-challenge location
		acmeChallengeLocation := &nginx.NgxLocation{
			Path: "^~ /.well-known/acme-challenge",
		}

		// Build location content using string builder
		var locationContent strings.Builder
		locationContent.WriteString("proxy_set_header Host $host;\n")
		locationContent.WriteString("proxy_set_header X-Real-IP $remote_addr;\n")
		locationContent.WriteString("proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;\n")
		locationContent.WriteString(fmt.Sprintf("proxy_pass http://127.0.0.1:%s;\n", settings.CertSettings.HTTPChallengePort))
		acmeChallengeLocation.Content = locationContent.String()

		ngxServer.Locations = append(ngxServer.Locations, acmeChallengeLocation)

		// Add maintenance mode location
		location := &nginx.NgxLocation{
			Path: "~ .*",
		}

		locationContent.Reset()
		// Build location content using string builder
		locationContent.WriteString("proxy_set_header Host $host;\n")
		locationContent.WriteString("proxy_set_header X-Real-IP $remote_addr;\n")
		locationContent.WriteString("proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;\n")
		locationContent.WriteString("proxy_set_header X-Forwarded-Proto $scheme;\n")
		locationContent.WriteString("proxy_set_header X-Forwarded-Host $http_host;\n")
		if siteName != "" {
			locationContent.WriteString(fmt.Sprintf("proxy_set_header %s \"%s\";\n", maintenanceSiteHeaderKey, escapeNginxQuotedValue(siteName)))
		}
		if payload.StartTime != "" {
			locationContent.WriteString(fmt.Sprintf("proxy_set_header %s \"%s\";\n", maintenanceStartTimeHeaderKey, escapeNginxQuotedValue(payload.StartTime)))
		}
		if payload.EndTime != "" {
			locationContent.WriteString(fmt.Sprintf("proxy_set_header %s \"%s\";\n", maintenanceEndTimeHeaderKey, escapeNginxQuotedValue(payload.EndTime)))
		}
		if payload.Contact != "" {
			locationContent.WriteString(fmt.Sprintf("proxy_set_header %s \"%s\";\n", maintenanceContactHeaderKey, escapeNginxQuotedValue(payload.Contact)))
		}
		if payload.AdditionInformation != "" {
			locationContent.WriteString(fmt.Sprintf("proxy_set_header %s \"%s\";\n", maintenanceAdditionalInfoHeaderKey, escapeNginxQuotedValue(payload.AdditionInformation)))
		}
		locationContent.WriteString("rewrite ^ /pages/maintenance break;\n")
		locationContent.WriteString(fmt.Sprintf("proxy_pass %s;\n", maintenanceHost))

		location.Content = locationContent.String()
		ngxServer.Locations = append(ngxServer.Locations, location)

		maintenanceMetaLocation := &nginx.NgxLocation{
			Path: "= /pages/maintenance/meta",
		}
		locationContent.Reset()
		locationContent.WriteString("proxy_set_header Host $host;\n")
		locationContent.WriteString("proxy_set_header X-Real-IP $remote_addr;\n")
		locationContent.WriteString("proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;\n")
		locationContent.WriteString("proxy_set_header X-Forwarded-Proto $scheme;\n")
		locationContent.WriteString("proxy_set_header X-Forwarded-Host $http_host;\n")
		if siteName != "" {
			locationContent.WriteString(fmt.Sprintf("proxy_set_header %s \"%s\";\n", maintenanceSiteHeaderKey, escapeNginxQuotedValue(siteName)))
		}
		if payload.StartTime != "" {
			locationContent.WriteString(fmt.Sprintf("proxy_set_header %s \"%s\";\n", maintenanceStartTimeHeaderKey, escapeNginxQuotedValue(payload.StartTime)))
		}
		if payload.EndTime != "" {
			locationContent.WriteString(fmt.Sprintf("proxy_set_header %s \"%s\";\n", maintenanceEndTimeHeaderKey, escapeNginxQuotedValue(payload.EndTime)))
		}
		if payload.Contact != "" {
			locationContent.WriteString(fmt.Sprintf("proxy_set_header %s \"%s\";\n", maintenanceContactHeaderKey, escapeNginxQuotedValue(payload.Contact)))
		}
		if payload.AdditionInformation != "" {
			locationContent.WriteString(fmt.Sprintf("proxy_set_header %s \"%s\";\n", maintenanceAdditionalInfoHeaderKey, escapeNginxQuotedValue(payload.AdditionInformation)))
		}
		locationContent.WriteString(fmt.Sprintf("proxy_pass %s://127.0.0.1:%d;\n", schema, nginxUIPort))
		maintenanceMetaLocation.Content = locationContent.String()
		ngxServer.Locations = append(ngxServer.Locations, maintenanceMetaLocation)

		// Add to configuration
		ngxConfig.Servers = append(ngxConfig.Servers, ngxServer)
	}

	// Generate configuration file content
	content, err := ngxConfig.BuildConfig()
	if err != nil {
		logger.Error("Failed to build maintenance config", err)
		return ""
	}

	return content
}

func createBypassMaintenanceConfig(conf *config.Config, siteName string, payload MaintenancePayload, bypassIP, maintenanceHost, schema string, nginxUIPort uint) (string, bool) {
	original := dumper.DumpConfig(conf, dumper.IndentedStyle)
	ngxConfig, err := nginx.ParseNgxConfigByContent(original)
	if err != nil {
		logger.Errorf("Failed to preserve site configuration for maintenance bypass: %v", err)
		return "", false
	}

	digest := sha256.Sum256([]byte(siteName))
	suffix := fmt.Sprintf("%x", digest[:4])
	bypassVariable := "nginx_ui_maintenance_bypass_" + suffix
	modeVariable := "nginx_ui_maintenance_mode_" + suffix
	locationName := "@nginx_ui_maintenance_" + suffix
	ngxConfig.Custom += fmt.Sprintf(`
geo $%s {
    default 0;
    %s 1;
}
map "$%s:$uri" $%s {
    default 1;
    ~^1: 0;
    ~^0:/\.well-known/acme-challenge/ 0;
    ~^0:/pages/maintenance/meta$ 0;
}
`, bypassVariable, bypassIP, bypassVariable, modeVariable)

	for _, server := range ngxConfig.Servers {
		server.Directives = append(server.Directives,
			&nginx.NgxDirective{Raw: fmt.Sprintf("error_page 418 = %s;", locationName)},
			&nginx.NgxDirective{Raw: fmt.Sprintf("if ($%s) {\n    return 418;\n}", modeVariable)},
		)
		server.Locations = append(server.Locations, &nginx.NgxLocation{
			Path:    locationName,
			Content: buildMaintenanceProxyContent(siteName, payload, maintenanceHost, true),
		})
		if !hasMaintenanceLocation(server, "/pages/maintenance/meta") {
			server.Locations = append(server.Locations, &nginx.NgxLocation{
				Path:    "= /pages/maintenance/meta",
				Content: buildMaintenanceProxyContent(siteName, payload, fmt.Sprintf("%s://127.0.0.1:%d", schema, nginxUIPort), false),
			})
		}
		if !hasMaintenanceLocation(server, "/.well-known/acme-challenge") {
			server.Locations = append(server.Locations, &nginx.NgxLocation{
				Path: "^~ /.well-known/acme-challenge",
				Content: "proxy_set_header Host $host;\n" +
					"proxy_set_header X-Real-IP $remote_addr;\n" +
					"proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;\n" +
					fmt.Sprintf("proxy_pass http://127.0.0.1:%s;\n", settings.CertSettings.HTTPChallengePort),
			})
		}
	}

	content, err := ngxConfig.BuildConfig()
	if err != nil {
		logger.Errorf("Failed to build maintenance bypass configuration: %v", err)
		return "", false
	}
	return content, true
}

func hasMaintenanceLocation(server *nginx.NgxServer, fragment string) bool {
	for _, location := range server.Locations {
		if strings.Contains(location.Path, fragment) {
			return true
		}
	}
	return false
}

func buildMaintenanceProxyContent(siteName string, payload MaintenancePayload, host string, rewrite bool) string {
	var content strings.Builder
	content.WriteString("proxy_set_header Host $host;\n")
	content.WriteString("proxy_set_header X-Real-IP $remote_addr;\n")
	content.WriteString("proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;\n")
	content.WriteString("proxy_set_header X-Forwarded-Proto $scheme;\n")
	content.WriteString("proxy_set_header X-Forwarded-Host $http_host;\n")
	for _, header := range []struct{ name, value string }{
		{maintenanceSiteHeaderKey, siteName},
		{maintenanceStartTimeHeaderKey, payload.StartTime},
		{maintenanceEndTimeHeaderKey, payload.EndTime},
		{maintenanceContactHeaderKey, payload.Contact},
		{maintenanceAdditionalInfoHeaderKey, payload.AdditionInformation},
	} {
		if header.value != "" {
			content.WriteString(fmt.Sprintf("proxy_set_header %s \"%s\";\n", header.name, escapeNginxQuotedValue(header.value)))
		}
	}
	if rewrite {
		content.WriteString("rewrite ^ /pages/maintenance break;\n")
	}
	content.WriteString(fmt.Sprintf("proxy_pass %s;\n", host))
	return content.String()
}

// escapeNginxQuotedValue escapes a value embedded in a double quoted nginx parameter.
func escapeNginxQuotedValue(value string) string {
	return strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(value)
}

// findServerBlocks finds all server blocks in a configuration
func findServerBlocks(block config.IBlock) []config.IDirective {
	var servers []config.IDirective

	if block == nil {
		return servers
	}

	for _, directive := range block.GetDirectives() {
		if directive.GetName() == "server" {
			servers = append(servers, directive)
		}
	}

	return servers
}

// extractDirectives extracts all directives with a specific name from a server block
func extractDirectives(server config.IDirective, name string) []config.IDirective {
	var directives []config.IDirective

	if server.GetBlock() == nil {
		return directives
	}

	for _, directive := range server.GetBlock().GetDirectives() {
		if directive.GetName() == name {
			directives = append(directives, directive)
		}
	}

	return directives
}

// extractMaintenanceServerDirectives extracts directives needed by the generated maintenance server.
func extractMaintenanceServerDirectives(server config.IDirective, baseDir string) []config.IDirective {
	expander := newMaintenanceIncludeExpander()
	initialBaseDir := resolveMaintenanceBaseDir(baseDir, expander.confDir)
	var directives []config.IDirective

	if server.GetBlock() == nil {
		return directives
	}

	for _, directive := range server.GetBlock().GetDirectives() {
		directives = append(directives, expander.extractServerDirective(directive, initialBaseDir, 0)...)
	}

	return directives
}

func newMaintenanceIncludeExpander() *maintenanceIncludeExpander {
	return &maintenanceIncludeExpander{
		confDir: filepath.Clean(nginx.GetConfPath()),
		visited: make(map[string]struct{}),
	}
}

// resolveMaintenanceBaseDir validates a caller-supplied include base directory and falls
// back to confDir if it is empty or escapes the nginx configuration directory.
func resolveMaintenanceBaseDir(baseDir, confDir string) string {
	if baseDir == "" {
		return confDir
	}
	candidate := filepath.Clean(baseDir)
	if helper.IsUnderDirectory(candidate, confDir) {
		return candidate
	}
	return confDir
}

func (e *maintenanceIncludeExpander) extractServerDirective(directive config.IDirective, baseDir string, depth int) []config.IDirective {
	name := directive.GetName()

	if _, ok := baseMaintenanceServerDirectives[name]; ok {
		return []config.IDirective{directive}
	}

	if strings.HasPrefix(name, "ssl_") {
		return []config.IDirective{directive}
	}

	if name == "include" {
		return e.extractIncludeDirective(directive, baseDir, depth)
	}

	return nil
}

func (e *maintenanceIncludeExpander) extractIncludeDirective(directive config.IDirective, baseDir string, depth int) []config.IDirective {
	params := extractParams(directive)
	if len(params) == 0 {
		return nil
	}

	includePath := strings.Trim(params[0], `"'`)
	if hasMaintenanceGlobMeta(includePath) {
		return e.extractWildcardInclude(includePath, baseDir, depth+1)
	}

	resolvedPath := e.resolveIncludePath(includePath, baseDir)
	if !e.isAllowedSingleInclude(resolvedPath) {
		logger.Debugf("%s: skipped disallowed include %s", maintenanceIncludeDebugLogPrefix, resolvedPath)
		return nil
	}

	return e.extractIncludeFile(resolvedPath, depth+1)
}

func (e *maintenanceIncludeExpander) extractWildcardInclude(includePath, baseDir string, depth int) []config.IDirective {
	pattern := e.resolveWildcardIncludePath(includePath, baseDir)
	staticDir := maintenanceGlobStaticDir(pattern)
	if staticDir == "" || !helper.IsUnderDirectory(staticDir, e.confDir) {
		logger.Debugf("%s: skipped disallowed wildcard include %s", maintenanceIncludeDebugLogPrefix, pattern)
		return nil
	}

	matches, err := nginx.Glob(pattern)
	if err != nil {
		logger.Debugf("%s: failed to expand wildcard %s: %v", maintenanceIncludeDebugLogPrefix, pattern, err)
		return nil
	}
	sort.Strings(matches)

	var directives []config.IDirective
	allowedMatches := 0
	for _, match := range matches {
		if !e.isAllowedWildcardMatch(match) {
			logger.Debugf("%s: skipped disallowed wildcard match %s", maintenanceIncludeDebugLogPrefix, match)
			continue
		}
		if allowedMatches >= maintenanceMaxWildcardMatches {
			logger.Debugf(
				"%s: wildcard %s exceeded %d allowed files",
				maintenanceIncludeDebugLogPrefix,
				pattern,
				maintenanceMaxWildcardMatches,
			)
			break
		}
		allowedMatches++

		directives = append(directives, e.extractIncludeFile(match, depth)...)
	}

	return directives
}

func (e *maintenanceIncludeExpander) isAllowedWildcardMatch(path string) bool {
	if !helper.IsUnderDirectory(path, e.confDir) {
		return false
	}

	info, err := nginx.Stat(path)
	if err != nil {
		logger.Debugf("%s: failed to stat wildcard match %s: %v", maintenanceIncludeDebugLogPrefix, path, err)
		return false
	}
	if info.IsDir() {
		return false
	}

	return true
}

func maintenanceGlobStaticDir(pattern string) string {
	firstGlobIndex := strings.IndexAny(pattern, "*?[")
	if firstGlobIndex == -1 {
		return filepath.Dir(pattern)
	}

	staticPrefix := pattern[:firstGlobIndex]
	if staticPrefix == "" {
		return ""
	}

	return filepath.Dir(staticPrefix)
}

func (e *maintenanceIncludeExpander) resolveIncludePath(includePath, baseDir string) string {
	if filepath.IsAbs(includePath) {
		return filepath.Clean(includePath)
	}

	// filepath.Join already cleans its output; check baseDir-relative location first
	// and fall back to confDir-relative only if it does not exist (nginx include
	// resolution semantics).
	candidate := filepath.Join(baseDir, includePath)
	if helper.IsUnderDirectory(candidate, e.confDir) {
		if _, err := nginx.Stat(candidate); err == nil {
			return candidate
		}
	}

	return e.resolveFallbackIncludePath(includePath)
}

func (e *maintenanceIncludeExpander) resolveWildcardIncludePath(includePath, baseDir string) string {
	var candidate string
	if filepath.IsAbs(includePath) {
		candidate = filepath.Clean(includePath)
	} else {
		candidate = filepath.Join(baseDir, includePath)
	}

	staticDir := maintenanceGlobStaticDir(candidate)
	if staticDir == "" || !helper.IsUnderDirectory(staticDir, e.confDir) {
		return e.resolveFallbackIncludePath(includePath)
	}

	// Stat the static prefix so a baseDir-relative wildcard that targets a
	// nonexistent directory still falls back to the confDir-relative pattern.
	if info, err := nginx.Stat(staticDir); err == nil && info.IsDir() {
		return candidate
	}

	return e.resolveFallbackIncludePath(includePath)
}

func (e *maintenanceIncludeExpander) resolveFallbackIncludePath(includePath string) string {
	fallback := filepath.Join(e.confDir, includePath)
	if hasMaintenanceGlobMeta(fallback) {
		staticDir := maintenanceGlobStaticDir(fallback)
		if staticDir != "" && helper.IsUnderDirectory(staticDir, e.confDir) {
			return fallback
		}
		return e.confDir
	}

	if helper.IsUnderDirectory(fallback, e.confDir) {
		return fallback
	}
	return e.confDir
}

func (e *maintenanceIncludeExpander) isAllowedSingleInclude(path string) bool {
	cleanPath := filepath.Clean(path)
	if cleanPath == certbotNginxTLSOptionsPath {
		info, err := nginx.Lstat(cleanPath)
		if err != nil {
			logger.Debugf("%s: failed to stat certbot include %s: %v", maintenanceIncludeDebugLogPrefix, cleanPath, err)
			return false
		}
		return info.Mode().IsRegular()
	}

	return helper.IsUnderDirectory(cleanPath, e.confDir)
}

func (e *maintenanceIncludeExpander) extractIncludeFile(path string, depth int) []config.IDirective {
	if depth > maintenanceMaxIncludeDepth {
		logger.Debugf("%s: skipped %s because include depth exceeded %d", maintenanceIncludeDebugLogPrefix, path, maintenanceMaxIncludeDepth)
		return nil
	}

	cleanPath := filepath.Clean(path)
	if !e.isAllowedSingleInclude(cleanPath) {
		logger.Debugf("%s: skipped disallowed include file %s", maintenanceIncludeDebugLogPrefix, cleanPath)
		return nil
	}

	if _, ok := e.visited[cleanPath]; ok {
		return nil
	}
	e.visited[cleanPath] = struct{}{}

	content, err := nginx.ReadFile(cleanPath)
	if err != nil {
		logger.Debugf("%s: failed to read %s: %v", maintenanceIncludeDebugLogPrefix, cleanPath, err)
		return nil
	}

	p := parser.NewStringParser(string(content), parser.WithSkipValidDirectivesErr())
	conf, err := p.Parse()
	if err != nil {
		logger.Debugf("%s: failed to parse %s: %v", maintenanceIncludeDebugLogPrefix, cleanPath, err)
		return nil
	}

	if conf.Block == nil {
		return nil
	}

	var directives []config.IDirective
	childBaseDir := filepath.Dir(cleanPath)
	for _, directive := range conf.Block.GetDirectives() {
		directives = append(directives, e.extractIncludedDirective(directive, childBaseDir, depth)...)
	}

	return directives
}

func (e *maintenanceIncludeExpander) extractIncludedDirective(directive config.IDirective, baseDir string, depth int) []config.IDirective {
	name := directive.GetName()

	if strings.HasPrefix(name, "ssl_") {
		return []config.IDirective{directive}
	}

	if name != "include" {
		return nil
	}

	return e.extractIncludeDirective(directive, baseDir, depth)
}

func hasMaintenanceGlobMeta(path string) bool {
	return strings.ContainsAny(path, "*?[")
}

func dumpMaintenanceDirective(directive config.IDirective) string {
	style := *dumper.IndentedStyle
	style.StartIndent = 0
	return dumper.DumpDirective(directive, &style)
}

// extractParams extracts all parameters from a directive
func extractParams(directive config.IDirective) []string {
	var params []string

	for _, param := range directive.GetParameters() {
		params = append(params, param.Value)
	}

	return params
}

// syncEnableMaintenance synchronizes enabling maintenance mode with other nodes
func syncEnableMaintenance(name string, payload MaintenancePayload) {
	nodes := getSyncNodes(name)

	wg := &sync.WaitGroup{}
	wg.Add(len(nodes))

	for _, node := range nodes {
		go func(node *model.Node) {
			defer func() {
				if err := recover(); err != nil {
					buf := make([]byte, 1024)
					runtime.Stack(buf, false)
					logger.Errorf("%s\n%s", err, buf)
				}
			}()
			defer wg.Done()

			client := nodeauth.NewRestyClient(node)
			client.SetBaseURL(node.URL)
			resp, err := client.R().
				SetBody(payload).
				Post(fmt.Sprintf("/api/sites/%s/maintenance", name))
			if err != nil {
				notification.Error("Enable Remote Site Maintenance Error", err.Error(), nil)
				return
			}
			if resp.StatusCode() != http.StatusOK {
				notification.Error("Enable Remote Site Maintenance Error", "Enable site %{name} maintenance on %{node} failed", NewSyncResult(node.Name, name, resp))
				return
			}
			notification.Success("Enable Remote Site Maintenance Success", "Enable site %{name} maintenance on %{node} successfully", NewSyncResult(node.Name, name, resp))
		}(node)
	}

	wg.Wait()
}

// syncDisableMaintenance synchronizes disabling maintenance mode with other nodes
func syncDisableMaintenance(name string) {
	nodes := getSyncNodes(name)

	wg := &sync.WaitGroup{}
	wg.Add(len(nodes))

	for _, node := range nodes {
		go func(node *model.Node) {
			defer func() {
				if err := recover(); err != nil {
					buf := make([]byte, 1024)
					runtime.Stack(buf, false)
					logger.Errorf("%s\n%s", err, buf)
				}
			}()
			defer wg.Done()

			client := nodeauth.NewRestyClient(node)
			client.SetBaseURL(node.URL)
			resp, err := client.R().
				Post(fmt.Sprintf("/api/sites/%s/enable", name))
			if err != nil {
				notification.Error("Disable Remote Site Maintenance Error", err.Error(), nil)
				return
			}
			if resp.StatusCode() != http.StatusOK {
				notification.Error("Disable Remote Site Maintenance Error", "Disable site %{name} maintenance on %{node} failed", NewSyncResult(node.Name, name, resp))
				return
			}
			notification.Success("Disable Remote Site Maintenance Success", "Disable site %{name} maintenance on %{node} successfully", NewSyncResult(node.Name, name, resp))
		}(node)
	}

	wg.Wait()
}
