package nginx_log

import (
	"path/filepath"

	"github.com/0xJacky/Nginx-UI/internal/cache"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/internal/nginx_log/utils"
)

// Use init function to automatically register callback
func init() {
	// Register the callback directly with the global registry
	cache.RegisterCallback("nginx_log.scanForLogDirectives", scanForLogDirectives)
}

// scanForLogDirectives scans and parses configuration files for log directives
func scanForLogDirectives(configPath string, content []byte) error {
	prefix := nginx.GetPrefix()

	// Remove existing log paths that originated from this config file
	RemoveLogPathsFromConfig(configPath)

	// Extract, validate and register log directives from the config content
	for _, directive := range utils.ScanLogDirectives(prefix, content) {
		if utils.IsValidLogPath(directive.Path) {
			AddLogPath(directive.Path, directive.Type, filepath.Base(directive.Path), configPath)
		}
	}

	return nil
}
