package indexer

import (
	"fmt"
	"sync/atomic"
	"testing"
	"time"
)

// BenchmarkDynamicShardScaling tests the performance of dynamic shard scaling operations
func BenchmarkDynamicShardScaling(b *testing.B) {
	configs := []struct {
		name          string
		initialShards int
		targetShards  int
		withData      bool
	}{
		{"ScaleUp_2to4_Empty", 2, 4, false},
		{"ScaleUp_2to4_WithData", 2, 4, true},
		{"ScaleDown_4to2_Empty", 4, 2, false},
		{"ScaleDown_4to2_WithData", 4, 2, true},
		{"ScaleUp_2to8_Empty", 2, 8, false},
		{"ScaleUp_2to8_WithData", 2, 8, true},
	}

	for _, cfg := range configs {
		b.Run(cfg.name, func(b *testing.B) {
			config := DefaultIndexerConfig()
			config.IndexPath = b.TempDir()
			config.ShardCount = cfg.initialShards

			dsm := NewEnhancedDynamicShardManager(config)
			if err := dsm.Initialize(); err != nil {
				b.Fatal(err)
			}
			defer dsm.Close()

			// Add test data if required
			if cfg.withData {
				seedTestData(b, dsm, 100) // 100 documents per shard
			}

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				start := time.Now()
				err := dsm.ScaleShards(cfg.targetShards)
				elapsed := time.Since(start)

				if err != nil {
					b.Fatalf("Scaling failed: %v", err)
				}

				// Log performance metrics
				b.ReportMetric(float64(elapsed.Nanoseconds()), "scaling_ns")
				b.ReportMetric(float64(cfg.targetShards), "target_shards")

				// Scale back for next iteration
				dsm.ScaleShards(cfg.initialShards)
			}
		})
	}
}

// BenchmarkDynamicShardAutoScaling tests the performance of auto-scaling decisions
func BenchmarkDynamicShardAutoScaling(b *testing.B) {
	config := DefaultIndexerConfig()
	config.IndexPath = b.TempDir()
	config.ShardCount = 4

	dsm := NewEnhancedDynamicShardManager(config)
	if err := dsm.Initialize(); err != nil {
		b.Fatal(err)
	}
	defer dsm.Close()

	// Seed with test data to trigger realistic metrics
	seedTestData(b, dsm, 1000)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		start := time.Now()
		err := dsm.AutoScale()
		elapsed := time.Since(start)

		if err != nil {
			b.Fatalf("Auto-scaling failed: %v", err)
		}

		b.ReportMetric(float64(elapsed.Nanoseconds()), "autoscale_decision_ns")
	}
}

// BenchmarkDynamicShardHealthMonitoring tests the performance of shard health checks
func BenchmarkDynamicShardHealthMonitoring(b *testing.B) {
	config := DefaultIndexerConfig()
	config.IndexPath = b.TempDir()
	config.ShardCount = 8

	dsm := NewEnhancedDynamicShardManager(config)
	if err := dsm.Initialize(); err != nil {
		b.Fatal(err)
	}
	defer dsm.Close()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		start := time.Now()
		dsm.updateShardHealth()
		elapsed := time.Since(start)

		b.ReportMetric(float64(elapsed.Nanoseconds()), "health_check_ns")
		b.ReportMetric(float64(dsm.GetShardCount()), "monitored_shards")
	}
}

