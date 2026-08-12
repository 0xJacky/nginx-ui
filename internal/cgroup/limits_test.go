package cgroup

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// useFixtureRoot points the cgroup readers at a temporary directory for the
// duration of a test.
func useFixtureRoot(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	previous := cgroupRoot
	cgroupRoot = root
	t.Cleanup(func() { cgroupRoot = previous })

	return root
}

func writeFixture(t *testing.T, path, content string) {
	t.Helper()

	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}

func TestCPUQuotaCgroupV2(t *testing.T) {
	root := useFixtureRoot(t)
	writeFixture(t, filepath.Join(root, "cpu.max"), "150000 100000\n")

	quota, ok := CPUQuota()
	require.True(t, ok)
	assert.InDelta(t, 1.5, quota, 0.0001)
}

func TestCPUQuotaCgroupV2Unlimited(t *testing.T) {
	root := useFixtureRoot(t)
	writeFixture(t, filepath.Join(root, "cpu.max"), "max 100000\n")

	_, ok := CPUQuota()
	assert.False(t, ok)
}

func TestCPUQuotaCgroupV1(t *testing.T) {
	root := useFixtureRoot(t)
	writeFixture(t, filepath.Join(root, "cpu", "cpu.cfs_quota_us"), "200000\n")
	writeFixture(t, filepath.Join(root, "cpu", "cpu.cfs_period_us"), "100000\n")

	quota, ok := CPUQuota()
	require.True(t, ok)
	assert.InDelta(t, 2.0, quota, 0.0001)
}

func TestCPUQuotaCgroupV1Unlimited(t *testing.T) {
	root := useFixtureRoot(t)
	// -1 is the kernel's "no bandwidth limit" sentinel.
	writeFixture(t, filepath.Join(root, "cpu", "cpu.cfs_quota_us"), "-1\n")
	writeFixture(t, filepath.Join(root, "cpu", "cpu.cfs_period_us"), "100000\n")

	_, ok := CPUQuota()
	assert.False(t, ok)
}

func TestCPUQuotaWithoutCgroupFilesystem(t *testing.T) {
	useFixtureRoot(t)

	_, ok := CPUQuota()
	assert.False(t, ok)
}

// TestAvailableCPUsClampedByQuota is the core regression guard for issue #1792:
// in an LXC container the affinity mask reports every host CPU, so worker pools
// sized from GOMAXPROCS oversubscribe the container by an order of magnitude.
func TestAvailableCPUsClampedByQuota(t *testing.T) {
	root := useFixtureRoot(t)
	// One core, the typical Proxmox LXC allocation, on a host with many more.
	writeFixture(t, filepath.Join(root, "cpu.max"), "100000 100000\n")

	assert.Equal(t, 1, AvailableCPUs())
}

func TestAvailableCPUsRoundsFractionalQuotaUp(t *testing.T) {
	root := useFixtureRoot(t)
	// 0.5 cores must still allow one worker, never zero.
	writeFixture(t, filepath.Join(root, "cpu.max"), "50000 100000\n")

	assert.Equal(t, 1, AvailableCPUs())
}

func TestAvailableCPUsFallsBackToGOMAXPROCSWithoutQuota(t *testing.T) {
	useFixtureRoot(t)

	assert.Equal(t, runtime.GOMAXPROCS(0), AvailableCPUs())
}

func TestAvailableCPUsNeverExceedsGOMAXPROCS(t *testing.T) {
	root := useFixtureRoot(t)
	// A quota far above the machine's capacity must not inflate the pool size.
	writeFixture(t, filepath.Join(root, "cpu.max"), "102400000 100000\n")

	assert.Equal(t, runtime.GOMAXPROCS(0), AvailableCPUs())
}

func TestMemoryLimitCgroupV2(t *testing.T) {
	root := useFixtureRoot(t)
	writeFixture(t, filepath.Join(root, "memory.max"), "536870912\n")

	limit, ok := MemoryLimit()
	require.True(t, ok)
	assert.Equal(t, int64(536870912), limit)
}

func TestMemoryLimitCgroupV2Unlimited(t *testing.T) {
	root := useFixtureRoot(t)
	writeFixture(t, filepath.Join(root, "memory.max"), "max\n")

	_, ok := MemoryLimit()
	assert.False(t, ok)
}

func TestMemoryLimitCgroupV1(t *testing.T) {
	root := useFixtureRoot(t)
	writeFixture(t, filepath.Join(root, "memory", "memory.limit_in_bytes"), "268435456\n")

	limit, ok := MemoryLimit()
	require.True(t, ok)
	assert.Equal(t, int64(268435456), limit)
}

func TestMemoryLimitIgnoresSentinelValue(t *testing.T) {
	root := useFixtureRoot(t)
	// The classic cgroup v1 "unlimited" sentinel.
	writeFixture(t, filepath.Join(root, "memory", "memory.limit_in_bytes"), "9223372036854771712\n")

	_, ok := MemoryLimit()
	assert.False(t, ok)
}

// stubTotalMemory replaces the host memory probe for the duration of a test.
func stubTotalMemory(t *testing.T, total uint64, err error) {
	t.Helper()

	previous := totalMemory
	totalMemory = func() (uint64, error) { return total, err }
	t.Cleanup(func() { totalMemory = previous })
}

func TestAvailableMemoryPrefersCgroupLimit(t *testing.T) {
	root := useFixtureRoot(t)
	writeFixture(t, filepath.Join(root, "memory.max"), "536870912\n") // 512MB container
	stubTotalMemory(t, 128<<30, nil)                                  // 128GB host

	available, ok := AvailableMemory()
	require.True(t, ok)
	assert.Equal(t, int64(536870912), available)
}

func TestAvailableMemoryFallsBackToHostTotal(t *testing.T) {
	useFixtureRoot(t)
	stubTotalMemory(t, 2<<30, nil)

	available, ok := AvailableMemory()
	require.True(t, ok)
	assert.Equal(t, int64(2<<30), available)
}

func TestAvailableMemoryIgnoresLimitAboveHostTotal(t *testing.T) {
	root := useFixtureRoot(t)
	writeFixture(t, filepath.Join(root, "memory.max"), "137438953472\n") // 128GB
	stubTotalMemory(t, 1<<30, nil)                                       // 1GB host

	available, ok := AvailableMemory()
	require.True(t, ok)
	assert.Equal(t, int64(1<<30), available)
}

func TestAvailableMemoryUnknown(t *testing.T) {
	useFixtureRoot(t)
	stubTotalMemory(t, 0, assert.AnError)

	_, ok := AvailableMemory()
	assert.False(t, ok)
}
