package nginx_log

import (
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/0xJacky/Nginx-UI/internal/nginx"
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
// RemoveLogPathsFromConfig free of special cases.
var (
	defaultLogRegistry      = make(map[string]*NginxLogCache)
	defaultLogRegistryMutex sync.RWMutex
)

// RefreshDefaultLogPaths resolves the nginx default access and error logs and
// replaces the default registry with the result. It returns the number of
// registered default paths.
//
// Without it a server whose access_log directives are all commented out - the
// Homebrew nginx.conf ships exactly like that - has no log path at all in the
// log list, even though the log file itself exists and is viewable, because
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
	changed := len(next) != len(previous)
	if !changed {
		for path := range previous {
			if _, kept := next[path]; !kept {
				changed = true
				break
			}
		}
	}
	defaultLogRegistryMutex.Unlock()

	if changed {
		logger.Infof("Registered %d nginx default log path(s): %s",
			len(resolved), strings.Join(defaultLogPathList(), ", "))
	}

	return len(resolved)
}

// resolveDefaultLogPaths returns the nginx default access and error logs that
// are viewable through the log whitelist, de-duplicated by path.
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
		// produce two log groups. The access log is resolved first and wins.
		if _, duplicate := seen[candidate.path]; duplicate {
			continue
		}
		seen[candidate.path] = struct{}{}

		// The whitelist always contains the directory of the default log paths,
		// so this normally passes. It is still enforced: both settings can be
		// pointed at an arbitrary path, and a path such as /dev/stdout resolves
		// to a device that must never be offered for reading.
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
