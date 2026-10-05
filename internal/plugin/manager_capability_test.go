package plugin

import (
	"testing"

	"github.com/0xJacky/Nginx-UI/internal/plugin/protocol"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// addCapabilityEntry registers a plugin in memory. approved controls whether
// the stored approval matches the manifest permissions.
func addCapabilityEntry(m *Manager, manifest *protocol.Manifest, enabled, approved bool) {
	row := &model.Plugin{PluginID: manifest.ID, Enabled: enabled}
	if approved {
		row.ApprovedPermissions = approvedSet(manifest)
	} else {
		row.ApprovedPermissions = []string{}
	}
	m.entries[manifest.ID] = m.newEntry(manifest.ID, manifest, row)
}

func capabilityPluginManifest(id string) *protocol.Manifest {
	m := capabilityManifest()
	m.ID = id
	return m
}

func TestManagerListsCapabilityEntries(t *testing.T) {
	m := newManager(t.TempDir())
	addCapabilityEntry(m, capabilityPluginManifest("io.github.b.plugin"), true, true)
	addCapabilityEntry(m, capabilityPluginManifest("io.github.a.plugin"), true, true)
	addCapabilityEntry(m, capabilityPluginManifest("io.github.disabled.plugin"), false, true)
	addCapabilityEntry(m, capabilityPluginManifest("io.github.unapproved.plugin"), true, false)

	channels := m.NotifyChannels()
	require.Len(t, channels, 2)
	assert.Equal(t, "io.github.a.plugin", channels[0].PluginID)
	assert.Equal(t, "mychat", channels[0].Channel.Code)
	assert.Equal(t, "io.github.b.plugin", channels[1].PluginID)

	kinds := m.ProbeKinds()
	require.Len(t, kinds, 2)
	assert.Equal(t, "tcp-banner", kinds[0].Kind.Code)

	tools := m.MCPTools()
	require.Len(t, tools, 4)
	assert.Equal(t, "io.github.a.plugin", tools[0].PluginID)
	assert.Equal(t, "list-zones", tools[0].Tool.Name)
	assert.Equal(t, "purge_cache", tools[1].Tool.Name)

	// The lowest plugin id owns a code several plugins declare.
	owner, ok := m.OwnerOf(protocol.CapabilityNotify, "mychat")
	assert.True(t, ok)
	assert.Equal(t, "io.github.a.plugin", owner)
	owner, ok = m.OwnerOf(protocol.CapabilityProbe, "tcp-banner")
	assert.True(t, ok)
	assert.Equal(t, "io.github.a.plugin", owner)
	_, ok = m.OwnerOf(protocol.CapabilityNotify, "unknown")
	assert.False(t, ok)
	_, ok = m.OwnerOf(protocol.CapabilityMCP, "purge_cache")
	assert.False(t, ok, "mcp tools are addressed by their published name, not by owner")

	backends := m.StorageBackends()
	require.Len(t, backends, 2)
	assert.Equal(t, "io.github.a.plugin", backends[0].PluginID)
	assert.Equal(t, "webdav", backends[0].Backend.Code)
	owner, ok = m.OwnerOf(protocol.CapabilityStorage, "webdav")
	assert.True(t, ok)
	assert.Equal(t, "io.github.a.plugin", owner)

	targets := m.DeployTargets()
	require.Len(t, targets, 2)
	assert.Equal(t, "mycdn", targets[0].Target.Code)
	owner, ok = m.OwnerOf(protocol.CapabilityCertDeploy, "mycdn")
	assert.True(t, ok)
	assert.Equal(t, "io.github.a.plugin", owner)

	sources := m.BlocklistSources()
	require.Len(t, sources, 2)
	assert.Equal(t, "io.github.a.plugin", sources[0].PluginID)
	assert.Equal(t, "threatfeed", sources[0].Source.Code)
	assert.Equal(t, 900, sources[0].Source.RefreshSeconds)
	owner, ok = m.OwnerOf(protocol.CapabilitySecurityBlocklist, "threatfeed")
	assert.True(t, ok)
	assert.Equal(t, "io.github.a.plugin", owner)

	providers := m.DiscoveryProviders()
	require.Len(t, providers, 2)
	assert.Equal(t, "registry", providers[1].Provider.Code)
	assert.Equal(t, "io.github.b.plugin", providers[1].PluginID)
	owner, ok = m.OwnerOf(protocol.CapabilityUpstreamDiscovery, "registry")
	assert.True(t, ok)
	assert.Equal(t, "io.github.a.plugin", owner)
	_, ok = m.OwnerOf(protocol.CapabilityUpstreamDiscovery, "unknown")
	assert.False(t, ok)
}

func TestManagerDeployTargetsNeedThePermission(t *testing.T) {
	m := newManager(t.TempDir())

	// A manifest that lost the permission (hand edited on disk) offers no
	// target kind and owns no code, even though it was approved as it is.
	manifest := capabilityPluginManifest("io.github.nodeploy.plugin")
	manifest.Permissions = []string{protocol.PermissionNetwork, protocol.PermissionMCP}
	addCapabilityEntry(m, manifest, true, true)
	assert.Empty(t, m.DeployTargets())
	_, ok := m.OwnerOf(protocol.CapabilityCertDeploy, "mycdn")
	assert.False(t, ok)

	// An approval that no longer matches the manifest serves nothing.
	addCapabilityEntry(m, capabilityPluginManifest("io.github.stale.plugin"), true, false)
	assert.Empty(t, m.DeployTargets())
	backends := m.StorageBackends()
	require.Len(t, backends, 1)
	assert.Equal(t, "io.github.nodeploy.plugin", backends[0].PluginID)
}

func TestManagerMCPToolsNeedTheCapabilityAndThePermission(t *testing.T) {
	m := newManager(t.TempDir())

	// A manifest that lost the permission (hand edited on disk) publishes
	// nothing even though it was approved as it is.
	manifest := capabilityPluginManifest("io.github.nomcp.plugin")
	manifest.Permissions = []string{protocol.PermissionNetwork}
	addCapabilityEntry(m, manifest, true, true)
	assert.Empty(t, m.MCPTools())

	withoutCapability := capabilityPluginManifest("io.github.nocap.plugin")
	withoutCapability.Capabilities = []string{protocol.CapabilityNotify}
	addCapabilityEntry(m, withoutCapability, true, true)
	assert.Empty(t, m.MCPTools())
}

func TestOwnerOfPrefersTheOfficialDNS01Plugin(t *testing.T) {
	m := newManager(t.TempDir())
	official := pluginManifest(OfficialDNS01PluginID)
	other := pluginManifest("aaa.dns")
	addCapabilityEntry(m, official, true, true)
	addCapabilityEntry(m, other, true, true)

	owner, ok := m.OwnerOf(protocol.CapabilityDNS01, "test")
	assert.True(t, ok)
	assert.Equal(t, OfficialDNS01PluginID, owner)
}

func TestMCPToolName(t *testing.T) {
	name := MCPToolName("io.github.example-org.cdn", "purge_cache")
	assert.Equal(t, "io_github_example-org_cdn__purge_cache", name)

	pluginID, tool, ok := ParseMCPToolName(name)
	assert.True(t, ok)
	assert.Equal(t, "io.github.example-org.cdn", pluginID)
	assert.Equal(t, "purge_cache", tool)

	// A tool name may itself contain a double underscore.
	pluginID, tool, ok = ParseMCPToolName(MCPToolName("com.example.x", "a__b"))
	assert.True(t, ok)
	assert.Equal(t, "com.example.x", pluginID)
	assert.Equal(t, "a__b", tool)

	for _, bad := range []string{"reload_nginx", "__tool", "com_example__", "Com_Example__tool"} {
		_, _, ok = ParseMCPToolName(bad)
		assert.False(t, ok, bad)
	}
}

func TestInfoListsPermissionsBeyondTheEarlierApproval(t *testing.T) {
	m := newManager(t.TempDir())
	manifest := capabilityPluginManifest("io.github.update.plugin")
	manifest.Permissions = []string{"kv", "network"}
	row := &model.Plugin{PluginID: manifest.ID, ApprovedPermissions: []string{"kv"}}
	m.entries[manifest.ID] = m.newEntry(manifest.ID, manifest, row)
	assert.Equal(t, []string{"network"}, m.infoOf(m.entries[manifest.ID]).NewPermissions)

	// A plugin never approved has nothing to compare with.
	row.ApprovedPermissions = nil
	assert.Empty(t, m.infoOf(m.entries[manifest.ID]).NewPermissions)
}
