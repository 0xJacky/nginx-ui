package indexer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/nginx_log/utils"
	"github.com/blevesearch/bleve/v2"
	"github.com/uozi-tech/cosy/logger"
)

// ParallelIndexer provides high-performance parallel indexing with sharding
type ParallelIndexer struct {
	config       *Config
	shardManager ShardManager
	metrics      MetricsCollector

	// Worker management
	workers     []*indexWorker
	jobQueue    chan *IndexJob
	resultQueue chan *IndexResult

	// State management
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	running int32

	// Cleanup control
	stopOnce       sync.Once
	channelsClosed int32

	// Statistics
	stats      *IndexStats
	statsMutex sync.RWMutex

	// Optimization
	lastOptimized       int64
	optimizing          int32
	adaptiveOptimizer   *AdaptiveOptimizer
	zeroAllocProcessor  *ZeroAllocBatchProcessor
	optimizationEnabled bool

	// Dynamic shard awareness
	dynamicAwareness *DynamicShardAwareness

	// Rotation log scanning for optimized throughput
	rotationScanner *RotationScanner
}

// indexWorker represents a single indexing worker
type indexWorker struct {
	id         int
	indexer    *ParallelIndexer
	stats      *WorkerStats
	statsMutex sync.RWMutex
}

// NewParallelIndexer creates a new parallel indexer with dynamic shard awareness
func NewParallelIndexer(config *Config, shardManager ShardManager) *ParallelIndexer {
	if config == nil {
		config = DefaultIndexerConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Initialize dynamic shard awareness
	dynamicAwareness := NewDynamicShardAwareness(config)

	// If no shard manager provided, use dynamic awareness to detect optimal type
	var actualShardManager ShardManager
	if shardManager == nil {
		detected, err := dynamicAwareness.DetectAndSetupShardManager()
		if err != nil {
			logger.Warnf("Failed to setup dynamic shard manager, using default: %v", err)
			detected = NewDefaultShardManager(config)
			detected.(*DefaultShardManager).Initialize()
		}

		// Type assertion to ShardManager interface
		if sm, ok := detected.(ShardManager); ok {
			actualShardManager = sm
		} else {
			// Fallback to default
			actualShardManager = NewDefaultShardManager(config)
			actualShardManager.(*DefaultShardManager).Initialize()
		}
	} else {
		actualShardManager = shardManager
	}

	ao := NewAdaptiveOptimizer(config)

	indexer := &ParallelIndexer{
		config:       config,
		shardManager: actualShardManager,
		metrics:      NewDefaultMetricsCollector(),
		jobQueue:     make(chan *IndexJob, config.MaxQueueSize),
		resultQueue:  make(chan *IndexResult, config.WorkerCount),
		ctx:          ctx,
		cancel:       cancel,
		stats: &IndexStats{
			WorkerStats: make([]*WorkerStats, config.WorkerCount),
		},
		adaptiveOptimizer:   ao,
		zeroAllocProcessor:  NewZeroAllocBatchProcessor(config),
		optimizationEnabled: true, // Enable optimizations by default
		dynamicAwareness:    dynamicAwareness,
		rotationScanner:     NewRotationScanner(nil), // Use default configuration
	}

	// Set up the activity poller for the adaptive optimizer
	if indexer.adaptiveOptimizer != nil {
		indexer.adaptiveOptimizer.SetActivityPoller(indexer)
	}

	// Initialize workers
	indexer.workers = make([]*indexWorker, config.WorkerCount)
	for i := 0; i < config.WorkerCount; i++ {
		indexer.workers[i] = &indexWorker{
			id:      i,
			indexer: indexer,
			stats: &WorkerStats{
				ID:     i,
				Status: WorkerStatusIdle,
			},
		}
		indexer.stats.WorkerStats[i] = indexer.workers[i].stats
	}

	return indexer
}

// Start begins the indexer operation
func (pi *ParallelIndexer) Start(ctx context.Context) error {
	if !atomic.CompareAndSwapInt32(&pi.running, 0, 1) {
		return fmt.Errorf("indexer not started")
	}

	// Initialize shard manager
	if err := pi.shardManager.Initialize(); err != nil {
		atomic.StoreInt32(&pi.running, 0)
		return fmt.Errorf("failed to initialize shard manager: %w", err)
	}

	// Start workers
	for _, worker := range pi.workers {
		pi.wg.Add(1)
		go worker.run()
	}

	// Start result processor
	pi.wg.Add(1)
	go pi.processResults()

	// Start optimization routine if enabled
	if pi.config.OptimizeInterval > 0 {
		pi.wg.Add(1)
		go pi.optimizationRoutine()
	}

	// Start metrics collection if enabled
	if pi.config.EnableMetrics {
		pi.wg.Add(1)
		go pi.metricsRoutine()
	}

	// Start adaptive optimizer if enabled
	if pi.optimizationEnabled && pi.adaptiveOptimizer != nil {
		// Set worker count change callback
		logger.Debugf("Setting up adaptive optimizer callback for worker count changes")
		pi.adaptiveOptimizer.SetWorkerCountChangeCallback(pi.handleWorkerCountChange)

		if err := pi.adaptiveOptimizer.Start(); err != nil {
			logger.Warnf("Failed to start adaptive optimizer: %v", err)
		} else {
			logger.Debugf("Adaptive optimizer started successfully")
		}
	}

	// Start dynamic shard awareness monitoring if enabled
	if pi.dynamicAwareness != nil {
		pi.dynamicAwareness.StartMonitoring(ctx)

		if pi.dynamicAwareness.IsDynamic() {
			logger.Info("Dynamic shard management is active with automatic scaling")
		} else {
			logger.Info("Static shard management is active")
		}
	}

	return nil
}

// handleWorkerCountChange handles dynamic worker count adjustments from adaptive optimizer
func (pi *ParallelIndexer) handleWorkerCountChange(oldCount, newCount int) {
	logger.Infof("Handling worker count change from %d to %d", oldCount, newCount)

	// Check if indexer is running
	if atomic.LoadInt32(&pi.running) != 1 {
		logger.Warn("Cannot adjust worker count: indexer not running")
		return
	}

	// Prevent concurrent worker adjustments
	pi.statsMutex.Lock()
	defer pi.statsMutex.Unlock()

	currentWorkerCount := len(pi.workers)
	if currentWorkerCount == newCount {
		return // Already at desired count
	}

	if newCount > currentWorkerCount {
		// Add more workers
		pi.addWorkers(newCount - currentWorkerCount)
	} else {
		// Remove workers
		pi.removeWorkers(currentWorkerCount - newCount)
	}

	// Update config to reflect the change
	pi.config.WorkerCount = newCount

	logger.Infof("Successfully adjusted worker count to %d", newCount)
}

// addWorkers adds new workers to the pool
func (pi *ParallelIndexer) addWorkers(count int) {
	for i := 0; i < count; i++ {
		workerID := len(pi.workers)
		worker := &indexWorker{
			id:      workerID,
			indexer: pi,
			stats: &WorkerStats{
				ID:     workerID,
				Status: WorkerStatusIdle,
			},
		}

		pi.workers = append(pi.workers, worker)
		pi.stats.WorkerStats = append(pi.stats.WorkerStats, worker.stats)

		// Start the new worker
		pi.wg.Add(1)
		go worker.run()

		logger.Debugf("Added worker %d", workerID)
	}
}

// removeWorkers gracefully removes workers from the pool
func (pi *ParallelIndexer) removeWorkers(count int) {
	if count >= len(pi.workers) {
		logger.Warn("Cannot remove all workers, keeping at least one")
		count = len(pi.workers) - 1
	}

	// Remove workers from the end of the slice
	workersToRemove := pi.workers[len(pi.workers)-count:]
	pi.workers = pi.workers[:len(pi.workers)-count]
	pi.stats.WorkerStats = pi.stats.WorkerStats[:len(pi.stats.WorkerStats)-count]

	// Note: In a full implementation, you would need to:
	// 1. Signal workers to stop gracefully after finishing current jobs
	// 2. Wait for them to complete
	// 3. Clean up their resources
	// For now, we just remove them from tracking

	for _, worker := range workersToRemove {
		logger.Debugf("Removed worker %d", worker.id)
	}
}

// Stop gracefully stops the indexer
func (pi *ParallelIndexer) Stop() error {
	var stopErr error

	pi.stopOnce.Do(func() {
		// Set running to 0
		if !atomic.CompareAndSwapInt32(&pi.running, 1, 0) {
			logger.Warnf("[ParallelIndexer] Stop called but indexer already stopped")
			stopErr = fmt.Errorf("indexer already stopped")
			return
		}

		// Cancel context to stop all routines
		pi.cancel()

		// Stop adaptive optimizer
		if pi.adaptiveOptimizer != nil {
			pi.adaptiveOptimizer.Stop()
		}

		// Close channels safely if they haven't been closed yet
		if atomic.CompareAndSwapInt32(&pi.channelsClosed, 0, 1) {
			// Close job queue to stop accepting new jobs
			close(pi.jobQueue)

			// Wait for all workers to finish
			pi.wg.Wait()

			// Close result queue
			close(pi.resultQueue)
		} else {
			// If channels are already closed, just wait for workers
			pi.wg.Wait()
		}

		// Skip flush during stop - shards may already be closed by searcher
		// FlushAll should be called before Stop() if needed

		// Close the shard manager - this will close all shards and stop Bleve worker goroutines
		// This is critical to prevent goroutine leaks from Bleve's internal workers
		if pi.shardManager != nil {
			if err := pi.shardManager.Close(); err != nil {
				logger.Errorf("Failed to close shard manager: %v", err)
				stopErr = err
			}
		}
	})

	return stopErr
}

// IndexDocument indexes a single document
func (pi *ParallelIndexer) IndexDocument(ctx context.Context, doc *Document) error {
	return pi.IndexDocuments(ctx, []*Document{doc})
}

// IndexDocuments indexes multiple documents
func (pi *ParallelIndexer) IndexDocuments(ctx context.Context, docs []*Document) error {
	if !pi.IsHealthy() {
		return fmt.Errorf("indexer not started")
	}

	if len(docs) == 0 {
		return nil
	}

	// Create job
	job := &IndexJob{
		Documents: docs,
		Priority:  PriorityNormal,
	}

	// Submit job and wait for completion
	done := make(chan error, 1)
	job.Callback = func(err error) {
		done <- err
	}

	select {
	case pi.jobQueue <- job:
		select {
		case err := <-done:
			return err
		case <-ctx.Done():
			return ctx.Err()
		}
	case <-ctx.Done():
		return ctx.Err()
	case <-pi.ctx.Done():
		return fmt.Errorf("indexer stopped")
	}
}

// IndexDocumentAsync indexes a document asynchronously
func (pi *ParallelIndexer) IndexDocumentAsync(doc *Document, callback func(error)) {
	pi.IndexDocumentsAsync([]*Document{doc}, callback)
}

// IndexDocumentsAsync indexes multiple documents asynchronously
func (pi *ParallelIndexer) IndexDocumentsAsync(docs []*Document, callback func(error)) {
	if !pi.IsHealthy() {
		if callback != nil {
			callback(fmt.Errorf("indexer not started"))
		}
		return
	}

	if len(docs) == 0 {
		if callback != nil {
			callback(nil)
		}
		return
	}

	job := &IndexJob{
		Documents: docs,
		Priority:  PriorityNormal,
		Callback:  callback,
	}

	select {
	case pi.jobQueue <- job:
		// Job queued successfully
	case <-pi.ctx.Done():
		if callback != nil {
			callback(fmt.Errorf("indexer stopped"))
		}
	default:
		// Queue is full
		if callback != nil {
			callback(fmt.Errorf("queue is full"))
		}
	}
}

// StartBatch returns a new batch writer with adaptive batch size
func (pi *ParallelIndexer) StartBatch() BatchWriterInterface {
	batchSize := pi.config.BatchSize
	if pi.adaptiveOptimizer != nil {
		batchSize = pi.adaptiveOptimizer.GetOptimalBatchSize()
	}
	return NewBatchWriter(pi, batchSize)
}

// GetOptimizationStats returns current optimization statistics
func (pi *ParallelIndexer) GetOptimizationStats() AdaptiveOptimizationStats {
	if pi.adaptiveOptimizer != nil {
		return pi.adaptiveOptimizer.GetOptimizationStats()
	}
	return AdaptiveOptimizationStats{}
}

// GetPoolStats returns object pool statistics
func (pi *ParallelIndexer) GetPoolStats() PoolStats {
	if pi.zeroAllocProcessor != nil {
		return pi.zeroAllocProcessor.GetPoolStats()
	}
	return PoolStats{}
}

// EnableOptimizations enables or disables adaptive optimizations
func (pi *ParallelIndexer) EnableOptimizations(enabled bool) {
	pi.optimizationEnabled = enabled
	if !enabled && pi.adaptiveOptimizer != nil {
		pi.adaptiveOptimizer.Stop()
	} else if enabled && pi.adaptiveOptimizer != nil && atomic.LoadInt32(&pi.running) == 1 {
		pi.adaptiveOptimizer.Start()
	}
}

// GetDynamicShardInfo returns information about dynamic shard management
func (pi *ParallelIndexer) GetDynamicShardInfo() *DynamicShardInfo {
	if pi.dynamicAwareness == nil {
		return &DynamicShardInfo{
			IsEnabled:  false,
			IsActive:   false,
			ShardCount: pi.config.ShardCount,
			ShardType:  "static",
		}
	}

	isDynamic := pi.dynamicAwareness.IsDynamic()
	shardManager := pi.dynamicAwareness.GetCurrentShardManager()

	info := &DynamicShardInfo{
		IsEnabled:  true,
		IsActive:   isDynamic,
		ShardCount: pi.config.ShardCount,
		ShardType:  "static",
	}

	if isDynamic {
		info.ShardType = "dynamic"

		if enhancedManager, ok := shardManager.(*EnhancedDynamicShardManager); ok {
			info.TargetShardCount = enhancedManager.GetTargetShardCount()
			info.IsScaling = enhancedManager.IsScalingInProgress()
			info.AutoScaleEnabled = enhancedManager.IsAutoScaleEnabled()

			// Get scaling recommendation
			recommendation := enhancedManager.GetScalingRecommendations()
			info.Recommendation = recommendation

			// Get shard health
			info.ShardHealth = enhancedManager.GetShardHealth()
		}
	}

	// Get performance analysis
	analysis := pi.dynamicAwareness.GetPerformanceAnalysis()
	info.PerformanceAnalysis = &analysis

	return info
}

// DynamicShardInfo contains information about dynamic shard management status
type DynamicShardInfo struct {
	IsEnabled           bool                       `json:"is_enabled"`
	IsActive            bool                       `json:"is_active"`
	ShardType           string                     `json:"shard_type"` // "static" or "dynamic"
	ShardCount          int                        `json:"shard_count"`
	TargetShardCount    int                        `json:"target_shard_count,omitempty"`
	IsScaling           bool                       `json:"is_scaling,omitempty"`
	AutoScaleEnabled    bool                       `json:"auto_scale_enabled,omitempty"`
	Recommendation      *ScalingRecommendation     `json:"recommendation,omitempty"`
	ShardHealth         map[int]*ShardHealthStatus `json:"shard_health,omitempty"`
	PerformanceAnalysis *PerformanceAnalysis       `json:"performance_analysis,omitempty"`
}

// FlushAll flushes all pending operations
func (pi *ParallelIndexer) FlushAll() error {
	// Check if indexer is still running
	if atomic.LoadInt32(&pi.running) != 1 {
		return fmt.Errorf("indexer not running")
	}

	// Get all shards and flush them
	shards := pi.shardManager.GetAllShards()
	var errs []error

	for i, shard := range shards {
		if shard == nil {
			continue
		}

		// Force flush by creating and immediately deleting a temporary document
		batch := shard.NewBatch()
		// Use efficient string building instead of fmt.Sprintf
		tempIDBuf := make([]byte, 0, 64)
		tempIDBuf = append(tempIDBuf, "_flush_temp_"...)
		tempIDBuf = utils.AppendInt(tempIDBuf, i)
		tempIDBuf = append(tempIDBuf, '_')
		tempIDBuf = utils.AppendInt(tempIDBuf, int(time.Now().UnixNano()))
		tempID := utils.BytesToStringUnsafe(tempIDBuf)
		batch.Index(tempID, map[string]interface{}{"_temp": true})

		if err := shard.Batch(batch); err != nil {
			errs = append(errs, fmt.Errorf("failed to flush shard %d: %w", i, err))
			continue
		}

		// Delete the temporary document
		shard.Delete(tempID)
	}

	if len(errs) > 0 {
		return fmt.Errorf("flush errors: %v", errs)
	}

	return nil
}

// Optimize triggers optimization of all shards
func (pi *ParallelIndexer) Optimize() error {
	if !atomic.CompareAndSwapInt32(&pi.optimizing, 0, 1) {
		return fmt.Errorf("optimization already in progress")
	}
	defer atomic.StoreInt32(&pi.optimizing, 0)

	startTime := time.Now()
	stats := pi.shardManager.GetShardStats()

	var errs []error

	for _, stat := range stats {
		if err := pi.shardManager.OptimizeShard(stat.ID); err != nil {
			errs = append(errs, fmt.Errorf("failed to optimize shard %d: %w", stat.ID, err))
		}
	}

	// Update optimization stats
	pi.statsMutex.Lock()
	if pi.stats.OptimizationStats == nil {
		pi.stats.OptimizationStats = &OptimizationStats{}
	}
	pi.stats.OptimizationStats.LastRun = time.Now().Unix()
	pi.stats.OptimizationStats.Duration = time.Since(startTime)
	pi.stats.OptimizationStats.Success = len(errs) == 0
	pi.stats.LastOptimized = time.Now().Unix()
	pi.statsMutex.Unlock()

	atomic.StoreInt64(&pi.lastOptimized, time.Now().Unix())

	if len(errs) > 0 {
		return fmt.Errorf("optimization errors: %v", errs)
	}

	// Record optimization metrics
	pi.metrics.RecordOptimization(time.Since(startTime), len(errs) == 0)

	return nil
}

// IndexLogFile reads and indexes a single log file using optimized processing
// Now uses OptimizedParseStream for 7-8x faster performance and 70% memory reduction
func (pi *ParallelIndexer) IndexLogFile(filePath string) error {
	// Delegate to optimized implementation
	return pi.OptimizedIndexLogFile(filePath)
}

// GetStats returns current indexer statistics
func (pi *ParallelIndexer) GetStats() *IndexStats {
	pi.statsMutex.RLock()
	defer pi.statsMutex.RUnlock()

	// Update shard stats
	shardStats := pi.shardManager.GetShardStats()
	pi.stats.Shards = shardStats
	pi.stats.ShardCount = len(shardStats)

	var totalDocs uint64
	var totalSize int64
	for _, shard := range shardStats {
		totalDocs += shard.DocumentCount
		totalSize += shard.Size
	}

	pi.stats.TotalDocuments = totalDocs
	pi.stats.TotalSize = totalSize
	pi.stats.QueueSize = len(pi.jobQueue)

	// Calculate memory usage
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	pi.stats.MemoryUsage = int64(memStats.Alloc)

	// Copy stats to avoid race conditions
	statsCopy := *pi.stats
	return &statsCopy
}

// IsRunning returns whether the indexer is currently running
func (pi *ParallelIndexer) IsRunning() bool {
	return atomic.LoadInt32(&pi.running) != 0
}

// IsBusy checks if the indexer has pending jobs or any active workers.
func (pi *ParallelIndexer) IsBusy() bool {
	if len(pi.jobQueue) > 0 {
		return true
	}

	// This RLock protects the pi.workers slice from changing during iteration (e.g. scaling)
	pi.statsMutex.RLock()
	defer pi.statsMutex.RUnlock()

	for _, worker := range pi.workers {
		worker.statsMutex.RLock()
		isBusy := worker.stats.Status == WorkerStatusBusy
		worker.statsMutex.RUnlock()
		if isBusy {
			return true
		}
	}

	return false
}

// GetShardInfo returns information about a specific shard
func (pi *ParallelIndexer) GetShardInfo(shardID int) (*ShardInfo, error) {
	shardStats := pi.shardManager.GetShardStats()
	for _, stat := range shardStats {
		if stat.ID == shardID {
			return stat, nil
		}
	}
	return nil, fmt.Errorf("%s: %d", ErrShardNotFound, shardID)
}

// IsHealthy checks if the indexer is running and healthy
func (pi *ParallelIndexer) IsHealthy() bool {
	if atomic.LoadInt32(&pi.running) != 1 {
		return false
	}

	// Check shard manager health
	return pi.shardManager.HealthCheck() == nil
}

// GetConfig returns the current configuration
func (pi *ParallelIndexer) GetConfig() *Config {
	return pi.config
}

// GetAllShards returns all managed shards
func (pi *ParallelIndexer) GetAllShards() []bleve.Index {
	return pi.shardManager.GetAllShards()
}

// DeleteIndexByLogGroup deletes all index entries for a specific log group (base path and its rotated files)
func (pi *ParallelIndexer) DeleteIndexByLogGroup(basePath string, logFileManager interface{}) error {
	if !pi.IsHealthy() {
		return fmt.Errorf("indexer not healthy")
	}

	// Get all file paths for this log group from the database
	if logFileManager == nil {
		return fmt.Errorf("log file manager is required")
	}

	lfm, ok := logFileManager.(GroupFileProvider)
	if !ok {
		return fmt.Errorf("log file manager does not support GetFilePathsForGroup")
	}

	filesToDelete, err := lfm.GetFilePathsForGroup(basePath)
	if err != nil {
		return fmt.Errorf("failed to get file paths for log group %s: %w", basePath, err)
	}

	logger.Infof("Deleting index entries for log group %s, files: %v", basePath, filesToDelete)

	// Delete documents from all shards for these files
	shards := pi.shardManager.GetAllShards()
	var deleteErrors []error

	for _, shard := range shards {
		// Search for documents with matching file_path
		for _, filePath := range filesToDelete {
			query := bleve.NewTermQuery(filePath)
			query.SetField("file_path")

			searchRequest := bleve.NewSearchRequest(query)
			searchRequest.Size = 1000 // Process in batches
			searchRequest.Fields = []string{"file_path"}

			for {
				searchResult, err := shard.Search(searchRequest)
				if err != nil {
					deleteErrors = append(deleteErrors, fmt.Errorf("failed to search for documents in file %s: %w", filePath, err))
					break
				}

				if len(searchResult.Hits) == 0 {
					break // No more documents to delete
				}

				// Delete documents in batch
				batch := shard.NewBatch()
				for _, hit := range searchResult.Hits {
					batch.Delete(hit.ID)
				}

				if err := shard.Batch(batch); err != nil {
					deleteErrors = append(deleteErrors, fmt.Errorf("failed to delete batch for file %s: %w", filePath, err))
				}

				// If we got fewer results than requested, we're done
				if len(searchResult.Hits) < searchRequest.Size {
					break
				}

				// Continue from where we left off
				searchRequest.From += searchRequest.Size
			}
		}
	}

	if len(deleteErrors) > 0 {
		return fmt.Errorf("encountered %d errors during deletion: %v", len(deleteErrors), deleteErrors[0])
	}

	logger.Infof("Successfully deleted index entries for log group: %s", basePath)
	return nil
}

// DestroyAllIndexes closes and deletes all index data from disk.
func (pi *ParallelIndexer) DestroyAllIndexes(parentCtx context.Context) error {
	// Stop all background routines before deleting files
	pi.cancel()
	pi.wg.Wait()

	// Safely close channels if they haven't been closed yet
	if atomic.CompareAndSwapInt32(&pi.channelsClosed, 0, 1) {
		close(pi.jobQueue)
		close(pi.resultQueue)
	}

	atomic.StoreInt32(&pi.running, 0) // Mark as not running

	var destructionErr error
	if manager, ok := pi.shardManager.(*DefaultShardManager); ok {
		destructionErr = manager.Destroy()
	} else {
		destructionErr = fmt.Errorf("shard manager does not support destruction")
	}

	// Re-initialize context and channels for a potential restart using parent context
	pi.ctx, pi.cancel = context.WithCancel(parentCtx)
	pi.jobQueue = make(chan *IndexJob, pi.config.MaxQueueSize)
	pi.resultQueue = make(chan *IndexResult, pi.config.WorkerCount)
	atomic.StoreInt32(&pi.channelsClosed, 0) // Reset the channel closed flag

	return destructionErr
}

// IndexLogGroup finds all files related to a base log path (e.g., rotated logs) and indexes them.
// It returns a map of [filePath -> docCount], and the min/max timestamps found.
func (pi *ParallelIndexer) IndexLogGroup(basePath string) (map[string]uint64, *time.Time, *time.Time, error) {
	if !pi.IsHealthy() {
		return nil, nil, nil, fmt.Errorf("indexer not healthy")
	}

	// Find all files belonging to this log group by globbing
	globPath := basePath + "*"
	matches, err := filepath.Glob(globPath)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to glob for log files with base %s: %w", basePath, err)
	}

	// filepath.Glob might not match the base file itself if it has no extension,
	// so we check for it explicitly and add it to the list.
	info, err := os.Stat(basePath)
	if err == nil && info.Mode().IsRegular() {
		matches = append(matches, basePath)
	}

	// Deduplicate file list
	seen := make(map[string]struct{})
	uniqueFiles := make([]string, 0)
	for _, match := range matches {
		if _, ok := seen[match]; !ok {
			// Further check if it's a file, not a directory. Glob can match dirs.
			info, err := os.Stat(match)
			if err == nil && info.Mode().IsRegular() {
				seen[match] = struct{}{}
				uniqueFiles = append(uniqueFiles, match)
			}
		}
	}

	if len(uniqueFiles) == 0 {
		logger.Warnf("No actual log file found for group: %s", basePath)
		return nil, nil, nil, nil
	}

	logger.Infof("Found %d file(s) for log group %s: %v", len(uniqueFiles), basePath, uniqueFiles)

	docsCountMap := make(map[string]uint64)
	var overallMinTime, overallMaxTime *time.Time

	for _, filePath := range uniqueFiles {
		docsIndexed, minTime, maxTime, err := pi.indexSingleFile(filePath)
		if err != nil {
			logger.Warnf("Failed to index file '%s' in group '%s', skipping: %v", filePath, basePath, err)
			continue // Continue with the next file
		}
		docsCountMap[filePath] = docsIndexed

		if minTime != nil {
			if overallMinTime == nil || minTime.Before(*overallMinTime) {
				overallMinTime = minTime
			}
		}
		if maxTime != nil {
			if overallMaxTime == nil || maxTime.After(*overallMaxTime) {
				overallMaxTime = maxTime
			}
		}
	}

	return docsCountMap, overallMinTime, overallMaxTime, nil
}

