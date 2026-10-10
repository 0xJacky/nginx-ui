package capability

import (
	"github.com/0xJacky/Nginx-UI/internal/nginx_log"
	"github.com/0xJacky/Nginx-UI/internal/plugin"
	"github.com/0xJacky/Nginx-UI/internal/plugin/protocol"
)

// LogFilesHost is the part of the plugin manager the log.files permission
// needs.
type LogFilesHost interface {
	// SetLogFiles connects host.logs.list and log.paths_changed.
	SetLogFiles(source plugin.LogFileSource)
}

// RegisterLogFiles lets plugins holding log.files list the nginx logs of the
// host and hear when the list changes. The list comes from the log registry,
// so it does not depend on the built-in indexer.
func RegisterLogFiles(h LogFilesHost) {
	h.SetLogFiles(NewLogFileSource(nginx_log.PluginLogFiles, nginx_log.SubscribeLogScan))
}

// NewLogFileSource builds the source from a lister and a scan subscription.
func NewLogFileSource(list func() []nginx_log.PluginLogFile, subscribe func(func()) func()) plugin.LogFileSource {
	return &logFileSource{list: list, subscribe: subscribe}
}

type logFileSource struct {
	list      func() []nginx_log.PluginLogFile
	subscribe func(func()) func()
}

func (s *logFileSource) LogFiles() []protocol.HostLogFile {
	files := s.list()
	out := make([]protocol.HostLogFile, len(files))
	for i, f := range files {
		out[i] = protocol.HostLogFile{Path: f.Path, Type: f.Type, Source: f.Source, ConfigFile: f.ConfigFile}
	}
	return out
}

func (s *logFileSource) SubscribeScan(notify func()) (unsubscribe func()) {
	return s.subscribe(notify)
}
