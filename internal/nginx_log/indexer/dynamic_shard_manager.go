package indexer

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/uozi-tech/cosy/logger"
)

// DynamicShardManager extends ShardManager with dynamic scaling capabilities
type DynamicShardManager interface {
	// Basic ShardManager interface (would be defined elsewhere)
	Initialize() error
	Close() error
	GetShardCount() int
	GetShard(shardID int) (interface{}, error) // Returns actual shard implementation
	
	// Dynamic scaling methods
	ScaleShards(targetCount int) error
	AutoScale(metrics LoadMetrics) error
	RebalanceShards() error
	GetShardMetrics() []ShardMetrics
	
	// Configuration
	SetAutoScaleEnabled(enabled bool)
	IsAutoScaleEnabled() bool
}

// LoadMetrics represents system load metrics for scaling decisions
type LoadMetrics struct {
	IndexingThroughput float64       `json:"indexing_throughput"` // docs/sec
	SearchLatency      time.Duration `json:"search_latency"`
	CPUUtilization     float64       `json:"cpu_utilization"`
	MemoryUsage        float64       `json:"memory_usage"`
	ShardSizes         []int64       `json:"shard_sizes"`
	ActiveQueries      int           `json:"active_queries"`
	QueueLength        int           `json:"queue_length"`
}

// ShardMetrics represents metrics for a single shard
type ShardMetrics struct {
	ShardID         int           `json:"shard_id"`
	DocumentCount   int64         `json:"document_count"`
	IndexSize       int64         `json:"index_size"`
	SearchLatency   time.Duration `json:"search_latency"`
	IndexingRate    float64       `json:"indexing_rate"`
	CPUUsage        float64       `json:"cpu_usage"`
	MemoryUsage     int64         `json:"memory_usage"`
	LastOptimized   time.Time     `json:"last_optimized"`
}

// DefaultDynamicShardManager implements DynamicShardManager
type DefaultDynamicShardManager struct {
	config            *Config
	currentShardCount int32
	shards            map[int]interface{} // Abstract shard storage
	shardsLock        sync.RWMutex
	
	// Auto-scaling
	autoScaleEnabled  bool
	scalingInProgress int32
	lastScaleTime     time.Time
	scalingCooldown   time.Duration
	
	// Monitoring
	metricsCollector  *ShardMetricsCollector
	loadThresholds    *ScalingThresholds
	
	// Context and control
	ctx      context.Context
	cancel   context.CancelFunc
	stopOnce sync.Once
}

// ScalingThresholds defines when to scale up or down
type ScalingThresholds struct {
	// Scale up thresholds
	MaxSearchLatency     time.Duration `json:"max_search_latency"`
	MaxCPUUtilization   float64       `json:"max_cpu_utilization"`
	MaxMemoryUsage      float64       `json:"max_memory_usage"`
	MaxDocsPerShard     int64         `json:"max_docs_per_shard"`
	MaxShardSize        int64         `json:"max_shard_size"`
	
	// Scale down thresholds
	MinSearchLatency     time.Duration `json:"min_search_latency"`
	MinCPUUtilization   float64       `json:"min_cpu_utilization"`
	MinDocsPerShard     int64         `json:"min_docs_per_shard"`
	MinShardSize        int64         `json:"min_shard_size"`
	
	// Constraints
	MinShards           int           `json:"min_shards"`
	MaxShards           int           `json:"max_shards"`
	ScalingCooldown     time.Duration `json:"scaling_cooldown"`
}

// ShardMetricsCollector collects and aggregates shard performance metrics
type ShardMetricsCollector struct {
	realCollector   *RealShardMetricsCollector  // Real metrics collector
	metrics         []ShardMetrics
	metricsLock     sync.RWMutex
	collectInterval time.Duration
	running         int32
	
	ctx    context.Context
	cancel context.CancelFunc
}

// NewDynamicShardManager creates a new dynamic shard manager
func NewDynamicShardManager(config *Config) *DefaultDynamicShardManager {
	ctx, cancel := context.WithCancel(context.Background())
	
	dsm := &DefaultDynamicShardManager{
		config:            config,
		currentShardCount: int32(config.ShardCount),
		shards:           make(map[int]interface{}),
		autoScaleEnabled:  true,
		scalingCooldown:   5 * time.Minute, // Prevent rapid scaling
		ctx:              ctx,
		cancel:           cancel,
		
		loadThresholds: &ScalingThresholds{
			MaxSearchLatency:  5 * time.Second,
			MaxCPUUtilization: 0.85,
			MaxMemoryUsage:   0.80,
			MaxDocsPerShard:  10000000, // 10M docs per shard
			MaxShardSize:     100 * 1024 * 1024 * 1024, // 100GB per shard
			
			MinSearchLatency:  1 * time.Second,
			MinCPUUtilization: 0.30,
			MinDocsPerShard:  1000000, // 1M docs minimum
			MinShardSize:     10 * 1024 * 1024 * 1024, // 10GB minimum
			
			MinShards:       2,
			MaxShards:       max(32, config.WorkerCount*2), // Scale with workers
			ScalingCooldown: 5 * time.Minute,
		},
	}
	
	// Initialize metrics collector
	dsm.metricsCollector = NewShardMetricsCollector(ctx, 30*time.Second)
	
	return dsm
}

