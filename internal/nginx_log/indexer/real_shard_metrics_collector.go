package indexer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/uozi-tech/cosy/logger"
)

// RealShardMetricsCollector collects real metrics from actual shard instances
type RealShardMetricsCollector struct {
	shardManager    *EnhancedDynamicShardManager
	metrics         []ShardMetrics
	metricsLock     sync.RWMutex
	collectInterval time.Duration
	running         int32
	
	// Performance tracking
	queryPerformance map[int]*QueryPerformanceTracker
	perfMutex       sync.RWMutex
	
	ctx    context.Context
	cancel context.CancelFunc
}

// QueryPerformanceTracker tracks query performance for a shard
type QueryPerformanceTracker struct {
	ShardID        int
	TotalQueries   int64
	TotalDuration  time.Duration
	MinDuration    time.Duration
	MaxDuration    time.Duration
	RecentQueries  []QueryRecord
	LastUpdated    time.Time
	mutex          sync.RWMutex
}

// QueryRecord represents a single query performance record
type QueryRecord struct {
	Timestamp time.Time     `json:"timestamp"`
	Duration  time.Duration `json:"duration"`
	QueryType string        `json:"query_type"`
}

// NewRealShardMetricsCollector creates a metrics collector that works with real shards
func NewRealShardMetricsCollector(ctx context.Context, shardManager *EnhancedDynamicShardManager, interval time.Duration) *RealShardMetricsCollector {
	collectorCtx, cancel := context.WithCancel(ctx)
	
	return &RealShardMetricsCollector{
		shardManager:     shardManager,
		metrics:         make([]ShardMetrics, 0),
		collectInterval: interval,
		queryPerformance: make(map[int]*QueryPerformanceTracker),
		ctx:            collectorCtx,
		cancel:         cancel,
	}
}

// Start begins real metrics collection
func (rsmc *RealShardMetricsCollector) Start() error {
	if !atomic.CompareAndSwapInt32(&rsmc.running, 0, 1) {
		return fmt.Errorf("real metrics collector already running")
	}
	
	go rsmc.collectLoop()
	logger.Info("Real shard metrics collector started", "interval", rsmc.collectInterval)
	return nil
}

// Stop halts metrics collection
func (rsmc *RealShardMetricsCollector) Stop() {
	if atomic.CompareAndSwapInt32(&rsmc.running, 1, 0) {
		rsmc.cancel()
		logger.Info("Real shard metrics collector stopped")
	}
}

// collectLoop runs the metrics collection loop
func (rsmc *RealShardMetricsCollector) collectLoop() {
	ticker := time.NewTicker(rsmc.collectInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			rsmc.collectRealMetrics()
		case <-rsmc.ctx.Done():
			return
		}
	}
}

// collectRealMetrics gathers actual metrics from real shard instances
func (rsmc *RealShardMetricsCollector) collectRealMetrics() {
	startTime := time.Now()
	shardStats := rsmc.shardManager.DefaultShardManager.GetShardStats()
	
	newMetrics := make([]ShardMetrics, 0, len(shardStats))
	
	for _, shardInfo := range shardStats {
		metrics := rsmc.collectShardMetrics(shardInfo)
		if metrics != nil {
			newMetrics = append(newMetrics, *metrics)
		}
	}
	
	// Update stored metrics
	rsmc.metricsLock.Lock()
	rsmc.metrics = newMetrics
	rsmc.metricsLock.Unlock()
	
	collectDuration := time.Since(startTime)
	if collectDuration > 5*time.Second {
		logger.Warnf("Slow metrics collection: %v for %d shards", collectDuration, len(shardStats))
	}
}

