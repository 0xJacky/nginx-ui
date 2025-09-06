package indexer

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// Mock config for testing
type mockConfigForAdaptive struct {
	workerCount int
	batchSize   int
}

func (m *mockConfigForAdaptive) GetWorkerCount() int {
	return m.workerCount
}

func (m *mockConfigForAdaptive) SetWorkerCount(count int) {
	m.workerCount = count
}

// Test helper to create adaptive optimizer with mock config
func createTestAdaptiveOptimizer(workerCount int) *AdaptiveOptimizer {
	config := &Config{
		WorkerCount: workerCount,
		BatchSize:   1000,
	}
	
	return NewAdaptiveOptimizer(config)
}

func TestAdaptiveOptimizer_NewAdaptiveOptimizer(t *testing.T) {
	config := &Config{
		WorkerCount: 8,
		BatchSize:   1000,
	}
	
	ao := NewAdaptiveOptimizer(config)
	
	if ao == nil {
		t.Fatal("NewAdaptiveOptimizer returned nil")
	}
	
	if ao.config.WorkerCount != 8 {
		t.Errorf("Expected worker count 8, got %d", ao.config.WorkerCount)
	}
	
	if ao.cpuMonitor.targetUtilization != 0.75 {
		t.Errorf("Expected target CPU utilization 0.75, got %f", ao.cpuMonitor.targetUtilization)
	}
	
	if ao.batchSizeController.baseBatchSize != 1000 {
		t.Errorf("Expected base batch size 1000, got %d", ao.batchSizeController.baseBatchSize)
	}
}

func TestAdaptiveOptimizer_SetWorkerCountChangeCallback(t *testing.T) {
	ao := createTestAdaptiveOptimizer(4)
	
	var callbackOldCount, callbackNewCount int
	callbackCalled := false
	
	ao.SetWorkerCountChangeCallback(func(oldCount, newCount int) {
		callbackOldCount = oldCount
		callbackNewCount = newCount
		callbackCalled = true
	})
	
	// Trigger a callback
	if ao.onWorkerCountChange != nil {
		ao.onWorkerCountChange(4, 6)
	}
	
	if !callbackCalled {
		t.Error("Expected callback to be called")
	}
	
	if callbackOldCount != 4 {
		t.Errorf("Expected old count 4, got %d", callbackOldCount)
	}
	
	if callbackNewCount != 6 {
		t.Errorf("Expected new count 6, got %d", callbackNewCount)
	}
}

func TestAdaptiveOptimizer_suggestWorkerIncrease(t *testing.T) {
	ao := createTestAdaptiveOptimizer(4)
	
	var actualOldCount, actualNewCount int
	var callbackCalled bool
	
	ao.SetWorkerCountChangeCallback(func(oldCount, newCount int) {
		actualOldCount = oldCount
		actualNewCount = newCount
		callbackCalled = true
	})
	
	// Test CPU underutilization scenario
	currentCPU := 0.5  // 50% utilization
	targetCPU := 0.8   // 80% target
	
	ao.suggestWorkerIncrease(currentCPU, targetCPU)
	
	if !callbackCalled {
		t.Error("Expected worker count change callback to be called")
	}
	
	if actualOldCount != 4 {
		t.Errorf("Expected old worker count 4, got %d", actualOldCount)
	}
	
	// Should increase workers, but not more than max allowed
	if actualNewCount <= 4 {
		t.Errorf("Expected new worker count to be greater than 4, got %d", actualNewCount)
	}
	
	// Verify config was updated
	if ao.config.WorkerCount != actualNewCount {
		t.Errorf("Expected config worker count to be updated to %d, got %d", actualNewCount, ao.config.WorkerCount)
	}
}

