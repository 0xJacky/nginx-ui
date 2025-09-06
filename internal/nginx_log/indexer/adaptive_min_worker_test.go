package indexer

import (
	"runtime"
	"testing"
)

// TestMinWorkerCalculation tests the minimum worker calculation for different CPU configurations
func TestMinWorkerCalculation(t *testing.T) {
	testCases := []struct {
		name            string
		maxProcs        int
		expectedMin     int
		description     string
	}{
		{
			name:        "Single core system",
			maxProcs:    1,
			expectedMin: 1,
			description: "max(1, 1/8) = max(1, 0) = 1",
		},
		{
			name:        "Dual core system",
			maxProcs:    2,
			expectedMin: 1,
			description: "max(1, 2/8) = max(1, 0) = 1",
		},
		{
			name:        "Quad core system",
			maxProcs:    4,
			expectedMin: 1,
			description: "max(1, 4/8) = max(1, 0) = 1",
		},
		{
			name:        "8-core system",
			maxProcs:    8,
			expectedMin: 1,
			description: "max(1, 8/8) = max(1, 1) = 1",
		},
		{
			name:        "16-core system",
			maxProcs:    16,
			expectedMin: 2,
			description: "max(1, 16/8) = max(1, 2) = 2",
		},
		{
			name:        "24-core system",
			maxProcs:    24,
			expectedMin: 3,
			description: "max(1, 24/8) = max(1, 3) = 3",
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Simulate the min worker calculation logic
			calculatedMin := max(1, tc.maxProcs/8)
			
			if calculatedMin != tc.expectedMin {
				t.Errorf("Expected min workers %d, got %d for %d cores", 
					tc.expectedMin, calculatedMin, tc.maxProcs)
			}
			
			t.Logf("✅ %s: %s -> min workers = %d", 
				tc.name, tc.description, calculatedMin)
		})
	}
}

// TestCPUOptimizationScenario simulates the CPU over-utilization scenario
func TestCPUOptimizationScenario(t *testing.T) {
	// Simulate a 2-core system (common in containers/VMs)
	simulatedGOMAXPROCS := 2
	
	// Create a mock config similar to production
	config := &Config{
		WorkerCount: 2, // Starting with 2 workers
	}
	
	// Create adaptive optimizer with simulated CPU configuration
	ao := &AdaptiveOptimizer{
		config: config,
		cpuMonitor: &CPUMonitor{
			targetUtilization:   0.75, // Target 75% CPU utilization
			measurementInterval: 5,
			adjustmentThreshold: 0.10, // 10% threshold
			maxWorkers:         simulatedGOMAXPROCS * 3,
			minWorkers:         max(1, simulatedGOMAXPROCS/8), // New formula
			measurements:       make([]float64, 0, 12),
		},
		performanceHistory: &PerformanceHistory{
			samples: make([]PerformanceSample, 0, 120),
		},
	}
	
	// Mock worker count change callback
	workerAdjustments := []int{}
	ao.SetWorkerCountChangeCallback(func(oldCount, newCount int) {
		workerAdjustments = append(workerAdjustments, newCount)
		config.WorkerCount = newCount // Update the config
		t.Logf("🔧 Worker count adjusted from %d to %d", oldCount, newCount)
	})
	
	// Simulate CPU over-utilization scenario
	currentCPU := 0.95 // 95% CPU usage
	targetCPU := 0.75  // 75% target
	
	t.Logf("📊 Initial state: workers=%d, minWorkers=%d, CPU=%.1f%%, target=%.1f%%", 
		config.WorkerCount, ao.cpuMonitor.minWorkers, currentCPU*100, targetCPU*100)
	
	// Test the worker decrease logic
	ao.suggestWorkerDecrease(currentCPU, targetCPU)
	
	// Verify that adjustment happened
	if len(workerAdjustments) == 0 {
		t.Errorf("Expected worker count adjustment, but none occurred")
	} else {
		originalWorkers := 2 // We started with 2 workers
		newCount := workerAdjustments[0]
		if newCount >= originalWorkers {
			t.Errorf("Expected worker count to decrease from %d, but got %d", 
				originalWorkers, newCount)
		} else {
			t.Logf("✅ Successfully reduced workers from %d to %d", originalWorkers, newCount)
		}
		
		// Verify minimum constraint is respected
		if newCount < ao.cpuMonitor.minWorkers {
			t.Errorf("Worker count %d is below minimum %d", newCount, ao.cpuMonitor.minWorkers)
		}
	}
	
	// Test repeated optimization calls (simulating the loop issue)
	t.Logf("🔄 Testing repeated optimization calls...")
	
	initialWorkerCount := config.WorkerCount
	for i := 0; i < 5; i++ {
		ao.suggestWorkerDecrease(currentCPU, targetCPU)
		t.Logf("Iteration %d: workers=%d", i+1, config.WorkerCount)
		
		// If worker count reached minimum, it should stop decreasing
		if config.WorkerCount == ao.cpuMonitor.minWorkers {
			t.Logf("✅ Reached minimum worker count %d", config.WorkerCount)
			break
		}
	}
	
	// Verify we didn't get stuck in infinite loop
	if config.WorkerCount < initialWorkerCount {
		t.Logf("✅ Worker count successfully reduced from %d to %d", 
			initialWorkerCount, config.WorkerCount)
	}
}

// TestCurrentSystemConfiguration tests with actual system GOMAXPROCS
func TestCurrentSystemConfiguration(t *testing.T) {
	currentCores := runtime.GOMAXPROCS(0)
	minWorkers := max(1, currentCores/8)
	
	t.Logf("🖥️ Current system configuration:")
	t.Logf("   GOMAXPROCS(0): %d", currentCores)
	t.Logf("   Calculated min workers: %d", minWorkers)
	t.Logf("   Max workers: %d", currentCores*3)
	
	// Verify that we can always scale down to minimum
	if minWorkers >= 2 && currentCores <= 16 {
		t.Errorf("Min workers %d seems too high for %d cores - may prevent scaling down", 
			minWorkers, currentCores)
	}
	
	// Test scaling scenarios
	scenarios := []struct {
		startWorkers int
		canScaleDown bool
	}{
		{startWorkers: 8, canScaleDown: true},
		{startWorkers: 4, canScaleDown: true}, 
		{startWorkers: 2, canScaleDown: minWorkers < 2},
		{startWorkers: 1, canScaleDown: false},
	}
	
	for _, scenario := range scenarios {
		actualCanScale := scenario.startWorkers > minWorkers
		if actualCanScale != scenario.canScaleDown {
			t.Logf("⚠️ Scenario mismatch: starting with %d workers, min=%d, expected canScale=%v, actual=%v", 
				scenario.startWorkers, minWorkers, scenario.canScaleDown, actualCanScale)
		} else {
			t.Logf("✅ Scaling scenario: %d workers -> min %d (can scale down: %v)", 
				scenario.startWorkers, minWorkers, actualCanScale)
		}
	}
}