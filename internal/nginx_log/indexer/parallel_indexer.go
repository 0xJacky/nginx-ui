package indexer

import (
	"bufio"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/blevesearch/bleve/v2"
	"github.com/uozi-tech/cosy/logger"
)

// ParallelIndexer provides high-performance parallel indexing with sharding
type ParallelIndexer struct {
	config       *IndexerConfig
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

// NewParallelIndexer creates a new parallel indexer
func NewParallelIndexer(config *IndexerConfig, shardManager ShardManager) *ParallelIndexer {
	if config == nil {
		config = DefaultIndexerConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	indexer := &ParallelIndexer{
		config:       config,
		shardManager: shardManager,
		metrics:      NewDefaultMetricsCollector(),
		jobQueue:     make(chan *IndexJob, config.MaxQueueSize),
		resultQueue:  make(chan *IndexResult, config.WorkerCount),
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

// Stop gracefully stops the indexer
func (pi *ParallelIndexer) Stop() error {
	if !atomic.CompareAndSwapInt32(&pi.running, 1, 0) {
		return fmt.Errorf("indexer stopped")
	}

	// Cancel context to stop all routines
	pi.cancel()

	// Close job queue to stop accepting new jobs
	close(pi.jobQueue)

	// Wait for all workers to finish
	pi.wg.Wait()

	// Close result queue
	close(pi.resultQueue)

	// Flush all remaining data
	if err := pi.FlushAll(); err != nil {
		return fmt.Errorf("failed to flush during stop: %w", err)
	}

	return nil
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
	default:
		return fmt.Errorf("queue is full")
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

// StartBatch returns a new batch writer
func (pi *ParallelIndexer) StartBatch() BatchWriterInterface {
	return NewBatchWriter(pi, pi.config.BatchSize)
}

// FlushAll flushes all pending operations
func (pi *ParallelIndexer) FlushAll() error {
	// Get all shards and flush them
	shards := pi.shardManager.GetAllShards()
	var errs []error

	for i, shard := range shards {
		if shard == nil {
			continue
		}

		// Force flush by creating and immediately deleting a temporary document
		batch := shard.NewBatch()
		tempID := fmt.Sprintf("_flush_temp_%d_%d", i, time.Now().UnixNano())
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

// IndexLogFile reads and indexes a single log file
func (pi *ParallelIndexer) IndexLogFile(filePath string) error {
	if !pi.IsHealthy() {
		return fmt.Errorf("indexer not healthy")
	}

	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open log file %s: %w", filePath, err)
	}
	defer file.Close()

	// Use a batch writer for efficient indexing
	batch := pi.StartBatch()
	scanner := bufio.NewScanner(file)
	docCount := 0

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		// In a real implementation, parse the log line into a structured format
		// For now, we create a simple document
		logDoc, err := ParseLogLine(line) // Assuming a parser function exists
		if err != nil {
			logger.Warnf("Skipping line due to parse error in file %s: %v", filePath, err)
			continue
		}
		logDoc.FilePath = filePath

		doc := &Document{
			ID:     fmt.Sprintf("%s-%d", filePath, docCount),
			Fields: logDoc,
		}

		if err := batch.Add(doc); err != nil {
			// This indicates an auto-flush occurred and failed.
			// Log the error and stop processing this file to avoid further issues.
			return fmt.Errorf("failed to add document to batch for %s (auto-flush might have failed): %w", filePath, err)
		}
		docCount++
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading log file %s: %w", filePath, err)
	}

	if _, err := batch.Flush(); err != nil {
		return fmt.Errorf("failed to flush batch for %s: %w", filePath, err)
	}

	return nil
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
func (pi *ParallelIndexer) GetConfig() *IndexerConfig {
	return pi.config
}

// GetAllShards returns all managed shards
func (pi *ParallelIndexer) GetAllShards() []bleve.Index {
	return pi.shardManager.GetAllShards()
}

// DestroyAllIndexes closes and deletes all index data from disk.
func (pi *ParallelIndexer) DestroyAllIndexes() error {
	// Stop all background routines before deleting files
	pi.cancel()
	pi.wg.Wait()
	close(pi.jobQueue)
	close(pi.resultQueue)

	atomic.StoreInt32(&pi.running, 0) // Mark as not running

	var destructionErr error
	if manager, ok := pi.shardManager.(*DefaultShardManager); ok {
		destructionErr = manager.Destroy()
	} else {
		destructionErr = fmt.Errorf("shard manager does not support destruction")
	}

	// Re-initialize context and channels for a potential restart
	pi.ctx, pi.cancel = context.WithCancel(context.Background())
	pi.jobQueue = make(chan *IndexJob, pi.config.MaxQueueSize)
	pi.resultQueue = make(chan *IndexResult, pi.config.WorkerCount)

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

// indexSingleFile contains the logic to process one physical log file.
// It returns the number of documents indexed from the file, and the min/max timestamps.
func (pi *ParallelIndexer) indexSingleFile(filePath string) (uint64, *time.Time, *time.Time, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return 0, nil, nil, fmt.Errorf("failed to open log file %s: %w", filePath, err)
	}
	defer file.Close()

	var reader io.Reader = file
	// Handle gzipped files
	if strings.HasSuffix(filePath, ".gz") {
		gz, err := gzip.NewReader(file)
		if err != nil {
			return 0, nil, nil, fmt.Errorf("failed to create gzip reader for %s: %w", filePath, err)
		}
		defer gz.Close()
		reader = gz
	}

	logger.Infof("Starting to process file: %s", filePath)

	batch := pi.StartBatch()
	scanner := bufio.NewScanner(reader)
	docCount := 0
	var minTime, maxTime *time.Time

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		logDoc, err := ParseLogLine(line)
		if err != nil {
			logger.Warnf("Skipping line due to parse error in file %s: %v", filePath, err)
			continue
		}
		logDoc.FilePath = filePath

		// Track min/max timestamps
		ts := time.Unix(logDoc.Timestamp, 0)
		if minTime == nil || ts.Before(*minTime) {
			minTime = &ts
		}
		if maxTime == nil || ts.After(*maxTime) {
			maxTime = &ts
		}

		doc := &Document{
			ID:     fmt.Sprintf("%s-%d", filePath, docCount),
			Fields: logDoc,
		}

		if err := batch.Add(doc); err != nil {
			// This indicates an auto-flush occurred and failed.
			// Log the error and stop processing this file to avoid further issues.
			return uint64(docCount), minTime, maxTime, fmt.Errorf("failed to add document to batch for %s (auto-flush might have failed): %w", filePath, err)
		}
		docCount++
	}

	if err := scanner.Err(); err != nil {
		return uint64(docCount), minTime, maxTime, fmt.Errorf("error reading log file %s: %w", filePath, err)
	}

	logger.Infof("Finished processing file: %s. Total lines processed: %d", filePath, docCount)

	if docCount > 0 {
		if _, err := batch.Flush(); err != nil {
			return uint64(docCount), minTime, maxTime, fmt.Errorf("failed to flush batch for %s: %w", filePath, err)
		}
	}

	return uint64(docCount), minTime, maxTime, nil
}

// UpdateConfig updates the indexer configuration
func (pi *ParallelIndexer) UpdateConfig(config *IndexerConfig) error {
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
		"timestamp":  doc.Timestamp,
		"ip":         doc.IP,
		"method":     doc.Method,
		"path":       doc.Path,
		"path_exact": doc.PathExact,
		"status":     doc.Status,
		"bytes_sent": doc.BytesSent,
		"file_path":  doc.FilePath,
		"raw":        doc.Raw,
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
	if doc.ISP != "" {
		docMap["isp"] = doc.ISP
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
func (pi *ParallelIndexer) indexSingleFileWithProgress(filePath string, progressTracker *ProgressTracker) (uint64, *time.Time, *time.Time, error) {
	// If no progress tracker, just call the original method
	if progressTracker == nil {
		return pi.indexSingleFile(filePath)
	}

	// Call the original indexing method to do the actual indexing work
	docsIndexed, minTime, maxTime, err := pi.indexSingleFile(filePath)
	if err != nil {
		return 0, nil, nil, err
	}

	// Just do one final progress update when done - no artificial delays
	if progressTracker != nil && docsIndexed > 0 {
		if strings.HasSuffix(filePath, ".gz") {
			progressTracker.UpdateFileProgress(filePath, int64(docsIndexed))
		} else {
			// Estimate position based on average line size
			estimatedPos := int64(docsIndexed * 150) // Assume ~150 bytes per line
			progressTracker.UpdateFileProgress(filePath, int64(docsIndexed), estimatedPos)
		}
	}

	// Return the actual timestamps from the original method
	return docsIndexed, minTime, maxTime, nil
}
