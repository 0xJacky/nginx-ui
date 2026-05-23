package site

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"

	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/internal/notification"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/go-resty/resty/v2"
	"github.com/tufanbarisyildirim/gonginx/config"
	"github.com/tufanbarisyildirim/gonginx/dumper"
	"github.com/tufanbarisyildirim/gonginx/parser"
	"github.com/uozi-tech/cosy/logger"
	cSettings "github.com/uozi-tech/cosy/settings"
)

const MaintenanceSuffix = "_nginx_ui_maintenance"

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

// EnableMaintenance enables maintenance mode for a site
func EnableMaintenance(name string) (err error) {
	// Check if the site exists in sites-available
	configFilePath, err := ResolveAvailablePath(name)
	_, err = os.Stat(configFilePath)
	if err != nil {
		return
	}

	// Path for the maintenance configuration file
	maintenanceConfigPath, err := ResolveEnabledPath(name + MaintenanceSuffix)
	if err != nil {
		return err
	}

	// Path for original configuration in sites-enabled
	originalEnabledPath, err := ResolveEnabledPath(name)
	if err != nil {
		return err
	}

	// Check if the site is already in maintenance mode
	if helper.FileExists(maintenanceConfigPath) {
		return
	}

	// Read the original configuration file
	content, err := os.ReadFile(configFilePath)
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
	maintenanceConfig := createMaintenanceConfig(conf, filepath.Dir(configFilePath))

	// Write maintenance configuration to file
	err = os.WriteFile(maintenanceConfigPath, []byte(maintenanceConfig), 0644)
	if err != nil {
		return
	}

	// Remove the original symlink from sites-enabled if it exists
	if helper.FileExists(originalEnabledPath) {
		err = os.Remove(originalEnabledPath)
		if err != nil {
			// If we couldn't remove the original, remove the maintenance file and return the error
			_ = os.Remove(maintenanceConfigPath)
			return
		}
	}

	// Test nginx config, if not pass, then restore original configuration
	res := nginx.Control(nginx.TestConfig)
	if res.IsError() {
		// Configuration error, cleanup and revert
		_ = os.Remove(maintenanceConfigPath)
		if helper.FileExists(originalEnabledPath + "_backup") {
			_ = os.Rename(originalEnabledPath+"_backup", originalEnabledPath)
		}
		return res.GetError()
	}

	// Reload nginx
	res = nginx.Control(nginx.Reload)
	if res.IsError() {
		return res.GetError()
	}

	// Synchronize with other nodes
	go syncEnableMaintenance(name)

	return nil
}

// DisableMaintenance disables maintenance mode for a site
func DisableMaintenance(name string) (err error) {
	// Check if the site is in maintenance mode
	maintenanceConfigPath, err := ResolveEnabledPath(name + MaintenanceSuffix)
	_, err = os.Stat(maintenanceConfigPath)
	if err != nil {
		return
	}

	// Original configuration paths
	configFilePath, err := ResolveAvailablePath(name)
	if err != nil {
		return err
	}

	enabledConfigFilePath, err := ResolveEnabledPath(name)
	if err != nil {
		return err
	}

	// Check if the original configuration exists
	_, err = os.Stat(configFilePath)
	if err != nil {
		return
	}

	// Create symlink to original configuration
	err = os.Symlink(configFilePath, enabledConfigFilePath)
	if err != nil {
		return
	}

	// Remove maintenance configuration
	err = os.Remove(maintenanceConfigPath)
	if err != nil {
		// If we couldn't remove the maintenance file, remove the new symlink and return the error
		_ = os.Remove(enabledConfigFilePath)
		return
	}

	// Test nginx config, if not pass, then revert
	res := nginx.Control(nginx.TestConfig)
	if res.IsError() {
		// Configuration error, cleanup and revert
		_ = os.Remove(enabledConfigFilePath)
		_ = os.Symlink(configFilePath, maintenanceConfigPath)
		return res.GetError()
	}

	// Reload nginx
	res = nginx.Control(nginx.Reload)
	if res.IsError() {
		return res.GetError()
	}

	// Synchronize with other nodes
	go syncDisableMaintenance(name)

	return nil
}

// createMaintenanceConfig creates a maintenance configuration based on the original config.
// baseDir is the directory used to resolve relative include directives; pass "" to fall back
// to the nginx configuration directory.
func createMaintenanceConfig(conf *config.Config, baseDir string) string {
	nginxUIPort := cSettings.ServerSettings.Port
	schema := "http"
	if cSettings.ServerSettings.EnableHTTPS {
		schema = "https"
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
		locationContent.WriteString("rewrite ^ /pages/maintenance break;\n")
		locationContent.WriteString(fmt.Sprintf("proxy_pass %s://127.0.0.1:%d;\n", schema, nginxUIPort))

		location.Content = locationContent.String()
		ngxServer.Locations = append(ngxServer.Locations, location)

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

	matches, err := filepath.Glob(pattern)
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

	info, err := os.Stat(path)
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
		if _, err := os.Stat(candidate); err == nil {
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
	if info, err := os.Stat(staticDir); err == nil && info.IsDir() {
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
		info, err := os.Lstat(cleanPath)
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

	content, err := os.ReadFile(cleanPath)
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
func syncEnableMaintenance(name string) {
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

			client := resty.New()
			client.SetBaseURL(node.URL)
			resp, err := client.R().
				SetHeader("X-Node-Secret", node.Token).
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

			client := resty.New()
			client.SetBaseURL(node.URL)
			resp, err := client.R().
				SetHeader("X-Node-Secret", node.Token).
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
