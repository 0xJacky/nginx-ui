package plugin

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConformanceAgainstFakePlugin drives Conformance against the package's
// own fake plugin (see supervisor_test.go's runTestPlugin / TestMain),
// launched the same way manager_test.go does: a run.sh launcher that
// re-execs this test binary with NGINX_UI_TEST_PLUGIN_MODE set.
func TestConformanceAgainstFakePlugin(t *testing.T) {
	usePluginProcesses(t, pluginModeNormal)

	dir := t.TempDir()
	manifest := pluginManifest("official.conformance")
	writePluginDir(t, dir, manifest)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	report, err := Conformance(ctx, dir, ConformanceOptions{Timeout: 25 * time.Second})
	require.NoError(t, err)
	require.NotEmpty(t, report.Cases)
	assert.True(t, report.Passed(), "%+v", report.Cases)

	byKey := make(map[string]CaseResult, len(report.Cases))
	for _, c := range report.Cases {
		byKey[c.Rule+":"+c.Name] = c
	}

	handshake, ok := byKey["LIFE-1:handshake"]
	require.True(t, ok)
	assert.Equal(t, StatusPass, handshake.Status)

	ping, ok := byKey["LIFE-8:plugin.ping"]
	require.True(t, ok)
	assert.Equal(t, StatusPass, ping.Status)

	unknown, ok := byKey["WIRE-6:unknown method"]
	require.True(t, ok)
	assert.Equal(t, StatusPass, unknown.Status)

	concurrent, ok := byKey["WIRE-4:concurrent pings"]
	require.True(t, ok)
	assert.Equal(t, StatusPass, concurrent.Status)

	// The fake plugin never registers a dns01.* handler, so the optional
	// methods must be reported as skipped, never as failed.
	options, ok := byKey["DNS01-10:dns01.options"]
	require.True(t, ok)
	assert.Equal(t, StatusSkip, options.Status)

	shutdown, ok := byKey["LIFE-10:shutdown and exit"]
	require.True(t, ok)
	assert.Equal(t, StatusPass, shutdown.Status)
}

// TestConformanceRejectsInvalidManifest checks that a plugin whose manifest
// does not even validate is reported as a tool error, not as failed cases,
// since Conformance cannot run anything against it at all.
func TestConformanceRejectsInvalidManifest(t *testing.T) {
	dir := t.TempDir()
	manifest := pluginManifest("official.conformance-bad")
	manifest.APIVersion = 0
	writePluginDir(t, dir, manifest)

	_, err := Conformance(context.Background(), dir, ConformanceOptions{})
	assert.Error(t, err)
}

// TestConformanceCapabilitiesFilter checks that ConformanceOptions.Capabilities
// limits which capability specific cases run.
func TestConformanceCapabilitiesFilter(t *testing.T) {
	usePluginProcesses(t, pluginModeNormal)

	dir := t.TempDir()
	manifest := pluginManifest("official.conformance-filtered")
	writePluginDir(t, dir, manifest)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	report, err := Conformance(ctx, dir, ConformanceOptions{Timeout: 25 * time.Second, Capabilities: []string{"http"}})
	require.NoError(t, err)

	for _, c := range report.Cases {
		assert.NotContains(t, c.Rule, "DNS01", "dns01 cases must not run when only http was requested: %+v", c)
	}
}
