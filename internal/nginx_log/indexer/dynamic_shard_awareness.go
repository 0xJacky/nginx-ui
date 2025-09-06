package indexer

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/uozi-tech/cosy/logger"
)

// DynamicShardAwareness provides automatic shard management detection and integration
type DynamicShardAwareness struct {
	config                *Config
	currentShardManager   interface{}  // Can be DefaultShardManager or EnhancedDynamicShardManager
	isDynamic             bool
	enhancedManager       *EnhancedDynamicShardManager
	
	// Monitoring and adaptation
	performanceMonitor    *PerformanceMonitor
	adaptationEnabled     bool
	lastAdaptation        time.Time
	adaptationCooldown    time.Duration
	
	mutex sync.RWMutex
}

// PerformanceMonitor tracks system performance for shard adaptation decisions
type PerformanceMonitor struct {
	samples             []PerformanceSample
	maxSamples          int
	currentThroughput   float64
	averageLatency      time.Duration
	lastOptimization    time.Time
	mutex              sync.RWMutex
}

// NewDynamicShardAwareness creates a new shard awareness system
func NewDynamicShardAwareness(config *Config) *DynamicShardAwareness {
	return &DynamicShardAwareness{
		config:             config,
		adaptationEnabled:  true,
		adaptationCooldown: 2 * time.Minute, // Conservative adaptation interval
		performanceMonitor: &PerformanceMonitor{
			samples:    make([]PerformanceSample, 0, 60), // Keep 60 samples (1 minute at 1s intervals)
			maxSamples: 60,
		},
	}
}

// DetectAndSetupShardManager automatically detects the optimal shard manager type
func (dsa *DynamicShardAwareness) DetectAndSetupShardManager() (interface{}, error) {
	dsa.mutex.Lock()
	defer dsa.mutex.Unlock()
	
	// Decision factors for dynamic vs static shard management
	factors := dsa.analyzeEnvironmentFactors()
	
	if dsa.shouldUseDynamicShards(factors) {
		logger.Info("Dynamic shard management detected as optimal", 
			"cpu_cores", factors.CPUCores,
			"memory_gb", factors.MemoryGB,
			"expected_load", factors.ExpectedLoad)
		
		// Create enhanced dynamic shard manager
		enhancedManager := NewEnhancedDynamicShardManager(dsa.config)
		dsa.enhancedManager = enhancedManager
		dsa.currentShardManager = enhancedManager
		dsa.isDynamic = true
		
		// Initialize the enhanced manager
		if err := enhancedManager.Initialize(); err != nil {
			logger.Warnf("Failed to initialize enhanced dynamic shard manager, falling back to static: %v", err)
			return dsa.setupStaticShardManager()
		}
		
		return enhancedManager, nil
	} else {
		logger.Info("Static shard management selected", 
			"cpu_cores", factors.CPUCores,
			"shard_count", dsa.config.ShardCount)
		
		return dsa.setupStaticShardManager()
	}
}

// EnvironmentFactors represents system environment analysis
type EnvironmentFactors struct {
	CPUCores        int     `json:"cpu_cores"`
	MemoryGB        float64 `json:"memory_gb"`
	ExpectedLoad    string  `json:"expected_load"`    // "low", "medium", "high", "variable"
	DataVolume      string  `json:"data_volume"`      // "small", "medium", "large", "growing"
	QueryPatterns   string  `json:"query_patterns"`   // "simple", "complex", "mixed"
	AvailableSpace  int64   `json:"available_space"`  // Available disk space in bytes
}

// analyzeEnvironmentFactors analyzes the current environment
func (dsa *DynamicShardAwareness) analyzeEnvironmentFactors() EnvironmentFactors {
	factors := EnvironmentFactors{
		CPUCores: runtime.GOMAXPROCS(0), // Use GOMAXPROCS for container-aware processor count
	}
	
	// Get memory info (simplified)
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	factors.MemoryGB = float64(m.Sys) / (1024 * 1024 * 1024)
	
	// Analyze expected load based on configuration
	factors.ExpectedLoad = dsa.analyzeExpectedLoad()
	factors.DataVolume = dsa.analyzeDataVolume()
	factors.QueryPatterns = dsa.analyzeQueryPatterns()
	
	// Check available disk space
	if stat, err := os.Stat(dsa.config.IndexPath); err == nil && stat.IsDir() {
		// Simple approximation - in production, use syscall for actual free space
		factors.AvailableSpace = 10 * 1024 * 1024 * 1024 // 10GB default assumption
	}
	
	return factors
}