// Initialize starts the dynamic shard manager
func (dsm *DefaultDynamicShardManager) Initialize() error {
	// Initialize base shards
	for i := 0; i < int(atomic.LoadInt32(&dsm.currentShardCount)); i++ {
		dsm.shards[i] = fmt.Sprintf("shard-%d", i) // Placeholder
	}
	
	// Start metrics collection
	if err := dsm.metricsCollector.Start(); err != nil {
		return fmt.Errorf("failed to start metrics collector: %w", err)
	}
	
	// Start auto-scaling monitor if enabled
	if dsm.autoScaleEnabled {
		go dsm.autoScaleMonitor()
	}
	
	logger.Info("Dynamic shard manager initialized", 
		"initial_shards", dsm.currentShardCount,
		"max_shards", dsm.loadThresholds.MaxShards)
	
	return nil
}

// Close shuts down the dynamic shard manager
func (dsm *DefaultDynamicShardManager) Close() error {
	var closeErr error
	dsm.stopOnce.Do(func() {
		dsm.cancel()
		
		// Stop metrics collection
		if dsm.metricsCollector != nil {
			dsm.metricsCollector.Stop()
		}
		
		// Close all shards
		dsm.shardsLock.Lock()
		defer dsm.shardsLock.Unlock()
		
		for id := range dsm.shards {
			// Close shard implementation
			delete(dsm.shards, id)
		}
	})
	
	return closeErr
}

// GetShardCount returns current number of shards
func (dsm *DefaultDynamicShardManager) GetShardCount() int {
	return int(atomic.LoadInt32(&dsm.currentShardCount))
}

// GetShard returns a specific shard
func (dsm *DefaultDynamicShardManager) GetShard(shardID int) (interface{}, error) {
	dsm.shardsLock.RLock()
	defer dsm.shardsLock.RUnlock()
	
	shard, exists := dsm.shards[shardID]
	if !exists {
		return nil, fmt.Errorf("shard %d does not exist", shardID)
	}
	
	return shard, nil
}

// ScaleShards scales to target shard count
func (dsm *DefaultDynamicShardManager) ScaleShards(targetCount int) error {
	if !atomic.CompareAndSwapInt32(&dsm.scalingInProgress, 0, 1) {
		return fmt.Errorf("scaling operation already in progress")
	}
	defer atomic.StoreInt32(&dsm.scalingInProgress, 0)
	
	currentCount := int(atomic.LoadInt32(&dsm.currentShardCount))
	
	// Validate target count
	if targetCount < dsm.loadThresholds.MinShards {
		targetCount = dsm.loadThresholds.MinShards
	}
	if targetCount > dsm.loadThresholds.MaxShards {
		targetCount = dsm.loadThresholds.MaxShards
	}
	
	if targetCount == currentCount {
		return nil // No change needed
	}
	
	logger.Info("Scaling shards", 
		"current", currentCount, 
		"target", targetCount)
	
	dsm.shardsLock.Lock()
	defer dsm.shardsLock.Unlock()
	
	if targetCount > currentCount {
		// Scale up - add new shards
		for i := currentCount; i < targetCount; i++ {
			dsm.shards[i] = fmt.Sprintf("shard-%d", i) // Create new shard
			logger.Debug("Created new shard", "shard_id", i)
		}
	} else {
		// Scale down - remove shards (would need data migration)
		for i := currentCount - 1; i >= targetCount; i-- {
			// TODO: Implement data migration before removal
			delete(dsm.shards, i)
			logger.Debug("Removed shard", "shard_id", i)
		}
	}
	
	atomic.StoreInt32(&dsm.currentShardCount, int32(targetCount))
	dsm.lastScaleTime = time.Now()
	
	logger.Info("Shard scaling completed", 
		"new_count", targetCount,
		"operation", map[bool]string{true: "scale_up", false: "scale_down"}[targetCount > currentCount])
	
	return nil
}

