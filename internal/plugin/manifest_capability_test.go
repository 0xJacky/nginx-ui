package plugin

import (
	"testing"

	"github.com/0xJacky/Nginx-UI/internal/plugin/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// capabilityManifest declares the notify, probe, mcp, storage, cert.deploy,
// security.blocklist, upstream.discovery and log.sink capabilities with one
// or two entries each.
func capabilityManifest() *protocol.Manifest {
	m := validManifest()
	m.Capabilities = []string{
		protocol.CapabilityNotify, protocol.CapabilityProbe, protocol.CapabilityMCP,
		protocol.CapabilityStorage, protocol.CapabilityCertDeploy,
		protocol.CapabilitySecurityBlocklist, protocol.CapabilityUpstreamDiscovery,
		protocol.CapabilityLogSink,
	}
	m.Permissions = []string{protocol.PermissionNetwork, protocol.PermissionMCP, protocol.PermissionCertDeploy, protocol.PermissionLogRead}
	m.DNS01 = nil
	m.LogSink = &protocol.ManifestLogSink{BatchSize: 512, FlushIntervalMS: 1000, Formats: []string{protocol.LogFormatCombined}}
	m.Notify = &protocol.ManifestNotify{Channels: []protocol.NotifyChannel{{
		Code: "mychat",
		Name: "MyChat",
		Configuration: &protocol.ConfigurationSchema{Fields: []protocol.ConfigurationField{
			{Key: "webhook_url", DisplayName: "Webhook URL", Required: true},
			{Key: "token", Type: protocol.ConfigurationFieldText, DisplayName: "Token", Secret: true},
		}},
	}}}
	m.Probe = &protocol.ManifestProbe{Kinds: []protocol.ProbeKind{{
		Code: "tcp-banner",
		Name: "TCP banner",
		Configuration: &protocol.ConfigurationSchema{Fields: []protocol.ConfigurationField{
			{Key: "port", Type: protocol.ConfigurationFieldNumber, DisplayName: "Port", Required: true},
			{Key: "tls", Type: protocol.ConfigurationFieldBool, DisplayName: "TLS"},
		}},
	}}}
	m.MCP = &protocol.ManifestMCP{Tools: []protocol.MCPTool{{
		Name:        "purge_cache",
		Description: "Purge cached paths of a zone.",
		InputSchema: map[string]any{"type": "object", "properties": map[string]any{"zone": map[string]any{"type": "string"}}},
	}, {
		Name:        "list-zones",
		Description: "List zones.",
	}}}
	m.Storage = &protocol.ManifestStorage{Backends: []protocol.StorageBackend{{
		Code: "webdav",
		Name: "WebDAV",
		Configuration: &protocol.ConfigurationSchema{Fields: []protocol.ConfigurationField{
			{Key: "url", DisplayName: "Server URL", Required: true},
			{Key: "password", DisplayName: "Password", Secret: true},
		}},
	}}}
	m.Deploy = &protocol.ManifestDeploy{Targets: []protocol.DeployTarget{{
		Code: "mycdn",
		Name: "MyCDN",
		Configuration: &protocol.ConfigurationSchema{Fields: []protocol.ConfigurationField{
			{Key: "zone_id", DisplayName: "Zone ID", Required: true},
			{Key: "api_token", DisplayName: "API token", Secret: true},
		}},
	}}}
	m.Blocklist = &protocol.ManifestBlocklist{Sources: []protocol.BlocklistSource{{
		Code:           "threatfeed",
		Name:           "ThreatFeed",
		RefreshSeconds: 900,
		Configuration: &protocol.ConfigurationSchema{Fields: []protocol.ConfigurationField{
			{Key: "api_key", DisplayName: "API key", Required: true, Secret: true},
			{Key: "min_score", Type: protocol.ConfigurationFieldNumber, DisplayName: "Minimum score"},
		}},
	}}}
	m.Discovery = &protocol.ManifestDiscovery{Providers: []protocol.DiscoveryProvider{{
		Code: "registry",
		Name: "Service registry",
		Configuration: &protocol.ConfigurationSchema{Fields: []protocol.ConfigurationField{
			{Key: "address", DisplayName: "Registry address", Required: true},
			{Key: "token", DisplayName: "Token", Secret: true},
		}},
	}}}
	return m
}

func TestValidateManifestAcceptsCapabilityBlocks(t *testing.T) {
	assert.NoError(t, ValidateManifest(capabilityManifest()))
}

func TestValidateManifestCapabilityBlocks(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(m *protocol.Manifest)
		reason string
	}{
		{"notify without block", func(m *protocol.Manifest) { m.Notify = nil }, "at least one channel"},
		{"notify without channels", func(m *protocol.Manifest) { m.Notify.Channels = nil }, "at least one channel"},
		{"notify code with upper case", func(m *protocol.Manifest) { m.Notify.Channels[0].Code = "MyChat" }, "notify channel code"},
		{"duplicate notify code", func(m *protocol.Manifest) {
			m.Notify.Channels = append(m.Notify.Channels, protocol.NotifyChannel{Code: "mychat", Name: "Again"})
		}, "declared twice"},
		{"notify channel without name", func(m *protocol.Manifest) { m.Notify.Channels[0].Name = "" }, "name is required"},
		{"field without key", func(m *protocol.Manifest) {
			m.Notify.Channels[0].Configuration.Fields[0].Key = ""
		}, "field key is required"},
		{"duplicate field key", func(m *protocol.Manifest) {
			m.Notify.Channels[0].Configuration.Fields[1].Key = "webhook_url"
		}, "declared twice"},
		{"unknown field type", func(m *protocol.Manifest) {
			m.Notify.Channels[0].Configuration.Fields[0].Type = "select"
		}, "unknown type"},
		{"field without display name", func(m *protocol.Manifest) {
			m.Notify.Channels[0].Configuration.Fields[0].DisplayName = ""
		}, "display_name"},
		{"probe without block", func(m *protocol.Manifest) { m.Probe = nil }, "at least one kind"},
		{"probe code too long", func(m *protocol.Manifest) {
			m.Probe.Kinds[0].Code = "a-very-long-probe-kind-code-that-overflows"
		}, "probe kind code"},
		{"probe kind without name", func(m *protocol.Manifest) { m.Probe.Kinds[0].Name = "" }, "name is required"},
		{"probe field with unknown type", func(m *protocol.Manifest) {
			m.Probe.Kinds[0].Configuration.Fields[1].Type = "secret"
		}, "unknown type"},
		{"mcp without block", func(m *protocol.Manifest) { m.MCP = nil }, "at least one tool"},
		{"mcp without permission", func(m *protocol.Manifest) {
			m.Permissions = []string{protocol.PermissionNetwork}
		}, "requires the mcp permission"},
		{"mcp tool name with dot", func(m *protocol.Manifest) { m.MCP.Tools[0].Name = "purge.cache" }, "mcp tool name"},
		{"mcp tool name with upper case", func(m *protocol.Manifest) { m.MCP.Tools[0].Name = "PurgeCache" }, "mcp tool name"},
		{"duplicate mcp tool", func(m *protocol.Manifest) { m.MCP.Tools[1].Name = "purge_cache" }, "declared twice"},
		{"mcp tool without description", func(m *protocol.Manifest) { m.MCP.Tools[0].Description = "" }, "description"},
		{"mcp schema of another type", func(m *protocol.Manifest) {
			m.MCP.Tools[0].InputSchema = map[string]any{"type": "array"}
		}, "type object"},
		{"storage without block", func(m *protocol.Manifest) { m.Storage = nil }, "at least one backend"},
		{"storage code with dot", func(m *protocol.Manifest) { m.Storage.Backends[0].Code = "web.dav" }, "storage backend code"},
		{"duplicate storage code", func(m *protocol.Manifest) {
			m.Storage.Backends = append(m.Storage.Backends, protocol.StorageBackend{Code: "webdav", Name: "Again"})
		}, "declared twice"},
		{"storage backend without name", func(m *protocol.Manifest) { m.Storage.Backends[0].Name = "" }, "name is required"},
		{"storage field of unknown type", func(m *protocol.Manifest) {
			m.Storage.Backends[0].Configuration.Fields[0].Type = "file"
		}, "unknown type"},
		{"deploy without block", func(m *protocol.Manifest) { m.Deploy = nil }, "at least one target"},
		{"deploy without permission", func(m *protocol.Manifest) {
			m.Permissions = []string{protocol.PermissionNetwork, protocol.PermissionMCP}
		}, "requires the cert.deploy permission"},
		{"deploy code too short", func(m *protocol.Manifest) { m.Deploy.Targets[0].Code = "x" }, "deploy target code"},
		{"duplicate deploy code", func(m *protocol.Manifest) {
			m.Deploy.Targets = append(m.Deploy.Targets, protocol.DeployTarget{Code: "mycdn", Name: "Again"})
		}, "declared twice"},
		{"deploy target without name", func(m *protocol.Manifest) { m.Deploy.Targets[0].Name = "" }, "name is required"},
		{"deploy field without key", func(m *protocol.Manifest) {
			m.Deploy.Targets[0].Configuration.Fields[1].Key = ""
		}, "field key is required"},
		{"blocklist without block", func(m *protocol.Manifest) { m.Blocklist = nil }, "at least one source"},
		{"blocklist without network", func(m *protocol.Manifest) {
			m.Permissions = []string{protocol.PermissionMCP, protocol.PermissionCertDeploy}
		}, "security.blocklist requires the network permission"},
		{"blocklist code with space", func(m *protocol.Manifest) { m.Blocklist.Sources[0].Code = "threat feed" }, "blocklist source code"},
		{"duplicate blocklist code", func(m *protocol.Manifest) {
			m.Blocklist.Sources = append(m.Blocklist.Sources, protocol.BlocklistSource{Code: "threatfeed", Name: "Again"})
		}, "declared twice"},
		{"blocklist source without name", func(m *protocol.Manifest) { m.Blocklist.Sources[0].Name = "" }, "name is required"},
		{"blocklist refresh too short", func(m *protocol.Manifest) { m.Blocklist.Sources[0].RefreshSeconds = 30 }, "refresh_seconds"},
		{"blocklist refresh negative", func(m *protocol.Manifest) { m.Blocklist.Sources[0].RefreshSeconds = -1 }, "refresh_seconds"},
		{"blocklist field of unknown type", func(m *protocol.Manifest) {
			m.Blocklist.Sources[0].Configuration.Fields[0].Type = "select"
		}, "unknown type"},
		{"discovery without block", func(m *protocol.Manifest) { m.Discovery = nil }, "at least one provider"},
		{"discovery without network", func(m *protocol.Manifest) {
			m.Capabilities = []string{protocol.CapabilityUpstreamDiscovery}
			m.Permissions = nil
		}, "upstream.discovery requires the network permission"},
		{"discovery code too long", func(m *protocol.Manifest) {
			m.Discovery.Providers[0].Code = "a-very-long-discovery-provider-code-that-overflows"
		}, "discovery provider code"},
		{"duplicate discovery code", func(m *protocol.Manifest) {
			m.Discovery.Providers = append(m.Discovery.Providers, protocol.DiscoveryProvider{Code: "registry", Name: "Again"})
		}, "declared twice"},
		{"discovery provider without name", func(m *protocol.Manifest) { m.Discovery.Providers[0].Name = "" }, "name is required"},
		{"log.sink without log.read", func(m *protocol.Manifest) {
			m.Permissions = []string{protocol.PermissionNetwork, protocol.PermissionMCP, protocol.PermissionCertDeploy}
		}, "log.sink requires the log.read permission"},
		{"log_sink batch too large", func(m *protocol.Manifest) { m.LogSink.BatchSize = protocol.MaxLogSinkBatchSize + 1 }, "log_sink.batch_size"},
		{"log_sink batch negative", func(m *protocol.Manifest) { m.LogSink.BatchSize = -1 }, "log_sink.batch_size"},
		{"log_sink flush too short", func(m *protocol.Manifest) { m.LogSink.FlushIntervalMS = 49 }, "log_sink.flush_interval_ms"},
		{"log_sink unknown format", func(m *protocol.Manifest) { m.LogSink.Formats = []string{"json"} }, "unknown format"},
		{"log_sink duplicate format", func(m *protocol.Manifest) {
			m.LogSink.Formats = []string{protocol.LogFormatRaw, protocol.LogFormatRaw}
		}, "listed twice"},
		{"negative memory hint", func(m *protocol.Manifest) {
			m.Server.Resources = &protocol.ManifestResources{MemoryMB: -1}
		}, "server.resources"},
		{"negative recommended memory", func(m *protocol.Manifest) {
			m.Server.Resources = &protocol.ManifestResources{RecommendedMemoryMB: -1}
		}, "server.resources"},
		{"negative cpu hint", func(m *protocol.Manifest) {
			m.Server.Resources = &protocol.ManifestResources{CPUPercent: -5}
		}, "server.resources"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := capabilityManifest()
			tc.mutate(m)
			err := ValidateManifest(m)
			require.Error(t, err)
			assert.ErrorIs(t, err, ErrManifestInvalid)
			assert.Contains(t, err.Error(), tc.reason)
		})
	}
}

