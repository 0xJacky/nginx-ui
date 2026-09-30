package plugin

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

// A supervisor reports its transitions without its lock held, so a report can
// arrive after a newer one. The listed state must still be the state of the
// supervisor, or a plugin shows as running while every call to it fails with
// "plugin is not running".
func TestStateReportsFollowTheSupervisorTheEntryHolds(t *testing.T) {
	m := newTestManager(t)
	current := NewSupervisor(SupervisorConfig{PluginID: "official.alpha"})
	item := &entry{id: "official.alpha", manifest: pluginManifest("official.alpha"), supervisor: current}
	m.mu.Lock()
	m.entries[item.id] = item
	m.mu.Unlock()

	stateOf := func() State {
		m.mu.RLock()
		defer m.mu.RUnlock()
		return item.state
	}

	// The process exited right after it came up: the supervisor waits for a
	// restart, and the report of the start arrives last.
	current.mu.Lock()
	current.state = StateStarting
	current.lastErr = errors.New("exit status 1")
	current.mu.Unlock()
	m.onStateChange(item.id, current, StateStarting, errors.New("exit status 1"))
	m.onStateChange(item.id, current, StateRunning, nil)
	assert.Equal(t, StateStarting, stateOf())

	// A supervisor dropped by a re-enable or an upgrade no longer speaks for
	// the plugin.
	current.mu.Lock()
	current.state = StateRunning
	current.lastErr = nil
	current.mu.Unlock()
	m.onStateChange(item.id, current, StateRunning, nil)
	previous := NewSupervisor(SupervisorConfig{PluginID: "official.alpha"})
	m.onStateChange(item.id, previous, StateStopped, nil)
	assert.Equal(t, StateRunning, stateOf())
}
