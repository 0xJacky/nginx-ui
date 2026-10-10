package plugin

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0xJacky/Nginx-UI/internal/plugin/protocol"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeCgroupRoot is a directory standing in for a cgroup v2 mount.
func fakeCgroupRoot(t *testing.T, controllers string) string {
	t.Helper()
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "cgroup.controllers"), []byte(controllers+"\n"), 0o644))
	return root
}

// readCgroupFile returns the file without the trailing newline the kernel
// appends, so the same assertions hold against a fake root and a real one.
func readCgroupFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	return strings.TrimSpace(string(data))
}

func TestEffectiveResourcesLetTheSmallerLimitWin(t *testing.T) {
	withHints := func(memory, cpu int) *protocol.Manifest {
		return &protocol.Manifest{Server: &protocol.ManifestServer{
			Command:   []string{"plugin"},
			Resources: &protocol.ManifestResources{MemoryMB: memory, CPUPercent: cpu},
		}}
	}
	tests := []struct {
		name     string
		host     ResourceLimits
		manifest *protocol.Manifest
		want     ResourceLimits
	}{
		{"no limits at all", ResourceLimits{}, withHints(0, 0), ResourceLimits{}},
		{"host limits only", ResourceLimits{MemoryMB: 512, CPUPercent: 100}, nil, ResourceLimits{MemoryMB: 512, CPUPercent: 100}},
		{"smaller hints win", ResourceLimits{MemoryMB: 512, CPUPercent: 100}, withHints(128, 50), ResourceLimits{MemoryMB: 128, CPUPercent: 50}},
		{"hints never raise", ResourceLimits{MemoryMB: 512, CPUPercent: 100}, withHints(4096, 400), ResourceLimits{MemoryMB: 512, CPUPercent: 100}},
		{"hints alone lower unlimited", ResourceLimits{}, withHints(256, 25), ResourceLimits{MemoryMB: 256, CPUPercent: 25}},
		{"mixed", ResourceLimits{MemoryMB: 1024}, withHints(0, 50), ResourceLimits{MemoryMB: 1024, CPUPercent: 50}},
		{"negative values are unlimited", ResourceLimits{MemoryMB: -1}, withHints(-5, -1), ResourceLimits{}},
		{"no server block", ResourceLimits{MemoryMB: 64}, &protocol.Manifest{}, ResourceLimits{MemoryMB: 64}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, EffectiveResources(tc.host, tc.manifest))
		})
	}
}

func TestCgroupNameIsOneSafeSegment(t *testing.T) {
	assert.Equal(t, "com.example.logs", cgroupName("com.example.logs"))
	assert.Equal(t, "com.example_x_y", cgroupName("com.example/x y"))
	assert.Equal(t, "_..", cgroupName(".."))
	assert.Equal(t, "_", cgroupName(""))
	assert.Equal(t, "._", cgroupName("./"))
}

func TestCreateCgroupWritesTheLimits(t *testing.T) {
	root := fakeCgroupRoot(t, "cpuset cpu io memory pids")
	// The root already passes cpu down, only memory is added there.
	require.NoError(t, os.WriteFile(filepath.Join(root, "cgroup.subtree_control"), []byte("cpu\n"), 0o644))

	cg, err := createCgroup(root, "com.example.logs", ResourceLimits{MemoryMB: 256, CPUPercent: 250})
	require.NoError(t, err)
	require.NotNil(t, cg)

	dir := filepath.Join(root, "nginx-ui", "plugins", "com.example.logs")
	assert.Equal(t, dir, cg.dir)
	assert.Equal(t, "268435456", readCgroupFile(t, filepath.Join(dir, "memory.max")))
	assert.Equal(t, "0", readCgroupFile(t, filepath.Join(dir, "memory.swap.max")))
	assert.Equal(t, "250000 100000", readCgroupFile(t, filepath.Join(dir, "cpu.max")))

	assert.Equal(t, "+memory", readCgroupFile(t, filepath.Join(root, "cgroup.subtree_control")))
	assert.Equal(t, "+memory +cpu", readCgroupFile(t, filepath.Join(root, "nginx-ui", "cgroup.subtree_control")))
	assert.Equal(t, "+memory +cpu", readCgroupFile(t, filepath.Join(root, "nginx-ui", "plugins", "cgroup.subtree_control")))

	require.NoError(t, cg.adopt(4242))
	assert.Equal(t, "4242", readCgroupFile(t, filepath.Join(dir, "cgroup.procs")))

	require.NoError(t, cg.remove())
	_, err = os.Stat(dir)
	assert.True(t, os.IsNotExist(err), "the group is removed after the process")
	_, err = os.Stat(filepath.Join(root, "nginx-ui", "plugins"))
	assert.NoError(t, err, "the parents stay for the other plugins")
	assert.NoError(t, cg.remove(), "removing twice is fine")
}