// shouldUseDynamicShards determines if dynamic shard management is beneficial
func (dsa *DynamicShardAwareness) shouldUseDynamicShards(factors EnvironmentFactors) bool {
	// Dynamic shards are beneficial when:
	
	// 1. High-core systems (8+ cores) can benefit from dynamic scaling
	if factors.CPUCores >= 8 {
		return true
	}
	
	// 2. Variable or high expected load
	if factors.ExpectedLoad == "high" || factors.ExpectedLoad == "variable" {
		return true
	}
	
	// 3. Large or growing data volumes
	if factors.DataVolume == "large" || factors.DataVolume == "growing" {
		return true
	}
	
	// 4. Systems with significant memory (4GB+) can handle dynamic overhead
	if factors.MemoryGB >= 4.0 {
		return true
	}
	
	// 5. Complex or mixed query patterns benefit from dynamic optimization
	if factors.QueryPatterns == "complex" || factors.QueryPatterns == "mixed" {
		return true
	}
	
	// Default to static for simpler environments
	return false
}

// setupStaticShardManager creates a static shard manager
func (dsa *DynamicShardAwareness) setupStaticShardManager() (interface{}, error) {
	staticManager := NewDefaultShardManager(dsa.config)
	dsa.currentShardManager = staticManager
	dsa.isDynamic = false
	
	if err := staticManager.Initialize(); err != nil {
		return nil, fmt.Errorf("failed to initialize static shard manager: %w", err)
	}
	
	return staticManager, nil
}

// analyzeExpectedLoad analyzes expected system load
func (dsa *DynamicShardAwareness) analyzeExpectedLoad() string {
	// Based on worker count and batch size configuration
	workerCount := dsa.config.WorkerCount
	batchSize := dsa.config.BatchSize
	
	// High configuration suggests high load expectations
	if workerCount >= 16 || batchSize >= 2000 {
		return "high"
	}
	
	// Variable load if workers are significantly higher than available processors
	if workerCount > runtime.GOMAXPROCS(0)*2 {
		return "variable"
	}
	
	// Medium configuration
	if workerCount >= 8 || batchSize >= 1000 {
		return "medium"
	}
	
	return "low"
}

// analyzeDataVolume analyzes expected data volume
func (dsa *DynamicShardAwareness) analyzeDataVolume() string {
	// Based on shard count and memory quota
	shardCount := dsa.config.ShardCount
	memoryQuota := dsa.config.MemoryQuota
	
	// Large configuration suggests large data volumes
	if shardCount >= 8 || memoryQuota >= 2*1024*1024*1024 { // 2GB+
		return "large"
	}
	
	// Growing if shard count is configured higher than default
	if shardCount > 4 {
		return "growing"
	}
	
	// Medium configuration
	if shardCount >= 4 || memoryQuota >= 1024*1024*1024 { // 1GB+
		return "medium"
	}
	
	return "small"
}

// analyzeQueryPatterns analyzes expected query complexity
func (dsa *DynamicShardAwareness) analyzeQueryPatterns() string {
	// Based on optimization interval and metrics enablement
	if dsa.config.OptimizeInterval <= 10*time.Minute {
		return "complex" // Frequent optimization suggests complex queries
	}
	
	if dsa.config.EnableMetrics {
		return "mixed" // Metrics collection suggests varied query patterns
	}
	
	return "simple"
}

// StartMonitoring begins performance monitoring for adaptation decisions
func (dsa *DynamicShardAwareness) StartMonitoring(ctx context.Context) {
	if !dsa.adaptationEnabled {
		return
	}
	
	go dsa.monitoringLoop(ctx)
}