// AutoScale performs automatic scaling based on load metrics
func (dsm *DefaultDynamicShardManager) AutoScale(metrics LoadMetrics) error {
	if !dsm.autoScaleEnabled {
		return nil
	}
	
	// Check cooldown period
	if time.Since(dsm.lastScaleTime) < dsm.scalingCooldown {
		return nil
	}
	
	currentShards := dsm.GetShardCount()
	decision := dsm.makeScalingDecision(metrics, currentShards)
	
	if decision.Action != "none" {
		logger.Info("Auto-scaling decision", 
			"action", decision.Action,
			"current_shards", currentShards,
			"target_shards", decision.TargetShards,
			"reason", decision.Reason)
		
		return dsm.ScaleShards(decision.TargetShards)
	}
	
	return nil
}

// ScalingDecision represents a scaling decision
type ScalingDecision struct {
	Action       string `json:"action"`        // "scale_up", "scale_down", "none"
	TargetShards int    `json:"target_shards"`
	Reason       string `json:"reason"`
	Confidence   float64 `json:"confidence"`   // 0.0-1.0
}

// makeScalingDecision analyzes metrics and decides on scaling
func (dsm *DefaultDynamicShardManager) makeScalingDecision(metrics LoadMetrics, currentShards int) ScalingDecision {
	thresholds := dsm.loadThresholds
	
	// Check scale-up conditions
	if metrics.SearchLatency > thresholds.MaxSearchLatency {
		return ScalingDecision{
			Action:       "scale_up",
			TargetShards: min(currentShards+2, thresholds.MaxShards),
			Reason:       fmt.Sprintf("High search latency: %v > %v", metrics.SearchLatency, thresholds.MaxSearchLatency),
			Confidence:   0.9,
		}
	}
	
	if metrics.CPUUtilization > thresholds.MaxCPUUtilization {
		return ScalingDecision{
			Action:       "scale_up",
			TargetShards: min(currentShards+1, thresholds.MaxShards),
			Reason:       fmt.Sprintf("High CPU utilization: %.2f > %.2f", metrics.CPUUtilization, thresholds.MaxCPUUtilization),
			Confidence:   0.8,
		}
	}
	
	// Check if any shard is too large
	maxShardSize := int64(0)
	for _, size := range metrics.ShardSizes {
		if size > maxShardSize {
			maxShardSize = size
		}
	}
	
	if maxShardSize > thresholds.MaxShardSize {
		return ScalingDecision{
			Action:       "scale_up",
			TargetShards: min(currentShards+1, thresholds.MaxShards),
			Reason:       fmt.Sprintf("Large shard detected: %d bytes > %d bytes", maxShardSize, thresholds.MaxShardSize),
			Confidence:   0.7,
		}
	}
	
	// Check scale-down conditions (more conservative)
	if currentShards > thresholds.MinShards &&
		metrics.SearchLatency < thresholds.MinSearchLatency &&
		metrics.CPUUtilization < thresholds.MinCPUUtilization {
		
		// Check if all shards are underutilized
		allShardsSmall := true
		for _, size := range metrics.ShardSizes {
			if size > thresholds.MinShardSize {
				allShardsSmall = false
				break
			}
		}
		
		if allShardsSmall {
			return ScalingDecision{
				Action:       "scale_down",
				TargetShards: max(currentShards-1, thresholds.MinShards),
				Reason:       "All shards underutilized",
				Confidence:   0.6,
			}
		}
	}
	
	return ScalingDecision{
		Action:       "none",
		TargetShards: currentShards,
		Reason:       "Current configuration optimal",
		Confidence:   0.5,
	}
}

// autoScaleMonitor runs the auto-scaling monitoring loop
func (dsm *DefaultDynamicShardManager) autoScaleMonitor() {
	ticker := time.NewTicker(60 * time.Second) // Check every minute
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			metrics := dsm.collectLoadMetrics()
			if err := dsm.AutoScale(metrics); err != nil {
				logger.Warnf("Auto-scaling failed: %v", err)
			}
		case <-dsm.ctx.Done():
			return
		}
	}
}

// collectLoadMetrics gathers current system metrics
func (dsm *DefaultDynamicShardManager) collectLoadMetrics() LoadMetrics {
	shardMetrics := dsm.GetShardMetrics()
	shardSizes := make([]int64, len(shardMetrics))
	
	var totalLatency time.Duration
	var totalCPU float64
	
	for i, shard := range shardMetrics {
		shardSizes[i] = shard.IndexSize
		totalLatency += shard.SearchLatency
		totalCPU += shard.CPUUsage
	}
	
	avgLatency := time.Duration(0)
	avgCPU := 0.0
	if len(shardMetrics) > 0 {
		avgLatency = totalLatency / time.Duration(len(shardMetrics))
		avgCPU = totalCPU / float64(len(shardMetrics))
	}
	
	return LoadMetrics{
		IndexingThroughput: dsm.getIndexingThroughput(),
		SearchLatency:      avgLatency,
		CPUUtilization:     avgCPU,
		MemoryUsage:        dsm.getMemoryUsage(),
		ShardSizes:         shardSizes,
		ActiveQueries:      dsm.getActiveQueries(),
		QueueLength:        dsm.getQueueLength(),
	}
}

