//go:build !linux

package plugin

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSupervisorLimitsAreANoOpOffLinux(t *testing.T) {
	root := fakeCgroupRoot(t, "cpu memory")
	supervisor := newTestSupervisor(t, pluginModeNormal, func(cfg *SupervisorConfig) {
		cfg.Resources = ResourceLimits{MemoryMB: 128, CPUPercent: 50}
		cfg.CgroupRoot = root
	})

	require.NoError(t, supervisor.Start(context.Background()))
	assert.Equal(t, StateRunning, supervisor.State())
	assert.Equal(t, ResourceStatus{MemoryLimitMB: 128, CPUPercent: 50}, supervisor.Resources(), "reported but not enforced")
	_, err := os.Stat(filepath.Join(root, "nginx-ui"))
	assert.True(t, os.IsNotExist(err), "no group is created")
	assert.True(t, cgroupNoticed.Load(), "the no-op is noted once")
}
