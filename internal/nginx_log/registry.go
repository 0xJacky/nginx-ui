package nginx_log

import (
	"path/filepath"
	"sort"
	"sync"

	"github.com/0xJacky/Nginx-UI/internal/nginx_log/utils"
	"github.com/uozi-tech/cosy/logger"
)

// NginxLogCache is a log path known to the host.
type NginxLogCache struct {
	Path       string `json:"path"`        // Path to the log file
	Type       string `json:"type"`        // Type of log: "access" or "error"
	Name       string `json:"name"`        // Name of the log file
	ConfigFile string `json:"config_file"` // Path to the configuration file that contains this log directive
}

// configLogRegistry holds every log path discovered by scanning the nginx
// configuration. The nginx default logs live in defaultLogRegistry instead.
var (
	configLogRegistry      = make(map[string]*NginxLogCache)
	configLogRegistryMutex sync.RWMutex
)

// AddLogPath adds a log path to the registry with the source config file
func AddLogPath(path, logType, name, configFile string) {
	configLogRegistryMutex.Lock()
	configLogRegistry[path] = &NginxLogCache{
		Path:       path,
		Type:       logType,
		Name:       name,
		ConfigFile: configFile,
	}
	configLogRegistryMutex.Unlock()
}

// RemoveLogPathsFromConfig removes all log paths associated with a specific config file
func RemoveLogPathsFromConfig(configFile string) {
	if configFile == defaultLogConfigFile {
		// defaultLogConfigFile is the marker carried by the nginx default
		// access/error logs, and no real configuration file can own an entry
		// tagged with it. Refusing the removal keeps a caller that lost the
		// config path from wiping the defaults.
		logger.Warn("Ignoring a request to remove nginx log paths for an empty config file")
		return
	}

	configLogRegistryMutex.Lock()
	for p, entry := range configLogRegistry {
		if entry.ConfigFile == configFile {
			delete(configLogRegistry, p)
		}
	}
	configLogRegistryMutex.Unlock()
}

// registryEntriesSnapshot returns every known log path: the ones discovered from
// the nginx configuration plus the nginx default access/error logs. The result
// is keyed by path, so a path that is both declared by a directive and a default
// log appears exactly once and can never produce a duplicate log group. The
// config-derived entry wins that collision so the UI keeps showing which file
// declares the path.
func registryEntriesSnapshot() []NginxLogCache {
	configLogRegistryMutex.RLock()
	merged := make(map[string]NginxLogCache, len(configLogRegistry))
	for path, entry := range configLogRegistry {
		merged[path] = *entry
	}
	configLogRegistryMutex.RUnlock()

	for _, entry := range defaultLogPathEntries() {
		if _, declared := merged[entry.Path]; declared {
			continue
		}
		merged[entry.Path] = entry
	}

	entries := make([]NginxLogCache, 0, len(merged))
	for _, entry := range merged {
		entries = append(entries, entry)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })

	return entries
}

// GetAllLogPaths returns all known log paths, optionally filtered
func GetAllLogPaths(filters ...func(*NginxLogCache) bool) []*NginxLogCache {
	var logs []*NginxLogCache
	for _, entry := range registryEntriesSnapshot() {
		e := entry
		if matchesAll(&e, filters) {
			logs = append(logs, &e)
		}
	}
	return logs
}

// GetAllLogPathsGrouped returns the known log paths grouped by their main log
// path, so a rotated file such as access.log.1 folds into access.log. The path
// of a group is its main log path.
func GetAllLogPathsGrouped(filters ...func(*NginxLogCache) bool) []*NginxLogCache {
	grouped := make(map[string]*NginxLogCache)
	order := make([]string, 0)
	for _, entry := range registryEntriesSnapshot() {
		base := utils.MainLogPathFromFile(entry.Path)
		if _, ok := grouped[base]; ok {
			continue
		}
		grouped[base] = &NginxLogCache{
			Path:       base,
			Type:       entry.Type,
			Name:       filepath.Base(base),
			ConfigFile: entry.ConfigFile,
		}
		order = append(order, base)
	}

	sort.Strings(order)

	result := make([]*NginxLogCache, 0, len(order))
	for _, base := range order {
		if matchesAll(grouped[base], filters) {
			result = append(result, grouped[base])
		}
	}
	return result
}

func matchesAll(entry *NginxLogCache, filters []func(*NginxLogCache) bool) bool {
	for _, f := range filters {
		if !f(entry) {
			return false
		}
	}
	return true
}
