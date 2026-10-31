package demo

import (
	"testing"

	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// withDemo flips the shared settings pointer for one test. NodeSettings is
// package-level state, so these tests must not call t.Parallel().
func withDemo(t *testing.T, enabled bool) {
	t.Helper()

	previous := settings.NodeSettings.Demo
	settings.NodeSettings.Demo = enabled
	t.Cleanup(func() { settings.NodeSettings.Demo = previous })
}

func TestInstallIsInertWhenDemoDisabled(t *testing.T) {
	withDemo(t, false)

	assert.Nil(t, Install(), "a normal node must install no overrides at all")
}

func TestInstallReportsEveryOverride(t *testing.T) {
	withDemo(t, true)

	applied := Install()

	require.Len(t, applied, len(overrides()))
	assert.Contains(t, applied, "log-fixtures")
}

func TestSeedIsStableAcrossCalls(t *testing.T) {
	first := seed("geo", "203.0.113.1")
	for range 100 {
		assert.Equal(t, first, seed("geo", "203.0.113.1"))
	}
	assert.NotEqual(t, first, seed("geo", "203.0.113.2"))
	assert.NotEqual(t, first, seed("upstream", "203.0.113.1"))
}

func TestRangeIntStaysInBounds(t *testing.T) {
	for i := range uint64(500) {
		got := rangeInt(i, 10, 20)
		assert.GreaterOrEqual(t, got, 10)
		assert.Less(t, got, 20)
	}

	// Degenerate ranges must not panic.
	assert.Equal(t, 5, rangeInt(123, 5, 5))
	assert.Equal(t, 5, rangeInt(123, 5, 1))
}

func TestPickHandlesEmptyTable(t *testing.T) {
	assert.Empty(t, pick([]string{}, 42))
	assert.Equal(t, "only", pick([]string{"only"}, 42))
}
