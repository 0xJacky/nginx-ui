package plugin

import (
	"slices"
	"sort"
	"strings"

	"github.com/0xJacky/Nginx-UI/internal/plugin/protocol"
)

// NotifyChannelEntry is one notify channel offered by one enabled plugin.
type NotifyChannelEntry struct {
	PluginID string                 `json:"plugin_id"`
	Channel  protocol.NotifyChannel `json:"channel"`
}

// ProbeKindEntry is one probe kind offered by one enabled plugin.
type ProbeKindEntry struct {
	PluginID string             `json:"plugin_id"`
	Kind     protocol.ProbeKind `json:"kind"`
}

// MCPToolEntry is one MCP tool offered by one enabled plugin that was granted
// the mcp permission.
type MCPToolEntry struct {
	PluginID string           `json:"plugin_id"`
	Tool     protocol.MCPTool `json:"tool"`
}

// StorageBackendEntry is one storage backend offered by one enabled plugin.
type StorageBackendEntry struct {
	PluginID string                  `json:"plugin_id"`
	Backend  protocol.StorageBackend `json:"backend"`
}

// DeployTargetEntry is one deploy target kind offered by one enabled plugin
// that was granted the cert.deploy permission.
type DeployTargetEntry struct {
	PluginID string                `json:"plugin_id"`
	Target   protocol.DeployTarget `json:"target"`
}

// declaredCodes lists the codes a manifest claims for a capability whose
// entries are addressed by code.
func declaredCodes(manifest *protocol.Manifest, capability string) []string {
	if manifest == nil {
		return nil
	}
	var codes []string
	switch capability {
	case protocol.CapabilityDNS01:
		if manifest.DNS01 != nil {
			for _, provider := range manifest.DNS01.Providers {
				codes = append(codes, provider.Code)
			}
		}
	case protocol.CapabilityNotify:
		if manifest.Notify != nil {
			for _, channel := range manifest.Notify.Channels {
				codes = append(codes, channel.Code)
			}
		}
	case protocol.CapabilityProbe:
		if manifest.Probe != nil {
			for _, kind := range manifest.Probe.Kinds {
				codes = append(codes, kind.Code)
			}
		}
	case protocol.CapabilityStorage:
		if manifest.Storage != nil {
			for _, backend := range manifest.Storage.Backends {
				codes = append(codes, backend.Code)
			}
		}
	case protocol.CapabilityCertDeploy:
		// A target kind is only served while the key may be handed over.
		if manifest.Deploy != nil && slices.Contains(manifest.Permissions, protocol.PermissionCertDeploy) {
			for _, target := range manifest.Deploy.Targets {
				codes = append(codes, target.Code)
			}
		}
	}
	return codes
}

// NotifyChannels lists every channel offered by an enabled notify plugin,
// ordered by plugin id and code. A code several plugins declare is listed once
// per plugin; OwnerOf decides which one serves it.
func (m *Manager) NotifyChannels() []NotifyChannelEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var channels []NotifyChannelEntry
	for _, item := range m.entries {
		if !enabledWithCapabilityLocked(item, protocol.CapabilityNotify) || item.manifest.Notify == nil {
			continue
		}
		for _, channel := range item.manifest.Notify.Channels {
			channels = append(channels, NotifyChannelEntry{PluginID: item.id, Channel: channel})
		}
	}
	sort.Slice(channels, func(i, j int) bool {
		if channels[i].PluginID == channels[j].PluginID {
			return channels[i].Channel.Code < channels[j].Channel.Code
		}
		return channels[i].PluginID < channels[j].PluginID
	})
	return channels
}

// ProbeKinds lists every kind offered by an enabled probe plugin, ordered by
// plugin id and code.
func (m *Manager) ProbeKinds() []ProbeKindEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var kinds []ProbeKindEntry
	for _, item := range m.entries {
		if !enabledWithCapabilityLocked(item, protocol.CapabilityProbe) || item.manifest.Probe == nil {
			continue
		}
		for _, kind := range item.manifest.Probe.Kinds {
			kinds = append(kinds, ProbeKindEntry{PluginID: item.id, Kind: kind})
		}
	}
	sort.Slice(kinds, func(i, j int) bool {
		if kinds[i].PluginID == kinds[j].PluginID {
			return kinds[i].Kind.Code < kinds[j].Kind.Code
		}
		return kinds[i].PluginID < kinds[j].PluginID
	})
	return kinds
}