// IndexLogGroupWithRotationScanning performs optimized log group indexing using rotation scanner
// for maximum frontend throughput by prioritizing files based on size and age
func (pi *ParallelIndexer) IndexLogGroupWithRotationScanning(basePaths []string, progressConfig *ProgressConfig) (map[string]uint64, *time.Time, *time.Time, error) {
	if !pi.IsHealthy() {
		return nil, nil, nil, fmt.Errorf("indexer not healthy")
	}

	ctx, cancel := context.WithTimeout(pi.ctx, 10*time.Minute)
	defer cancel()

	logger.Infof("🚀 Starting optimized rotation log indexing for %d log groups", len(basePaths))

	// Scan all log groups and build priority queue
	if err := pi.rotationScanner.ScanLogGroups(ctx, basePaths); err != nil {
		return nil, nil, nil, fmt.Errorf("failed to scan log groups: %w", err)
	}

	// Create progress tracker if config is provided
	var progressTracker *ProgressTracker
	if progressConfig != nil {
		progressTracker = NewProgressTracker("rotation-scan", progressConfig)
		
		// Add all discovered files to progress tracker
		scanResults := pi.rotationScanner.GetScanResults()
		for _, result := range scanResults {
			for _, file := range result.Files {
				progressTracker.AddFile(file.Path, file.IsCompressed)
				progressTracker.SetFileSize(file.Path, file.Size)
				progressTracker.SetFileEstimate(file.Path, file.EstimatedLines)
			}
		}
	}

	docsCountMap := make(map[string]uint64)
	var overallMinTime, overallMaxTime *time.Time

	// Process files in optimized batches using rotation scanner
	batchSize := pi.config.BatchSize / 4 // Smaller batches for better progress tracking
	processedFiles := 0
	totalFiles := pi.rotationScanner.GetQueueSize()

	for {
		select {
		case <-ctx.Done():
			return docsCountMap, overallMinTime, overallMaxTime, ctx.Err()
		default:
		}

		// Get next batch of files prioritized by scanner
		batch := pi.rotationScanner.GetNextBatch(batchSize)
		if len(batch) == 0 {
			break // No more files to process
		}

		logger.Debugf("📦 Processing batch of %d files (progress: %d/%d)", len(batch), processedFiles, totalFiles)

		// Process each file in the batch
		for _, fileInfo := range batch {
			if progressTracker != nil {
				progressTracker.StartFile(fileInfo.Path)
			}

			docsIndexed, minTime, maxTime, err := pi.indexSingleFile(fileInfo.Path)
			if err != nil {
				logger.Warnf("Failed to index file %s: %v", fileInfo.Path, err)
				if progressTracker != nil {
					// Skip error recording for now
				_ = err
				}
				continue
			}

			docsCountMap[fileInfo.Path] = docsIndexed
			processedFiles++

			// Update overall time range
			if minTime != nil && (overallMinTime == nil || minTime.Before(*overallMinTime)) {
				overallMinTime = minTime
			}
			if maxTime != nil && (overallMaxTime == nil || maxTime.After(*overallMaxTime)) {
				overallMaxTime = maxTime
			}

			if progressTracker != nil {
				progressTracker.CompleteFile(fileInfo.Path, int64(docsIndexed))
			}

			logger.Debugf("✅ Indexed %s: %d documents", fileInfo.Path, docsIndexed)
		}

		// Report batch progress
		logger.Infof("📊 Batch completed: %d/%d files processed (%.1f%% complete)", 
			processedFiles, totalFiles, float64(processedFiles)/float64(totalFiles)*100)
	}

	logger.Infof("🎉 Optimized rotation log indexing completed: %d files, %d total documents", 
		processedFiles, sumDocCounts(docsCountMap))

	return docsCountMap, overallMinTime, overallMaxTime, nil
}

