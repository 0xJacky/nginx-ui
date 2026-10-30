package event

import (
	"encoding/json"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// resetProcessingStatus clears every flag on the singleton manager before and
// after the test, so tests do not depend on each other's leftovers.
func resetProcessingStatus(t *testing.T) *ProcessingStatusManager {
	t.Helper()
	m := GetProcessingStatusManager()
	reset := func() {
		m.UpdateIndexScanning(false)
		m.UpdateAutoCertProcessing(false)
	}
	reset()
	t.Cleanup(reset)
	return m
}

func TestGetProcessingStatusManagerReturnsSingleton(t *testing.T) {
	assert.Same(t, GetProcessingStatusManager(), GetProcessingStatusManager())
}

func TestProcessingStatusUpdatesPublishFullStatus(t *testing.T) {
	tests := []struct {
		name   string
		update func(m *ProcessingStatusManager, value bool)
		want   ProcessingStatusData
	}{
		{
			name:   "index scanning",
			update: (*ProcessingStatusManager).UpdateIndexScanning,
			want:   ProcessingStatusData{IndexScanning: true, Plugins: []PluginActivity{}},
		},
		{
			name:   "auto cert processing",
			update: (*ProcessingStatusManager).UpdateAutoCertProcessing,
			want:   ProcessingStatusData{AutoCertProcessing: true, Plugins: []PluginActivity{}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := resetProcessingStatus(t)
			hub := installGlobalHub(t)

			tt.update(m, true)

			assert.Equal(t, tt.want, m.GetCurrentStatus())
			messages := hub.snapshot()
			require.NotEmpty(t, messages)
			assert.Equal(t, recordedMessage{event: string(TypeProcessingStatus), data: tt.want}, messages[0])
		})
	}
}

func TestProcessingStatusUnchangedValueDoesNotPublish(t *testing.T) {
	tests := []struct {
		name   string
		update func(m *ProcessingStatusManager, value bool)
	}{
		{name: "index scanning", update: (*ProcessingStatusManager).UpdateIndexScanning},
		{name: "auto cert processing", update: (*ProcessingStatusManager).UpdateAutoCertProcessing},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := resetProcessingStatus(t)
			tt.update(m, true)

			hub := installGlobalHub(t)
			tt.update(m, true)
			assert.Empty(t, hub.snapshot(), "re-applying the current value should not publish")
		})
	}

	t.Run("false while already false", func(t *testing.T) {
		m := resetProcessingStatus(t)
		hub := installGlobalHub(t)

		m.UpdateIndexScanning(false)
		m.UpdateAutoCertProcessing(false)

		assert.Empty(t, hub.snapshot())
	})
}

func TestProcessingStatusFlagsAreIndependent(t *testing.T) {
	m := resetProcessingStatus(t)
	hub := installGlobalHub(t)

	m.UpdateIndexScanning(true)
	m.UpdateAutoCertProcessing(true)
	m.UpdateIndexScanning(false)

	assert.Equal(t, ProcessingStatusData{AutoCertProcessing: true, Plugins: []PluginActivity{}}, m.GetCurrentStatus())
	assert.Equal(t, []recordedMessage{
		{event: string(TypeProcessingStatus), data: ProcessingStatusData{IndexScanning: true, Plugins: []PluginActivity{}}},
		{event: string(TypeProcessingStatus), data: ProcessingStatusData{IndexScanning: true, AutoCertProcessing: true, Plugins: []PluginActivity{}}},
		{event: string(TypeProcessingStatus), data: ProcessingStatusData{AutoCertProcessing: true, Plugins: []PluginActivity{}}},
	}, hub.snapshot())
}

func TestBroadcastCurrentStatusAlwaysPublishes(t *testing.T) {
	m := resetProcessingStatus(t)
	m.UpdateAutoCertProcessing(true)
	hub := installGlobalHub(t)

	m.BroadcastCurrentStatus()
	m.BroadcastCurrentStatus()

	want := recordedMessage{event: string(TypeProcessingStatus), data: ProcessingStatusData{AutoCertProcessing: true, Plugins: []PluginActivity{}}}
	assert.Equal(t, []recordedMessage{want, want}, hub.snapshot())
}

