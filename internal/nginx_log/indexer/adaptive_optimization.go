package indexer

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/uozi-tech/cosy/logger"
)

// IndexerActivityPoller defines an interface to check if the indexer is busy.
type IndexerActivityPoller interface {
	IsBusy() bool
}

// AdaptiveOptimizer provides intelligent batch size adjustment and CPU monitoring
type AdaptiveOptimizer struct {
	config              *Config
	cpuMonitor          *CPUMonitor
	batchSizeController *BatchSizeController
	performanceHistory  *PerformanceHistory

	// State
	running int32
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup

	// Metrics
	optimizationsMade int64
	avgThroughput     float64
	avgLatency        time.Duration
	metricsMutex      sync.RWMutex

	// Callbacks
	onWorkerCountChange func(oldCount, newCount int)

	// Activity Poller
	activityPoller IndexerActivityPoller

	// Concurrency-safe mirror of worker count
	workerCount int64
}

// CPUMonitor monitors CPU utilization and suggests worker adjustments
type CPUMonitor struct {
	targetUtilization   float64
	measurementInterval time.Duration
	adjustmentThreshold float64
	maxWorkers          int
	minWorkers          int

	currentUtilization   float64
	measurements         []float64
	measurementsMutex    sync.RWMutex
	maxSamples           int
	lastValidUtilization float64
}

// BatchSizeController dynamically adjusts batch sizes based on performance metrics
type BatchSizeController struct {
	baseBatchSize    int
	minBatchSize     int
	maxBatchSize     int
	adjustmentFactor float64

	currentBatchSize int32
	latencyThreshold time.Duration
	throughputTarget float64

	adjustmentHistory []BatchAdjustment
	historyMutex      sync.RWMutex
}

// PerformanceHistory tracks performance metrics for optimization decisions
type PerformanceHistory struct {
	samples    []PerformanceSample
	maxSamples int
	mutex      sync.RWMutex

	movingAvgWindow int
}

// PerformanceSample represents a single performance measurement
type PerformanceSample struct {
	Timestamp   time.Time     `json:"timestamp"`
	Throughput  float64       `json:"throughput"`
	Latency     time.Duration `json:"latency"`
	CPUUsage    float64       `json:"cpu_usage"`
	BatchSize   int           `json:"batch_size"`
	WorkerCount int           `json:"worker_count"`
}

// BatchAdjustment represents a batch size adjustment decision
type BatchAdjustment struct {
	Timestamp        time.Time `json:"timestamp"`
	OldBatchSize     int       `json:"old_batch_size"`
	NewBatchSize     int       `json:"new_batch_size"`
	Reason           string    `json:"reason"`
	ThroughputImpact float64   `json:"throughput_impact"`
}

