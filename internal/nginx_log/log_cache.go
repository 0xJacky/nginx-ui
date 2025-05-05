package nginx_log

import (
	"sync"
)

// NginxLogCache represents a cached log entry from nginx configuration
type NginxLogCache struct {
	Path string `json:"path"` // Path to the log file
	Type string `json:"type"` // Type of log: "access" or "error"
	Name string `json:"name"` // Name of the log file
}

var (
	// logCache is the map to store all found log files
	logCache   = make(map[string]*NginxLogCache)
	cacheMutex sync.RWMutex
)

// AddLogPath adds a log path to the log cache
func AddLogPath(path, logType, name string) {
	cacheMutex.Lock()
	defer cacheMutex.Unlock()

	logCache[path] = &NginxLogCache{
		Path: path,
		Type: logType,
		Name: name,
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