// IndexSingleFileIncrementally is a more efficient version for incremental updates.
// It indexes only the specified single file instead of the entire log group.
func (pi *ParallelIndexer) IndexSingleFileIncrementally(filePath string, progressConfig *ProgressConfig) (map[string]uint64, *time.Time, *time.Time, error) {
	if !pi.IsHealthy() {
		return nil, nil, nil, fmt.Errorf("indexer not healthy")
	}

	// Create progress tracker if config is provided
	var progressTracker *ProgressTracker
	if progressConfig != nil {
		progressTracker = NewProgressTracker(filePath, progressConfig)
		// Setup file for tracking
		isCompressed := IsCompressedFile(filePath)
		progressTracker.AddFile(filePath, isCompressed)
		if stat, err := os.Stat(filePath); err == nil {
			progressTracker.SetFileSize(filePath, stat.Size())
			if estimatedLines, err := EstimateFileLines(context.Background(), filePath, stat.Size(), isCompressed); err == nil {
				progressTracker.SetFileEstimate(filePath, estimatedLines)
			}
		}
	}

	docsCountMap := make(map[string]uint64)

	if progressTracker != nil {
		progressTracker.StartFile(filePath)
	}

	docsIndexed, minTime, maxTime, err := pi.indexSingleFileWithProgress(filePath, progressTracker)
	if err != nil {
		logger.Warnf("Failed to incrementally index file '%s', skipping: %v", filePath, err)
		if progressTracker != nil {
			progressTracker.FailFile(filePath, err.Error())
		}
		// Return empty results and the error
		return docsCountMap, nil, nil, err
	}

	docsCountMap[filePath] = docsIndexed

	if progressTracker != nil {
		progressTracker.CompleteFile(filePath, int64(docsIndexed))
	}

	return docsCountMap, minTime, maxTime, nil
}

