package nginx

import (
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/elliotchance/orderedmap/v3"
)

const (
	ModuleStream = "stream"
)

type Module struct {
	Name    string `json:"name"`
	Params  string `json:"params,omitempty"`
	Dynamic bool   `json:"dynamic"`
	Loaded  bool   `json:"loaded"`
}

// modulesCache stores the cached modules list and related metadata
var (
	modulesCache     = orderedmap.NewOrderedMap[string, *Module]()
	modulesCacheLock sync.RWMutex
	lastPIDPath      string
	lastPIDModTime   time.Time
	lastPIDSize      int64
)

// clearModulesCache clears the modules cache
func clearModulesCache() {
	modulesCacheLock.Lock()
	defer modulesCacheLock.Unlock()

	modulesCache = orderedmap.NewOrderedMap[string, *Module]()
	lastPIDPath = ""
	lastPIDModTime = time.Time{}
	lastPIDSize = 0
}

// ClearModulesCache clears the modules cache (public version for external use)
func ClearModulesCache() {
	clearModulesCache()
}

// isPIDFileChanged checks if the PID file has changed since the last check
func isPIDFileChanged() bool {
	pidPath := GetPIDPath()

	// If PID path has changed, consider it changed
	if pidPath != lastPIDPath {
		return true
	}

	// If Nginx is not running, consider PID changed
	if !IsRunning() {
		return true
	}

	// Check if PID file has changed (modification time or size)
	fileInfo, err := os.Stat(pidPath)
	if err != nil {
		return true
	}

	modTime := fileInfo.ModTime()
	size := fileInfo.Size()

	return modTime != lastPIDModTime || size != lastPIDSize
}

// updatePIDFileInfo updates the stored PID file information
func updatePIDFileInfo() {
	pidPath := GetPIDPath()

	if fileInfo, err := os.Stat(pidPath); err == nil {
		modulesCacheLock.Lock()
		defer modulesCacheLock.Unlock()

		lastPIDPath = pidPath
		lastPIDModTime = fileInfo.ModTime()
		lastPIDSize = fileInfo.Size()
	}
}

// addLoadedDynamicModules discovers modules loaded via load_module statements
// that might not be present in the configure arguments (e.g., externally installed modules)
func addLoadedDynamicModules() {
	// Get nginx -T output to find load_module statements
	out := getNginxT()
	if out == "" {
		return
	}

	// Use the shared regex function to find loaded dynamic modules
	loadModuleRe := GetLoadModuleRegex()
	matches := loadModuleRe.FindAllStringSubmatch(out, -1)

	modulesCacheLock.Lock()
	defer modulesCacheLock.Unlock()

	for _, match := range matches {
		if len(match) > 1 {
			// Extract the module name from load_module statement and normalize it
			loadModuleName := match[1]
			normalizedName := normalizeModuleNameFromLoadModule(loadModuleName)

			// Check if this module is already in our cache
			if _, exists := modulesCache.Get(normalizedName); !exists {
				// This is a module that's loaded but not in configure args
				// Add it as a dynamic module that's loaded
				modulesCache.Set(normalizedName, &Module{
					Name:    normalizedName,
					Params:  "",
					Dynamic: true, // Loaded via load_module, so it's dynamic
					Loaded:  true, // We found it in load_module statements, so it's loaded
				})
			}
		}
	}
}

// updateDynamicModulesStatus checks which dynamic modules are actually loaded in the running Nginx
func updateDynamicModulesStatus() {
	modulesCacheLock.Lock()
	defer modulesCacheLock.Unlock()

	// If cache is empty, there's nothing to update
	if modulesCache.Len() == 0 {
		return
	}

	// Get nginx -T output to check for loaded modules
	out := getNginxT()
	if out == "" {
		return
	}

	// Use the shared regex function to find loaded dynamic modules
	loadModuleRe := GetLoadModuleRegex()
	matches := loadModuleRe.FindAllStringSubmatch(out, -1)

	for _, match := range matches {
		if len(match) > 1 {
			// Extract the module name from load_module statement and normalize it
			loadModuleName := match[1]
			normalizedName := normalizeModuleNameFromLoadModule(loadModuleName)

			// Try to find the module in our cache using the normalized name
			module, ok := modulesCache.Get(normalizedName)
			if ok {
				module.Loaded = true
			}
		}
	}
}