// RebalanceShards redistributes data across shards for optimal performance
func (dsm *DefaultDynamicShardManager) RebalanceShards() error {
	// This would implement sophisticated data rebalancing
	logger.Info("Shard rebalancing initiated")
	
	// TODO: Implement actual rebalancing logic
	// 1. Analyze current data distribution
	// 2. Calculate optimal distribution
	// 3. Create migration plan
	// 4. Execute migration with minimal downtime
	
	return nil
}

// GetShardMetrics returns current metrics for all shards
func (dsm *DefaultDynamicShardManager) GetShardMetrics() []ShardMetrics {
	if dsm.metricsCollector != nil {
		return dsm.metricsCollector.GetMetrics()
	}
	return []ShardMetrics{}
}

// SetAutoScaleEnabled enables or disables auto-scaling
func (dsm *DefaultDynamicShardManager) SetAutoScaleEnabled(enabled bool) {
	dsm.autoScaleEnabled = enabled
	logger.Info("Auto-scaling setting changed", "enabled", enabled)
}

// IsAutoScaleEnabled returns current auto-scaling status
func (dsm *DefaultDynamicShardManager) IsAutoScaleEnabled() bool {
	return dsm.autoScaleEnabled
}

// Helper methods for metrics collection
func (dsm *DefaultDynamicShardManager) getIndexingThroughput() float64 {
	// TODO: Get actual throughput from indexer
	return 1000.0 // Placeholder
}

func (dsm *DefaultDynamicShardManager) getMemoryUsage() float64 {
	// TODO: Get actual memory usage
	return 0.5 // Placeholder
}

func (dsm *DefaultDynamicShardManager) getActiveQueries() int {
	// TODO: Get actual active query count
	return 0 // Placeholder
}

func (dsm *DefaultDynamicShardManager) getQueueLength() int {
	// TODO: Get actual queue length
	return 0 // Placeholder
}

// NewShardMetricsCollector creates a new metrics collector
func NewShardMetricsCollector(ctx context.Context, interval time.Duration) *ShardMetricsCollector {
	collectorCtx, cancel := context.WithCancel(ctx)
	
	return &ShardMetricsCollector{
		metrics:         make([]ShardMetrics, 0),
		collectInterval: interval,
		ctx:            collectorCtx,
		cancel:         cancel,
	}
}

// Start begins metrics collection
func (smc *ShardMetricsCollector) Start() error {
	if smc.realCollector != nil {
		return smc.realCollector.Start()
	}
	
	if !atomic.CompareAndSwapInt32(&smc.running, 0, 1) {
		return fmt.Errorf("metrics collector already running")
	}
	
	go smc.collectLoop()
	return nil
}

// Stop halts metrics collection
func (smc *ShardMetricsCollector) Stop() {
	if smc.realCollector != nil {
		smc.realCollector.Stop()
		return
	}
	
	if atomic.CompareAndSwapInt32(&smc.running, 1, 0) {
		smc.cancel()
	}
}

// collectLoop runs the metrics collection loop
func (smc *ShardMetricsCollector) collectLoop() {
	ticker := time.NewTicker(smc.collectInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			smc.collectMetrics()
		case <-smc.ctx.Done():
			return
		}
	}
}

// collectMetrics gathers current shard metrics
func (smc *ShardMetricsCollector) collectMetrics() {
	// TODO: Implement actual metrics collection from shards
	smc.metricsLock.Lock()
	defer smc.metricsLock.Unlock()
	
	// Placeholder metrics
	smc.metrics = []ShardMetrics{
		{
			ShardID:       0,
			DocumentCount: 1000000,
			IndexSize:     1024 * 1024 * 1024, // 1GB
			SearchLatency: 100 * time.Millisecond,
			IndexingRate:  500.0,
			CPUUsage:      0.4,
			MemoryUsage:   512 * 1024 * 1024, // 512MB
			LastOptimized: time.Now().Add(-1 * time.Hour),
		},
	}
}

// GetMetrics returns current shard metrics
func (smc *ShardMetricsCollector) GetMetrics() []ShardMetrics {
	if smc.realCollector != nil {
		return smc.realCollector.GetMetrics()
	}
	
	smc.metricsLock.RLock()
	defer smc.metricsLock.RUnlock()
	
	// Return copy to avoid race conditions
	metrics := make([]ShardMetrics, len(smc.metrics))
	copy(metrics, smc.metrics)
	return metrics
}