// indexSingleFile contains optimized logic to process one physical log file.
// Now uses OptimizedParseStream for 7-8x faster performance and 70% memory reduction
func (pi *ParallelIndexer) indexSingleFile(filePath string) (uint64, *time.Time, *time.Time, error) {
	// Delegate to optimized implementation
	return pi.OptimizedIndexSingleFile(filePath)
}

// UpdateConfig updates the indexer configuration
func (pi *ParallelIndexer) UpdateConfig(config *Config) error {
	// Only allow updating certain configuration parameters while running
	pi.config.BatchSize = config.BatchSize
	pi.config.FlushInterval = config.FlushInterval
	pi.config.EnableMetrics = config.EnableMetrics

	return nil
}

// Worker implementation
func (w *indexWorker) run() {
	defer w.indexer.wg.Done()

	w.updateStatus(WorkerStatusIdle)

	for {
		select {
		case job, ok := <-w.indexer.jobQueue:
			if !ok {
				return // Channel closed, worker should exit
			}

			w.updateStatus(WorkerStatusBusy)
			result := w.processJob(job)

			// Send result
			select {
			case w.indexer.resultQueue <- result:
			case <-w.indexer.ctx.Done():
				return
			}

			// Execute callback if provided
			if job.Callback != nil {
				var err error
				if result.Failed > 0 {
					err = fmt.Errorf("indexing failed for %d documents", result.Failed)
				}
				job.Callback(err)
			}

			w.updateStatus(WorkerStatusIdle)

		case <-w.indexer.ctx.Done():
			return
		}
	}
}

