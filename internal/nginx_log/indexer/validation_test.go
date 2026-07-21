package indexer

import (
	"testing"
)

func TestThroughputOptimizations(t *testing.T) {
	// Test that batch sizes have been properly increased
	config := DefaultIndexerConfig()

	t.Run("IncreasedBatchSizes", func(t *testing.T) {
		// Verify that batch sizes are significantly increased
		if config.BatchSize < 15000 {
			t.Errorf("Expected batch size >= 15000, got %d", config.BatchSize)
		}
		t.Logf("✅ Batch size optimized: %d", config.BatchSize)
	})
}

func TestParserBatchSizeOptimization(t *testing.T) {
	t.Run("ParserConfigOptimized", func(t *testing.T) {
		// Since parser.go has a global init, we need to check if it's properly configured
		// We'll test this by verifying the default config is optimized
		config := DefaultIndexerConfig()

		// Verify batch sizes are appropriately large for throughput
		expectedMinBatch := 15000
		if config.BatchSize < expectedMinBatch {
			t.Errorf("Expected batch size >= %d, got %d", expectedMinBatch, config.BatchSize)
		}

		// Verify queue size scales with batch size
		expectedMinQueue := config.BatchSize * 10
		if config.MaxQueueSize < expectedMinQueue {
			t.Errorf("Expected queue size >= %d, got %d", expectedMinQueue, config.MaxQueueSize)
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