func TestValidateManifestAcceptsTheMCPPermissionAlone(t *testing.T) {
	m := validManifest()
	m.Permissions = append(m.Permissions, protocol.PermissionMCP, protocol.PermissionCertDeploy)
	assert.NoError(t, ValidateManifest(m))
}

// lintCapabilityManifest is goodManifest serving the capabilities of
// capabilityManifest.
func lintCapabilityManifest() *protocol.Manifest {
	m := goodManifest()
	blocks := capabilityManifest()
	m.Capabilities = blocks.Capabilities
	m.Permissions = blocks.Permissions
	m.DNS01 = nil
	m.Notify = blocks.Notify
	m.Probe = blocks.Probe
	m.MCP = blocks.MCP
	m.Storage = blocks.Storage
	m.Deploy = blocks.Deploy
	m.Blocklist = blocks.Blocklist
	m.Discovery = blocks.Discovery
	m.LogSink = blocks.LogSink
	m.Server.Resources = &protocol.ManifestResources{MemoryMB: 128, CPUPercent: 50}
	return m
}

func TestLintCapabilityBlocksHaveNoFindings(t *testing.T) {
	dir := writeLintFixture(t, lintFixture{manifest: lintCapabilityManifest()})
	report, err := Lint(dir)
	require.NoError(t, err)
	assert.Empty(t, report.Findings, "%+v", report.Findings)
}

