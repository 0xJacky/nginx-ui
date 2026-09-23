package plugin

import (
	"context"
	"encoding/json"
	"fmt"
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
	protocol.CapabilityStorage,
	protocol.CapabilityCertDeploy,
	protocol.CapabilitySecurityBlocklist,
	protocol.CapabilityUpstreamDiscovery,
	protocol.CapabilityLogSink,
}

// testPluginNewCapability answers the notify, probe, mcp, storage,
// cert.deploy, security.blocklist and upstream.discovery methods of the fake
// plugin the same way on both transports.
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
	case protocol.MethodStorageValidate, protocol.MethodStorageList:
		var decoded protocol.StorageListParams
		_ = json.Unmarshal(params, &decoded)
		if decoded.Config["url"] == "" {
			return nil, &protocol.Error{Code: protocol.CodeInvalidConfig, Message: "url is required",
				Data: protocol.InvalidConfigData{Field: "url"}}, true
		}
		if method == protocol.MethodStorageValidate {
			return protocol.EmptyResult{}, nil, true
		}
		return protocol.StorageListResult{Objects: []protocol.StorageObject{}}, nil, true
	case protocol.MethodDeployValidate:
		return protocol.EmptyResult{}, nil, true
	case protocol.MethodDeployPush:
		var decoded protocol.DeployPushParams
		_ = json.Unmarshal(params, &decoded)
		if !decoded.DryRun || decoded.Certificate.PrivateKeyPEM == "" {
			return nil, &protocol.Error{Code: protocol.CodeInternalError, Message: "only dry runs are expected"}, true
		}
		return protocol.DeployPushResult{Message: "would push " + decoded.Certificate.Name}, nil, true
	case protocol.MethodBlocklistFetch:
		var decoded protocol.BlocklistFetchParams
		_ = json.Unmarshal(params, &decoded)
		if decoded.Config["api_key"] == "" {
			return nil, &protocol.Error{Code: protocol.CodeInvalidConfig, Message: "api_key is required",
				Data: protocol.InvalidConfigData{Field: "api_key"}}, true
		}
		return protocol.BlocklistFetchResult{Entries: []protocol.BlocklistEntry{{CIDR: "203.0.113.0/24"}}}, nil, true
	case protocol.MethodDiscoveryResolve:
		return protocol.DiscoveryResolveResult{Targets: []protocol.DiscoveryTarget{{Address: "10.0.0.5", Port: 8080}}}, nil, true
	default:
		return nil, nil, false
	}
}

// allCapsManifest declares what pluginModeAllCaps serves.
func allCapsManifest(id string) *protocol.Manifest {
	manifest := pluginManifest(id)
	manifest.Capabilities = allTestCapabilities
	manifest.Permissions = []string{protocol.PermissionMCP, protocol.PermissionCertDeploy, protocol.PermissionNetwork, protocol.PermissionLogRead}
	manifest.Notify = &protocol.ManifestNotify{Channels: []protocol.NotifyChannel{{
		Code: "mychat",
		Name: "MyChat",
		Configuration: &protocol.ConfigurationSchema{Fields: []protocol.ConfigurationField{
			{Key: "webhook_url", DisplayName: "Webhook URL", Required: true},
		}},
	}}}
	manifest.Probe = &protocol.ManifestProbe{Kinds: []protocol.ProbeKind{{Code: "tcp-banner", Name: "TCP banner"}}}
	manifest.MCP = &protocol.ManifestMCP{Tools: []protocol.MCPTool{{Name: "purge_cache", Description: "Purge the cache."}}}
	manifest.Storage = &protocol.ManifestStorage{Backends: []protocol.StorageBackend{{
		Code: "webdav",
		Name: "WebDAV",
		Configuration: &protocol.ConfigurationSchema{Fields: []protocol.ConfigurationField{
			{Key: "url", DisplayName: "URL", Required: true},
		}},
	}}}
	// A kind without required fields answers the dry run with a result.
	manifest.Deploy = &protocol.ManifestDeploy{Targets: []protocol.DeployTarget{{Code: "local-copy", Name: "Local copy"}}}
	// A source with a required field is rejected, a provider without one
	// answers with targets.
	manifest.Blocklist = &protocol.ManifestBlocklist{Sources: []protocol.BlocklistSource{{
		Code: "threatfeed",
		Name: "ThreatFeed",
		Configuration: &protocol.ConfigurationSchema{Fields: []protocol.ConfigurationField{
			{Key: "api_key", DisplayName: "API key", Required: true, Secret: true},
		}},
	}}}
	manifest.Discovery = &protocol.ManifestDiscovery{Providers: []protocol.DiscoveryProvider{{Code: "static", Name: "Static"}}}
	return manifest
}

