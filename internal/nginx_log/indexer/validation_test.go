package indexer

import (
	"testing"
)

func TestThroughputOptimizations(t *testing.T) {
	// Test that batch sizes have been properly increased
	config := DefaultIndexerConfig()

	t.Run("BatchSizeFollowsMemoryBudget", func(t *testing.T) {
		// A parsed line costs about a kilobyte until its block is encoded, so
		// the batch is sized from memory, not stretched for throughput: the
		// column store is not batch-bound the way Bleve was.
		if config.BatchSize != ParseBatchSize() {
			t.Errorf("Expected batch size %d from the memory budget, got %d", ParseBatchSize(), config.BatchSize)
		}
		if config.BatchSize < 1000 || config.BatchSize > 5000 {
			t.Errorf("Expected batch size within 1000-5000, got %d", config.BatchSize)
		}
	})
}

func TestParserBatchSizeOptimization(t *testing.T) {
	t.Run("ParserConfigOptimized", func(t *testing.T) {
		// Since parser.go has a global init, we need to check if it's properly configured
		// We'll test this by verifying the default config is optimized
		config := DefaultIndexerConfig()

		// Batch sizes follow the memory budget tiers.
		cases := []struct {
			mem  int64
			ok   bool
			want int
		}{
			{0, false, 2000},
			{512 << 20, true, 1000},
			{2 << 30, true, 2000},
			{16 << 30, true, 5000},
		}
		previous := availableMemory
		t.Cleanup(func() { availableMemory = previous })
		for _, c := range cases {
			availableMemory = func() (int64, bool) { return c.mem, c.ok }
			if got := ParseBatchSize(); got != c.want {
				t.Errorf("memory %d: expected batch size %d, got %d", c.mem, c.want, got)
			}
		}
		availableMemory = previous

		// IndexDocuments is synchronous, so the queue only needs a small
		// multiple of the worker count. Large queues retain entire batches.
		expectedMaxQueue := config.WorkerCount * 4
		if config.MaxQueueSize > expectedMaxQueue {
			t.Errorf("Expected queue size <= %d, got %d", expectedMaxQueue, config.MaxQueueSize)
		}

		t.Logf("✅ Parser configuration optimized: BatchSize=%d, QueueSize=%d",
			config.BatchSize, config.MaxQueueSize)
	})
}

// Benchmark to verify performance characteristics
func BenchmarkBatchSizeCalculation(b *testing.B) {
	config := DefaultIndexerConfig()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = config.BatchSize * 2
	}
}