func TestCreateCgroupWritesOnlyTheLimitsThatApply(t *testing.T) {
	root := fakeCgroupRoot(t, "cpu io pids")
	cg, err := createCgroup(root, "com.example.cpu", ResourceLimits{CPUPercent: 50})
	require.NoError(t, err)
	assert.Equal(t, "50000 100000", readCgroupFile(t, filepath.Join(cg.dir, "cpu.max")))
	_, err = os.Stat(filepath.Join(cg.dir, "memory.max"))
	assert.True(t, os.IsNotExist(err))
	assert.Equal(t, "+cpu", readCgroupFile(t, filepath.Join(root, "nginx-ui", "cgroup.subtree_control")))

	// A group left behind by a crash is replaced.
	require.NoError(t, os.WriteFile(filepath.Join(cg.dir, "stale"), []byte("x"), 0o644))
	cg, err = createCgroup(root, "com.example.cpu", ResourceLimits{CPUPercent: 25})
	require.NoError(t, err)
	_, err = os.Stat(filepath.Join(cg.dir, "stale"))
	assert.True(t, os.IsNotExist(err))
	assert.Equal(t, "25000 100000", readCgroupFile(t, filepath.Join(cg.dir, "cpu.max")))
}

func TestCreateCgroupRefusesWhatItCannotEnforce(t *testing.T) {
	t.Run("no limits", func(t *testing.T) {
		root := fakeCgroupRoot(t, "cpu memory")
		cg, err := createCgroup(root, "com.example.none", ResourceLimits{})
		require.NoError(t, err)
		assert.Nil(t, cg)
		_, err = os.Stat(filepath.Join(root, "nginx-ui"))
		assert.True(t, os.IsNotExist(err), "no group without a limit")
	})
	t.Run("no cgroup v2", func(t *testing.T) {
		_, err := createCgroup(t.TempDir(), "com.example.v1", ResourceLimits{MemoryMB: 64})
		assert.ErrorIs(t, err, errCgroupV2Missing)
	})
	t.Run("controller missing", func(t *testing.T) {
		root := fakeCgroupRoot(t, "cpu pids")
		_, err := createCgroup(root, "com.example.mem", ResourceLimits{MemoryMB: 64})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "memory controller")
	})
	t.Run("unwritable hierarchy", func(t *testing.T) {
		root := fakeCgroupRoot(t, "cpu memory")
		// Where the base group should be there is a file nobody can turn
		// into a directory, which fails for root as well.
		require.NoError(t, os.WriteFile(filepath.Join(root, "nginx-ui"), nil, 0o444))
		cg, err := createCgroup(root, "com.example.ro", ResourceLimits{MemoryMB: 64})
		require.Error(t, err)
		assert.Nil(t, cg)
	})
}

func TestResourceStatusReportsTheLimits(t *testing.T) {
	assert.Equal(t, ResourceStatus{MemoryLimitMB: 128, CPUPercent: 50, Enforced: true},
		resourceStatus(ResourceLimits{MemoryMB: 128, CPUPercent: 50}, true))
	assert.Equal(t, ResourceStatus{}, resourceStatus(ResourceLimits{MemoryMB: -1}, false))
}

func TestManagerReportsTheResourceLimits(t *testing.T) {
	original := *settings.PluginSettings
	t.Cleanup(func() { *settings.PluginSettings = original })
	settings.PluginSettings.MemoryLimitMB = 128
	settings.PluginSettings.CPUPercent = 0
	settings.PluginSettings.CgroupRoot = t.TempDir()

	m := newTestManager(t)
	hinted := pluginManifest("official.hinted")
	hinted.Server.Resources = &protocol.ManifestResources{MemoryMB: 64, CPUPercent: 30}
	writePluginDir(t, filepath.Join(m.Dir(), "official.hinted"), hinted)
	writePluginDir(t, filepath.Join(m.Dir(), "official.plain"), pluginManifest("official.plain"))
	content := pluginManifest("official.content")
	content.Server = nil
	content.Capabilities = nil
	content.DNS01 = nil
	content.Content = &protocol.ManifestContent{Templates: "templates"}
	writePluginDir(t, filepath.Join(m.Dir(), "official.content"), content)
	require.NoError(t, m.LoadOffline(context.Background()))

	info, err := m.Get("official.hinted")
	require.NoError(t, err)
	require.NotNil(t, info.Resources)
	assert.Equal(t, ResourceStatus{MemoryLimitMB: 64, CPUPercent: 30}, *info.Resources, "the smaller hint wins, nothing runs")

	info, err = m.Get("official.plain")
	require.NoError(t, err)
	require.NotNil(t, info.Resources)
	assert.Equal(t, ResourceStatus{MemoryLimitMB: 128}, *info.Resources)

	info, err = m.Get("official.content")
	require.NoError(t, err)
	assert.Nil(t, info.Resources, "a plugin without a process has no limits")
}
