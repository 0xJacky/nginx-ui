package plugin

import (
	"testing"

	"github.com/0xJacky/Nginx-UI/internal/plugin/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// capabilityManifest declares the notify, probe and mcp capabilities with one
// entry each.
func capabilityManifest() *protocol.Manifest {
	m := validManifest()
	m.Capabilities = []string{protocol.CapabilityNotify, protocol.CapabilityProbe, protocol.CapabilityMCP}
	m.Permissions = []string{protocol.PermissionNetwork, protocol.PermissionMCP}
	m.DNS01 = nil
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
	m.Permissions = append(m.Permissions, protocol.PermissionMCP)
	assert.NoError(t, ValidateManifest(m))
}

// lintCapabilityManifest is goodManifest serving the three capabilities.
func lintCapabilityManifest() *protocol.Manifest {
	m := goodManifest()
	blocks := capabilityManifest()
	m.Capabilities = blocks.Capabilities
	m.Permissions = blocks.Permissions
	m.DNS01 = nil
	m.Notify = blocks.Notify
	m.Probe = blocks.Probe
	m.MCP = blocks.MCP
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
		{"notify without block", func(m *protocol.Manifest) { m.Notify = nil }, LevelError, "MAN-31"},
		{"bad notify code", func(m *protocol.Manifest) { m.Notify.Channels[0].Code = "My Chat" }, LevelError, "NOTIFY-2"},
		{"notify channel without name", func(m *protocol.Manifest) { m.Notify.Channels[0].Name = "" }, LevelError, "NOTIFY-3"},
		{"notify field of unknown type", func(m *protocol.Manifest) {
			m.Notify.Channels[0].Configuration.Fields[0].Type = "color"
		}, LevelError, "NOTIFY-4"},
		{"probe without block", func(m *protocol.Manifest) { m.Probe = nil }, LevelError, "MAN-32"},
		{"duplicate probe code", func(m *protocol.Manifest) {
			m.Probe.Kinds = append(m.Probe.Kinds, protocol.ProbeKind{Code: "tcp-banner", Name: "Again"})
		}, LevelError, "PROBE-2"},
		{"probe field without display name", func(m *protocol.Manifest) {
			m.Probe.Kinds[0].Configuration.Fields[0].DisplayName = ""
		}, LevelError, "PROBE-3"},
		{"mcp without permission", func(m *protocol.Manifest) {
			m.Permissions = []string{protocol.PermissionNetwork}
		}, LevelError, "MAN-33"},
		{"mcp without tools", func(m *protocol.Manifest) { m.MCP = nil }, LevelError, "MAN-33"},
		{"bad mcp tool name", func(m *protocol.Manifest) { m.MCP.Tools[0].Name = "-purge" }, LevelError, "MCP-2"},
		{"mcp schema of another type", func(m *protocol.Manifest) {
			m.MCP.Tools[0].InputSchema = map[string]any{"type": "string"}
		}, LevelError, "MCP-3"},
		{"notify without network", func(m *protocol.Manifest) {
			m.Permissions = []string{protocol.PermissionMCP}
		}, LevelWarning, "SEC-3"},
		{"mcp permission without capability", func(m *protocol.Manifest) {
			m.Capabilities = []string{protocol.CapabilityNotify}
		}, LevelWarning, "SEC-5"},
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
