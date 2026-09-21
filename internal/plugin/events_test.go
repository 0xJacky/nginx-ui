package plugin

import (
	"context"
	"testing"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/event"
	"github.com/0xJacky/Nginx-UI/internal/plugin/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEventsReachASubscribedRunningPlugin(t *testing.T) {
	usePluginProcesses(t, pluginModeNormal)
	m := newTestManager(t)
	ctx := context.Background()
	m.Start(ctx)

	manifest := pluginManifest("official.alpha")
	manifest.Events = []string{protocol.EventNginxReloaded}
	_, err := m.Install(ctx, buildTestPackage(t, manifest, nil), InstallOptions{Enable: true})
	require.NoError(t, err)

	event.Publish(event.Event{Type: event.TypeNginxReloaded, Data: map[string]any{"ok": true}})

	assert.Eventually(t, func() bool {
		return logsContain(supervisorOf(m, "official.alpha"), "event "+protocol.EventNginxReloaded)
	}, 10*time.Second, 20*time.Millisecond, "the plugin never received the event")

	// An event nobody subscribed to is not delivered.
	event.Publish(event.Event{Type: event.TypeSiteSaved})
	time.Sleep(100 * time.Millisecond)
	assert.False(t, logsContain(supervisorOf(m, "official.alpha"), "event "+protocol.EventSiteSaved))
}

func TestEventsAreDroppedWhenTheQueueIsFull(t *testing.T) {
	m := newTestManager(t)

	manifest := pluginManifest("official.alpha")
	manifest.Events = []string{protocol.EventNginxReloaded}
	item := &entry{
		id:         "official.alpha",
		manifest:   manifest,
		state:      StateRunning,
		supervisor: NewSupervisor(SupervisorConfig{PluginID: "official.alpha"}),
		// A queue nobody drains is the state the counter exists for.
		events: make(chan protocol.EventNotification, 1),
	}
	m.mu.Lock()
	m.entries[item.id] = item
	m.mu.Unlock()

	m.dispatchEvent(event.Event{Type: event.TypeNginxReloaded})
	assert.Equal(t, int64(0), item.dropped.Load())

	m.dispatchEvent(event.Event{Type: event.TypeNginxReloaded})
	assert.Equal(t, int64(1), item.dropped.Load())

	info := m.infoOf(item)
	assert.Equal(t, int64(1), info.DroppedEvents)
}

func TestCronDefinitionUnderstandsBothSyntaxes(t *testing.T) {
	definition, err := cronDefinition("@every 10m")
	require.NoError(t, err)
	assert.NotNil(t, definition)

	definition, err = cronDefinition("*/5 * * * *")
	require.NoError(t, err)
	assert.NotNil(t, definition)

	_, err = cronDefinition("")
	assert.Error(t, err)
	_, err = cronDefinition("@every nonsense")
	assert.Error(t, err)
	_, err = cronDefinition("* * * *")
	assert.Error(t, err)
}