func TestConformanceRunsTheNewCapabilityCases(t *testing.T) {
	usePluginProcesses(t, pluginModeAllCaps)

	dir := t.TempDir()
	writePluginDir(t, dir, allCapsManifest("official.conformance-caps"))

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	report, err := Conformance(ctx, dir, ConformanceOptions{Timeout: 50 * time.Second, HandshakeTimeout: testHandshakeTimeout})
	require.NoError(t, err)
	assert.True(t, report.Passed(), "%+v", report.Cases)

	byKey := casesByKey(report)
	for _, transport := range []string{protocol.TransportStdio, protocol.TransportGRPC} {
		for _, key := range []string{
			"NOTIFY-8:notify.validate",
			"PROBE-5:probe.check",
			"MCP-6:mcp.call unknown tool",
			"STORAGE-10:storage.validate",
			"STORAGE-8:storage.list",
			"DEPLOY-9:deploy.validate",
			"DEPLOY-6:deploy.push dry_run",
			"BLOCKLIST-6:blocklist.fetch",
			"DISCOVERY-5:discovery.resolve",
		} {
			c, ok := byKey[transport+"|"+key]
			if assert.True(t, ok, "missing case %s over %s in %+v", key, transport, report.Cases) {
				assert.Equal(t, StatusPass, c.Status, "%s over %s: %s", key, transport, c.Message)
			}
		}
	}
	logOnStdio, ok := byKey["stdio|LOGSINK-4:log.push on stdio"]
	if assert.True(t, ok, "missing LOGSINK-4 in %+v", report.Cases) {
		assert.Equal(t, StatusPass, logOnStdio.Status, logOnStdio.Message)
	}
	logStream, ok := byKey["grpc|LOGSINK-5:log.push stream"]
	if assert.True(t, ok, "missing LOGSINK-5 in %+v", report.Cases) {
		assert.Equal(t, StatusPass, logStream.Status, logStream.Message)
	}

	parity, ok := byKey["|TRANSPORT-1:identical results"]
	require.True(t, ok)
	assert.Equal(t, StatusPass, parity.Status, parity.Message)
	assert.Contains(t, parity.Message, protocol.MethodNotifyValidate)
	assert.Contains(t, parity.Message, protocol.MethodMCPCall)
	assert.Contains(t, parity.Message, protocol.MethodStorageValidate)
	assert.Contains(t, parity.Message, protocol.MethodDeployValidate)
	assert.Contains(t, parity.Message, protocol.MethodBlocklistFetch)
	assert.NotContains(t, parity.Message, protocol.MethodDiscoveryResolve, "a provider without required fields is not compared")

	// Limiting the run to one capability skips the others.
	report, err = Conformance(ctx, dir, ConformanceOptions{
		HandshakeTimeout: testHandshakeTimeout,
		Timeout:          50 * time.Second, Transport: TransportFlagStdio, Capabilities: []string{protocol.CapabilityProbe},
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

func TestConformanceStorageAndDeployCases(t *testing.T) {
	var got CaseStatus
	var message string
	record := func(_, _ string, status CaseStatus, _ time.Duration, format string, args ...any) {
		got = status
		message = fmt.Sprintf(format, args...)
	}
	required := &protocol.ConfigurationSchema{Fields: []protocol.ConfigurationField{{Key: "url", DisplayName: "URL", Required: true}}}
	withField := &protocol.Error{Code: protocol.CodeInvalidConfig, Data: map[string]any{"field": "url"}}

	backend := protocol.StorageBackend{Code: "webdav", Configuration: required}
	open := protocol.StorageBackend{Code: "local"}
	listTests := []struct {
		name    string
		backend protocol.StorageBackend
		caller  staticCaller
		want    CaseStatus
	}{
		{"required field rejected", backend, staticCaller{err: withField}, StatusPass},
		{"rejected without field", backend, staticCaller{err: &protocol.Error{Code: protocol.CodeInvalidConfig}}, StatusFail},
		{"accepted although required", backend, staticCaller{result: map[string]any{"objects": []any{}}}, StatusFail},
		{"open backend lists", open, staticCaller{result: map[string]any{"objects": []any{map[string]any{"key": "a", "size": 1.0}}}}, StatusPass},
		{"open backend lists nothing", open, staticCaller{result: map[string]any{}}, StatusPass},
		{"objects of another shape", open, staticCaller{result: map[string]any{"objects": "a"}}, StatusFail},
		{"open backend rejects", open, staticCaller{err: withField}, StatusFail},
		{"vendor failure", open, staticCaller{err: &protocol.Error{Code: protocol.CodeInternalError}}, StatusFail},
	}
	for _, tc := range listTests {
		t.Run("storage.list "+tc.name, func(t *testing.T) {
			runStorageList(t.Context(), tc.caller, tc.backend, record)
			assert.Equal(t, tc.want, got, message)
		})
	}

	runStorageValidate(t.Context(), staticCaller{err: withField}, backend, record)
	assert.Equal(t, StatusPass, got, message)
	runStorageValidate(t.Context(), staticCaller{err: &protocol.Error{Code: protocol.CodeUnsupported}}, backend, record)
	assert.Equal(t, StatusSkip, got, message)

	kind := protocol.DeployTarget{Code: "mycdn", Configuration: required}
	openKind := protocol.DeployTarget{Code: "local-copy"}
	pushTests := []struct {
		name   string
		kind   protocol.DeployTarget
		caller staticCaller
		want   CaseStatus
	}{
		{"required field rejected", kind, staticCaller{err: withField}, StatusPass},
		{"accepted although required", kind, staticCaller{result: protocol.DeployPushResult{}}, StatusFail},
		{"required kind fails otherwise", kind, staticCaller{err: &protocol.Error{Code: protocol.CodeInternalError}}, StatusFail},
		{"open kind answers", openKind, staticCaller{result: protocol.DeployPushResult{Message: "ok"}}, StatusPass},
		{"open kind fails", openKind, staticCaller{err: &protocol.Error{Code: protocol.CodeInternalError}}, StatusFail},
	}
	for _, tc := range pushTests {
		t.Run("deploy.push "+tc.name, func(t *testing.T) {
			runDeployDryRun(t.Context(), tc.caller, tc.kind, record)
			assert.Equal(t, tc.want, got, message)
		})
	}

	runDeployValidate(t.Context(), staticCaller{}, openKind, record)
	assert.Equal(t, StatusPass, got, message)
	runDeployValidate(t.Context(), staticCaller{}, kind, record)
	assert.Equal(t, StatusFail, got, message)

	certificate, err := conformanceCertificate()
	require.NoError(t, err)
	assert.Contains(t, certificate.CertificatePEM, "BEGIN CERTIFICATE")
	assert.Contains(t, certificate.PrivateKeyPEM, "BEGIN PRIVATE KEY")
	assert.Equal(t, []string{"conformance.invalid"}, certificate.Domains)
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

func TestConformanceBlocklistAndDiscoveryCases(t *testing.T) {
	var got CaseStatus
	var message string
	record := func(_, _ string, status CaseStatus, _ time.Duration, format string, args ...any) {
		got = status
		message = fmt.Sprintf(format, args...)
	}
	required := &protocol.ConfigurationSchema{Fields: []protocol.ConfigurationField{{Key: "api_key", DisplayName: "API key", Required: true}}}
	withField := &protocol.Error{Code: protocol.CodeInvalidConfig, Data: map[string]any{"field": "api_key"}}

	source := protocol.BlocklistSource{Code: "threatfeed", Configuration: required}
	open := protocol.BlocklistSource{Code: "public"}
	fetchTests := []struct {
		name   string
		source protocol.BlocklistSource
		caller staticCaller
		want   CaseStatus
	}{
		{"required field rejected", source, staticCaller{err: withField}, StatusPass},
		{"rejected without field", source, staticCaller{err: &protocol.Error{Code: protocol.CodeInvalidConfig}}, StatusFail},
		{"accepted although required", source, staticCaller{result: map[string]any{"entries": []any{}}}, StatusFail},
		{"open source lists", open, staticCaller{result: map[string]any{"entries": []any{map[string]any{"cidr": "2001:db8::/32"}}}}, StatusPass},
		{"open source lists nothing", open, staticCaller{result: map[string]any{}}, StatusPass},
		{"open source names a field", open, staticCaller{err: withField}, StatusPass},
		{"an entry is no address", open, staticCaller{result: map[string]any{"entries": []any{map[string]any{"cidr": "example.com"}}}}, StatusFail},
		{"entries of another shape", open, staticCaller{result: map[string]any{"entries": "a"}}, StatusFail},
		{"source failure", open, staticCaller{err: &protocol.Error{Code: protocol.CodeInternalError}}, StatusFail},
	}
	for _, tc := range fetchTests {
		t.Run("blocklist.fetch "+tc.name, func(t *testing.T) {
			runBlocklistFetch(t.Context(), tc.caller, tc.source, record)
			assert.Equal(t, tc.want, got, message)
		})
	}

	provider := protocol.DiscoveryProvider{Code: "registry", Configuration: required}
	openProvider := protocol.DiscoveryProvider{Code: "static"}
	serviceField := &protocol.Error{Code: protocol.CodeInvalidConfig, Data: map[string]any{"field": "service"}}
	resolveTests := []struct {
		name     string
		provider protocol.DiscoveryProvider
		caller   staticCaller
		want     CaseStatus
	}{
		{"required field rejected", provider, staticCaller{err: withField}, StatusPass},
		{"accepted although required", provider, staticCaller{result: map[string]any{"targets": []any{}}}, StatusFail},
		{"unknown service", openProvider, staticCaller{err: serviceField}, StatusPass},
		{"open provider resolves", openProvider, staticCaller{result: map[string]any{"targets": []any{map[string]any{"address": "10.0.0.1", "port": 80}}}}, StatusPass},
		{"a port out of range", openProvider, staticCaller{result: map[string]any{"targets": []any{map[string]any{"address": "10.0.0.1", "port": 70000}}}}, StatusFail},
		{"provider failure", openProvider, staticCaller{err: &protocol.Error{Code: protocol.CodeInternalError}}, StatusFail},
	}
	for _, tc := range resolveTests {
		t.Run("discovery.resolve "+tc.name, func(t *testing.T) {
			runDiscoveryResolve(t.Context(), tc.caller, tc.provider, record)
			assert.Equal(t, tc.want, got, message)
		})
	}
}