// MCPTools lists every tool of an enabled mcp plugin whose approved
// permissions include mcp, ordered by plugin id and tool name.
func (m *Manager) MCPTools() []MCPToolEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var tools []MCPToolEntry
	for _, item := range m.entries {
		if !enabledWithCapabilityLocked(item, protocol.CapabilityMCP) || item.manifest.MCP == nil {
			continue
		}
		// enabledWithCapabilityLocked already requires the approved set to
		// match the manifest, so the manifest permission is the granted one.
		if !slices.Contains(item.manifest.Permissions, protocol.PermissionMCP) {
			continue
		}
		for _, tool := range item.manifest.MCP.Tools {
			tools = append(tools, MCPToolEntry{PluginID: item.id, Tool: tool})
		}
	}
	sort.Slice(tools, func(i, j int) bool {
		if tools[i].PluginID == tools[j].PluginID {
			return tools[i].Tool.Name < tools[j].Tool.Name
		}
		return tools[i].PluginID < tools[j].PluginID
	})
	return tools
}

// StorageBackends lists every backend offered by an enabled storage plugin,
// ordered by plugin id and code.
func (m *Manager) StorageBackends() []StorageBackendEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var backends []StorageBackendEntry
	for _, item := range m.entries {
		if !enabledWithCapabilityLocked(item, protocol.CapabilityStorage) || item.manifest.Storage == nil {
			continue
		}
		for _, backend := range item.manifest.Storage.Backends {
			backends = append(backends, StorageBackendEntry{PluginID: item.id, Backend: backend})
		}
	}
	sort.Slice(backends, func(i, j int) bool {
		if backends[i].PluginID == backends[j].PluginID {
			return backends[i].Backend.Code < backends[j].Backend.Code
		}
		return backends[i].PluginID < backends[j].PluginID
	})
	return backends
}

// DeployTargets lists every target kind of an enabled cert.deploy plugin
// whose approved permissions include cert.deploy, ordered by plugin id and
// code.
func (m *Manager) DeployTargets() []DeployTargetEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var targets []DeployTargetEntry
	for _, item := range m.entries {
		if !enabledWithCapabilityLocked(item, protocol.CapabilityCertDeploy) || item.manifest.Deploy == nil {
			continue
		}
		// enabledWithCapabilityLocked already requires the approved set to
		// match the manifest, so the manifest permission is the granted one.
		if !slices.Contains(item.manifest.Permissions, protocol.PermissionCertDeploy) {
			continue
		}
		for _, target := range item.manifest.Deploy.Targets {
			targets = append(targets, DeployTargetEntry{PluginID: item.id, Target: target})
		}
	}
	sort.Slice(targets, func(i, j int) bool {
		if targets[i].PluginID == targets[j].PluginID {
			return targets[i].Target.Code < targets[j].Target.Code
		}
		return targets[i].PluginID < targets[j].PluginID
	})
	return targets
}

// mcpToolSeparator joins the plugin prefix and the tool name. Plugin ids never
// contain an underscore, so the first separator always ends the prefix.
const mcpToolSeparator = "__"

// MCPToolName is the name a plugin tool is published under on the MCP server
// (spec NAME-11): the plugin id with every dot replaced by an underscore, two
// underscores and the tool name.
func MCPToolName(pluginID, tool string) string {
	return strings.ReplaceAll(pluginID, ".", "_") + mcpToolSeparator + tool
}

// ParseMCPToolName reverses MCPToolName.
func ParseMCPToolName(name string) (pluginID, tool string, ok bool) {
	prefix, tool, found := strings.Cut(name, mcpToolSeparator)
	if !found || prefix == "" || tool == "" {
		return "", "", false
	}
	pluginID = strings.ReplaceAll(prefix, "_", ".")
	if !IsValidID(pluginID) {
		return "", "", false
	}
	return pluginID, tool, true
}
