package indexer

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubResourceBudget simulates a container that is allowed the given number of
// CPUs and bytes of memory, regardless of what the host actually has.
func stubResourceBudget(t *testing.T, cpus int, memoryBytes int64, memoryKnown bool) {
	t.Helper()

	previousCPUs := availableCPUs
	previousMemory := availableMemory
	availableCPUs = func() int { return cpus }
	availableMemory = func() (int64, bool) { return memoryBytes, memoryKnown }
	t.Cleanup(func() {
		availableCPUs = previousCPUs
		availableMemory = previousMemory
	})
}

// TestDefaultIndexerConfigRespectsContainerCPUBudget is the regression guard for
// issue #1792. A Proxmox LXC container gets one core while the affinity mask
// advertises the whole host, and sizing the pools from the host number is what
// turned the post-upgrade index rebuild into an OOM.
func TestDefaultIndexerConfigRespectsContainerCPUBudget(t *testing.T) {
	stubResourceBudget(t, 1, 512*1024*1024, true)

	config := DefaultIndexerConfig()

	assert.Equal(t, 2, config.WorkerCount, "worker count must not exceed the small-container floor")
	assert.Equal(t, 1, config.FileGroupConcurrency, "a single-core container must index one file at a time")
	assert.Equal(t, 15000, config.BatchSize, "batch size must use the smallest tier on a single core")
	assert.Equal(t, 1, config.ShardCount)
}

func TestDefaultIndexerConfigCapsLargeHosts(t *testing.T) {
	stubResourceBudget(t, 64, 256*1024*1024*1024, true)

	config := DefaultIndexerConfig()

	assert.LessOrEqual(t, config.WorkerCount, maxDefaultWorkerCount)
	assert.LessOrEqual(t, config.FileGroupConcurrency, maxDefaultFileGroupConcurrency)
	assert.LessOrEqual(t, config.MemoryQuota, maxIndexMemoryQuota)
}

func TestDefaultIndexerConfigScalesBetweenExtremes(t *testing.T) {
	stubResourceBudget(t, 4, 8*1024*1024*1024, true)

	config := DefaultIndexerConfig()

	assert.Equal(t, 4, config.WorkerCount)
	assert.Equal(t, 2, config.FileGroupConcurrency)
	assert.Equal(t, 18000, config.BatchSize)
	assert.Equal(t, max(4, config.WorkerCount*2), config.MaxQueueSize)
}

// TestDefaultMemoryQuotaFollowsContainerLimit checks that the indexer
// backpressure valve is sized against the container, not the host. Before the
// fix the quota was a hardcoded 1GB, which never engaged inside a 512MB LXC.
func TestDefaultMemoryQuotaFollowsContainerLimit(t *testing.T) {
	tests := []struct {
		name      string
		available int64
		known     bool
		expected  int64
	}{
		{
			name:      "512MB container gets a quarter of its budget",
			available: 512 * 1024 * 1024,
			known:     true,
			expected:  128 * 1024 * 1024,
		},
		{
			name:      "tiny container is floored, not zeroed",
			available: 64 * 1024 * 1024,
			known:     true,
			expected:  minIndexMemoryQuota,
		},
		{
			name:      "large host is capped at the historical budget",
			available: 128 * 1024 * 1024 * 1024,
			known:     true,
			expected:  maxIndexMemoryQuota,
		},
		{
			name:      "unknown budget keeps the historical default",
			available: 0,
			known:     false,
			expected:  maxIndexMemoryQuota,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			stubResourceBudget(t, 4, testCase.available, testCase.known)

			assert.Equal(t, testCase.expected, DefaultMemoryQuota())
		})
	}
}

// TestGetConfigNeverExceedsContainerMemoryBudget makes sure an aggressive
// scenario profile cannot re-introduce a quota larger than the container.
func TestGetConfigNeverExceedsContainerMemoryBudget(t *testing.T) {
	stubResourceBudget(t, 16, 1024*1024*1024, true)

	config := GetConfig("max_performance")

	require.NotNil(t, config)
	assert.LessOrEqual(t, config.MemoryQuota, DefaultMemoryQuota())
	assert.Equal(t, max(4, config.WorkerCount*2), config.MaxQueueSize)
}