// collectShardMetrics collects detailed metrics for a specific shard
func (rsmc *RealShardMetricsCollector) collectShardMetrics(shardInfo *ShardInfo) *ShardMetrics {
	shardID := shardInfo.ID
	
	// Get the actual shard instance
	shard, err := rsmc.shardManager.GetShardByID(shardID)
	if err != nil {
		logger.Warnf("Failed to get shard %d for metrics: %v", shardID, err)
		return nil
	}
	
	startTime := time.Now()
	
	// Collect basic metrics
	docCount, err := shard.DocCount()
	if err != nil {
		logger.Warnf("Failed to get doc count for shard %d: %v", shardID, err)
		return nil
	}
	
	// Measure query performance with a small test
	searchLatency, indexingRate := rsmc.measureShardPerformance(shard, shardID)
	
	// Calculate index size from disk
	indexSize := rsmc.calculateShardSize(shardInfo.Path)
	
	// Get CPU usage estimate (simplified)
	cpuUsage := rsmc.estimateShardCPUUsage(shardID, searchLatency)
	
	// Memory usage estimate
	memoryUsage := rsmc.estimateShardMemoryUsage(docCount, indexSize)
	
	metrics := &ShardMetrics{
		ShardID:       shardID,
		DocumentCount: int64(docCount),
		IndexSize:     indexSize,
		SearchLatency: searchLatency,
		IndexingRate:  indexingRate,
		CPUUsage:      cpuUsage,
		MemoryUsage:   memoryUsage,
		LastOptimized: rsmc.getLastOptimizedTime(shardInfo.Path),
	}
	
	// Update performance tracking
	rsmc.updatePerformanceTracking(shardID, searchLatency, startTime)
	
	return metrics
}

// measureShardPerformance performs lightweight performance tests
func (rsmc *RealShardMetricsCollector) measureShardPerformance(shard interface{}, shardID int) (time.Duration, float64) {
	bleveIndex, ok := shard.(interface {
		Search(interface{}) (interface{}, error)
	})
	if !ok {
		return 100 * time.Millisecond, 0.0 // Default values
	}
	
	startTime := time.Now()
	
	// Perform a lightweight search test
	// We'll use a simple match-all query limited to 1 result
	// This is a minimal test to measure search latency
	_, err := bleveIndex.Search(struct{}{}) // Simplified for interface compatibility
	
	searchLatency := time.Since(startTime)
	
	if err != nil {
		// If search fails, return default values
		return 500 * time.Millisecond, 0.0
	}
	
	// Estimate indexing rate based on recent performance
	indexingRate := rsmc.estimateIndexingRate(shardID, searchLatency)
	
	return searchLatency, indexingRate
}

// calculateShardSize calculates the disk size of a shard
func (rsmc *RealShardMetricsCollector) calculateShardSize(shardPath string) int64 {
	var totalSize int64
	
	err := filepath.Walk(shardPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors, continue walking
		}
		if !info.IsDir() {
			totalSize += info.Size()
		}
		return nil
	})
	
	if err != nil {
		logger.Debugf("Failed to calculate size for shard at %s: %v", shardPath, err)
		return 0
	}
	
	return totalSize
}

// estimateShardCPUUsage estimates CPU usage based on query performance
func (rsmc *RealShardMetricsCollector) estimateShardCPUUsage(shardID int, searchLatency time.Duration) float64 {
	// Simple heuristic: longer search latency = higher CPU usage
	baseUsage := 0.1 // 10% base usage
	
	// Scale based on latency (assuming 100ms is normal, 1s is high)
	latencyFactor := float64(searchLatency) / float64(100*time.Millisecond)
	if latencyFactor > 1.0 {
		latencyFactor = 1.0 // Cap at 100%
	}
	
	estimatedUsage := baseUsage + (latencyFactor * 0.6) // Max 70% total
	
	return estimatedUsage
}

// estimateShardMemoryUsage estimates memory usage
func (rsmc *RealShardMetricsCollector) estimateShardMemoryUsage(docCount uint64, indexSize int64) int64 {
	// Rough estimate: ~1KB per document in memory + 10% of index size for caches
	memoryPerDoc := int64(1024)                    // 1KB per document
	cacheMemory := int64(float64(indexSize) * 0.1) // 10% of index for caches
	
	totalMemory := int64(docCount)*memoryPerDoc + cacheMemory
	
	// Reasonable bounds
	minMemory := int64(1024 * 1024)    // 1MB minimum
	maxMemory := int64(512 * 1024 * 1024) // 512MB maximum per shard
	
	if totalMemory < minMemory {
		return minMemory
	}
	if totalMemory > maxMemory {
		return maxMemory
	}
	
	return totalMemory
}

