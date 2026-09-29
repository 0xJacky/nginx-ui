package nginx_log

import (
	"path/filepath"

	"github.com/0xJacky/Nginx-UI/internal/cache"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/internal/nginx_log/sink"
	"github.com/0xJacky/Nginx-UI/internal/nginx_log/utils"
)

// Use init function to automatically register callback
func init() {
	// Register the callback directly with the global registry
	cache.RegisterCallback("nginx_log.scanForLogDirectives", scanForLogDirectives)

	// Re-resolve the nginx default access/error logs once per scan sweep. A
	// post-scan callback is the right hook for this: it runs exactly once after
	// every sweep - the initial scan at boot, a scan triggered by a config file
	// change and the five-minute periodic scan - instead of once per config
	// file, and it runs after every scanForLogDirectives call, so a default path
	// that a rescan just removed is registered again in the same sweep.
	cache.RegisterPostScanCallback(func() {
		RefreshDefaultLogPaths()
		notifyLogScan()
	})

	// The log.sink plugins follow the access logs of the log list, limited to
	// the whitelist of the log viewer. The feeder only runs while a plugin
	// subscribed.
	sink.SetSource(sink.Source{Paths: accessLogPaths, Allowed: utils.IsValidLogPath})
}

// accessLogPaths lists the access logs the host knows.
func accessLogPaths() []string {
	logs := GetAllLogPaths(func(log *NginxLogCache) bool { return log.Type == "access" })
	paths := make([]string, 0, len(logs))
	for _, log := range logs {
		paths = append(paths, log.Path)
	}
	return paths
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

	// The removal above also drops a default log path when this config file
	// declares it, so put the defaults back. Registering them last keeps the
	// default marker on the shared path, which is what protects it from the next
	// removal. This only replays the already resolved paths; resolving them
	// again is the post-scan callback's job.
	reapplyDefaultLogPaths()

	return nil
}