// NewAdaptiveOptimizer creates a new adaptive optimizer
func NewAdaptiveOptimizer(config *Config) *AdaptiveOptimizer {
	ctx, cancel := context.WithCancel(context.Background())

	ao := &AdaptiveOptimizer{
		config: config,
		cpuMonitor: &CPUMonitor{
			targetUtilization:   0.75, // Target 75% CPU utilization (more conservative)
			measurementInterval: 5 * time.Second,
			adjustmentThreshold: 0.10,                            // Adjust if 10% deviation from target (more sensitive)
			maxWorkers:          runtime.GOMAXPROCS(0) * 6,       // Allow scaling up to 6x CPU cores for I/O-bound workloads
			minWorkers:          max(2, runtime.GOMAXPROCS(0)/4), // Minimum 2 workers or 1/4 of cores for baseline performance
			measurements:        make([]float64, 0, 12),          // 1 minute history at 5s intervals
			maxSamples:          12,
		},
		batchSizeController: &BatchSizeController{
			baseBatchSize:    config.BatchSize,
			minBatchSize:     max(500, config.BatchSize/6), // Higher minimum for throughput
			maxBatchSize:     config.BatchSize * 6,         // Increased to 6x for maximum throughput
			adjustmentFactor: 0.25,                         // 25% adjustment steps for faster scaling
			currentBatchSize: int32(config.BatchSize),
			latencyThreshold: 8 * time.Second, // Higher latency tolerance for throughput
			throughputTarget: 50.0,            // Target 50 MB/s - higher throughput target
		},
		performanceHistory: &PerformanceHistory{
			samples:         make([]PerformanceSample, 0, 120), // 2 minutes of 1s samples
			maxSamples:      120,
			movingAvgWindow: 12, // 12-sample moving average
		},
		ctx:    ctx,
		cancel: cancel,
	}

	// Log initialization parameters for debugging
	logger.Infof("Adaptive optimizer initialized: workers=[%d, %d, %d] (min, current, max), target_cpu=%.1f%%, threshold=%.1f%%",
		ao.cpuMonitor.minWorkers, config.WorkerCount, ao.cpuMonitor.maxWorkers,
		ao.cpuMonitor.targetUtilization*100, ao.cpuMonitor.adjustmentThreshold*100)

	// Initialize atomic mirror of worker count
	atomic.StoreInt64(&ao.workerCount, int64(config.WorkerCount))

	return ao
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
	// Only adjust when indexer is actively processing
	if !ao.isIndexerBusy() {
		return
	}
	// Get current CPU utilization
	cpuUsage := ao.getCurrentCPUUtilization()

	ao.cpuMonitor.measurementsMutex.Lock()
	// Filter spurious near-zero values using last valid utilization
	if cpuUsage < 0.005 && ao.cpuMonitor.lastValidUtilization > 0 {
		cpuUsage = ao.cpuMonitor.lastValidUtilization
	}
	// Maintain fixed-size window to avoid capacity growth
	if ao.cpuMonitor.maxSamples > 0 && len(ao.cpuMonitor.measurements) == ao.cpuMonitor.maxSamples {
		copy(ao.cpuMonitor.measurements, ao.cpuMonitor.measurements[1:])
		ao.cpuMonitor.measurements[len(ao.cpuMonitor.measurements)-1] = cpuUsage
	} else {
		ao.cpuMonitor.measurements = append(ao.cpuMonitor.measurements, cpuUsage)
	}
	ao.cpuMonitor.currentUtilization = cpuUsage
	if cpuUsage >= 0.005 {
		ao.cpuMonitor.lastValidUtilization = cpuUsage
	}
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
	// Only adjust when indexer is actively processing
	if !ao.isIndexerBusy() {
		return
	}
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
	// Only adjust when indexer is actively processing
	if !ao.isIndexerBusy() {
		return
	}
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
		Reason:           reason,
		ThroughputImpact: throughput,
	}

	ao.batchSizeController.historyMutex.Lock()
	if len(ao.batchSizeController.adjustmentHistory) == 50 {
		copy(ao.batchSizeController.adjustmentHistory, ao.batchSizeController.adjustmentHistory[1:])
		ao.batchSizeController.adjustmentHistory[len(ao.batchSizeController.adjustmentHistory)-1] = adjustment
	} else {
		ao.batchSizeController.adjustmentHistory = append(ao.batchSizeController.adjustmentHistory, adjustment)
	}
	ao.batchSizeController.historyMutex.Unlock()

	logger.Debugf("Batch size adjusted: old_size=%d, new_size=%d, reason=%s", oldSize, newSize, reason)
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
		CPUUsage:    ao.GetCPUUtilization(),
		BatchSize:   int(atomic.LoadInt32(&ao.batchSizeController.currentBatchSize)),
		WorkerCount: int(atomic.LoadInt64(&ao.workerCount)),
	}

	ao.performanceHistory.mutex.Lock()
	if ao.performanceHistory.maxSamples > 0 && len(ao.performanceHistory.samples) == ao.performanceHistory.maxSamples {
		copy(ao.performanceHistory.samples, ao.performanceHistory.samples[1:])
		ao.performanceHistory.samples[len(ao.performanceHistory.samples)-1] = sample
	} else {
		ao.performanceHistory.samples = append(ao.performanceHistory.samples, sample)
	}

	// Update moving averages
	ao.updateMovingAverages()
	ao.performanceHistory.mutex.Unlock()
}

