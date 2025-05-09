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
	modulesCache     = orderedmap.NewOrderedMap[string, Module]()
	modulesCacheLock sync.RWMutex
	lastPIDPath      string
	lastPIDModTime   time.Time
	lastPIDSize      int64
)

// clearModulesCache clears the modules cache
func clearModulesCache() {
	modulesCacheLock.Lock()
	defer modulesCacheLock.Unlock()

	modulesCache = orderedmap.NewOrderedMap[string, Module]()
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
	if !IsNginxRunning() {
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

	// Create a map of loaded dynamic modules
	loadedDynamicModules := make(map[string]bool)
	for _, match := range matches {
		if len(match) > 1 {
			// Extract the module name without path and suffix
			moduleName := match[1]
			// Some normalization to match format in GetModules
			moduleName = strings.TrimPrefix(moduleName, "ngx_")
			moduleName = strings.TrimSuffix(moduleName, "_module")
			loadedDynamicModules[moduleName] = true
		}
	}

	// Update the status for each module in the cache
	for key := range modulesCache.Keys() {
		// If the module is already marked as dynamic, check if it's actually loaded
		if loadedDynamicModules[key] {
			modulesCache.Set(key, Module{
				Name:    key,
				Dynamic: true,
				Loaded:  true,
			})
		}
	}
}

func GetModules() *orderedmap.OrderedMap[string, Module] {
	modulesCacheLock.RLock()
	cachedModules := modulesCache
	modulesCacheLock.RUnlock()

	// If we have cached modules and PID file hasn't changed, return cached modules
	if cachedModules.Len() > 0 && !isPIDFileChanged() {
		return cachedModules
	}

	// If PID has changed or we don't have cached modules, get fresh modules
	out := getNginxV()

	// Regular expression to find built-in modules in nginx -V output
	builtinRe := regexp.MustCompile(`--with-([a-zA-Z0-9_-]+)(_module)?`)
	builtinMatches := builtinRe.FindAllStringSubmatch(out, -1)

	// Extract built-in module names from matches and put in map for quick lookup
	moduleMap := make(map[string]bool)
	for _, match := range builtinMatches {
		if len(match) > 1 {
			module := match[1]
			moduleMap[module] = true
		}
	}

	// Regular expression to find dynamic modules in nginx -V output
	dynamicRe := regexp.MustCompile(`--with-([a-zA-Z0-9_-]+)(_module)?=dynamic`)
	dynamicMatches := dynamicRe.FindAllStringSubmatch(out, -1)

	// Extract dynamic module names from matches
	for _, match := range dynamicMatches {
		if len(match) > 1 {
			module := match[1]
			// Only add if not already in list (to avoid duplicates)
			if !moduleMap[module] {
				moduleMap[module] = true
			}
		}
	}

	// Update cache
	modulesCacheLock.Lock()
	modulesCache = orderedmap.NewOrderedMap[string, Module]()
	for module := range moduleMap {
		// Mark modules as built-in (loaded) or dynamic (potentially not loaded)
		if strings.Contains(out, "--with-"+module+"=dynamic") {
			modulesCache.Set(module, Module{
				Name:    module,
				Dynamic: true,
				Loaded:  true,
			})
		} else {
			modulesCache.Set(module, Module{
				Name:    module,
				Dynamic: true,
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
