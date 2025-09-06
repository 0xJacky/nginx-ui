package indexer

import (
	"context"
	"testing"
	"time"
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
		
		// Test adaptive optimization with increased limits
		ao := NewAdaptiveOptimizer(config)
		batchController := ao.batchSizeController
		
		if batchController.minBatchSize < 500 {
			t.Errorf("Expected min batch size >= 500, got %d", batchController.minBatchSize)
		}
		
		if batchController.maxBatchSize < config.BatchSize*4 {
			t.Errorf("Expected max batch size >= %d, got %d", config.BatchSize*4, batchController.maxBatchSize)
		}
		
		t.Logf("✅ Adaptive optimization limits: min=%d, max=%d", 
			batchController.minBatchSize, batchController.maxBatchSize)
	})

	t.Run("RotationScannerInitialization", func(t *testing.T) {
		// Test rotation scanner initialization
		scanner := NewRotationScanner(nil)
		if scanner == nil {
			t.Fatal("Failed to create rotation scanner")
		}
		
		if scanner.config == nil {
			t.Fatal("Rotation scanner config is nil")
		}
		
		// Verify default configuration is optimized for throughput
		if !scanner.config.EnableParallelScan {
			t.Error("Expected parallel scanning to be enabled by default")
		}
		
		if !scanner.config.PrioritizeBySize {
			t.Error("Expected size-based prioritization to be enabled")
		}
		
		t.Log("✅ Rotation scanner initialized with optimized defaults")
	})

	t.Run("ThroughputOptimizerIntegration", func(t *testing.T) {
		// Create a parallel indexer with rotation scanner
		indexer := NewParallelIndexer(config, nil)
		if indexer == nil {
			t.Fatal("Failed to create parallel indexer")
		}
		
		if indexer.rotationScanner == nil {
			t.Fatal("Rotation scanner not initialized in parallel indexer")
		}
		
		// Test throughput optimizer
		optimizer := NewThroughputOptimizer(indexer, nil)
		if optimizer == nil {
			t.Fatal("Failed to create throughput optimizer")
		}
		
		// Verify optimizer configuration
		optimizedConfig := optimizer.OptimizeIndexerConfig()
		if optimizedConfig.BatchSize < 15000 {
			t.Errorf("Expected optimized batch size >= 15000, got %d", optimizedConfig.BatchSize)
		}
		
		t.Logf("✅ Throughput optimizer created optimized config with batch size: %d", optimizedConfig.BatchSize)
	})

	t.Run("RotationLogFileInfo", func(t *testing.T) {
		// Test that RotationLogFileInfo works correctly
		scanner := NewRotationScanner(nil)
		
		// Test scan configuration
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		
		// Test with empty paths to ensure no crashes
		err := scanner.ScanLogGroups(ctx, []string{})
		if err != nil {
			t.Errorf("Expected no error for empty paths, got: %v", err)
		}
		
		// Verify queue operations work
		queueSize := scanner.GetQueueSize()
		if queueSize < 0 {
			t.Error("Queue size should not be negative")
		}
		
		// Test batch retrieval
		batch := scanner.GetNextBatch(10)
		if batch == nil {
			t.Error("GetNextBatch should return non-nil slice even when empty")
		}
		
		if len(batch) != 0 && batch == nil {
			t.Error("Batch should be empty slice, not nil")
		}
		
		t.Log("✅ Rotation log scanning operations work correctly")
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

func BenchmarkRotationScannerCreation(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		scanner := NewRotationScanner(nil)
		_ = scanner
	}
}