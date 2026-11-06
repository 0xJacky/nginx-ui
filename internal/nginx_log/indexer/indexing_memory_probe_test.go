package indexer

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestIndexingMemoryProbe measures the heap that indexing a real access log
// takes, which is what indexingAmplification has to track. It is opt-in:
//
//	INDEXER_MEMORY_PROBE_LOG=/path/to/access.log \
//	  GOWORK=off go test -tags=unembed -run TestIndexingMemoryProbe -v ./internal/nginx_log/indexer/
//
// The indexer is sized as it would be on a 1 CPU / 1 GiB host.
// INDEXER_MEMORY_PROBE_BATCH overrides the batch size, and
// INDEXER_MEMORY_PROBE_QUOTA (bytes) the memory quota; a quota of 0 disables
// the memory bound, which isolates the cost of one batch of a given size.
func TestIndexingMemoryProbe(t *testing.T) {
	source := os.Getenv("INDEXER_MEMORY_PROBE_LOG")
	if source == "" {
		t.Skip("INDEXER_MEMORY_PROBE_LOG is not set")
	}

	originalCPUs, originalMemory := availableCPUs, availableMemory
	availableCPUs = func() int { return 1 }
	availableMemory = func() (int64, bool) { return 1 << 30, true }
	t.Cleanup(func() { availableCPUs, availableMemory = originalCPUs, originalMemory })

	logDir := t.TempDir()
	allowLogDir(t, logDir)
	data, err := os.ReadFile(source)
	require.NoError(t, err)
	logPath := filepath.Join(logDir, "access.log")
	require.NoError(t, os.WriteFile(logPath, data, 0o600))

	config := DefaultIndexerConfig()
	config.IndexPath = t.TempDir()
	if value := os.Getenv("INDEXER_MEMORY_PROBE_BATCH"); value != "" {
		config.BatchSize, err = strconv.Atoi(value)
		require.NoError(t, err)
	}
	if value := os.Getenv("INDEXER_MEMORY_PROBE_QUOTA"); value != "" {
		config.MemoryQuota, err = strconv.ParseInt(value, 10, 64)
		require.NoError(t, err)
	}

	pi := NewParallelIndexer(config, NewGroupedShardManager(config))
	require.NoError(t, pi.Start(context.Background()))
	t.Cleanup(func() { _ = pi.Stop() })

	runtime.GC()
	var baseline runtime.MemStats
	runtime.ReadMemStats(&baseline)

	var peakHeap uint64
	stop := make(chan struct{})
	var sampler sync.WaitGroup
	sampler.Go(func() {
		var stats runtime.MemStats
		ticker := time.NewTicker(10 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
			}
			runtime.ReadMemStats(&stats)
			peakHeap = max(peakHeap, stats.HeapInuse)
		}
	})

	started := time.Now()
	docs, _, _, err := pi.IndexSingleFileWithProgress(logPath, nil)
	elapsed := time.Since(started)
	close(stop)
	sampler.Wait()
	require.NoError(t, err)

	t.Logf("batch=%d quota=%dMiB batchBudget=%dMiB docs=%d elapsed=%s",
		config.BatchSize, config.MemoryQuota>>20, pi.batchMemoryBudget()>>20, docs, elapsed.Round(time.Millisecond))
	t.Logf("heap in use: baseline=%dMiB peak=%dMiB", baseline.HeapInuse>>20, peakHeap>>20)
}