func TestProcessingStatusConcurrentUpdates(t *testing.T) {
	m := resetProcessingStatus(t)
	installGlobalHub(t)

	const iterations = 100
	updates := []func(bool){m.UpdateIndexScanning, m.UpdateAutoCertProcessing}

	var wg sync.WaitGroup
	for _, update := range updates {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range iterations {
				update(i%2 == 0)
				_ = m.GetCurrentStatus()
			}
		}()
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for range iterations {
			m.BroadcastCurrentStatus()
		}
	}()
	wg.Wait()

	// The last iteration of each goroutine writes false (iterations is even).
	assert.Equal(t, ProcessingStatusData{Plugins: []PluginActivity{}}, m.GetCurrentStatus())
}

func newTestProcessingManager() *ProcessingStatusManager {
	return &ProcessingStatusManager{status: ProcessingStatusData{Plugins: []PluginActivity{}}}
}

func TestPluginActivitySetReplaceAndRemove(t *testing.T) {
	m := newTestProcessingManager()

	e := captureOne(t, func() { assert.True(t, m.SetPluginActivity("com.example.a", "indexing", "Indexing...", true)) })
	assert.Equal(t, TypeProcessingStatus, e.Type)
	data, ok := e.Data.(ProcessingStatusData)
	require.True(t, ok)
	assert.Equal(t, []PluginActivity{{PluginID: "com.example.a", Key: "indexing", Label: "Indexing..."}}, data.Plugins)

	// The same key replaces the label instead of adding an entry.
	m.SetPluginActivity("com.example.a", "indexing", "Still indexing...", true)
	assert.Equal(t, []PluginActivity{{PluginID: "com.example.a", Key: "indexing", Label: "Still indexing..."}}, m.GetCurrentStatus().Plugins)

	// Two plugins may use the same key.
	m.SetPluginActivity("com.example.b", "indexing", "Other", true)
	assert.Len(t, m.GetCurrentStatus().Plugins, 2)

	m.SetPluginActivity("com.example.a", "indexing", "", false)
	assert.Equal(t, []PluginActivity{{PluginID: "com.example.b", Key: "indexing", Label: "Other"}}, m.GetCurrentStatus().Plugins)
}

func TestPluginActivityRemovingAnUnknownEntryIsSilent(t *testing.T) {
	m := newTestProcessingManager()

	var published int
	unsubscribe := Subscribe(func(Event) { published++ })
	defer unsubscribe()

	assert.True(t, m.SetPluginActivity("com.example.a", "missing", "", false))
	m.ClearPluginActivities("com.example.a")
	assert.Zero(t, published)
}

func TestPluginActivityLimit(t *testing.T) {
	m := newTestProcessingManager()

	for i := range MaxPluginActivities {
		require.True(t, m.SetPluginActivity("com.example.a", fmt.Sprintf("k%d", i), "x", true))
	}
	assert.False(t, m.SetPluginActivity("com.example.a", "extra", "x", true))
	// Updating an existing entry and other plugins are not affected.
	assert.True(t, m.SetPluginActivity("com.example.a", "k0", "y", true))
	assert.True(t, m.SetPluginActivity("com.example.b", "extra", "x", true))
}

func TestPluginActivityClear(t *testing.T) {
	m := newTestProcessingManager()
	m.SetPluginActivity("com.example.a", "one", "One", true)
	m.SetPluginActivity("com.example.a", "two", "Two", true)
	m.SetPluginActivity("com.example.b", "one", "One", true)

	m.ClearPluginActivities("com.example.a")

	assert.Equal(t, []PluginActivity{{PluginID: "com.example.b", Key: "one", Label: "One"}}, m.GetCurrentStatus().Plugins)
}

func TestProcessingStatusJSONHasEmptyPluginList(t *testing.T) {
	m := newTestProcessingManager()
	m.SetPluginActivity("com.example.a", "one", "One", true)
	m.ClearPluginActivities("com.example.a")

	encoded, err := json.Marshal(m.GetCurrentStatus())
	require.NoError(t, err)
	assert.Contains(t, string(encoded), `"plugins":[]`)
}

func TestPublishedPluginListIsNotEditedLater(t *testing.T) {
	m := newTestProcessingManager()
	e := captureOne(t, func() { m.SetPluginActivity("com.example.a", "one", "One", true) })
	published := e.Data.(ProcessingStatusData)

	m.SetPluginActivity("com.example.a", "one", "Changed", true)
	assert.Equal(t, "One", published.Plugins[0].Label)
}
