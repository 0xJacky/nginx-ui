package nginx_log

import (
	"sort"
	"sync"

	"github.com/0xJacky/Nginx-UI/internal/nginx_log/utils"
)

// Sources of a PluginLogFile.
const (
	PluginLogSourceConfig  = "config"
	PluginLogSourceDefault = "default"
)

// PluginLogFile is one log file the host offers to plugins.
type PluginLogFile struct {
	Path string
	// Type is "access" or "error".
	Type string
	// Source is PluginLogSourceConfig or PluginLogSourceDefault.
	Source string
	// ConfigFile is the configuration file naming the path, empty for a
	// default log.
	ConfigFile string
}

// PluginLogFiles lists the log files plugins may read. It reads the registry
// only, so it works whether or not the indexer runs. Paths outside the log
// viewer whitelist are left out and each path appears once.
func PluginLogFiles() []PluginLogFile {
	entries := registryEntriesSnapshot()
	files := make([]PluginLogFile, 0, len(entries))
	for _, entry := range entries {
		if entry.Type != logTypeAccess && entry.Type != logTypeError {
			continue
		}
		if !utils.IsValidLogPath(entry.Path) {
			continue
		}
		file := PluginLogFile{
			Path:       entry.Path,
			Type:       entry.Type,
			Source:     PluginLogSourceDefault,
			ConfigFile: entry.ConfigFile,
		}
		if entry.ConfigFile != defaultLogConfigFile {
			file.Source = PluginLogSourceConfig
		} else {
			file.ConfigFile = ""
		}
		files = append(files, file)
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	return files
}

var (
	scanListenersMu sync.Mutex
	scanListenerSeq int
	scanListeners   = map[int]func(){}
)

// SubscribeLogScan calls fn after every sweep of the configuration scan, when
// the log paths may have changed. fn must not block.
func SubscribeLogScan(fn func()) (unsubscribe func()) {
	scanListenersMu.Lock()
	defer scanListenersMu.Unlock()
	scanListenerSeq++
	id := scanListenerSeq
	scanListeners[id] = fn
	return func() {
		scanListenersMu.Lock()
		delete(scanListeners, id)
		scanListenersMu.Unlock()
	}
}

// notifyLogScan tells the subscribers a scan sweep finished.
func notifyLogScan() {
	scanListenersMu.Lock()
	listeners := make([]func(), 0, len(scanListeners))
	for _, fn := range scanListeners {
		listeners = append(listeners, fn)
	}
	scanListenersMu.Unlock()

	for _, fn := range listeners {
		fn()
	}
}
