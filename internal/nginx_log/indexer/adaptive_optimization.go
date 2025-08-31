package indexer

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/uozi-tech/cosy/logger"
)

// AdaptiveOptimizer provides intelligent batch size adjustment and CPU monitoring
type AdaptiveOptimizer struct {
	config                *Config
	cpuMonitor           *CPUMonitor
	batchSizeController  *BatchSizeController
	performanceHistory   *PerformanceHistory
	
	// State
	running              int32
	ctx                  context.Context
	cancel              context.CancelFunc
	wg                  sync.WaitGroup
	
	// Metrics
	optimizationsMade   int64
	avgThroughput      float64
	avgLatency         time.Duration
	metricsMutex       sync.RWMutex
}

// CPUMonitor monitors CPU utilization and suggests worker adjustments
type CPUMonitor struct {
	targetUtilization    float64
	measurementInterval  time.Duration
	adjustmentThreshold  float64
	maxWorkers          int
	minWorkers          int
	
	currentUtilization  float64
	measurements        []float64
	measurementsMutex   sync.RWMutex
}

// BatchSizeController dynamically adjusts batch sizes based on performance metrics
type BatchSizeController struct {
	baseBatchSize       int
	minBatchSize        int
	maxBatchSize        int
	adjustmentFactor    float64
	
	currentBatchSize    int32
	latencyThreshold    time.Duration
	throughputTarget    float64
	
	adjustmentHistory   []BatchAdjustment
	historyMutex       sync.RWMutex
}

// PerformanceHistory tracks performance metrics for optimization decisions
type PerformanceHistory struct {
	samples            []PerformanceSample
	maxSamples         int
	mutex              sync.RWMutex
	
	movingAvgWindow    int
	avgThroughput      float64
	avgLatency         time.Duration
}

// PerformanceSample represents a single performance measurement
type PerformanceSample struct {
	Timestamp    time.Time     `json:"timestamp"`
	Throughput   float64       `json:"throughput"`
	Latency      time.Duration `json:"latency"`
	CPUUsage     float64       `json:"cpu_usage"`
	BatchSize    int           `json:"batch_size"`
	WorkerCount  int           `json:"worker_count"`
}

// BatchAdjustment represents a batch size adjustment decision
type BatchAdjustment struct {
	Timestamp     time.Time     `json:"timestamp"`
	OldBatchSize  int           `json:"old_batch_size"`
	NewBatchSize  int           `json:"new_batch_size"`
	Reason        string        `json:"reason"`
	ThroughputImpact float64    `json:"throughput_impact"`
}

// NewAdaptiveOptimizer creates a new adaptive optimizer
func NewAdaptiveOptimizer(config *Config) *AdaptiveOptimizer {
	ctx, cancel := context.WithCancel(context.Background())
	
	return &AdaptiveOptimizer{
		config: config,
		cpuMonitor: &CPUMonitor{
			targetUtilization:   0.80, // Target 80% CPU utilization
			measurementInterval: 5 * time.Second,
			adjustmentThreshold: 0.15, // Adjust if 15% deviation from target
			maxWorkers:         runtime.NumCPU() * 3,
			minWorkers:         max(2, runtime.NumCPU()/2),
			measurements:       make([]float64, 0, 12), // 1 minute history at 5s intervals
		},
		batchSizeController: &BatchSizeController{
			baseBatchSize:     config.BatchSize,
			minBatchSize:      max(100, config.BatchSize/4),
			maxBatchSize:      config.BatchSize * 3,
			adjustmentFactor:  0.2, // 20% adjustment steps
			currentBatchSize:  int32(config.BatchSize),
			latencyThreshold:  5 * time.Second,
			throughputTarget:  25.0, // Target 25 MB/s
		},
		performanceHistory: &PerformanceHistory{
			samples:         make([]PerformanceSample, 0, 120), // 2 minutes of 1s samples
			maxSamples:      120,
			movingAvgWindow: 12, // 12-sample moving average
		},
		ctx:    ctx,
		cancel: cancel,
	}
}