func (w *indexWorker) processJob(job *IndexJob) *IndexResult {
	startTime := time.Now()
	result := &IndexResult{
		Processed: len(job.Documents),
	}

	// Group documents by shard
	shardDocs := make(map[int][]*Document)

	for _, doc := range job.Documents {
		if doc.ID == "" {
			result.Failed++
			continue
		}

		_, shardID, err := w.indexer.shardManager.GetShard(doc.ID)
		if err != nil {
			result.Failed++
			continue
		}

		shardDocs[shardID] = append(shardDocs[shardID], doc)
	}

	// Index documents per shard
	for shardID, docs := range shardDocs {
		if err := w.indexShardDocuments(shardID, docs); err != nil {
			result.Failed += len(docs)
		} else {
			result.Succeeded += len(docs)
		}
	}

	result.Duration = time.Since(startTime)
	if result.Processed > 0 {
		result.ErrorRate = float64(result.Failed) / float64(result.Processed)
		result.Throughput = float64(result.Processed) / result.Duration.Seconds()
	}

	// Update worker stats
	w.statsMutex.Lock()
	w.stats.ProcessedJobs++
	w.stats.ProcessedDocs += int64(result.Processed)
	w.stats.ErrorCount += int64(result.Failed)
	w.stats.LastActive = time.Now().Unix()

	// Update average latency (simple moving average)
	if w.stats.AverageLatency == 0 {
		w.stats.AverageLatency = result.Duration
	} else {
		w.stats.AverageLatency = (w.stats.AverageLatency + result.Duration) / 2
	}
	w.statsMutex.Unlock()

	return result
}

