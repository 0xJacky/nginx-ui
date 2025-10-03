package indexer

import (
	"context"
	"fmt"
	"time"

	"github.com/uozi-tech/cosy/logger"
)

// ThroughputOptimizer provides high-level APIs for optimized log indexing
type ThroughputOptimizer struct {
	indexer *ParallelIndexer
	config  *ThroughputOptimizerConfig
}

// ThroughputOptimizerConfig configures the throughput optimizer
type ThroughputOptimizerConfig struct {
	UseRotationScanning bool          `json:"use_rotation_scanning"`
	MaxBatchSize        int           `json:"max_batch_size"`
	TimeoutPerGroup     time.Duration `json:"timeout_per_group"`
	EnableProgressTracking bool       `json:"enable_progress_tracking"`
}

// NewThroughputOptimizer creates a new throughput optimizer
func NewThroughputOptimizer(indexer *ParallelIndexer, config *ThroughputOptimizerConfig) *ThroughputOptimizer {
	if config == nil {
		config = DefaultThroughputOptimizerConfig()
	}

	return &ThroughputOptimizer{
		indexer: indexer,
		config:  config,
	}
}

// DefaultThroughputOptimizerConfig returns default configuration
func DefaultThroughputOptimizerConfig() *ThroughputOptimizerConfig {
	return &ThroughputOptimizerConfig{
		UseRotationScanning:    true,
		MaxBatchSize:           25000,
		TimeoutPerGroup:        10 * time.Minute,
		EnableProgressTracking: true,
	}
}

// IndexMultipleLogGroups indexes multiple log groups using the best strategy
func (to *ThroughputOptimizer) IndexMultipleLogGroups(ctx context.Context, basePaths []string) (*OptimizedIndexingResult, error) {
	start := time.Now()
	
	logger.Infof("🚀 Starting optimized indexing for %d log groups", len(basePaths))
	
	var progressConfig *ProgressConfig
	if to.config.EnableProgressTracking {
		progressConfig = &ProgressConfig{
			NotifyInterval: 5 * time.Second,
		}
	}

	var docsCountMap map[string]uint64
	var minTime, maxTime *time.Time
	var err error

	if to.config.UseRotationScanning && len(basePaths) > 1 {
		// Use rotation scanning for multiple log groups
		logger.Info("📊 Using rotation scanning strategy for optimal throughput")
		docsCountMap, minTime, maxTime, err = to.indexer.IndexLogGroupWithRotationScanning(basePaths, progressConfig)
	} else {
		// Fall back to traditional method for single group or if scanning disabled
		logger.Info("📁 Using traditional indexing strategy")
		docsCountMap = make(map[string]uint64)
		
		for _, basePath := range basePaths {
			groupDocs, groupMin, groupMax, groupErr := to.indexer.IndexLogGroup(basePath)
			if groupErr != nil {
				logger.Warnf("Failed to index log group %s: %v", basePath, groupErr)
				continue
			}
			
			// Merge results
			for path, count := range groupDocs {
				docsCountMap[path] = count
			}
			
			// Update time range
			if groupMin != nil && (minTime == nil || groupMin.Before(*minTime)) {
				minTime = groupMin
			}
			if groupMax != nil && (maxTime == nil || groupMax.After(*maxTime)) {
				maxTime = groupMax
			}
		}
	}

	if err != nil {
		return nil, fmt.Errorf("failed to index log groups: %w", err)
	}

	// Calculate results
	totalFiles := len(docsCountMap)
	totalDocuments := sumDocCounts(docsCountMap)
	duration := time.Since(start)
	
	result := &OptimizedIndexingResult{
		TotalFiles:         totalFiles,
		TotalDocuments:     totalDocuments,
		ProcessingDuration: duration,
		MinTimestamp:       minTime,
		MaxTimestamp:       maxTime,
		FileCounts:         docsCountMap,
		ThroughputDocsPerSec: float64(totalDocuments) / duration.Seconds(),
		Strategy:           getStrategyName(to.config.UseRotationScanning, len(basePaths)),
	}

	logger.Infof("🎉 Optimized indexing completed: %d files, %d documents in %v (%.0f docs/sec)", 
		totalFiles, totalDocuments, duration, result.ThroughputDocsPerSec)

	return result, nil
}

// OptimizedIndexingResult contains the results of optimized indexing
type OptimizedIndexingResult struct {
	TotalFiles           int                    `json:"total_files"`
	TotalDocuments       uint64                 `json:"total_documents"`
	ProcessingDuration   time.Duration          `json:"processing_duration"`
	MinTimestamp         *time.Time             `json:"min_timestamp,omitempty"`
	MaxTimestamp         *time.Time             `json:"max_timestamp,omitempty"`
	FileCounts           map[string]uint64      `json:"file_counts"`
	ThroughputDocsPerSec float64                `json:"throughput_docs_per_sec"`
	Strategy             string                 `json:"strategy"`
}

// GetRotationScanStats returns statistics from the rotation scanner
func (to *ThroughputOptimizer) GetRotationScanStats() map[string]interface{} {
	if to.indexer.rotationScanner == nil {
		return map[string]interface{}{
			"enabled": false,
			"message": "Rotation scanner not initialized",
		}
	}

	scanResults := to.indexer.rotationScanner.GetScanResults()
	queueSize := to.indexer.rotationScanner.GetQueueSize()
	isScanning := to.indexer.rotationScanner.IsScanning()

	totalFiles := 0
	var totalSize int64
	var totalEstimatedLines int64

	for _, result := range scanResults {
		totalFiles += result.TotalFiles
		totalSize += result.TotalSize
		totalEstimatedLines += result.EstimatedLines
	}

	return map[string]interface{}{
		"enabled":                true,
		"log_groups_scanned":     len(scanResults),
		"total_files_discovered": totalFiles,
		"total_size_bytes":       totalSize,
		"total_estimated_lines":  totalEstimatedLines,
		"queue_size":             queueSize,
		"scanning_in_progress":   isScanning,
		"scan_results":           scanResults,
	}
}

// OptimizeIndexerConfig optimizes indexer configuration based on system resources
func (to *ThroughputOptimizer) OptimizeIndexerConfig() *Config {
	currentConfig := to.indexer.config
	
	// Create optimized config based on current settings
	optimized := &Config{
		IndexPath:         currentConfig.IndexPath,
		ShardCount:        currentConfig.ShardCount,
		WorkerCount:       currentConfig.WorkerCount,
		BatchSize:         to.config.MaxBatchSize, // Use throughput-optimized batch size
		FlushInterval:     currentConfig.FlushInterval,
		MaxQueueSize:      to.config.MaxBatchSize * 15, // Larger queue for throughput
		EnableCompression: currentConfig.EnableCompression,
		MemoryQuota:       currentConfig.MemoryQuota,
		MaxSegmentSize:    currentConfig.MaxSegmentSize,
		OptimizeInterval:  currentConfig.OptimizeInterval,
		EnableMetrics:     currentConfig.EnableMetrics,
	}

	logger.Infof("📈 Generated optimized config: BatchSize=%d, MaxQueueSize=%d", 
		optimized.BatchSize, optimized.MaxQueueSize)

	return optimized
}

// getStrategyName returns a human-readable strategy name
func getStrategyName(useRotationScanning bool, numGroups int) string {
	if useRotationScanning && numGroups > 1 {
		return "rotation_scanning_parallel"
	} else if numGroups > 1 {
		return "traditional_parallel"
	} else {
		return "traditional_single"
	}
}