// monitoringLoop runs continuous performance monitoring
func (dsa *DynamicShardAwareness) monitoringLoop(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Second) // Sample every second
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			dsa.collectPerformanceSample()
			
			// Check if adaptation is needed every 30 samples (30 seconds)
			if len(dsa.performanceMonitor.samples) > 0 && len(dsa.performanceMonitor.samples)%30 == 0 {
				dsa.considerAdaptation()
			}
			
		case <-ctx.Done():
			return
		}
	}
}

// collectPerformanceSample collects current performance data
func (dsa *DynamicShardAwareness) collectPerformanceSample() {
	dsa.performanceMonitor.mutex.Lock()
	defer dsa.performanceMonitor.mutex.Unlock()
	
	sample := PerformanceSample{
		Timestamp:    time.Now(),
		Throughput:   dsa.getCurrentThroughput(),
		Latency:      dsa.getCurrentLatency(),
		CPUUsage:     dsa.getCurrentCPUUsage(),
		WorkerCount:  dsa.config.WorkerCount,
	}
	
	// Add sample
	dsa.performanceMonitor.samples = append(dsa.performanceMonitor.samples, sample)
	
	// Rotate samples if we exceed max
	if len(dsa.performanceMonitor.samples) > dsa.performanceMonitor.maxSamples {
		dsa.performanceMonitor.samples = dsa.performanceMonitor.samples[1:]
	}
	
	// Update current metrics
	dsa.updateCurrentMetrics()
}

// considerAdaptation evaluates whether dynamic adaptations should be made
func (dsa *DynamicShardAwareness) considerAdaptation() {
	// Check cooldown
	if time.Since(dsa.lastAdaptation) < dsa.adaptationCooldown {
		return
	}
	
	dsa.mutex.RLock()
	isDynamic := dsa.isDynamic
	enhancedManager := dsa.enhancedManager
	dsa.mutex.RUnlock()
	
	if !isDynamic || enhancedManager == nil {
		return // Only adapt if using dynamic shard manager
	}
	
	// Get performance analysis
	analysis := dsa.analyzeCurrentPerformance()
	
	if analysis.ShouldAdapt {
		logger.Info("Performance analysis suggests adaptation", 
			"reason", analysis.Reason,
			"confidence", analysis.Confidence)
		
		// Trigger auto-scaling on the enhanced manager
		if err := enhancedManager.AutoScale(); err != nil {
			logger.Warnf("Auto-scaling adaptation failed: %v", err)
		} else {
			dsa.lastAdaptation = time.Now()
		}
	}
}

// PerformanceAnalysis represents performance analysis results
type PerformanceAnalysis struct {
	ShouldAdapt bool    `json:"should_adapt"`
	Reason      string  `json:"reason"`
	Confidence  float64 `json:"confidence"`
	
	CurrentThroughput float64       `json:"current_throughput"`
	AverageLatency    time.Duration `json:"average_latency"`
	TrendAnalysis     string        `json:"trend_analysis"`
}

// analyzeCurrentPerformance analyzes current performance trends
func (dsa *DynamicShardAwareness) analyzeCurrentPerformance() PerformanceAnalysis {
	dsa.performanceMonitor.mutex.RLock()
	defer dsa.performanceMonitor.mutex.RUnlock()
	
	samples := dsa.performanceMonitor.samples
	if len(samples) < 30 { // Need at least 30 samples for analysis
		return PerformanceAnalysis{
			ShouldAdapt: false,
			Reason:      "Insufficient performance data",
			Confidence:  0.0,
		}
	}
	
	// Analyze recent vs historical performance
	recentSamples := samples[len(samples)-10:] // Last 10 samples
	historicalSamples := samples[:len(samples)-10]
	
	recentAvgThroughput := dsa.calculateAverageThroughput(recentSamples)
	historicalAvgThroughput := dsa.calculateAverageThroughput(historicalSamples)
	
	recentAvgLatency := dsa.calculateAverageLatency(recentSamples)
	historicalAvgLatency := dsa.calculateAverageLatency(historicalSamples)
	
	// Check for performance degradation
	throughputDrop := (historicalAvgThroughput - recentAvgThroughput) / historicalAvgThroughput
	latencyIncrease := float64(recentAvgLatency - historicalAvgLatency) / float64(historicalAvgLatency)
	
	// Adaptation triggers
	if throughputDrop > 0.20 { // 20% throughput drop
		return PerformanceAnalysis{
			ShouldAdapt:       true,
			Reason:            fmt.Sprintf("Throughput dropped by %.1f%%", throughputDrop*100),
			Confidence:        0.8,
			CurrentThroughput: recentAvgThroughput,
			AverageLatency:    recentAvgLatency,
			TrendAnalysis:     "degrading",
		}
	}
	
	if latencyIncrease > 0.50 { // 50% latency increase
		return PerformanceAnalysis{
			ShouldAdapt:       true,
			Reason:            fmt.Sprintf("Latency increased by %.1f%%", latencyIncrease*100),
			Confidence:        0.7,
			CurrentThroughput: recentAvgThroughput,
			AverageLatency:    recentAvgLatency,
			TrendAnalysis:     "degrading",
		}
	}
	
	return PerformanceAnalysis{
		ShouldAdapt:       false,
		Reason:            "Performance stable",
		Confidence:        0.6,
		CurrentThroughput: recentAvgThroughput,
		AverageLatency:    recentAvgLatency,
		TrendAnalysis:     "stable",
	}
}