// BenchmarkAdaptiveWorkerScaling tests the performance of adaptive worker count adjustments
func BenchmarkAdaptiveWorkerScaling(b *testing.B) {
	scenarios := []struct {
		name           string
		initialWorkers int
		cpuUtilization float64
		targetCPU      float64
	}{
		{"LowCPU_ScaleUp", 4, 0.3, 0.75},
		{"HighCPU_ScaleDown", 16, 0.95, 0.75},
		{"OptimalCPU_NoChange", 8, 0.75, 0.75},
		{"VeryLowCPU_AggressiveScaleUp", 2, 0.1, 0.75},
		{"VeryHighCPU_AggressiveScaleDown", 24, 0.99, 0.75},
	}

	for _, scenario := range scenarios {
		b.Run(scenario.name, func(b *testing.B) {
			config := &Config{
				WorkerCount: scenario.initialWorkers,
				BatchSize:   1000,
			}

			ao := NewAdaptiveOptimizer(config)

			var adjustmentCount int32
			var adjustmentTime int64

			ao.SetWorkerCountChangeCallback(func(oldCount, newCount int) {
				atomic.AddInt32(&adjustmentCount, 1)
			})

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				start := time.Now()

				if scenario.cpuUtilization < scenario.targetCPU {
					ao.suggestWorkerIncrease(scenario.cpuUtilization, scenario.targetCPU)
				} else if scenario.cpuUtilization > scenario.targetCPU {
					ao.suggestWorkerDecrease(scenario.cpuUtilization, scenario.targetCPU)
				}

				elapsed := time.Since(start)
				atomic.AddInt64(&adjustmentTime, elapsed.Nanoseconds())
			}

			// Report metrics
			avgTime := float64(atomic.LoadInt64(&adjustmentTime)) / float64(b.N)
			b.ReportMetric(avgTime, "worker_adjustment_ns")
			b.ReportMetric(float64(atomic.LoadInt32(&adjustmentCount)), "total_adjustments")
		})
	}
}

// BenchmarkAdaptiveBatchSizeOptimization tests the performance of batch size optimization
func BenchmarkAdaptiveBatchSizeOptimization(b *testing.B) {
	scenarios := []struct {
		name         string
		throughput   float64
		latency      time.Duration
		initialBatch int
	}{
		{"LowThroughput_IncreaseBatch", 15.0, 1*time.Second, 1000},
		{"HighLatency_DecreaseBatch", 30.0, 8*time.Second, 2000},
		{"OptimalPerformance_NoChange", 25.0, 2*time.Second, 1500},
		{"VeryLowThroughput_AggressiveIncrease", 5.0, 500*time.Millisecond, 500},
		{"VeryHighLatency_AggressiveDecrease", 20.0, 15*time.Second, 3000},
	}

	for _, scenario := range scenarios {
		b.Run(scenario.name, func(b *testing.B) {
			config := &Config{
				WorkerCount: 8,
				BatchSize:   scenario.initialBatch,
			}

			ao := NewAdaptiveOptimizer(config)

			// Seed performance history
			for i := 0; i < 10; i++ {
				sample := PerformanceSample{
					Timestamp:   time.Now().Add(-time.Duration(i) * time.Second),
					Throughput:  scenario.throughput + float64(i%3-1)*2, // Add some variance
					Latency:     scenario.latency,
					CPUUsage:    0.7,
					BatchSize:   scenario.initialBatch,
					WorkerCount: config.WorkerCount,
				}
				ao.performanceHistory.mutex.Lock()
				ao.performanceHistory.samples = append(ao.performanceHistory.samples, sample)
				ao.performanceHistory.mutex.Unlock()
			}

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				start := time.Now()
				ao.optimizeBatchSize()
				elapsed := time.Since(start)

				b.ReportMetric(float64(elapsed.Nanoseconds()), "batch_optimization_ns")
				b.ReportMetric(float64(ao.GetOptimalBatchSize()), "current_batch_size")
			}
		})
	}
}

// BenchmarkAdaptiveOptimizerConcurrency tests concurrent access to adaptive optimization
func BenchmarkAdaptiveOptimizerConcurrency(b *testing.B) {
	config := &Config{
		WorkerCount: 8,
		BatchSize:   1000,
	}

	ao := NewAdaptiveOptimizer(config)

	var adjustmentCount int32
	ao.SetWorkerCountChangeCallback(func(oldCount, newCount int) {
		atomic.AddInt32(&adjustmentCount, 1)
	})

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			start := time.Now()

			// Simulate concurrent operations
			go func() {
				ao.GetOptimalBatchSize()
				ao.GetCPUUtilization()
			}()

			go func() {
				ao.optimizeBatchSize()
			}()

			go func() {
				if time.Now().UnixNano()%2 == 0 {
					ao.suggestWorkerIncrease(0.4, 0.75)
				} else {
					ao.suggestWorkerDecrease(0.9, 0.75)
				}
			}()

			elapsed := time.Since(start)
			b.ReportMetric(float64(elapsed.Nanoseconds()), "concurrent_operation_ns")
		}
	})

	b.ReportMetric(float64(atomic.LoadInt32(&adjustmentCount)), "total_concurrent_adjustments")
}