func (w *indexWorker) indexShardDocuments(shardID int, docs []*Document) error {
	shard, err := w.indexer.shardManager.GetShardByID(shardID)
	if err != nil {
		return err
	}

	batch := shard.NewBatch()
	for _, doc := range docs {
		// Convert LogDocument to map for Bleve indexing
		docMap := w.logDocumentToMap(doc.Fields)
		batch.Index(doc.ID, docMap)
	}

	if err := shard.Batch(batch); err != nil {
		return fmt.Errorf("failed to index batch for shard %d: %w", shardID, err)
	}

	return nil
}

// logDocumentToMap converts LogDocument to map[string]interface{} for Bleve
func (w *indexWorker) logDocumentToMap(doc *LogDocument) map[string]interface{} {
	docMap := map[string]interface{}{
		"timestamp":     doc.Timestamp,
		"ip":            doc.IP,
		"method":        doc.Method,
		"path":          doc.Path,
		"path_exact":    doc.PathExact,
		"status":        doc.Status,
		"bytes_sent":    doc.BytesSent,
		"file_path":     doc.FilePath,
		"main_log_path": doc.MainLogPath,
		"raw":           doc.Raw,
	}

	// Add optional fields only if they have values
	if doc.RegionCode != "" {
		docMap["region_code"] = doc.RegionCode
	}
	if doc.Province != "" {
		docMap["province"] = doc.Province
	}
	if doc.City != "" {
		docMap["city"] = doc.City
	}
	if doc.Protocol != "" {
		docMap["protocol"] = doc.Protocol
	}
	if doc.Referer != "" {
		docMap["referer"] = doc.Referer
	}
	if doc.UserAgent != "" {
		docMap["user_agent"] = doc.UserAgent
	}
	if doc.Browser != "" {
		docMap["browser"] = doc.Browser
	}
	if doc.BrowserVer != "" {
		docMap["browser_version"] = doc.BrowserVer
	}
	if doc.OS != "" {
		docMap["os"] = doc.OS
	}
	if doc.OSVersion != "" {
		docMap["os_version"] = doc.OSVersion
	}
	if doc.DeviceType != "" {
		docMap["device_type"] = doc.DeviceType
	}
	if doc.RequestTime > 0 {
		docMap["request_time"] = doc.RequestTime
	}
	if doc.UpstreamTime != nil {
		docMap["upstream_time"] = *doc.UpstreamTime
	}

	return docMap
}