// SetWorkerCountChangeCallback sets the callback function for worker count changes
func (ao *AdaptiveOptimizer) SetWorkerCountChangeCallback(callback func(oldCount, newCount int)) {
	ao.onWorkerCountChange = callback
}

// SetActivityPoller sets the poller to check for indexer activity.
func (ao *AdaptiveOptimizer) SetActivityPoller(poller IndexerActivityPoller) {
	ao.activityPoller = poller
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
		AvgThroughput:     ao.avgThroughput,
		AvgLatency:        ao.avgLatency,
		CPUUtilization:    ao.GetCPUUtilization(),
	}
}

// AdaptiveOptimizationStats represents current optimization statistics
type AdaptiveOptimizationStats struct {
	OptimizationsMade int64         `json:"optimizations_made"`
	CurrentBatchSize  int           `json:"current_batch_size"`
	AvgThroughput     float64       `json:"avg_throughput"`
	AvgLatency        time.Duration `json:"avg_latency"`
	CPUUtilization    float64       `json:"cpu_utilization"`
}

// Helper functions
func (ao *AdaptiveOptimizer) getCurrentCPUUtilization() float64 {
	// Get CPU utilization since the last call.
	// Interval 0 means non-blocking and compares to the last measurement.
	// The first call will return 0.
	percentages, err := cpu.Percent(0, false)
	if err != nil || len(percentages) == 0 {
		logger.Warnf("Failed to get real CPU utilization, falling back to goroutine heuristic: %v", err)
		// Fallback to the old, less accurate method
		numGoroutines := float64(runtime.NumGoroutine())
		maxProcs := float64(runtime.GOMAXPROCS(0))

		// Simple heuristic: more goroutines = higher CPU usage
		utilization := numGoroutines / (maxProcs * 10)
		if utilization > 0.95 {
			utilization = 0.95
		}
		return utilization
	}

	// gopsutil returns a slice, for overall usage (percpu=false), it's the first element.
	// The value is a percentage (e.g., 8.3), so we convert it to a 0.0-1.0 scale for our calculations.
	return percentages[0] / 100.0
}

func (ao *AdaptiveOptimizer) getCurrentThroughput() float64 {
	ao.metricsMutex.RLock()
	v := ao.avgThroughput
	ao.metricsMutex.RUnlock()
	return v
}

func (ao *AdaptiveOptimizer) getCurrentLatency() time.Duration {
	ao.metricsMutex.RLock()
	v := ao.avgLatency
	ao.metricsMutex.RUnlock()
	return v
}