// BenchmarkEnhancedDynamicShardManager_GetShardHealth tests performance of health status retrieval
func BenchmarkEnhancedDynamicShardManager_GetShardHealth(b *testing.B) {
	shardCounts := []int{2, 4, 8, 16}

	for _, shardCount := range shardCounts {
		b.Run(fmt.Sprintf("Shards_%d", shardCount), func(b *testing.B) {
			config := DefaultIndexerConfig()
			config.IndexPath = b.TempDir()
			config.ShardCount = shardCount

			dsm := NewEnhancedDynamicShardManager(config)
			if err := dsm.Initialize(); err != nil {
				b.Fatal(err)
			}
			defer dsm.Close()

			// Warm up health status
			dsm.updateShardHealth()

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				start := time.Now()
				health := dsm.GetShardHealth()
				elapsed := time.Since(start)

				if len(health) != shardCount {
					b.Fatalf("Expected %d health statuses, got %d", shardCount, len(health))
				}

				b.ReportMetric(float64(elapsed.Nanoseconds()), "get_health_ns")
				b.ReportMetric(float64(len(health)), "health_entries")
			}
		})
	}
}

// BenchmarkLoadMetricsCollection tests the performance of collecting load metrics
func BenchmarkLoadMetricsCollection(b *testing.B) {
	config := DefaultIndexerConfig()
	config.IndexPath = b.TempDir()
	config.ShardCount = 8

	dsm := NewEnhancedDynamicShardManager(config)
	if err := dsm.Initialize(); err != nil {
		b.Fatal(err)
	}
	defer dsm.Close()

	// Seed with data to make metrics collection realistic
	seedTestData(b, dsm, 500)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		start := time.Now()
		metrics := dsm.collectCurrentLoadMetrics()
		elapsed := time.Since(start)

		b.ReportMetric(float64(elapsed.Nanoseconds()), "metrics_collection_ns")
		b.ReportMetric(float64(len(metrics.ShardSizes)), "monitored_shards")
		b.ReportMetric(float64(metrics.SearchLatency.Nanoseconds()), "max_latency_ns")
	}
}

// BenchmarkScalingDecisionMaking tests the performance of scaling decision algorithms
func BenchmarkScalingDecisionMaking(b *testing.B) {
	config := DefaultIndexerConfig()
	config.IndexPath = b.TempDir()
	config.ShardCount = 4

	dsm := NewEnhancedDynamicShardManager(config)
	if err := dsm.Initialize(); err != nil {
		b.Fatal(err)
	}
	defer dsm.Close()

	// Create test metrics
	testMetrics := LoadMetrics{
		SearchLatency:  2 * time.Second,
		ShardSizes:     []int64{1024 * 1024 * 1024, 2 * 1024 * 1024 * 1024, 512 * 1024 * 1024, 1536 * 1024 * 1024},
		CPUUtilization: 0.85,
		ActiveQueries:  10,
		QueueLength:    50,
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		start := time.Now()
		decision := dsm.makeScalingDecision(testMetrics)
		elapsed := time.Since(start)

		b.ReportMetric(float64(elapsed.Nanoseconds()), "decision_making_ns")
		b.ReportMetric(float64(decision.TargetShards), "target_shards")
		b.ReportMetric(decision.Confidence, "decision_confidence")
	}
}

// Helper function to seed test data for realistic benchmarks
func seedTestData(b *testing.B, dsm *EnhancedDynamicShardManager, docsPerShard int) {
	b.Helper()

	for shardID := 0; shardID < dsm.GetShardCount(); shardID++ {
		shard, err := dsm.GetShardByID(shardID)
		if err != nil {
			b.Fatalf("Failed to get shard %d: %v", shardID, err)
		}

		batch := shard.NewBatch()
		for i := 0; i < docsPerShard; i++ {
			docID := fmt.Sprintf("shard%d_doc%d", shardID, i)
			docData := map[string]interface{}{
				"content": fmt.Sprintf("Test content for shard %d document %d", shardID, i),
				"type":    "benchmark_test",
				"shard":   shardID,
				"index":   i,
			}
			batch.Index(docID, docData)
		}

		if err := shard.Batch(batch); err != nil {
			b.Fatalf("Failed to batch index in shard %d: %v", shardID, err)
		}
	}
}