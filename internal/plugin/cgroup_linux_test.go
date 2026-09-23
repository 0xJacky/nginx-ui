//go:build linux

package plugin

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSupervisorStartsThePluginInItsCgroup(t *testing.T) {
	root := fakeCgroupRoot(t, "cpu memory pids")
	supervisor := newTestSupervisor(t, pluginModeNormal, func(cfg *SupervisorConfig) {
		cfg.Resources = ResourceLimits{MemoryMB: 128, CPUPercent: 50}
		cfg.CgroupRoot = root
	})

	require.NoError(t, supervisor.Start(context.Background()))
	status := supervisor.Resources()
	assert.Equal(t, ResourceStatus{MemoryLimitMB: 128, CPUPercent: 50, Enforced: true}, status)

	dir := cgroupDir(root, "official.test")
	assert.Equal(t, "134217728", readCgroupFile(t, filepath.Join(dir, "memory.max")))
	assert.Equal(t, "50000 100000", readCgroupFile(t, filepath.Join(dir, "cpu.max")))
	// A directory is no cgroup, so clone3 refuses it and the process is
	// moved in through cgroup.procs instead.
	supervisor.mu.Lock()
	pid := supervisor.proc.cmd.Process.Pid
	supervisor.mu.Unlock()
	assert.Equal(t, itoa(pid), readCgroupFile(t, filepath.Join(dir, "cgroup.procs")))

	require.NoError(t, supervisor.Stop(context.Background()))
	_, err := os.Stat(dir)
	assert.True(t, os.IsNotExist(err), "the group is removed once the process exited")
	assert.False(t, supervisor.Resources().Enforced)
}

func TestSupervisorRunsUnconfinedWhenTheHierarchyIsNotWritable(t *testing.T) {
	root := fakeCgroupRoot(t, "cpu memory")
	require.NoError(t, os.WriteFile(filepath.Join(root, "nginx-ui"), nil, 0o444))
	cgroupWarned.Store(false)
	t.Cleanup(func() { cgroupWarned.Store(false) })

	supervisor := newTestSupervisor(t, pluginModeNormal, func(cfg *SupervisorConfig) {
		cfg.Resources = ResourceLimits{MemoryMB: 64}
		cfg.CgroupRoot = root
	})
	require.NoError(t, supervisor.Start(context.Background()))
	assert.Equal(t, StateRunning, supervisor.State())
	assert.Equal(t, ResourceStatus{MemoryLimitMB: 64}, supervisor.Resources())
	assert.True(t, cgroupWarned.Load(), "the failure is logged once at warning level")
}

func TestSupervisorWithoutCgroupV2RunsUnconfined(t *testing.T) {
	supervisor := newTestSupervisor(t, pluginModeNormal, func(cfg *SupervisorConfig) {
		cfg.Resources = ResourceLimits{CPUPercent: 10}
		cfg.CgroupRoot = t.TempDir()
	})
	require.NoError(t, supervisor.Start(context.Background()))
	assert.Equal(t, ResourceStatus{CPUPercent: 10}, supervisor.Resources())
}