// GetLoadModuleRegex returns a compiled regular expression to match nginx load_module statements.
// It matches both quoted and unquoted module paths:
//   - load_module "/usr/local/nginx/modules/ngx_stream_module.so";
//   - load_module modules/ngx_http_upstream_fair_module.so;
//
// The regex captures the module name (without path and extension).
func GetLoadModuleRegex() *regexp.Regexp {
	// Pattern explanation:
	// load_module\s+ - matches "load_module" followed by whitespace
	// "? - optional opening quote
	// (?:[^"\s]+/)? - non-capturing group for optional path (any non-quote, non-space chars ending with /)
	// ([a-zA-Z0-9_-]+) - capturing group for module name
	// \.so - matches ".so" extension
	// "? - optional closing quote
	// \s*; - optional whitespace followed by semicolon
	return regexp.MustCompile(`load_module\s+"?(?:[^"\s]+/)?([a-zA-Z0-9_-]+)\.so"?\s*;`)
}

// normalizeModuleNameFromLoadModule converts a module name from load_module statement
// to match the format used in configure arguments.
// Examples:
//   - "ngx_stream_module" -> "stream"
//   - "ngx_http_geoip_module" -> "http_geoip"
//   - "ngx_stream_geoip_module" -> "stream_geoip"
//   - "ngx_http_image_filter_module" -> "http_image_filter"
func normalizeModuleNameFromLoadModule(moduleName string) string {
	// Remove "ngx_" prefix if present
	normalized := strings.TrimPrefix(moduleName, "ngx_")

	// Remove "_module" suffix if present
	normalized = strings.TrimSuffix(normalized, "_module")

	return normalized
}

// normalizeModuleNameFromConfigure converts a module name from configure arguments
// to a consistent format for internal use.
// Examples:
//   - "stream" -> "stream"
//   - "http_geoip_module" -> "http_geoip"
//   - "http_image_filter_module" -> "http_image_filter"
func normalizeModuleNameFromConfigure(moduleName string) string {
	// Remove "_module" suffix if present to keep consistent format
	normalized := strings.TrimSuffix(moduleName, "_module")

	return normalized
}

// getExpectedLoadModuleName converts a configure argument module name
// to the expected load_module statement module name.
// Examples:
//   - "stream" -> "ngx_stream_module"
//   - "http_geoip" -> "ngx_http_geoip_module"
//   - "stream_geoip" -> "ngx_stream_geoip_module"
func getExpectedLoadModuleName(configureModuleName string) string {
	normalized := normalizeModuleNameFromConfigure(configureModuleName)
	return "ngx_" + normalized + "_module"
}

// normalizeAddModuleName converts a module name from --add-module arguments
// to a consistent format for internal use.
// Examples:
//   - "ngx_devel_kit" -> "devel_kit"
//   - "echo-nginx-module" -> "echo_nginx"
//   - "headers-more-nginx-module" -> "headers_more_nginx"
//   - "ngx_lua" -> "lua"
//   - "set-misc-nginx-module" -> "set_misc_nginx"
//   - "ngx_stream_lua" -> "stream_lua"
func normalizeAddModuleName(addModuleName string) string {
	// Convert dashes to underscores
	normalized := strings.ReplaceAll(addModuleName, "-", "_")

	// Remove common prefixes
	normalized = strings.TrimPrefix(normalized, "ngx_")

	// Remove common suffixes - prioritize longer suffixes first
	if strings.HasSuffix(normalized, "_nginx_module") {
		// For modules ending with "_nginx_module", remove only "_module" to keep "_nginx"
		normalized = strings.TrimSuffix(normalized, "_module")
	} else if strings.HasSuffix(normalized, "_module") {
		normalized = strings.TrimSuffix(normalized, "_module")
	}

	return normalized
}

