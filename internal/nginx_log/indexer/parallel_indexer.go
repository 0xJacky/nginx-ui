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
	memoryLimit *indexMemoryLimiter

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
	lastOptimized int64
	optimizing    int32
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
	// NOTE: dynamic shard awareness removed; GroupedShardManager is the default

	// If no shard manager provided, use grouped shard manager by default (per SHARD_GROUPS_PLAN)
	var actualShardManager ShardManager
	if shardManager == nil {
		gsm := NewGroupedShardManager(config)
		actualShardManager = gsm
	} else {
		actualShardManager = shardManager
	}

	indexer := &ParallelIndexer{
		config:       config,
		shardManager: actualShardManager,
		metrics:      NewDefaultMetricsCollector(),
		jobQueue:     make(chan *IndexJob, config.MaxQueueSize),
		resultQueue:  make(chan *IndexResult, config.WorkerCount),
		memoryLimit:  newIndexMemoryLimiter(config.MemoryQuota),
		ctx:          ctx,
		cancel:       cancel,
		stats: &IndexStats{
			WorkerStats: make([]*WorkerStats, config.WorkerCount),
		},
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

	return nil
}

// WorkerCount returns the configured worker count.
func (pi *ParallelIndexer) WorkerCount() int {
	return pi.config.WorkerCount
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
	memoryBytes := estimateDocumentsBytes(docs)
	if err := pi.memoryLimit.acquire(ctx, pi.ctx.Done(), memoryBytes); err != nil {
		return err
	}
	enqueued := false
	defer func() {
		if !enqueued {
			pi.memoryLimit.release(memoryBytes)
		}
	}()

	// Create job
	job := &IndexJob{
		Documents:   docs,
		Priority:    PriorityNormal,
		memoryBytes: memoryBytes,
	}

	// Submit job and wait for completion
	done := make(chan error, 1)
	job.Callback = func(err error) {
		done <- err
	}

	select {
	case pi.jobQueue <- job:
		enqueued = true
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

// StartBatch returns a new batch writer using the configured batch size
func (pi *ParallelIndexer) StartBatch() BatchWriterInterface {
	return NewBatchWriter(pi, pi.config.BatchSize)
}

// FlushAll flushes all pending operations
func (pi *ParallelIndexer) FlushAll() error {
	if atomic.LoadInt32(&pi.running) != 1 {
		return fmt.Errorf("indexer not running")
	}
	// Bleve Batch calls are synchronous and return only after Scorch has
	// accepted and persisted the mutation. There is no pending application
	// buffer to flush here.
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

// GetStats returns current indexer statistics
func (pi *ParallelIndexer) GetStats() *IndexStats {
	// Gather derived values before taking the lock; writing them into the
	// shared stats struct under an RLock would be a data race.
	shardStats := pi.shardManager.GetShardStats()

	var totalDocs uint64
	var totalSize int64
	for _, shard := range shardStats {
		totalDocs += shard.DocumentCount
		totalSize += shard.Size
	}

	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	pi.statsMutex.RLock()
	statsCopy := *pi.stats
	pi.statsMutex.RUnlock()

	statsCopy.Shards = shardStats
	statsCopy.ShardCount = len(shardStats)
	statsCopy.TotalDocuments = totalDocs
	statsCopy.TotalSize = totalSize
	statsCopy.QueueSize = len(pi.jobQueue)
	statsCopy.MemoryUsage = int64(memStats.Alloc)
	statsCopy.QueueMemoryUsage = pi.memoryLimit.usage()

	return &statsCopy
}

// IsRunning returns whether the indexer is currently running
func (pi *ParallelIndexer) IsRunning() bool {
	return atomic.LoadInt32(&pi.running) != 0
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
					// Stop paging this file to avoid refetching the same documents forever
					break
				}

				// If we got fewer results than requested, we're done
				if len(searchResult.Hits) < searchRequest.Size {
					break
				}

				// Deleted documents no longer match the query, so keep searching
				// from the beginning; advancing From would skip surviving documents.
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
	if manager, ok := interface{}(pi.shardManager).(interface{ Destroy() error }); ok {
		destructionErr = manager.Destroy()
	} else {
		destructionErr = fmt.Errorf("shard manager does not support destruction")
	}

	// Re-initialize context and channels for a potential restart using parent context
	pi.ctx, pi.cancel = context.WithCancel(parentCtx)
	pi.jobQueue = make(chan *IndexJob, pi.config.MaxQueueSize)
	pi.resultQueue = make(chan *IndexResult, pi.WorkerCount())
	atomic.StoreInt32(&pi.channelsClosed, 0) // Reset the channel closed flag

	return destructionErr
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
			result := func() *IndexResult {
				defer w.indexer.memoryLimit.release(job.memoryBytes)
				return w.processJob(job)
			}()

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

	// Group documents by the resolved shard object. GetShardForDocument returns
	// both a group-local shard ID and the actual shard; only the latter is
	// unambiguous once more than one log group exists.
	shardDocuments := make(map[bleve.Index][]*Document)

	for _, doc := range job.Documents {
		if doc.ID == "" || doc.Fields == nil || doc.Fields.MainLogPath == "" {
			result.Failed++
			continue
		}

		shard, _, err := w.indexer.shardManager.GetShardForDocument(doc.Fields.MainLogPath, doc.ID)
		if err != nil {
			result.Failed++
			continue
		}
		shardDocuments[shard] = append(shardDocuments[shard], doc)
	}

	// Index documents per resolved shard.
	for shard, docs := range shardDocuments {
		if err := w.indexShardDocuments(shard, docs); err != nil {
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

func (w *indexWorker) indexShardDocuments(shard bleve.Index, docs []*Document) error {
	batch := shard.NewBatch()
	for _, doc := range docs {
		// Convert LogDocument to map for Bleve indexing
		docMap := w.logDocumentToMap(doc.Fields)
		batch.Index(doc.ID, docMap)
	}

	if err := shard.Batch(batch); err != nil {
		return fmt.Errorf("failed to index batch for shard %q: %w", shard.Name(), err)
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
	// Validate log path before accessing it
	if utils.IsValidLogPath(basePath) {
		info, err := os.Stat(basePath)
		if err == nil && info.Mode().IsRegular() {
			matches = append(matches, basePath)
		}
	}

	// Deduplicate file list
	seen := make(map[string]struct{})
	uniqueFiles := make([]string, 0)
	for _, match := range matches {
		if _, ok := seen[match]; !ok {
			// Further check if it's a file, not a directory. Glob can match dirs.
			// Validate log path before accessing it
			if utils.IsValidLogPath(match) {
				info, err := os.Stat(match)
				if err == nil && info.Mode().IsRegular() {
					seen[match] = struct{}{}
					uniqueFiles = append(uniqueFiles, match)
				}
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

			// Get file size for progress calculation. The line estimate is set
			// by IndexSingleFileWithProgress (size/150), which previously
			// overwrote the sampling-based estimate anyway — so the 1MB
			// sampling read per file was pure wasted I/O and is skipped here.
			if stat, err := os.Stat(filePath); err == nil {
				progressTracker.SetFileSize(filePath, stat.Size())
			}
		}
	}

	docsCountMap := make(map[string]uint64)
	var docsCountMu sync.RWMutex
	var overallMinTime, overallMaxTime *time.Time
	var timeMu sync.Mutex

	// Process files in parallel with controlled concurrency
	var fileWg sync.WaitGroup
	// Use FileGroupConcurrency config if set, otherwise fallback to WorkerCount
	maxConcurrency := pi.config.FileGroupConcurrency
	if maxConcurrency <= 0 {
		maxConcurrency = pi.WorkerCount()
		if maxConcurrency <= 0 {
			maxConcurrency = 4 // Fallback default
		}
	}
	fileSemaphore := make(chan struct{}, maxConcurrency)

	logger.Infof("Processing %d files in log group %s with concurrency=%d", len(uniqueFiles), basePath, maxConcurrency)

	for _, filePath := range uniqueFiles {
		fileWg.Add(1)
		go func(fp string) {
			defer fileWg.Done()

			// Acquire semaphore for controlled concurrency
			fileSemaphore <- struct{}{}
			defer func() { <-fileSemaphore }()

			if progressTracker != nil {
				progressTracker.StartFile(fp)
			}

			docsIndexed, minTime, maxTime, err := pi.indexSingleFileWithProgress(fp, progressTracker)
			if err != nil {
				logger.Warnf("Failed to index file '%s' in group '%s', skipping: %v", fp, basePath, err)
				if progressTracker != nil {
					progressTracker.FailFile(fp, err.Error())
				}
				return // Skip this file
			}

			// Thread-safe update of docsCountMap
			docsCountMu.Lock()
			docsCountMap[fp] = docsIndexed
			docsCountMu.Unlock()

			if progressTracker != nil {
				progressTracker.CompleteFile(fp, int64(docsIndexed))
			}

			// Thread-safe update of time ranges
			timeMu.Lock()
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
			timeMu.Unlock()
		}(filePath)
	}

	// Wait for all files to complete
	fileWg.Wait()

	return docsCountMap, overallMinTime, overallMaxTime, nil
}

// indexSingleFileWithProgress indexes a single file with progress updates
// Now uses the optimized implementation with full progress tracking integration
func (pi *ParallelIndexer) indexSingleFileWithProgress(filePath string, progressTracker *ProgressTracker) (uint64, *time.Time, *time.Time, error) {
	// Delegate to optimized implementation with progress tracking
	return pi.IndexSingleFileWithProgress(filePath, progressTracker)
}

// CountDocsByMainLogPath returns the exact number of documents indexed for a given log group (main log path)
// by querying all shards and summing results.
func (pi *ParallelIndexer) CountDocsByMainLogPath(basePath string) (uint64, error) {
	if !pi.IsHealthy() {
		return 0, fmt.Errorf("indexer not healthy")
	}

	var total uint64
	var errs []error

	// Build term query on main_log_path
	q := bleve.NewTermQuery(basePath)
	q.SetField("main_log_path")

	shards := pi.shardManager.GetAllShards()
	for i, shard := range shards {
		if shard == nil {
			continue
		}
		req := bleve.NewSearchRequest(q)
		// We only need counts
		req.Size = 0

		res, err := shard.Search(req)
		if err != nil {
			errs = append(errs, fmt.Errorf("shard %d search failed: %w", i, err))
			continue
		}
		total += uint64(res.Total)
	}

	if len(errs) > 0 {
		return total, fmt.Errorf("%d shard errors (partial count=%d), e.g. %v", len(errs), total, errs[0])
	}
	return total, nil
}

// CountDocsByFilePath returns the exact number of documents indexed for a specific physical log file path
// by querying all shards and summing results.
func (pi *ParallelIndexer) CountDocsByFilePath(filePath string) (uint64, error) {
	if !pi.IsHealthy() {
		return 0, fmt.Errorf("indexer not healthy")
	}

	var total uint64
	var errs []error

	// Build term query on file_path
	q := bleve.NewTermQuery(filePath)
	q.SetField("file_path")

	shards := pi.shardManager.GetAllShards()
	for i, shard := range shards {
		if shard == nil {
			continue
		}
		req := bleve.NewSearchRequest(q)
		// We only need counts
		req.Size = 0

		res, err := shard.Search(req)
		if err != nil {
			errs = append(errs, fmt.Errorf("shard %d search failed: %w", i, err))
			continue
		}
		total += uint64(res.Total)
	}

	if len(errs) > 0 {
		return total, fmt.Errorf("%d shard errors (partial count=%d), e.g. %v", len(errs), total, errs[0])
	}
	return total, nil
}