// Helper methods for performance calculation
func (dsa *DynamicShardAwareness) calculateAverageThroughput(samples []PerformanceSample) float64 {
	if len(samples) == 0 {
		return 0.0
	}
	
	total := 0.0
	for _, sample := range samples {
		total += sample.Throughput
	}
	
	return total / float64(len(samples))
}

func (dsa *DynamicShardAwareness) calculateAverageLatency(samples []PerformanceSample) time.Duration {
	if len(samples) == 0 {
		return 0
	}
	
	var total time.Duration
	for _, sample := range samples {
		total += sample.Latency
	}
	
	return total / time.Duration(len(samples))
}

// getCurrentThroughput gets current system throughput (placeholder)
func (dsa *DynamicShardAwareness) getCurrentThroughput() float64 {
	// TODO: Integration with actual indexer metrics
	return 1000.0 // Default placeholder
}

// getCurrentLatency gets current system latency (placeholder) 
func (dsa *DynamicShardAwareness) getCurrentLatency() time.Duration {
	// TODO: Integration with actual indexer metrics
	return 100 * time.Millisecond // Default placeholder
}

// getCurrentCPUUsage gets current CPU usage (placeholder)
func (dsa *DynamicShardAwareness) getCurrentCPUUsage() float64 {
	// TODO: Integration with actual system metrics
	return 0.5 // Default placeholder
}

// updateCurrentMetrics updates current performance metrics
func (dsa *DynamicShardAwareness) updateCurrentMetrics() {
	samplesLen := len(dsa.performanceMonitor.samples)
	if samplesLen == 0 {
		return
	}
	
	// Get recent samples with bounds checking
	recentCount := 10
	if samplesLen < recentCount {
		recentCount = samplesLen
	}
	
	recent := dsa.performanceMonitor.samples[samplesLen-recentCount:]
	dsa.performanceMonitor.currentThroughput = dsa.calculateAverageThroughput(recent)
	dsa.performanceMonitor.averageLatency = dsa.calculateAverageLatency(recent)
}

// GetCurrentShardManager returns the current shard manager
func (dsa *DynamicShardAwareness) GetCurrentShardManager() interface{} {
	dsa.mutex.RLock()
	defer dsa.mutex.RUnlock()
	return dsa.currentShardManager
}

// IsDynamic returns whether dynamic shard management is active
func (dsa *DynamicShardAwareness) IsDynamic() bool {
	dsa.mutex.RLock()
	defer dsa.mutex.RUnlock()
	return dsa.isDynamic
}

// GetPerformanceAnalysis returns current performance analysis
func (dsa *DynamicShardAwareness) GetPerformanceAnalysis() PerformanceAnalysis {
	return dsa.analyzeCurrentPerformance()
}

// SetAdaptationEnabled enables or disables automatic adaptation
func (dsa *DynamicShardAwareness) SetAdaptationEnabled(enabled bool) {
	dsa.adaptationEnabled = enabled
	logger.Info("Dynamic shard adaptation setting changed", "enabled", enabled)
}