package plugin

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/event"
	"github.com/0xJacky/Nginx-UI/internal/plugin/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeLogFileSource is a LogFileSource whose list and scans the test drives.
type fakeLogFileSource struct {
	mu          sync.Mutex
	files       []protocol.HostLogFile
	notify      func()
	unsubscribe atomic.Int32
}

func (f *fakeLogFileSource) LogFiles() []protocol.HostLogFile {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]protocol.HostLogFile(nil), f.files...)
}

func (f *fakeLogFileSource) SubscribeScan(notify func()) func() {
	f.mu.Lock()
	f.notify = notify
	f.mu.Unlock()
	return func() { f.unsubscribe.Add(1) }
}

func (f *fakeLogFileSource) set(files ...protocol.HostLogFile) {
	f.mu.Lock()
	f.files = files
	f.mu.Unlock()
}

// scan simulates the post-scan callback of the host.
func (f *fakeLogFileSource) scan() {
	f.mu.Lock()
	notify := f.notify
	f.mu.Unlock()
	notify()
}

func accessLog(path string) protocol.HostLogFile {
	return protocol.HostLogFile{Path: path, Type: "access", Source: "config", ConfigFile: "/etc/nginx/nginx.conf"}
}

// addRunningPlugin registers a running plugin with a delivery queue and no
// process.
func addRunningPlugin(m *Manager, id string, events, permissions []string, state State) chan protocol.EventNotification {
	manifest := pluginManifest(id)
	manifest.Events = events
	manifest.Permissions = permissions
	queue := make(chan protocol.EventNotification, 16)
	item := &entry{
		id:         id,
		manifest:   manifest,
		state:      state,
		supervisor: NewSupervisor(SupervisorConfig{PluginID: id}),
		events:     queue,
	}
	m.mu.Lock()
	m.entries[id] = item
	m.mu.Unlock()
	return queue
}

func received(queue chan protocol.EventNotification, wait time.Duration) (protocol.EventNotification, bool) {
	select {
	case n := <-queue:
		return n, true
	case <-time.After(wait):
		return protocol.EventNotification{}, false
	}
}

func TestLogPathsChangedNeedsPermissionAndManifestEntry(t *testing.T) {
	m := newTestManager(t)
	listed := []string{protocol.EventLogPathsChanged}
	granted := []string{protocol.PermissionLogFiles}

	both := addRunningPlugin(m, "official.both", listed, granted, StateRunning)
	noPermission := addRunningPlugin(m, "official.noperm", listed, nil, StateRunning)
	notListed := addRunningPlugin(m, "official.nolist", nil, granted, StateRunning)
	stopped := addRunningPlugin(m, "official.stopped", listed, granted, StateStopped)

	m.dispatchEvent(event.Event{Type: event.TypeLogPathsChanged})

	n, ok := received(both, time.Second)
	require.True(t, ok)
	assert.Equal(t, protocol.EventLogPathsChanged, n.Type)
	assert.Nil(t, n.Data)
	for name, queue := range map[string]chan protocol.EventNotification{
		"without the permission": noPermission,
		"without the entry":      notListed,
		"not running":            stopped,
	} {
		_, ok := received(queue, 50*time.Millisecond)
		assert.False(t, ok, "a plugin %s must not receive the event", name)
	}
}

func TestOtherEventsNeedNoPermission(t *testing.T) {
	m := newTestManager(t)
	queue := addRunningPlugin(m, "official.alpha", []string{protocol.EventNginxReloaded}, nil, StateRunning)

	m.dispatchEvent(event.Event{Type: event.TypeNginxReloaded})

	_, ok := received(queue, time.Second)
	assert.True(t, ok)
}

func TestLogPathsChangedIsCoalescedAndOnlySentOnChange(t *testing.T) {
	m := newTestManager(t)
	m.logPathsDelay = 30 * time.Millisecond
	queue := addRunningPlugin(m, "official.alpha",
		[]string{protocol.EventLogPathsChanged}, []string{protocol.PermissionLogFiles}, StateRunning)

	source := &fakeLogFileSource{}
	source.set(accessLog("/var/log/nginx/a.log"))
	m.SetLogFiles(source)

	// A burst of scans is one notification.
	for range 5 {
		source.scan()
	}
	_, ok := received(queue, time.Second)
	require.True(t, ok, "the first list is announced")
	_, ok = received(queue, 150*time.Millisecond)
	assert.False(t, ok, "a burst is coalesced")

	// A scan that finds the same list sends nothing.
	source.scan()
	_, ok = received(queue, 150*time.Millisecond)
	assert.False(t, ok, "an unchanged list is not announced")

	// A changed list is announced again.
	source.set(accessLog("/var/log/nginx/a.log"), accessLog("/var/log/nginx/b.log"))
	source.scan()
	_, ok = received(queue, time.Second)
	assert.True(t, ok)

	// Going back and forth inside one window changes nothing.
	source.set(accessLog("/var/log/nginx/a.log"))
	source.scan()
	source.set(accessLog("/var/log/nginx/a.log"), accessLog("/var/log/nginx/b.log"))
	source.scan()
	_, ok = received(queue, 150*time.Millisecond)
	assert.False(t, ok)
}