func TestAdaptiveOptimizer_suggestWorkerDecrease(t *testing.T) {
	ao := createTestAdaptiveOptimizer(8)
	
	var actualOldCount, actualNewCount int
	var callbackCalled bool
	
	ao.SetWorkerCountChangeCallback(func(oldCount, newCount int) {
		actualOldCount = oldCount
		actualNewCount = newCount
		callbackCalled = true
	})
	
	// Test CPU over-utilization scenario
	currentCPU := 0.95  // 95% utilization
	targetCPU := 0.8    // 80% target
	
	ao.suggestWorkerDecrease(currentCPU, targetCPU)
	
	if !callbackCalled {
		t.Error("Expected worker count change callback to be called")
	}
	
	if actualOldCount != 8 {
		t.Errorf("Expected old worker count 8, got %d", actualOldCount)
	}
	
	// Should decrease workers, but not below minimum
	if actualNewCount >= 8 {
		t.Errorf("Expected new worker count to be less than 8, got %d", actualNewCount)
	}
	
	// Should not go below minimum
	if actualNewCount < ao.cpuMonitor.minWorkers {
		t.Errorf("New worker count %d should not be below minimum %d", actualNewCount, ao.cpuMonitor.minWorkers)
	}
	
	// Verify config was updated
	if ao.config.WorkerCount != actualNewCount {
		t.Errorf("Expected config worker count to be updated to %d, got %d", actualNewCount, ao.config.WorkerCount)
	}
}

func TestAdaptiveOptimizer_adjustWorkerCount_NoChange(t *testing.T) {
	ao := createTestAdaptiveOptimizer(4)
	
	var callbackCalled bool
	ao.SetWorkerCountChangeCallback(func(oldCount, newCount int) {
		callbackCalled = true
	})
	
	// Test no change scenario
	ao.adjustWorkerCount(4) // Same as current
	
	if callbackCalled {
		t.Error("Expected no callback when worker count doesn't change")
	}
	
	if ao.config.WorkerCount != 4 {
		t.Errorf("Expected worker count to remain 4, got %d", ao.config.WorkerCount)
	}
}

func TestAdaptiveOptimizer_adjustWorkerCount_InvalidCount(t *testing.T) {
	ao := createTestAdaptiveOptimizer(4)
	
	var callbackCalled bool
	ao.SetWorkerCountChangeCallback(func(oldCount, newCount int) {
		callbackCalled = true
	})
	
	// Test invalid count (0 or negative)
	ao.adjustWorkerCount(0)
	ao.adjustWorkerCount(-1)
	
	if callbackCalled {
		t.Error("Expected no callback for invalid worker counts")
	}
	
	if ao.config.WorkerCount != 4 {
		t.Errorf("Expected worker count to remain 4, got %d", ao.config.WorkerCount)
	}
}

func TestAdaptiveOptimizer_GetOptimalBatchSize(t *testing.T) {
	ao := createTestAdaptiveOptimizer(4)
	
	// Initial batch size should be from config
	batchSize := ao.GetOptimalBatchSize()
	expectedInitial := int32(1000)
	if batchSize != int(expectedInitial) {
		t.Errorf("Expected initial batch size %d, got %d", expectedInitial, batchSize)
	}
	
	// Test updating batch size
	newBatchSize := int32(1500)
	atomic.StoreInt32(&ao.batchSizeController.currentBatchSize, newBatchSize)
	
	batchSize = ao.GetOptimalBatchSize()
	if batchSize != int(newBatchSize) {
		t.Errorf("Expected updated batch size %d, got %d", newBatchSize, batchSize)
	}
}

func TestAdaptiveOptimizer_measureAndAdjustCPU_WithinThreshold(t *testing.T) {
	ao := createTestAdaptiveOptimizer(4)
	
	var callbackCalled bool
	ao.SetWorkerCountChangeCallback(func(oldCount, newCount int) {
		callbackCalled = true
	})
	
	// Mock CPU measurements within threshold
	ao.cpuMonitor.measurements = []float64{0.78, 0.79, 0.81, 0.82} // Around 0.8 target
	
	ao.measureAndAdjustCPU()
	
	// Should not trigger worker adjustment if within threshold
	if callbackCalled {
		t.Error("Expected no worker adjustment when CPU is within threshold")
	}
}

func TestAdaptiveOptimizer_GetCPUUtilization(t *testing.T) {
	ao := createTestAdaptiveOptimizer(4)
	
	// Set current utilization
	ao.cpuMonitor.currentUtilization = 0.75
	
	utilization := ao.GetCPUUtilization()
	if utilization != 0.75 {
		t.Errorf("Expected CPU utilization 0.75, got %f", utilization)
	}
}

