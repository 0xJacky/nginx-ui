package plugin

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/plugin/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// pluginModeAllCaps serves gRPC and every capability of the fake plugin.
const pluginModeAllCaps = "allcaps"

var allTestCapabilities = []string{
	protocol.CapabilityDNS01,
	protocol.CapabilityNotify,
	protocol.CapabilityProbe,
	protocol.CapabilityMCP,
}

// testPluginNewCapability answers the notify, probe and mcp methods of the
// fake plugin the same way on both transports.
func testPluginNewCapability(method string, params json.RawMessage) (any, error, bool) {
	switch method {
	case protocol.MethodNotifyValidate:
		var decoded protocol.NotifyValidateParams
		_ = json.Unmarshal(params, &decoded)
		if decoded.Config["webhook_url"] == "" {
			return nil, &protocol.Error{Code: protocol.CodeInvalidConfig, Message: "webhook_url is required",
				Data: protocol.InvalidConfigData{Field: "webhook_url"}}, true
		}
		return protocol.EmptyResult{}, nil, true
	case protocol.MethodProbeCheck:
		return protocol.ProbeCheckResult{Status: protocol.ProbeStatusDown, LatencyMS: 1, Message: "unreachable"}, nil, true
	case protocol.MethodMCPCall:
		var decoded protocol.MCPCallParams
		_ = json.Unmarshal(params, &decoded)
		if decoded.Tool != "purge_cache" {
			return nil, &protocol.Error{Code: protocol.CodeInvalidParams, Message: "unknown tool: " + decoded.Tool}, true
		}
		return protocol.MCPCallResult{Content: []protocol.MCPContent{{Type: protocol.MCPContentTypeText, Text: "done"}}}, nil, true
	default:
		return nil, nil, false
	}
}

// allCapsManifest declares what pluginModeAllCaps serves.
func allCapsManifest(id string) *protocol.Manifest {
	manifest := pluginManifest(id)
	manifest.Capabilities = allTestCapabilities
	manifest.Permissions = []string{protocol.PermissionMCP}
	manifest.Notify = &protocol.ManifestNotify{Channels: []protocol.NotifyChannel{{
		Code: "mychat",
		Name: "MyChat",
		Configuration: &protocol.ConfigurationSchema{Fields: []protocol.ConfigurationField{
			{Key: "webhook_url", DisplayName: "Webhook URL", Required: true},
		}},
	}}}
	manifest.Probe = &protocol.ManifestProbe{Kinds: []protocol.ProbeKind{{Code: "tcp-banner", Name: "TCP banner"}}}
	manifest.MCP = &protocol.ManifestMCP{Tools: []protocol.MCPTool{{Name: "purge_cache", Description: "Purge the cache."}}}
	return manifest
}

func TestConformanceRunsTheNewCapabilityCases(t *testing.T) {
	usePluginProcesses(t, pluginModeAllCaps)

	dir := t.TempDir()
	writePluginDir(t, dir, allCapsManifest("official.conformance-caps"))

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	report, err := Conformance(ctx, dir, ConformanceOptions{Timeout: 50 * time.Second})
	require.NoError(t, err)
	assert.True(t, report.Passed(), "%+v", report.Cases)

	byKey := casesByKey(report)
	for _, transport := range []string{protocol.TransportStdio, protocol.TransportGRPC} {
		for _, key := range []string{
			"NOTIFY-8:notify.validate",
			"PROBE-5:probe.check",
			"MCP-6:mcp.call unknown tool",
		} {
			c, ok := byKey[transport+"|"+key]
			if assert.True(t, ok, "missing case %s over %s in %+v", key, transport, report.Cases) {
				assert.Equal(t, StatusPass, c.Status, "%s over %s: %s", key, transport, c.Message)
			}
		}
	}
	parity, ok := byKey["|TRANSPORT-1:identical results"]
	require.True(t, ok)
	assert.Equal(t, StatusPass, parity.Status, parity.Message)
	assert.Contains(t, parity.Message, protocol.MethodNotifyValidate)
	assert.Contains(t, parity.Message, protocol.MethodMCPCall)

	// Limiting the run to one capability skips the others.
	report, err = Conformance(ctx, dir, ConformanceOptions{
		Timeout: 50 * time.Second, Transport: TransportFlagStdio, Capabilities: []string{protocol.CapabilityProbe},
	})
	require.NoError(t, err)
	byKey = casesByKey(report)
	_, hasProbe := byKey["stdio|PROBE-5:probe.check"]
	_, hasNotify := byKey["stdio|NOTIFY-8:notify.validate"]
	_, hasMCP := byKey["stdio|MCP-6:mcp.call unknown tool"]
	assert.True(t, hasProbe)
	assert.False(t, hasNotify)
	assert.False(t, hasMCP)
}

