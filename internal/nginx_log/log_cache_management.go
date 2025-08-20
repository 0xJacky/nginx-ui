package nginx_log

import (
	"sync"
)

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

// RemoveLogPathsFromConfig removes all log paths associated with a specific config file
func RemoveLogPathsFromConfig(configFile string) {
	cacheMutex.Lock()
	defer cacheMutex.Unlock()

	for path, logEntry := range logCache {
		if logEntry.ConfigFile == configFile {
			delete(logCache, path)
		}
	}
}

// GetAllLogPaths returns all cached log paths, optionally filtered
func GetAllLogPaths(filters ...func(*NginxLogCache) bool) []*NginxLogCache {
	cacheMutex.RLock()
	defer cacheMutex.RUnlock()

	var logs []*NginxLogCache
	
	for _, logEntry := range logCache {
		// Apply all filters
		include := true
		for _, filter := range filters {
			if !filter(logEntry) {
				include = false
				break
			}
		}
		
		if include {
			logs = append(logs, logEntry)
		}
	}

	return logs
}

// ClearLogCache clears the entire log cache
func ClearLogCache() {
	cacheMutex.Lock()
	defer cacheMutex.Unlock()

	logCache = make(map[string]*NginxLogCache)
}