package nginx_log

import (
	"sync"
)

// NginxLogCache represents a cached log entry from nginx configuration
type NginxLogCache struct {
	Path       string `json:"path"`        // Path to the log file
	Type       string `json:"type"`        // Type of log: "access" or "error"
	Name       string `json:"name"`        // Name of the log file
	ConfigFile string `json:"config_file"` // Path to the configuration file that contains this log directive
}

var (
	// logCache is the map to store all found log files
	logCache   = make(map[string]*NginxLogCache)
	cacheMutex sync.RWMutex
)

// AddLogPath adds a log path to the log cache with the source config file
func AddLogPath(path, logType, name, configFile string) {
	cacheMutex.Lock()
	defer cacheMutex.Unlock()

	logCache[path] = &NginxLogCache{
		Path:       path,
		Type:       logType,
		Name:       name,
		ConfigFile: configFile,
	}
}

// RemoveLogPathsFromConfig removes all log paths that come from a specific config file
func RemoveLogPathsFromConfig(configFile string) {
	cacheMutex.Lock()
	defer cacheMutex.Unlock()

	for path, cache := range logCache {
		if cache.ConfigFile == configFile {
			delete(logCache, path)
		}
	}
}

// GetAllLogPaths returns all cached log paths
func GetAllLogPaths(filters ...func(*NginxLogCache) bool) []*NginxLogCache {
	cacheMutex.RLock()
	defer cacheMutex.RUnlock()

	result := make([]*NginxLogCache, 0, len(logCache))
	for _, cache := range logCache {
		flag := true
		if len(filters) > 0 {
			for _, filter := range filters {
				if !filter(cache) {
					flag = false
					break
				}
			}
		}
		if flag {
			result = append(result, cache)
		}
	}

	return result
}

// ClearLogCache clears all entries in the log cache
func ClearLogCache() {
	cacheMutex.Lock()
	defer cacheMutex.Unlock()

	// Clear the cache
	logCache = make(map[string]*NginxLogCache)
}