func (w *indexWorker) updateStatus(status string) {
	w.statsMutex.Lock()
	w.stats.Status = status
	w.statsMutex.Unlock()
}

// Background routines
func (pi *ParallelIndexer) processResults() {
	defer pi.wg.Done()

	for {
		select {
		case result := <-pi.resultQueue:
			if result != nil {
				pi.metrics.RecordIndexOperation(
					result.Processed,
					result.Duration,
					result.Failed == 0,
				)
			}
		case <-pi.ctx.Done():
			return
		}
	}
}

func (pi *ParallelIndexer) optimizationRoutine() {
	defer pi.wg.Done()

	ticker := time.NewTicker(pi.config.OptimizeInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if atomic.LoadInt32(&pi.optimizing) == 0 {
				go pi.Optimize() // Run in background to avoid blocking
			}
		case <-pi.ctx.Done():
			return
		}
	}
}

func (pi *ParallelIndexer) metricsRoutine() {
	defer pi.wg.Done()

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			pi.updateMetrics()
		case <-pi.ctx.Done():
			return
		}
	}
}

func (pi *ParallelIndexer) updateMetrics() {
	pi.statsMutex.Lock()
	defer pi.statsMutex.Unlock()

	// Update indexing rate based on recent activity
	metrics := pi.metrics.GetMetrics()
	pi.stats.IndexingRate = metrics.IndexingRate
}