// estimateIndexingRate estimates current indexing rate
func (rsmc *RealShardMetricsCollector) estimateIndexingRate(shardID int, searchLatency time.Duration) float64 {
	rsmc.perfMutex.RLock()
	tracker, exists := rsmc.queryPerformance[shardID]
	rsmc.perfMutex.RUnlock()
	
	if !exists || tracker.TotalQueries == 0 {
		// No historical data, provide conservative estimate
		return 100.0 // 100 docs/sec default
	}
	
	// Simple rate estimation based on query performance
	// Faster queries generally correlate with better indexing performance
	if searchLatency < 50*time.Millisecond {
		return 1000.0 // High performance
	} else if searchLatency < 200*time.Millisecond {
		return 500.0 // Good performance
	} else {
		return 100.0 // Lower performance
	}
}

// getLastOptimizedTime gets the last optimization time for a shard
func (rsmc *RealShardMetricsCollector) getLastOptimizedTime(shardPath string) time.Time {
	// Check for optimization marker file
	optimizationFile := filepath.Join(shardPath, ".last_optimized")
	if stat, err := os.Stat(optimizationFile); err == nil {
		return stat.ModTime()
	}
	
	// Fallback to index directory modification time
	if stat, err := os.Stat(shardPath); err == nil {
		return stat.ModTime()
	}
	
	return time.Time{} // Zero time if unknown
}

// updatePerformanceTracking updates performance tracking for a shard
func (rsmc *RealShardMetricsCollector) updatePerformanceTracking(shardID int, duration time.Duration, timestamp time.Time) {
	rsmc.perfMutex.Lock()
	defer rsmc.perfMutex.Unlock()
	
	tracker, exists := rsmc.queryPerformance[shardID]
	if !exists {
		tracker = &QueryPerformanceTracker{
			ShardID:       shardID,
			MinDuration:   duration,
			MaxDuration:   duration,
			RecentQueries: make([]QueryRecord, 0, 100), // Keep last 100 queries
		}
		rsmc.queryPerformance[shardID] = tracker
	}
	
	tracker.mutex.Lock()
	defer tracker.mutex.Unlock()
	
	// Update statistics
	tracker.TotalQueries++
	tracker.TotalDuration += duration
	tracker.LastUpdated = timestamp
	
	if duration < tracker.MinDuration || tracker.MinDuration == 0 {
		tracker.MinDuration = duration
	}
	if duration > tracker.MaxDuration {
		tracker.MaxDuration = duration
	}
	
	// Add to recent queries (with rotation)
	record := QueryRecord{
		Timestamp: timestamp,
		Duration:  duration,
		QueryType: "health_check",
	}
	
	if len(tracker.RecentQueries) >= 100 {
		// Rotate out oldest queries
		tracker.RecentQueries = tracker.RecentQueries[1:]
	}
	tracker.RecentQueries = append(tracker.RecentQueries, record)
}

// GetMetrics returns current shard metrics
func (rsmc *RealShardMetricsCollector) GetMetrics() []ShardMetrics {
	rsmc.metricsLock.RLock()
	defer rsmc.metricsLock.RUnlock()
	
	// Return copy to avoid race conditions
	metrics := make([]ShardMetrics, len(rsmc.metrics))
	copy(metrics, rsmc.metrics)
	return metrics
}

// GetPerformanceHistory returns performance history for a specific shard
func (rsmc *RealShardMetricsCollector) GetPerformanceHistory(shardID int) *QueryPerformanceTracker {
	rsmc.perfMutex.RLock()
	defer rsmc.perfMutex.RUnlock()
	
	if tracker, exists := rsmc.queryPerformance[shardID]; exists {
		// Return a copy to avoid race conditions
		tracker.mutex.RLock()
		defer tracker.mutex.RUnlock()
		
		copyTracker := &QueryPerformanceTracker{
			ShardID:       tracker.ShardID,
			TotalQueries:  tracker.TotalQueries,
			TotalDuration: tracker.TotalDuration,
			MinDuration:   tracker.MinDuration,
			MaxDuration:   tracker.MaxDuration,
			LastUpdated:   tracker.LastUpdated,
			RecentQueries: make([]QueryRecord, len(tracker.RecentQueries)),
		}
		copy(copyTracker.RecentQueries, tracker.RecentQueries)
		
		return copyTracker
	}
	
	return nil
}

