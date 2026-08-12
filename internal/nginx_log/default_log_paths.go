package nginx_log

import (
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/internal/nginx_log/indexer"
	"github.com/0xJacky/Nginx-UI/internal/nginx_log/utils"
	"github.com/uozi-tech/cosy/logger"
)

// Log types recorded for a log path. They mirror the values produced by
// utils.ScanLogDirectives so nothing downstream can tell a default log path
// apart from one declared by an access_log/error_log directive.
const (
	logTypeAccess = "access"
	logTypeError  = "error"
)

// defaultLogConfigFile is the ConfigFile marker stored for the nginx default
// access and error logs.
//
// Those two paths are not declared by any configuration file: they come from
// settings.NginxSettings.AccessLogPath / ErrorLogPath, or from the
// --http-log-path / --error-log-path build flags reported by `nginx -V`.
// Recording a real configuration file here would make RemoveLogPathsFromConfig
// drop them the next time that file is rescanned, so the marker must be a value
// no configuration path can ever equal. The empty string is also what the index
// metadata already stores for entries without an owning configuration file, so
// the UI keeps rendering an empty "config_file" for them.
const defaultLogConfigFile = ""

// defaultLogRegistry holds the nginx default access and error log paths.
//
// It is deliberately kept apart from configLogRegistry. Entries in that map
// belong to the configuration file that declared them and are dropped whenever
// the file is rescanned without the directive, while the default paths have to
// survive every rescan; keeping them in their own map also keeps
// RemoveLogPathsFromConfig free of special cases. Like configLogRegistry it
// lives in the package rather than inside LogFileManager, so the paths outlive
// the StopServices/InitializeServices cycle that turning advanced indexing off
// and on again performs.
var (
	defaultLogRegistry      = make(map[string]*NginxLogCache)
	defaultLogRegistryMutex sync.RWMutex
)

// RefreshDefaultLogPaths resolves the nginx default access and error logs,
// replaces the default registry with the result and hands it to the running
// LogFileManager. It returns the number of registered default paths.
//
// Without it a server whose access_log directives are all commented out - the
// Homebrew nginx.conf ships exactly like that - has no log path at all to
// index, even though the log file itself exists and is previewable, because
// scanForLogDirectives can only discover paths that a directive spells out.
func RefreshDefaultLogPaths() int {
	resolved := resolveDefaultLogPaths()

	next := make(map[string]*NginxLogCache, len(resolved))
	for _, entry := range resolved {
		next[entry.Path] = entry
	}

	defaultLogRegistryMutex.Lock()
	previous := defaultLogRegistry
	defaultLogRegistry = next

	stale := make([]string, 0, len(previous))
	for path := range previous {
		if _, kept := next[path]; !kept {
			stale = append(stale, path)
		}
	}
	changed := len(stale) > 0 || len(next) != len(previous)
	defaultLogRegistryMutex.Unlock()

	manager := GetLogFileManager()

	for _, path := range stale {
		// A path that stopped being a default may still be declared by an
		// access_log directive, in which case it stays owned by that config file
		// and must not be dropped from the manager.
		if isConfigLogPath(path) {
			continue
		}
		if manager != nil {
			manager.RemoveLogPath(path)
		}
	}

	applyDefaultLogPaths(manager)

	if changed {
		logger.Infof("Registered %d nginx default log path(s) for indexing: %s",
			len(resolved), strings.Join(defaultLogPathList(), ", "))
	}

	return len(resolved)
}

// resolveDefaultLogPaths returns the nginx default access and error logs that
// are usable as index sources, de-duplicated by path.
func resolveDefaultLogPaths() []*NginxLogCache {
	candidates := []struct {
		path    string
		logType string
	}{
		{path: nginx.GetAccessLogPath(), logType: logTypeAccess},
		{path: nginx.GetErrorLogPath(), logType: logTypeError},
	}

	resolved := make([]*NginxLogCache, 0, len(candidates))
	seen := make(map[string]struct{}, len(candidates))

	for _, candidate := range candidates {
		if candidate.path == "" {
			continue
		}

		// A single file configured as both the access and the error log must not
		// produce two log groups. The access log is resolved first and wins,
		// because it is the type a rebuild actually indexes.
		if _, duplicate := seen[candidate.path]; duplicate {
			continue
		}
		seen[candidate.path] = struct{}{}

		// The whitelist always contains the directory of the default log paths,
		// so this normally passes. It is still enforced: both settings can be
		// pointed at an arbitrary path, and a path such as /dev/stdout resolves
		// to a device that must never be handed to the indexer.
		if !utils.IsValidLogPath(candidate.path) {
			logger.Debugf("Skipping nginx default %s log %q: not a regular file inside the log directory whitelist",
				candidate.logType, candidate.path)
			continue
		}

		resolved = append(resolved, &NginxLogCache{
			Path:       candidate.path,
			Type:       candidate.logType,
			Name:       filepath.Base(candidate.path),
			ConfigFile: defaultLogConfigFile,
		})
	}

	return resolved
}

// applyDefaultLogPaths hands the registered default log paths to the given
// LogFileManager. It is idempotent and tolerates a nil manager, which is the
// state while advanced indexing is disabled.
func applyDefaultLogPaths(manager *indexer.LogFileManager) int {
	if manager == nil {
		return 0
	}

	entries := defaultLogPathEntries()
	for _, entry := range entries {
		manager.AddLogPath(entry.Path, entry.Type, entry.Name, entry.ConfigFile)
	}

	return len(entries)
}

// reapplyDefaultLogPaths pushes the already resolved default log paths back into
// the running LogFileManager without resolving them again. A config rescan first
// removes every path owned by the rescanned file, which also drops the default
// path when that file happens to declare it, so the defaults have to be
// re-asserted right afterwards.
func reapplyDefaultLogPaths() int {
	return applyDefaultLogPaths(GetLogFileManager())
}

// defaultLogPathEntries returns a copy of the registered default log paths.
func defaultLogPathEntries() []NginxLogCache {
	defaultLogRegistryMutex.RLock()
	defer defaultLogRegistryMutex.RUnlock()

	entries := make([]NginxLogCache, 0, len(defaultLogRegistry))
	for _, entry := range defaultLogRegistry {
		entries = append(entries, *entry)
	}

	return entries
}

// defaultLogPathList returns the registered default log paths in a stable order,
// for logging.
func defaultLogPathList() []string {
	defaultLogRegistryMutex.RLock()
	defer defaultLogRegistryMutex.RUnlock()

	paths := make([]string, 0, len(defaultLogRegistry))
	for path := range defaultLogRegistry {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	return paths
}

// isDefaultLogPath reports whether the path is one of the nginx default logs.
func isDefaultLogPath(path string) bool {
	defaultLogRegistryMutex.RLock()
	defer defaultLogRegistryMutex.RUnlock()

	_, ok := defaultLogRegistry[path]
	return ok
}

// isConfigLogPath reports whether the path is declared by a configuration file.
func isConfigLogPath(path string) bool {
	configLogRegistryMutex.RLock()
	defer configLogRegistryMutex.RUnlock()

	_, ok := configLogRegistry[path]
	return ok
}