func TestSetLogFilesReplacesAndDisconnects(t *testing.T) {
	m := newTestManager(t)
	m.logPathsDelay = 10 * time.Millisecond
	queue := addRunningPlugin(m, "official.alpha",
		[]string{protocol.EventLogPathsChanged}, []string{protocol.PermissionLogFiles}, StateRunning)

	first := &fakeLogFileSource{}
	first.set(accessLog("/var/log/nginx/a.log"))
	m.SetLogFiles(first)
	assert.Equal(t, first.files, m.logFileList())

	second := &fakeLogFileSource{}
	m.SetLogFiles(second)
	assert.EqualValues(t, 1, first.unsubscribe.Load(), "the old subscription is dropped")
	assert.Empty(t, m.logFileList())

	// A pending notification of the old source never fires.
	first.scan()
	m.SetLogFiles(nil)
	assert.EqualValues(t, 1, second.unsubscribe.Load())
	_, ok := received(queue, 100*time.Millisecond)
	assert.False(t, ok)

	// No source means an empty, non nil list.
	logs := m.logFileList()
	assert.NotNil(t, logs)
	assert.Empty(t, logs)
	assert.Equal(t, []protocol.HostLogFile{}, m.hostBackend().LogsList("official.alpha"))
}

func TestActivityIsNamespacedLimitedAndCleared(t *testing.T) {
	m := newTestManager(t)
	addRunningPlugin(m, "official.alpha", nil, nil, StateRunning)
	addRunningPlugin(m, "official.beta", nil, nil, StateRunning)
	backend := m.hostBackend()
	status := event.GetProcessingStatusManager()
	t.Cleanup(func() {
		status.ClearPluginActivities("official.alpha")
		status.ClearPluginActivities("official.beta")
	})

	set := func(id, key string, active bool) error {
		return backend.ActivitySet(id, protocol.HostActivitySetParams{Key: key, Label: "Working " + key, Active: active})
	}
	pluginEntries := func(id string) []event.PluginActivity {
		var out []event.PluginActivity
		for _, item := range status.GetCurrentStatus().Plugins {
			if item.PluginID == id {
				out = append(out, item)
			}
		}
		return out
	}

	require.NoError(t, set("official.alpha", "indexing", true))
	require.NoError(t, set("official.beta", "indexing", true))
	assert.Equal(t, []event.PluginActivity{{PluginID: "official.alpha", Key: "indexing", Label: "Working indexing"}}, pluginEntries("official.alpha"))
	assert.Len(t, pluginEntries("official.beta"), 1, "the same key of another plugin is a separate entry")

	// One plugin cannot clear another plugin's entry: removing acts on its own.
	require.NoError(t, set("official.alpha", "indexing", false))
	assert.Empty(t, pluginEntries("official.alpha"))
	assert.Len(t, pluginEntries("official.beta"), 1)

	for i := range event.MaxPluginActivities {
		require.NoError(t, set("official.alpha", "k"+string(rune('a'+i)), true))
	}
	assert.Error(t, set("official.alpha", "over", true))
	status.ClearPluginActivities("official.alpha")

	// A process that ends takes its entries with it, whichever way it ends.
	for _, state := range []State{StateStopped, StateError, StateStarting} {
		m.onStateChange("official.alpha", nil, StateRunning, nil)
		require.NoError(t, set("official.beta", "x", true))
		require.NoError(t, set("official.alpha", "x", true))
		m.onStateChange("official.alpha", nil, state, nil)
		assert.Empty(t, pluginEntries("official.alpha"), "state %s", state)
		assert.NotEmpty(t, pluginEntries("official.beta"), "other plugins are untouched")
	}

	// A call that arrives after the process ended is ignored.
	m.onStateChange("official.alpha", nil, StateStopped, nil)
	require.NoError(t, set("official.alpha", "late", true))
	assert.Empty(t, pluginEntries("official.alpha"))
	require.NoError(t, set("official.unknown", "late", true))
	for _, item := range status.GetCurrentStatus().Plugins {
		assert.NotEqual(t, "official.unknown", item.PluginID)
	}

	// A plugin that stays up keeps them.
	m.onStateChange("official.alpha", nil, StateRunning, nil)
	require.NoError(t, set("official.alpha", "x", true))
	m.onStateChange("official.alpha", nil, StateRunning, nil)
	assert.NotEmpty(t, pluginEntries("official.alpha"))
}

func TestActivityIsClearedWhenAPluginIsStoppedOrUninstalled(t *testing.T) {
	usePluginProcesses(t, pluginModeNormal)
	m := newTestManager(t)
	ctx := context.Background()
	m.Start(ctx)
	status := event.GetProcessingStatusManager()
	t.Cleanup(func() { status.ClearPluginActivities("official.alpha") })

	_, err := m.Install(ctx, buildTestPackage(t, pluginManifest("official.alpha"), nil), InstallOptions{Enable: true})
	require.NoError(t, err)

	has := func() bool {
		for _, item := range status.GetCurrentStatus().Plugins {
			if item.PluginID == "official.alpha" {
				return true
			}
		}
		return false
	}

	require.NoError(t, m.hostBackend().ActivitySet("official.alpha", protocol.HostActivitySetParams{Key: "a", Label: "A", Active: true}))
	require.True(t, has())
	_, err = m.Disable(ctx, "official.alpha")
	require.NoError(t, err)
	assert.False(t, has(), "disabling clears the entries")

	_, err = m.Enable(ctx, "official.alpha", true)
	require.NoError(t, err)
	require.NoError(t, m.hostBackend().ActivitySet("official.alpha", protocol.HostActivitySetParams{Key: "a", Label: "A", Active: true}))
	require.True(t, has())
	require.NoError(t, m.Uninstall(ctx, "official.alpha", false))
	assert.False(t, has(), "uninstalling clears the entries")
}