// Start begins the adaptive optimization process
func (ao *AdaptiveOptimizer) Start() error {
	if !atomic.CompareAndSwapInt32(&ao.running, 0, 1) {
		logger.Error("Adaptive optimizer already running")
		return fmt.Errorf("adaptive optimizer already running")
	}
	
	// Start CPU monitoring
	ao.wg.Add(1)
	go ao.cpuMonitoringLoop()
	
	// Start batch size optimization
	ao.wg.Add(1)
	go ao.batchOptimizationLoop()
	
	// Start performance tracking
	ao.wg.Add(1)
	go ao.performanceTrackingLoop()
	
	logger.Info("Adaptive optimizer started")
	return nil
}

// Stop halts the adaptive optimization process
func (ao *AdaptiveOptimizer) Stop() {
	if !atomic.CompareAndSwapInt32(&ao.running, 1, 0) {
		return
	}
	
	ao.cancel()
	ao.wg.Wait()
	
	logger.Info("Adaptive optimizer stopped")
}

// cpuMonitoringLoop continuously monitors CPU utilization
func (ao *AdaptiveOptimizer) cpuMonitoringLoop() {
	defer ao.wg.Done()
	
	ticker := time.NewTicker(ao.cpuMonitor.measurementInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			ao.measureAndAdjustCPU()
		case <-ao.ctx.Done():
			return
		}
	}
}

// measureAndAdjustCPU measures current CPU utilization and suggests adjustments
func (ao *AdaptiveOptimizer) measureAndAdjustCPU() {
	// Get current CPU utilization
	cpuUsage := ao.getCurrentCPUUtilization()
	
	ao.cpuMonitor.measurementsMutex.Lock()
	ao.cpuMonitor.measurements = append(ao.cpuMonitor.measurements, cpuUsage)
	if len(ao.cpuMonitor.measurements) > cap(ao.cpuMonitor.measurements) {
		// Remove oldest measurement
		ao.cpuMonitor.measurements = ao.cpuMonitor.measurements[1:]
	}
	ao.cpuMonitor.currentUtilization = cpuUsage
	ao.cpuMonitor.measurementsMutex.Unlock()
	
	// Calculate average CPU utilization
	ao.cpuMonitor.measurementsMutex.RLock()
	avgCPU := ao.calculateAverageCPU()
	ao.cpuMonitor.measurementsMutex.RUnlock()
	
	// Determine if adjustment is needed
	targetCPU := ao.cpuMonitor.targetUtilization
	if avgCPU < targetCPU-ao.cpuMonitor.adjustmentThreshold {
		// CPU underutilized - suggest increasing workers
		ao.suggestWorkerIncrease(avgCPU, targetCPU)
	} else if avgCPU > targetCPU+ao.cpuMonitor.adjustmentThreshold {
		// CPU over-utilized - suggest decreasing workers  
		ao.suggestWorkerDecrease(avgCPU, targetCPU)
	}
}

// batchOptimizationLoop continuously optimizes batch sizes
func (ao *AdaptiveOptimizer) batchOptimizationLoop() {
	defer ao.wg.Done()
	
	ticker := time.NewTicker(10 * time.Second) // Adjust batch size every 10 seconds
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			ao.optimizeBatchSize()
		case <-ao.ctx.Done():
			return
		}
	}
}

// optimizeBatchSize analyzes performance and adjusts batch size
func (ao *AdaptiveOptimizer) optimizeBatchSize() {
	ao.performanceHistory.mutex.RLock()
	if len(ao.performanceHistory.samples) < 5 {
		ao.performanceHistory.mutex.RUnlock()
		return // Not enough data
	}
	
	recentSamples := ao.performanceHistory.samples[max(0, len(ao.performanceHistory.samples)-5):]
	avgThroughput := ao.calculateAverageThroughput(recentSamples)
	avgLatency := ao.calculateAverageLatency(recentSamples)
	ao.performanceHistory.mutex.RUnlock()
	
	currentBatchSize := int(atomic.LoadInt32(&ao.batchSizeController.currentBatchSize))
	newBatchSize := ao.calculateOptimalBatchSize(avgThroughput, avgLatency, currentBatchSize)
	
	if newBatchSize != currentBatchSize {
		ao.adjustBatchSize(currentBatchSize, newBatchSize, avgThroughput, avgLatency)
		atomic.AddInt64(&ao.optimizationsMade, 1)
	}
}