func TestAdaptiveOptimizer_GetOptimizationStats(t *testing.T) {
	ao := createTestAdaptiveOptimizer(4)
	
	// Set some test values
	atomic.StoreInt64(&ao.optimizationsMade, 5)
	ao.avgThroughput = 25.5
	ao.avgLatency = 2 * time.Second
	ao.cpuMonitor.currentUtilization = 0.85
	
	stats := ao.GetOptimizationStats()
	
	if stats.OptimizationsMade != 5 {
		t.Errorf("Expected 5 optimizations made, got %d", stats.OptimizationsMade)
	}
	
	if stats.AvgThroughput != 25.5 {
		t.Errorf("Expected avg throughput 25.5, got %f", stats.AvgThroughput)
	}
	
	if stats.AvgLatency != 2*time.Second {
		t.Errorf("Expected avg latency 2s, got %v", stats.AvgLatency)
	}
	
	if stats.CPUUtilization != 0.85 {
		t.Errorf("Expected CPU utilization 0.85, got %f", stats.CPUUtilization)
	}
	
	if stats.CurrentBatchSize != 1000 {
		t.Errorf("Expected current batch size 1000, got %d", stats.CurrentBatchSize)
	}
}

func TestAdaptiveOptimizer_StartStop(t *testing.T) {
	ao := createTestAdaptiveOptimizer(4)
	
	// Test start
	err := ao.Start()
	if err != nil {
		t.Fatalf("Failed to start adaptive optimizer: %v", err)
	}
	
	// Verify running state
	if atomic.LoadInt32(&ao.running) != 1 {
		t.Error("Expected adaptive optimizer to be running")
	}
	
	// Test starting again (should fail)
	err = ao.Start()
	if err == nil {
		t.Error("Expected error when starting already running optimizer")
	}
	
	// Small delay to let goroutines start
	time.Sleep(100 * time.Millisecond)
	
	// Test stop
	ao.Stop()
	
	// Verify stopped state
	if atomic.LoadInt32(&ao.running) != 0 {
		t.Error("Expected adaptive optimizer to be stopped")
	}
}

func TestAdaptiveOptimizer_WorkerAdjustmentLimits(t *testing.T) {
	// Test maximum worker limit
	ao := createTestAdaptiveOptimizer(16) // Start with high count
	ao.cpuMonitor.maxWorkers = 20
	
	var actualNewCount int
	ao.SetWorkerCountChangeCallback(func(oldCount, newCount int) {
		actualNewCount = newCount
	})
	
	// Try to increase beyond max
	ao.suggestWorkerIncrease(0.2, 0.8) // Very low CPU, should want to increase
	
	if actualNewCount > ao.cpuMonitor.maxWorkers {
		t.Errorf("New worker count %d exceeds maximum %d", actualNewCount, ao.cpuMonitor.maxWorkers)
	}
	
	// Test minimum worker limit
	ao2 := createTestAdaptiveOptimizer(3)
	ao2.cpuMonitor.minWorkers = 2
	
	ao2.SetWorkerCountChangeCallback(func(oldCount, newCount int) {
		actualNewCount = newCount
	})
	
	// Try to decrease below min
	ao2.suggestWorkerDecrease(0.98, 0.8) // Very high CPU, should want to decrease
	
	if actualNewCount < ao2.cpuMonitor.minWorkers {
		t.Errorf("New worker count %d below minimum %d", actualNewCount, ao2.cpuMonitor.minWorkers)
	}
}

func TestAdaptiveOptimizer_ConcurrentAccess(t *testing.T) {
	ao := createTestAdaptiveOptimizer(4)
	
	var wg sync.WaitGroup
	var adjustmentCount int32
	
	ao.SetWorkerCountChangeCallback(func(oldCount, newCount int) {
		atomic.AddInt32(&adjustmentCount, 1)
	})
	
	// Simulate concurrent CPU measurements and adjustments
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			
			// Simulate alternating high and low CPU
			if i%2 == 0 {
				ao.suggestWorkerIncrease(0.4, 0.8)
			} else {
				ao.suggestWorkerDecrease(0.95, 0.8)
			}
		}()
	}
	
	wg.Wait()
	
	// Verify that some adjustments were made
	finalCount := atomic.LoadInt32(&adjustmentCount)
	if finalCount == 0 {
		t.Error("Expected some worker adjustments to be made")
	}
	
	// Verify final state is valid
	if ao.config.WorkerCount < ao.cpuMonitor.minWorkers || ao.config.WorkerCount > ao.cpuMonitor.maxWorkers {
		t.Errorf("Final worker count %d outside valid range [%d, %d]", 
			ao.config.WorkerCount, ao.cpuMonitor.minWorkers, ao.cpuMonitor.maxWorkers)
	}
}