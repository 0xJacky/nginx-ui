package site

import (
	"fmt"
	"net/http"
	"os"
	"runtime"
	"strings"
	"sync"

	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/internal/notification"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/go-resty/resty/v2"
	"github.com/tufanbarisyildirim/gonginx/config"
	"github.com/tufanbarisyildirim/gonginx/parser"
	"github.com/uozi-tech/cosy"
	"github.com/uozi-tech/cosy/logger"
	cSettings "github.com/uozi-tech/cosy/settings"
)

const MaintenanceSuffix = "_nginx_ui_maintenance"

// EnableMaintenance enables maintenance mode for a site
func EnableMaintenance(name string) (err error) {
	// Check if the site exists in sites-available
	configFilePath := nginx.GetConfPath("sites-available", name)
	_, err = os.Stat(configFilePath)
	if err != nil {
		return
	}

	// Path for the maintenance configuration file
	maintenanceConfigPath := nginx.GetConfPath("sites-enabled", name+MaintenanceSuffix)

	// Path for original configuration in sites-enabled
	originalEnabledPath := nginx.GetConfPath("sites-enabled", name)

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
	maintenanceConfig := createMaintenanceConfig(conf)

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
	output := nginx.TestConf()
	if nginx.GetLogLevel(output) > nginx.Warn {
		// Configuration error, cleanup and revert
		_ = os.Remove(maintenanceConfigPath)
		if helper.FileExists(originalEnabledPath + "_backup") {
			_ = os.Rename(originalEnabledPath+"_backup", originalEnabledPath)
		}
		return cosy.WrapErrorWithParams(ErrNginxTestFailed, output)
	}

	// Reload nginx
	output = nginx.Reload()
	if nginx.GetLogLevel(output) > nginx.Warn {
		return cosy.WrapErrorWithParams(ErrNginxReloadFailed, output)
	}

	// Synchronize with other nodes
	go syncEnableMaintenance(name)

	return nil
}

// DisableMaintenance disables maintenance mode for a site
func DisableMaintenance(name string) (err error) {
	// Check if the site is in maintenance mode
	maintenanceConfigPath := nginx.GetConfPath("sites-enabled", name+MaintenanceSuffix)
	_, err = os.Stat(maintenanceConfigPath)
	if err != nil {
		return
	}

	// Original configuration paths
	configFilePath := nginx.GetConfPath("sites-available", name)
	enabledConfigFilePath := nginx.GetConfPath("sites-enabled", name)

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
	output := nginx.TestConf()
	if nginx.GetLogLevel(output) > nginx.Warn {
		// Configuration error, cleanup and revert
		_ = os.Remove(enabledConfigFilePath)
		_ = os.Symlink(configFilePath, maintenanceConfigPath)
		return fmt.Errorf("%s", output)
	}

	// Reload nginx
	output = nginx.Reload()
	if nginx.GetLogLevel(output) > nginx.Warn {
		return fmt.Errorf("%s", output)
	}

	// Synchronize with other nodes
	go syncDisableMaintenance(name)

	return nil
}

// createMaintenanceConfig creates a maintenance configuration based on the original config
func createMaintenanceConfig(conf *config.Config) string {
	nginxUIPort := cSettings.ServerSettings.Port
	schema := "http"
	if cSettings.ServerSettings.EnableHTTPS {
		schema = "https"
	}

	// Create new configuration
	ngxConfig := nginx.NewNgxConfig("")

	// Find all server blocks in the original configuration
	serverBlocks := findServerBlocks(conf.Block)

	// Create maintenance mode configuration for each server block
	for _, server := range serverBlocks {
		ngxServer := nginx.NewNgxServer()

		// Copy listen directives
		listenDirectives := extractDirectives(server, "listen")
		for _, directive := range listenDirectives {
			ngxDirective := &nginx.NgxDirective{
				Directive: directive.GetName(),
				Params:    strings.Join(extractParams(directive), " "),
			}
			ngxServer.Directives = append(ngxServer.Directives, ngxDirective)
		}

		// Copy server_name directives
		serverNameDirectives := extractDirectives(server, "server_name")
		for _, directive := range serverNameDirectives {
			ngxDirective := &nginx.NgxDirective{
				Directive: directive.GetName(),
				Params:    strings.Join(extractParams(directive), " "),
			}
			ngxServer.Directives = append(ngxServer.Directives, ngxDirective)
		}

		// Copy SSL certificate directives
		sslCertDirectives := extractDirectives(server, "ssl_certificate")
		for _, directive := range sslCertDirectives {
			ngxDirective := &nginx.NgxDirective{
				Directive: directive.GetName(),
				Params:    strings.Join(extractParams(directive), " "),
			}
			ngxServer.Directives = append(ngxServer.Directives, ngxDirective)
		}

		// Copy SSL certificate key directives
		sslKeyDirectives := extractDirectives(server, "ssl_certificate_key")
		for _, directive := range sslKeyDirectives {
			ngxDirective := &nginx.NgxDirective{
				Directive: directive.GetName(),
				Params:    strings.Join(extractParams(directive), " "),
			}
			ngxServer.Directives = append(ngxServer.Directives, ngxDirective)
		}

		// Copy http2 directives
		http2Directives := extractDirectives(server, "http2")
		for _, directive := range http2Directives {
			ngxDirective := &nginx.NgxDirective{
				Directive: directive.GetName(),
				Params:    strings.Join(extractParams(directive), " "),
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
		locationContent.WriteString(fmt.Sprintf("rewrite ^ /pages/maintenance break;\n"))
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
		go func(node *model.Environment) {
			defer func() {
				if err := recover(); err != nil {
					buf := make([]byte, 1024)
					runtime.Stack(buf, false)
					logger.Error(err)
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
		go func(node *model.Environment) {
			defer func() {
				if err := recover(); err != nil {
					buf := make([]byte, 1024)
					runtime.Stack(buf, false)
					logger.Error(err)
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