func (ao *AdaptiveOptimizer) calculateAverageCPU() float64 {
	if len(ao.cpuMonitor.measurements) == 0 {
		return 0
	}

	sum := 0.0
	for _, usage := range ao.cpuMonitor.measurements {
		sum += usage
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

	avgThroughput := ao.calculateAverageThroughput(recentSamples)
	avgLatency := ao.calculateAverageLatency(recentSamples)

	ao.metricsMutex.Lock()
	ao.avgThroughput = avgThroughput
	ao.avgLatency = avgLatency
	ao.metricsMutex.Unlock()
}

func (ao *AdaptiveOptimizer) suggestWorkerIncrease(currentCPU, targetCPU float64) {
	// If already at max workers, do nothing.
	currentWorkers := int(atomic.LoadInt64(&ao.workerCount))
	if currentWorkers >= ao.cpuMonitor.maxWorkers {
		return
	}

	// If the indexer is not busy, don't scale up workers even if CPU is low.
	if ao.activityPoller != nil && !ao.activityPoller.IsBusy() {
		return
	}

	logger.Debug("CPU underutilized, adjusting workers upward",
		"current_cpu", currentCPU, "target_cpu", targetCPU)

	// Calculate suggested increase (conservative approach)
	cpuUtilizationGap := targetCPU - currentCPU
	increaseRatio := cpuUtilizationGap / targetCPU

	// Limit increase to maximum 25% at a time and at least 1 worker
	maxIncrease := max(1, int(float64(currentWorkers)*0.25))
	suggestedIncrease := max(1, int(float64(currentWorkers)*increaseRatio))
	actualIncrease := min(maxIncrease, suggestedIncrease)

	newWorkerCount := min(ao.cpuMonitor.maxWorkers, currentWorkers+actualIncrease)

	if newWorkerCount > currentWorkers {
		ao.adjustWorkerCount(newWorkerCount)
		logger.Infof("Increased workers from %d to %d due to CPU underutilization",
			currentWorkers, newWorkerCount)
	}
}

func (ao *AdaptiveOptimizer) suggestWorkerDecrease(currentCPU, targetCPU float64) {
	// If the indexer is not busy, don't adjust workers
	if !ao.isIndexerBusy() {
		return
	}
	// If already at min workers, do nothing.
	currentWorkers := int(atomic.LoadInt64(&ao.workerCount))
	if currentWorkers <= ao.cpuMonitor.minWorkers {
		logger.Debugf("Worker count is already at its minimum (%d), skipping decrease.", currentWorkers)
		return
	}

	logger.Debug("CPU over-utilized, adjusting workers downward",
		"current_cpu", currentCPU, "target_cpu", targetCPU)

	// Calculate suggested decrease (conservative approach)
	cpuOverUtilization := currentCPU - targetCPU
	decreaseRatio := cpuOverUtilization / targetCPU // Use target CPU as base for more accurate calculation

	// Limit decrease to maximum 25% at a time and at least 1 worker
	maxDecrease := max(1, int(float64(currentWorkers)*0.25))
	suggestedDecrease := max(1, int(float64(currentWorkers)*decreaseRatio*0.5)) // More conservative decrease
	actualDecrease := min(maxDecrease, suggestedDecrease)

	newWorkerCount := max(ao.cpuMonitor.minWorkers, currentWorkers-actualDecrease)

	logger.Debugf("Worker decrease calculation: current=%d, suggested=%d, min=%d, new=%d",
		currentWorkers, suggestedDecrease, ao.cpuMonitor.minWorkers, newWorkerCount)

	if newWorkerCount < currentWorkers {
		logger.Debugf("About to adjust worker count from %d to %d", currentWorkers, newWorkerCount)
		ao.adjustWorkerCount(newWorkerCount)
		logger.Infof("Decreased workers from %d to %d due to CPU over-utilization",
			currentWorkers, newWorkerCount)
	} else {
		logger.Debugf("Worker count adjustment skipped: new=%d not less than current=%d", newWorkerCount, currentWorkers)
	}
}

// adjustWorkerCount dynamically adjusts the worker count at runtime
func (ao *AdaptiveOptimizer) adjustWorkerCount(newCount int) {
	// Only adjust when indexer is actively processing
	if !ao.isIndexerBusy() {
		logger.Debugf("Skipping worker adjustment while idle: requested=%d", newCount)
		return
	}
	oldCount := int(atomic.LoadInt64(&ao.workerCount))
	if newCount <= 0 || newCount == oldCount {
		logger.Debugf("Skipping worker adjustment: newCount=%d, currentCount=%d", newCount, oldCount)
		return
	}

	logger.Infof("Adjusting worker count from %d to %d", oldCount, newCount)

	// Update atomic mirror then keep config in sync
	atomic.StoreInt64(&ao.workerCount, int64(newCount))
	ao.config.WorkerCount = newCount

	// Notify the indexer about worker count change
	// This would typically trigger a worker pool resize in the parallel indexer
	if ao.onWorkerCountChange != nil {
		logger.Debugf("Calling worker count change callback: %d -> %d", oldCount, newCount)
		ao.onWorkerCountChange(oldCount, newCount)
	} else {
		logger.Warnf("Worker count change callback is nil - worker adjustment will not take effect")
	}

	// Log the adjustment for monitoring
	atomic.AddInt64(&ao.optimizationsMade, 1)
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

// isIndexerBusy reports whether the indexer is currently processing work.
// When no poller is configured, it returns false to avoid unintended adjustments.
func (ao *AdaptiveOptimizer) isIndexerBusy() bool {
	if ao.activityPoller == nil {
		return false
	}
	return ao.activityPoller.IsBusy()
}
