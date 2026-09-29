package plugin

import (
	"slices"
	"strconv"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/event"
	"github.com/0xJacky/Nginx-UI/internal/plugin/jsonrpc"
	"github.com/0xJacky/Nginx-UI/internal/plugin/protocol"
)

// logPathsDebounce is how long a burst of scans is coalesced into one
// log.paths_changed notification (spec HOST-18).
const logPathsDebounce = time.Second

// LogFileSource connects host.logs.list and log.paths_changed to the nginx log
// registry of the host.
type LogFileSource interface {
	// LogFiles lists the log files a plugin holding log.files may read.
	LogFiles() []protocol.HostLogFile
	// SubscribeScan calls notify after every configuration scan, which may
	// have changed the list. notify must not block.
	SubscribeScan(notify func()) (unsubscribe func())
}

// SetLogFiles connects the log file listing. Passing nil disconnects it.
func (m *Manager) SetLogFiles(source LogFileSource) {
	m.logFilesMu.Lock()
	defer m.logFilesMu.Unlock()

	if m.logFilesUnsub != nil {
		m.logFilesUnsub()
		m.logFilesUnsub = nil
	}
	if m.logPathsTimer != nil {
		m.logPathsTimer.Stop()
		m.logPathsTimer = nil
	}
	m.logFiles = source
	m.logPathsSent = nil
	if source != nil {
		m.logFilesUnsub = source.SubscribeScan(m.scheduleLogPathsChanged)
	}
}

// logFileList returns the current list, never nil.
func (m *Manager) logFileList() []protocol.HostLogFile {
	m.logFilesMu.Lock()
	source := m.logFiles
	m.logFilesMu.Unlock()

	if source == nil {
		return []protocol.HostLogFile{}
	}
	logs := source.LogFiles()
	if logs == nil {
		return []protocol.HostLogFile{}
	}
	return logs
}

// scheduleLogPathsChanged starts or extends the debounce window.
func (m *Manager) scheduleLogPathsChanged() {
	m.logFilesMu.Lock()
	defer m.logFilesMu.Unlock()

	if m.logFiles == nil {
		return
	}
	delay := m.logPathsDelay
	if delay <= 0 {
		delay = logPathsDebounce
	}
	if m.logPathsTimer != nil {
		m.logPathsTimer.Stop()
	}
	m.logPathsTimer = time.AfterFunc(delay, m.flushLogPathsChanged)
}

// flushLogPathsChanged sends log.paths_changed when the list differs from the
// one sent last.
func (m *Manager) flushLogPathsChanged() {
	m.logFilesMu.Lock()
	source := m.logFiles
	m.logFilesMu.Unlock()
	if source == nil {
		return
	}

	current := source.LogFiles()
	if current == nil {
		current = []protocol.HostLogFile{}
	}

	m.logFilesMu.Lock()
	if m.logFiles != source || (m.logPathsSent != nil && slices.Equal(m.logPathsSent, current)) {
		m.logFilesMu.Unlock()
		return
	}
	m.logPathsSent = current
	m.logFilesMu.Unlock()

	m.dispatchEvent(event.Event{Type: event.TypeLogPathsChanged})
}

// LogsList implements HostBackend.
func (b *hostBackend) LogsList(string) []protocol.HostLogFile {
	return b.manager.logFileList()
}

// ActivitySet implements HostBackend. Entries are namespaced by plugin id in
// the processing status of the host.
func (b *hostBackend) ActivitySet(pluginID string, p protocol.HostActivitySetParams) error {
	// The manager lock is held while setting, so a call racing with the exit
	// of the plugin cannot add an entry after onStateChange cleared them.
	m := b.manager
	m.mu.RLock()
	defer m.mu.RUnlock()
	if item, ok := m.entries[pluginID]; p.Active && (!ok || (item.state != StateRunning && item.state != StateStarting)) {
		return nil
	}

	status := event.GetProcessingStatusManager()
	if !status.SetPluginActivity(pluginID, p.Key, p.Label, p.Active) {
		return jsonrpc.Errorf(protocol.CodeInvalidParams, "at most "+strconv.Itoa(event.MaxPluginActivities)+" activity entries per plugin")
	}
	return nil
}

// clearActivities removes every processing entry of a plugin whose process
// ended.
func (m *Manager) clearActivities(id string) {
	event.GetProcessingStatusManager().ClearPluginActivities(id)
}