func TestLintCapabilityBlocks(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(m *protocol.Manifest)
		level  Level
		rule   string
	}{
		{"notify without block", func(m *protocol.Manifest) { m.Notify = nil }, LevelError, RuleNotifyBlock},
		{"bad notify code", func(m *protocol.Manifest) { m.Notify.Channels[0].Code = "My Chat" }, LevelError, RuleNotifyCode},
		{"notify channel without name", func(m *protocol.Manifest) { m.Notify.Channels[0].Name = "" }, LevelError, RuleNotifyName},
		{"notify field of unknown type", func(m *protocol.Manifest) {
			m.Notify.Channels[0].Configuration.Fields[0].Type = "color"
		}, LevelError, RuleConfigurationFields},
		{"probe without block", func(m *protocol.Manifest) { m.Probe = nil }, LevelError, RuleProbeBlock},
		{"duplicate probe code", func(m *protocol.Manifest) {
			m.Probe.Kinds = append(m.Probe.Kinds, protocol.ProbeKind{Code: "tcp-banner", Name: "Again"})
		}, LevelError, RuleProbeCode},
		{"probe field without display name", func(m *protocol.Manifest) {
			m.Probe.Kinds[0].Configuration.Fields[0].DisplayName = ""
		}, LevelError, RuleProbeKind},
		{"mcp without permission", func(m *protocol.Manifest) {
			m.Permissions = []string{protocol.PermissionNetwork}
		}, LevelError, RuleMCPBlock},
		{"mcp without tools", func(m *protocol.Manifest) { m.MCP = nil }, LevelError, RuleMCPBlock},
		{"bad mcp tool name", func(m *protocol.Manifest) { m.MCP.Tools[0].Name = "-purge" }, LevelError, RuleMCPTool},
		{"mcp schema of another type", func(m *protocol.Manifest) {
			m.MCP.Tools[0].InputSchema = map[string]any{"type": "string"}
		}, LevelError, RuleMCPInputSchema},
		{"notify without network", func(m *protocol.Manifest) {
			m.Permissions = []string{protocol.PermissionMCP}
		}, LevelWarning, RuleNetworkPermission},
		{"mcp permission without capability", func(m *protocol.Manifest) {
			m.Capabilities = []string{protocol.CapabilityNotify}
		}, LevelWarning, RuleUnusedPermission},
		{"storage without block", func(m *protocol.Manifest) { m.Storage = nil }, LevelError, RuleStorageBlock},
		{"bad storage code", func(m *protocol.Manifest) { m.Storage.Backends[0].Code = "Web DAV" }, LevelError, RuleStorageCode},
		{"storage backend without name", func(m *protocol.Manifest) { m.Storage.Backends[0].Name = "" }, LevelError, RuleStorageBackend},
		{"storage field without display name", func(m *protocol.Manifest) {
			m.Storage.Backends[0].Configuration.Fields[0].DisplayName = ""
		}, LevelError, RuleStorageBackend},
		{"deploy without permission", func(m *protocol.Manifest) {
			m.Permissions = []string{protocol.PermissionNetwork, protocol.PermissionMCP}
		}, LevelError, RuleDeployBlock},
		{"deploy without targets", func(m *protocol.Manifest) { m.Deploy = nil }, LevelError, RuleDeployBlock},
		{"duplicate deploy code", func(m *protocol.Manifest) {
			m.Deploy.Targets = append(m.Deploy.Targets, protocol.DeployTarget{Code: "mycdn", Name: "Again"})
		}, LevelError, RuleDeployCode},
		{"deploy target without name", func(m *protocol.Manifest) { m.Deploy.Targets[0].Name = "" }, LevelError, RuleDeployTarget},
		{"deploy field of unknown type", func(m *protocol.Manifest) {
			m.Deploy.Targets[0].Configuration.Fields[0].Type = "select"
		}, LevelError, RuleDeployTarget},
		{"storage without network", func(m *protocol.Manifest) {
			m.Capabilities = []string{protocol.CapabilityStorage}
			m.Permissions = nil
		}, LevelWarning, RuleNetworkPermission},
		{"deploy permission without capability", func(m *protocol.Manifest) {
			m.Capabilities = []string{protocol.CapabilityStorage}
		}, LevelWarning, RuleUnusedPermission},
		{"blocklist without network", func(m *protocol.Manifest) {
			m.Permissions = []string{protocol.PermissionMCP, protocol.PermissionCertDeploy}
		}, LevelError, RuleBlocklistBlock},
		{"blocklist without sources", func(m *protocol.Manifest) { m.Blocklist = nil }, LevelError, RuleBlocklistBlock},
		{"bad blocklist code", func(m *protocol.Manifest) { m.Blocklist.Sources[0].Code = "Threat" }, LevelError, RuleBlocklistCode},
		{"blocklist source without name", func(m *protocol.Manifest) { m.Blocklist.Sources[0].Name = "" }, LevelError, RuleBlocklistSource},
		{"blocklist refresh too short", func(m *protocol.Manifest) { m.Blocklist.Sources[0].RefreshSeconds = 59 }, LevelError, RuleBlocklistSource},
		{"blocklist field without key", func(m *protocol.Manifest) {
			m.Blocklist.Sources[0].Configuration.Fields[1].Key = ""
		}, LevelError, RuleBlocklistSource},
		{"discovery without network", func(m *protocol.Manifest) {
			m.Capabilities = []string{protocol.CapabilityUpstreamDiscovery}
			m.Permissions = nil
		}, LevelError, RuleDiscoveryBlock},
		{"discovery without providers", func(m *protocol.Manifest) { m.Discovery = nil }, LevelError, RuleDiscoveryBlock},
		{"duplicate discovery code", func(m *protocol.Manifest) {
			m.Discovery.Providers = append(m.Discovery.Providers, protocol.DiscoveryProvider{Code: "registry", Name: "Again"})
		}, LevelError, RuleDiscoveryCode},
		{"discovery field without display name", func(m *protocol.Manifest) {
			m.Discovery.Providers[0].Configuration.Fields[0].DisplayName = ""
		}, LevelError, RuleDiscoveryProvider},
		{"log.sink without log.read", func(m *protocol.Manifest) {
			m.Permissions = []string{protocol.PermissionNetwork, protocol.PermissionMCP, protocol.PermissionCertDeploy}
		}, LevelError, RuleLogSinkPermission},
		{"log.read without log.sink", func(m *protocol.Manifest) {
			m.Capabilities = []string{protocol.CapabilityStorage}
		}, LevelWarning, RuleUnusedPermission},
		{"log_sink block without log.sink", func(m *protocol.Manifest) {
			m.Capabilities = []string{protocol.CapabilityStorage}
		}, LevelWarning, RuleLogSinkBlock},
		{"log_sink batch too large", func(m *protocol.Manifest) { m.LogSink.BatchSize = 5000 }, LevelError, RuleLogSinkBatch},
		{"log_sink flush too short", func(m *protocol.Manifest) { m.LogSink.FlushIntervalMS = 10 }, LevelError, RuleLogSinkBatch},
		{"log_sink unknown format", func(m *protocol.Manifest) { m.LogSink.Formats = []string{"ltsv"} }, LevelError, RuleLogSinkFormats},
		{"log_sink duplicate format", func(m *protocol.Manifest) {
			m.LogSink.Formats = []string{protocol.LogFormatCombined, protocol.LogFormatCombined}
		}, LevelError, RuleLogSinkFormats},
		{"negative memory hint", func(m *protocol.Manifest) { m.Server.Resources.MemoryMB = -1 }, LevelError, RuleServerResources},
		{"negative recommended memory", func(m *protocol.Manifest) { m.Server.Resources.RecommendedMemoryMB = -1 }, LevelError, RuleServerResources},
		{"negative cpu hint", func(m *protocol.Manifest) { m.Server.Resources.CPUPercent = -1 }, LevelError, RuleServerResources},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := lintCapabilityManifest()
			tc.mutate(m)
			report, err := Lint(writeLintFixture(t, lintFixture{manifest: m}))
			require.NoError(t, err)
			assertHasFinding(t, report, tc.level, tc.rule)
		})
	}
}

func TestValidateManifestAcceptsTheDefaultBlocklistRefresh(t *testing.T) {
	m := capabilityManifest()
	m.Blocklist.Sources[0].RefreshSeconds = 0
	assert.NoError(t, ValidateManifest(m))
	m.Blocklist.Sources[0].RefreshSeconds = protocol.MinBlocklistRefreshSeconds
	assert.NoError(t, ValidateManifest(m))
}

func TestValidateManifestAcceptsLogSinkDefaults(t *testing.T) {
	m := capabilityManifest()
	m.LogSink = nil
	assert.NoError(t, ValidateManifest(m), "the log_sink block is optional")
	m.LogSink = &protocol.ManifestLogSink{BatchSize: protocol.MaxLogSinkBatchSize, FlushIntervalMS: protocol.MinLogSinkFlushIntervalMS,
		Formats: []string{protocol.LogFormatRaw, protocol.LogFormatCombined}}
	assert.NoError(t, ValidateManifest(m))
	m.Server.Resources = &protocol.ManifestResources{MemoryMB: 64, CPUPercent: 250}
	assert.NoError(t, ValidateManifest(m))
}