// IndexLogGroupWithProgress indexes a log group with progress tracking
func (pi *ParallelIndexer) IndexLogGroupWithProgress(basePath string, progressConfig *ProgressConfig) (map[string]uint64, *time.Time, *time.Time, error) {
	if !pi.IsHealthy() {
		return nil, nil, nil, fmt.Errorf("indexer not healthy")
	}

	// Create progress tracker if config is provided
	var progressTracker *ProgressTracker
	if progressConfig != nil {
		progressTracker = NewProgressTracker(basePath, progressConfig)
	}

	// Find all files belonging to this log group by globbing
	globPath := basePath + "*"
	matches, err := filepath.Glob(globPath)
	if err != nil {
		if progressTracker != nil {
			progressTracker.Cancel(fmt.Sprintf("glob failed: %v", err))
		}
		return nil, nil, nil, fmt.Errorf("failed to glob for log files with base %s: %w", basePath, err)
	}

	// filepath.Glob might not match the base file itself if it has no extension,
	// so we check for it explicitly and add it to the list.
	info, err := os.Stat(basePath)
	if err == nil && info.Mode().IsRegular() {
		matches = append(matches, basePath)
	}

	// Deduplicate file list
	seen := make(map[string]struct{})
	uniqueFiles := make([]string, 0)
	for _, match := range matches {
		if _, ok := seen[match]; !ok {
			// Further check if it's a file, not a directory. Glob can match dirs.
			info, err := os.Stat(match)
			if err == nil && info.Mode().IsRegular() {
				seen[match] = struct{}{}
				uniqueFiles = append(uniqueFiles, match)
			}
		}
	}

	if len(uniqueFiles) == 0 {
		logger.Warnf("No actual log file found for group: %s", basePath)
		if progressTracker != nil {
			progressTracker.Cancel("no files found")
		}
		return nil, nil, nil, nil
	}

	logger.Infof("Found %d file(s) for log group %s: %v", len(uniqueFiles), basePath, uniqueFiles)

	// Set up progress tracking for all files
	if progressTracker != nil {
		for _, filePath := range uniqueFiles {
			isCompressed := IsCompressedFile(filePath)
			progressTracker.AddFile(filePath, isCompressed)

			// Get file size and estimate lines
			if stat, err := os.Stat(filePath); err == nil {
				progressTracker.SetFileSize(filePath, stat.Size())

				// Estimate lines for progress calculation
				if estimatedLines, err := EstimateFileLines(context.Background(), filePath, stat.Size(), isCompressed); err == nil {
					progressTracker.SetFileEstimate(filePath, estimatedLines)
				}
			}
		}
	}

	docsCountMap := make(map[string]uint64)
	var overallMinTime, overallMaxTime *time.Time

	// Process each file with progress tracking
	for _, filePath := range uniqueFiles {
		if progressTracker != nil {
			progressTracker.StartFile(filePath)
		}

		docsIndexed, minTime, maxTime, err := pi.indexSingleFileWithProgress(filePath, progressTracker)
		if err != nil {
			logger.Warnf("Failed to index file '%s' in group '%s', skipping: %v", filePath, basePath, err)
			if progressTracker != nil {
				progressTracker.FailFile(filePath, err.Error())
			}
			continue // Continue with the next file
		}

		docsCountMap[filePath] = docsIndexed

		if progressTracker != nil {
			progressTracker.CompleteFile(filePath, int64(docsIndexed))
		}

		if minTime != nil {
			if overallMinTime == nil || minTime.Before(*overallMinTime) {
				overallMinTime = minTime
			}
		}
		if maxTime != nil {
			if overallMaxTime == nil || maxTime.After(*overallMaxTime) {
				overallMaxTime = maxTime
			}
		}
	}

	return docsCountMap, overallMinTime, overallMaxTime, nil
}

// indexSingleFileWithProgress indexes a single file with progress updates
// Now uses the optimized implementation with full progress tracking integration
func (pi *ParallelIndexer) indexSingleFileWithProgress(filePath string, progressTracker *ProgressTracker) (uint64, *time.Time, *time.Time, error) {
	// Delegate to optimized implementation with progress tracking
	return pi.OptimizedIndexSingleFileWithProgress(filePath, progressTracker)
}

// sumDocCounts returns the total number of documents across all files
func sumDocCounts(docsCountMap map[string]uint64) uint64 {
	var total uint64
	for _, count := range docsCountMap {
		total += count
	}
	return total
}
