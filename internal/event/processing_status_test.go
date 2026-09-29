package event

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