func TestConformanceNotifyValidateCases(t *testing.T) {
	channel := protocol.NotifyChannel{Code: "mychat", Configuration: &protocol.ConfigurationSchema{
		Fields: []protocol.ConfigurationField{{Key: "webhook_url", DisplayName: "URL", Required: true}},
	}}
	optional := protocol.NotifyChannel{Code: "open"}

	tests := []struct {
		name    string
		channel protocol.NotifyChannel
		err     error
		want    CaseStatus
	}{
		{"rejected with field", channel, &protocol.Error{Code: protocol.CodeInvalidConfig, Data: map[string]any{"field": "webhook_url"}}, StatusPass},
		{"rejected without field", channel, &protocol.Error{Code: protocol.CodeInvalidConfig}, StatusFail},
		{"accepted although required", channel, nil, StatusFail},
		{"accepted without required fields", optional, nil, StatusPass},
		{"unsupported", channel, &protocol.Error{Code: protocol.CodeUnsupported}, StatusSkip},
		{"internal error", channel, &protocol.Error{Code: protocol.CodeInternalError}, StatusFail},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var got CaseStatus
			runNotifyValidate(t.Context(), staticCaller{err: tc.err}, tc.channel,
				func(_, _ string, status CaseStatus, _ time.Duration, _ string, _ ...any) { got = status })
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestConformanceProbeAndMCPCases(t *testing.T) {
	record := func(got *CaseStatus) recorder {
		return func(_, _ string, status CaseStatus, _ time.Duration, _ string, _ ...any) { *got = status }
	}

	var got CaseStatus
	runProbeCheck(t.Context(), staticCaller{result: protocol.ProbeCheckResult{Status: "sideways"}}, "tcp", record(&got))
	assert.Equal(t, StatusFail, got, "an unknown status fails")
	runProbeCheck(t.Context(), staticCaller{result: protocol.ProbeCheckResult{Status: protocol.ProbeStatusUp}}, "tcp", record(&got))
	assert.Equal(t, StatusPass, got)
	runProbeCheck(t.Context(), staticCaller{err: &protocol.Error{Code: protocol.CodeInternalError, Message: "dial failed"}}, "tcp", record(&got))
	assert.Equal(t, StatusFail, got, "an unreachable target must be a down result")

	runMCPUnknownTool(t.Context(), staticCaller{err: &protocol.Error{Code: protocol.CodeInvalidParams}}, "x", record(&got))
	assert.Equal(t, StatusPass, got)
	runMCPUnknownTool(t.Context(), staticCaller{result: protocol.MCPCallResult{}}, "x", record(&got))
	assert.Equal(t, StatusFail, got, "an unknown tool must not answer with a result")

	assert.Equal(t, "nginx-ui-conformance-unknown-tool-x", unknownMCPTool(&protocol.Manifest{MCP: &protocol.ManifestMCP{
		Tools: []protocol.MCPTool{{Name: "nginx-ui-conformance-unknown-tool"}},
	}}))
}

// staticCaller answers every call with one result or one error.
type staticCaller struct {
	result any
	err    error
}

func (c staticCaller) Call(_ context.Context, _ string, _ any, result any) error {
	if c.err != nil {
		return c.err
	}
	if result == nil || c.result == nil {
		return nil
	}
	encoded, err := json.Marshal(c.result)
	if err != nil {
		return err
	}
	return json.Unmarshal(encoded, result)
}

func (staticCaller) Notify(context.Context, string, any) error { return nil }