func GetModules() *orderedmap.OrderedMap[string, *Module] {
	modulesCacheLock.RLock()
	cachedModules := modulesCache
	modulesCacheLock.RUnlock()

	// If we have cached modules and PID file hasn't changed, return cached modules
	if cachedModules.Len() > 0 && !isPIDFileChanged() {
		return cachedModules
	}

	// If PID has changed or we don't have cached modules, get fresh modules
	out := getNginxV()

	// Update cache
	modulesCacheLock.Lock()
	modulesCache = orderedmap.NewOrderedMap[string, *Module]()

	// Regular expression to find --with- module parameters with values
	paramRe := regexp.MustCompile(`--with-([a-zA-Z0-9_-]+)(?:_module)?(?:=([^"'\s]+|"[^"]*"|'[^']*'))?`)
	paramMatches := paramRe.FindAllStringSubmatch(out, -1)

	// Extract module names and parameters from --with- matches
	for _, match := range paramMatches {
		if len(match) > 1 {
			module := match[1]
			var params string

			// Check if there's a parameter value
			if len(match) > 2 && match[2] != "" {
				params = match[2]
				// Remove surrounding quotes if present
				params = strings.TrimPrefix(params, "'")
				params = strings.TrimPrefix(params, "\"")
				params = strings.TrimSuffix(params, "'")
				params = strings.TrimSuffix(params, "\"")
			}

			// Special handling for configuration options like cc-opt, not actual modules
			if module == "cc-opt" || module == "ld-opt" || module == "prefix" {
				modulesCache.Set(module, &Module{
					Name:    module,
					Params:  params,
					Dynamic: false,
					Loaded:  true,
				})
				continue
			}

			// Normalize the module name for consistent internal representation
			normalizedModuleName := normalizeModuleNameFromConfigure(module)

			// Determine if the module is dynamic
			isDynamic := false
			if strings.Contains(out, "--with-"+module+"=dynamic") ||
				strings.Contains(out, "--with-"+module+"_module=dynamic") {
				isDynamic = true
			}

			if params == "dynamic" {
				params = ""
			}

			modulesCache.Set(normalizedModuleName, &Module{
				Name:    normalizedModuleName,
				Params:  params,
				Dynamic: isDynamic,
				Loaded:  !isDynamic, // Static modules are always loaded
			})
		}
	}

	// Regular expression to find --add-module parameters
	// Matches patterns like: --add-module=../ngx_devel_kit-0.3.3 or --add-module=../echo-nginx-module-0.63
	addModuleRe := regexp.MustCompile(`--add-module=(?:[^/\s]+/)?([^/\s-]+(?:-[^/\s-]+)*)-[0-9.]+`)
	addModuleMatches := addModuleRe.FindAllStringSubmatch(out, -1)

	// Extract module names from --add-module matches
	for _, match := range addModuleMatches {
		if len(match) > 1 {
			moduleName := match[1]
			// Convert dashes to underscores for consistency
			normalizedName := strings.ReplaceAll(moduleName, "-", "_")
			// Further normalize the name
			finalNormalizedName := normalizeAddModuleName(normalizedName)

			// Add-modules are statically compiled, so they're always loaded but not dynamic
			modulesCache.Set(finalNormalizedName, &Module{
				Name:    finalNormalizedName,
				Params:  "",
				Dynamic: false, // --add-module creates static modules
				Loaded:  true,  // Static modules are always loaded
			})
		}
	}

	modulesCacheLock.Unlock()

	// Also check for modules loaded via load_module statements that might not be in configure args
	addLoadedDynamicModules()

	// Update dynamic modules status by checking if they're actually loaded
	updateDynamicModulesStatus()

	// Update PID file info
	updatePIDFileInfo()

	return modulesCache
}

// IsModuleLoaded checks if a module is loaded in Nginx
func IsModuleLoaded(module string) bool {
	// Get fresh modules to ensure we have the latest state
	GetModules()

	modulesCacheLock.RLock()
	defer modulesCacheLock.RUnlock()

	status, exists := modulesCache.Get(module)
	if !exists {
		return false
	}

	return status.Loaded
}
