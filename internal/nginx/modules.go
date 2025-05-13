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

	// Regular expression to find loaded dynamic modules in nginx -T output
	// Look for lines like "load_module modules/ngx_http_image_filter_module.so;"
	loadModuleRe := regexp.MustCompile(`load_module\s+(?:modules/|/.*/)([a-zA-Z0-9_-]+)\.so;`)
	matches := loadModuleRe.FindAllStringSubmatch(out, -1)

	for _, match := range matches {
		if len(match) > 1 {
			// Extract the module name without path and suffix
			moduleName := match[1]
			// Some normalization to match format in GetModules
			moduleName = strings.TrimPrefix(moduleName, "ngx_")
			moduleName = strings.TrimSuffix(moduleName, "_module")
			module, ok := modulesCache.Get(moduleName)
			if ok {
				module.Loaded = true
			}
		}
	}
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

	// Regular expression to find module parameters with values
	paramRe := regexp.MustCompile(`--with-([a-zA-Z0-9_-]+)(?:_module)?(?:=([^"'\s]+|"[^"]*"|'[^']*'))?`)
	paramMatches := paramRe.FindAllStringSubmatch(out, -1)

	// Update cache
	modulesCacheLock.Lock()
	modulesCache = orderedmap.NewOrderedMap[string, *Module]()

	// Extract module names and parameters from matches
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

			// Determine if the module is dynamic
			isDynamic := false
			if strings.Contains(out, "--with-"+module+"=dynamic") ||
				strings.Contains(out, "--with-"+module+"_module=dynamic") {
				isDynamic = true
			}

			if params == "dynamic" {
				params = ""
			}

			modulesCache.Set(module, &Module{
				Name:    module,
				Params:  params,
				Dynamic: isDynamic,
				Loaded:  !isDynamic, // Static modules are always loaded
			})
		}
	}

	modulesCacheLock.Unlock()

	// Update dynamic modules status by checking if they're actually loaded
	updateDynamicModulesStatus()

	// Update PID file info
	updatePIDFileInfo()

	return modulesCache
}

// IsModuleLoaded checks if a module is loaded in Nginx
func IsModuleLoaded(module string) bool {
	// Ensure modules are in the cache
	if modulesCache.Len() == 0 {
		GetModules()
	}

	modulesCacheLock.RLock()
	defer modulesCacheLock.RUnlock()

	status, exists := modulesCache.Get(module)
	if !exists {
		return false
	}

	return status.Loaded
}