// calculateOptimalBatchSize determines the optimal batch size based on current performance
func (ao *AdaptiveOptimizer) calculateOptimalBatchSize(throughput float64, latency time.Duration, currentBatch int) int {
	controller := ao.batchSizeController
	
	// If latency is too high, reduce batch size
	if latency > controller.latencyThreshold {
		reduction := int(float64(currentBatch) * controller.adjustmentFactor)
		newSize := currentBatch - max(50, reduction)
		return max(controller.minBatchSize, newSize)
	}
	
	// If throughput is below target and latency is acceptable, increase batch size
	if throughput < controller.throughputTarget && latency < controller.latencyThreshold/2 {
		increase := int(float64(currentBatch) * controller.adjustmentFactor)
		newSize := currentBatch + max(100, increase)
		return min(controller.maxBatchSize, newSize)
	}
	
	// Current batch size seems optimal
	return currentBatch
}

// adjustBatchSize applies the batch size adjustment
func (ao *AdaptiveOptimizer) adjustBatchSize(oldSize, newSize int, throughput float64, latency time.Duration) {
	atomic.StoreInt32(&ao.batchSizeController.currentBatchSize, int32(newSize))
	
	var reason string
	if newSize > oldSize {
		reason = "Increasing batch size to improve throughput"
	} else {
		reason = "Reducing batch size to improve latency"
	}
	
	// Record adjustment
	adjustment := BatchAdjustment{
		Timestamp:        time.Now(),
		OldBatchSize:     oldSize,
		NewBatchSize:     newSize,
		Reason:          reason,
		ThroughputImpact: throughput,
	}
	
	ao.batchSizeController.historyMutex.Lock()
	ao.batchSizeController.adjustmentHistory = append(ao.batchSizeController.adjustmentHistory, adjustment)
	if len(ao.batchSizeController.adjustmentHistory) > 50 {
		// Keep only recent 50 adjustments
		ao.batchSizeController.adjustmentHistory = ao.batchSizeController.adjustmentHistory[1:]
	}
	ao.batchSizeController.historyMutex.Unlock()
	
	logger.Infof("Batch size adjusted: old_size=%d, new_size=%d, reason=%s", oldSize, newSize, reason)
}

// performanceTrackingLoop continuously tracks performance metrics
func (ao *AdaptiveOptimizer) performanceTrackingLoop() {
	defer ao.wg.Done()
	
	ticker := time.NewTicker(1 * time.Second) // Sample every second
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			ao.recordPerformanceSample()
		case <-ao.ctx.Done():
			return
		}
	}
}

// recordPerformanceSample records current performance metrics
func (ao *AdaptiveOptimizer) recordPerformanceSample() {
	sample := PerformanceSample{
		Timestamp:   time.Now(),
		Throughput:  ao.getCurrentThroughput(),
		Latency:     ao.getCurrentLatency(),
		CPUUsage:    ao.cpuMonitor.currentUtilization,
		BatchSize:   int(atomic.LoadInt32(&ao.batchSizeController.currentBatchSize)),
		WorkerCount: ao.config.WorkerCount,
	}
	
	ao.performanceHistory.mutex.Lock()
	ao.performanceHistory.samples = append(ao.performanceHistory.samples, sample)
	if len(ao.performanceHistory.samples) > ao.performanceHistory.maxSamples {
		// Remove oldest sample
		ao.performanceHistory.samples = ao.performanceHistory.samples[1:]
	}
	
	// Update moving averages
	ao.updateMovingAverages()
	ao.performanceHistory.mutex.Unlock()
}

// GetOptimalBatchSize returns the current optimal batch size
func (ao *AdaptiveOptimizer) GetOptimalBatchSize() int {
	return int(atomic.LoadInt32(&ao.batchSizeController.currentBatchSize))
}

// GetCPUUtilization returns the current CPU utilization
func (ao *AdaptiveOptimizer) GetCPUUtilization() float64 {
	ao.cpuMonitor.measurementsMutex.RLock()
	defer ao.cpuMonitor.measurementsMutex.RUnlock()
	return ao.cpuMonitor.currentUtilization
}

// GetOptimizationStats returns current optimization statistics
func (ao *AdaptiveOptimizer) GetOptimizationStats() AdaptiveOptimizationStats {
	ao.metricsMutex.RLock()
	defer ao.metricsMutex.RUnlock()
	
	return AdaptiveOptimizationStats{
		OptimizationsMade: atomic.LoadInt64(&ao.optimizationsMade),
		CurrentBatchSize:  int(atomic.LoadInt32(&ao.batchSizeController.currentBatchSize)),
		AvgThroughput:    ao.avgThroughput,
		AvgLatency:       ao.avgLatency,
		CPUUtilization:   ao.cpuMonitor.currentUtilization,
	}
}

// AdaptiveOptimizationStats represents current optimization statistics
type AdaptiveOptimizationStats struct {
	OptimizationsMade int64         `json:"optimizations_made"`
	CurrentBatchSize  int           `json:"current_batch_size"`
	AvgThroughput    float64       `json:"avg_throughput"`
	AvgLatency       time.Duration `json:"avg_latency"`
	CPUUtilization   float64       `json:"cpu_utilization"`
}

// Helper functions
func (ao *AdaptiveOptimizer) getCurrentCPUUtilization() float64 {
	// This is a simplified implementation
	// In production, you'd use a proper CPU monitoring library
	runtime.GC()
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	
	// Approximate CPU usage based on GC activity and goroutines
	numGoroutines := float64(runtime.NumGoroutine())
	numCPU := float64(runtime.NumCPU())
	
	// Simple heuristic: more goroutines = higher CPU usage
	utilization := numGoroutines / (numCPU * 10)
	if utilization > 0.95 {
		utilization = 0.95
	}
	return utilization
}

func (ao *AdaptiveOptimizer) getCurrentThroughput() float64 {
	// This would be implemented to get actual throughput from the indexer
	ao.metricsMutex.RLock()
	defer ao.metricsMutex.RUnlock()
	return ao.avgThroughput
}

func (ao *AdaptiveOptimizer) getCurrentLatency() time.Duration {
	// This would be implemented to get actual latency from the indexer
	ao.metricsMutex.RLock()
	defer ao.metricsMutex.RUnlock()
	return ao.avgLatency
}

func (ao *AdaptiveOptimizer) calculateAverageCPU() float64 {
	if len(ao.cpuMonitor.measurements) == 0 {
		return 0
	}
	
	sum := 0.0
	for _, cpu := range ao.cpuMonitor.measurements {
		sum += cpu
	}
	return sum / float64(len(ao.cpuMonitor.measurements))
}

func (ao *AdaptiveOptimizer) calculateAverageThroughput(samples []PerformanceSample) float64 {
	if len(samples) == 0 {
		return 0
	}
	
	sum := 0.0
	for _, sample := range samples {
		sum += sample.Throughput
	}
	return sum / float64(len(samples))
}

func (ao *AdaptiveOptimizer) calculateAverageLatency(samples []PerformanceSample) time.Duration {
	if len(samples) == 0 {
		return 0
	}
	
	var sum time.Duration
	for _, sample := range samples {
		sum += sample.Latency
	}
	return sum / time.Duration(len(samples))
}

func (ao *AdaptiveOptimizer) updateMovingAverages() {
	if len(ao.performanceHistory.samples) == 0 {
		return
	}
	
	windowSize := min(ao.performanceHistory.movingAvgWindow, len(ao.performanceHistory.samples))
	recentSamples := ao.performanceHistory.samples[len(ao.performanceHistory.samples)-windowSize:]
	
	ao.avgThroughput = ao.calculateAverageThroughput(recentSamples)
	ao.avgLatency = ao.calculateAverageLatency(recentSamples)
}

func (ao *AdaptiveOptimizer) suggestWorkerIncrease(currentCPU, targetCPU float64) {
	logger.Debug("CPU underutilized, consider increasing workers", 
		"current_cpu", currentCPU, "target_cpu", targetCPU)
}

func (ao *AdaptiveOptimizer) suggestWorkerDecrease(currentCPU, targetCPU float64) {
	logger.Debug("CPU over-utilized, consider decreasing workers", 
		"current_cpu", currentCPU, "target_cpu", targetCPU)
}

// Utility functions
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}