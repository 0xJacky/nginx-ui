package indexer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"
)

// RebuildManager handles index rebuilding operations
type RebuildManager struct {
	indexer          *ParallelIndexer
	persistence      *PersistenceManager
	progressManager  *ProgressManager
	shardManager     ShardManager
	config           *RebuildConfig
	rebuilding       int32 // atomic flag
	lastRebuildTime  time.Time
	mu               sync.RWMutex
}

// RebuildConfig contains configuration for rebuild operations
type RebuildConfig struct {
	BatchSize          int           `json:"batch_size"`
	MaxConcurrency     int           `json:"max_concurrency"`
	DeleteBeforeRebuild bool         `json:"delete_before_rebuild"`
	ProgressInterval   time.Duration `json:"progress_interval"`
	TimeoutPerFile     time.Duration `json:"timeout_per_file"`
}

// DefaultRebuildConfig returns default rebuild configuration
func DefaultRebuildConfig() *RebuildConfig {
	return &RebuildConfig{
		BatchSize:          1000,
		MaxConcurrency:     4,
		DeleteBeforeRebuild: true,
		ProgressInterval:   5 * time.Second,
		TimeoutPerFile:     30 * time.Minute,
	}
}

// NewRebuildManager creates a new rebuild manager
func NewRebuildManager(indexer *ParallelIndexer, persistence *PersistenceManager, progressManager *ProgressManager, shardManager ShardManager, config *RebuildConfig) *RebuildManager {
	if config == nil {
		config = DefaultRebuildConfig()
	}
	
	return &RebuildManager{
		indexer:         indexer,
		persistence:     persistence,
		progressManager: progressManager,
		shardManager:    shardManager,
		config:          config,
	}
}

// RebuildAll rebuilds all indexes from scratch
func (rm *RebuildManager) RebuildAll(ctx context.Context) error {
	// Check if already rebuilding
	if !atomic.CompareAndSwapInt32(&rm.rebuilding, 0, 1) {
		return fmt.Errorf("rebuild already in progress")
	}
	defer atomic.StoreInt32(&rm.rebuilding, 0)
	
	startTime := time.Now()
	rm.mu.Lock()
	rm.lastRebuildTime = startTime
	rm.mu.Unlock()
	
	// Get all log groups to rebuild
	logGroups, err := rm.getAllLogGroups()
	if err != nil {
		return fmt.Errorf("failed to get log groups: %w", err)
	}
	
	if len(logGroups) == 0 {
		return fmt.Errorf("no log groups found to rebuild")
	}
	
	// Delete existing indexes if configured
	if rm.config.DeleteBeforeRebuild {
		if err := rm.deleteAllIndexes(); err != nil {
			return fmt.Errorf("failed to delete existing indexes: %w", err)
		}
	}
	
	// Reset persistence records
	if rm.persistence != nil {
		if err := rm.resetAllPersistenceRecords(); err != nil {
			return fmt.Errorf("failed to reset persistence records: %w", err)
		}
	}
	
	// Create progress tracker for overall rebuild
	rebuildProgress := &RebuildProgress{
		TotalGroups:     len(logGroups),
		CompletedGroups: 0,
		StartTime:       startTime,
	}
	
	// Process each log group
	errors := make([]error, 0)
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, rm.config.MaxConcurrency)
	
	for _, logGroup := range logGroups {
		wg.Add(1)
		go func(group string) {
			defer wg.Done()
			
			// Acquire semaphore
			semaphore <- struct{}{}
			defer func() { <-semaphore }()
			
			// Check context
			if ctx.Err() != nil {
				return
			}
			
			// Rebuild this log group
			if err := rm.rebuildLogGroup(ctx, group); err != nil {
				rm.mu.Lock()
				errors = append(errors, fmt.Errorf("failed to rebuild group %s: %w", group, err))
				rm.mu.Unlock()
			} else {
				// Update progress
				rm.mu.Lock()
				rebuildProgress.CompletedGroups++
				rm.mu.Unlock()
				
				// Notify progress
				rm.notifyRebuildProgress(rebuildProgress)
			}
		}(logGroup)
	}
	
	// Wait for all groups to complete
	wg.Wait()
	
	// Check for errors
	if len(errors) > 0 {
		return fmt.Errorf("rebuild completed with %d errors: %v", len(errors), errors)
	}
	
	rebuildProgress.CompletedTime = time.Now()
	rebuildProgress.Duration = time.Since(startTime)
	
	// Notify completion
	rm.notifyRebuildComplete(rebuildProgress)
	
	return nil
}

// RebuildSingle rebuilds index for a single log group
func (rm *RebuildManager) RebuildSingle(ctx context.Context, logGroupPath string) error {
	// Check if already rebuilding
	if !atomic.CompareAndSwapInt32(&rm.rebuilding, 0, 1) {
		return fmt.Errorf("rebuild already in progress")
	}
	defer atomic.StoreInt32(&rm.rebuilding, 0)
	
	startTime := time.Now()
	
	// Delete existing index for this log group if configured
	if rm.config.DeleteBeforeRebuild {
		if err := rm.deleteLogGroupIndex(logGroupPath); err != nil {
			return fmt.Errorf("failed to delete existing index: %w", err)
		}
	}
	
	// Reset persistence records for this group
	if rm.persistence != nil {
		if err := rm.resetLogGroupPersistence(logGroupPath); err != nil {
			return fmt.Errorf("failed to reset persistence: %w", err)
		}
	}
	
	// Rebuild the log group
	if err := rm.rebuildLogGroup(ctx, logGroupPath); err != nil {
		return fmt.Errorf("failed to rebuild log group: %w", err)
	}
	
	duration := time.Since(startTime)
	
	// Notify completion
	rm.notifySingleRebuildComplete(logGroupPath, duration)
	
	return nil
}

// rebuildLogGroup rebuilds index for a single log group
func (rm *RebuildManager) rebuildLogGroup(ctx context.Context, logGroupPath string) error {
	// Get all files for this log group
	files, err := rm.discoverLogGroupFiles(logGroupPath)
	if err != nil {
		return fmt.Errorf("failed to discover files: %w", err)
	}
	
	if len(files) == 0 {
		return fmt.Errorf("no files found for log group %s", logGroupPath)
	}
	
	// Create progress tracker for this log group
	progressConfig := &ProgressConfig{
		OnProgress: func(pn ProgressNotification) {
			// Handle progress notifications
			rm.handleProgressNotification(logGroupPath, pn)
		},
		OnCompletion: func(cn CompletionNotification) {
			// Handle completion notifications
			rm.handleCompletionNotification(logGroupPath, cn)
		},
	}
	
	tracker := rm.progressManager.GetTracker(logGroupPath, progressConfig)
	
	// Add all files to tracker
	for _, file := range files {
		tracker.AddFile(file.Path, file.IsCompressed)
		if file.EstimatedLines > 0 {
			tracker.SetFileEstimate(file.Path, file.EstimatedLines)
		}
		if file.Size > 0 {
			tracker.SetFileSize(file.Path, file.Size)
		}
	}
	
	// Process each file
	for _, file := range files {
		// Check context
		if ctx.Err() != nil {
			tracker.FailFile(file.Path, ctx.Err().Error())
			return ctx.Err()
		}
		
		// Create file-specific context with timeout
		fileCtx, cancel := context.WithTimeout(ctx, rm.config.TimeoutPerFile)
		
		// Start processing
		tracker.StartFile(file.Path)
		
		// Index the file
		err := rm.indexFile(fileCtx, file, tracker)
		cancel()
		
		if err != nil {
			tracker.FailFile(file.Path, err.Error())
			return fmt.Errorf("failed to index file %s: %w", file.Path, err)
		}
		
		// Mark as completed
		tracker.CompleteFile(file.Path, file.ProcessedLines)
		
		// Update persistence
		if rm.persistence != nil {
			if err := rm.persistence.MarkFileAsIndexed(file.Path, file.DocumentCount, file.LastPosition); err != nil {
				// Log but don't fail
				// logger.Warnf("Failed to update persistence for %s: %v", file.Path, err)
			}
		}
	}
	
	return nil
}

// LogGroupFile represents a file in a log group
type LogGroupFile struct {
	Path           string
	Size           int64
	IsCompressed   bool
	EstimatedLines int64
	ProcessedLines int64
	DocumentCount  uint64
	LastPosition   int64
}

// discoverLogGroupFiles discovers all files for a log group
func (rm *RebuildManager) discoverLogGroupFiles(logGroupPath string) ([]*LogGroupFile, error) {
	dir := filepath.Dir(logGroupPath)
	
	// Remove any rotation suffixes to get the base name
	mainPath := getMainLogPathFromFile(logGroupPath)
	
	files := make([]*LogGroupFile, 0)
	
	// Walk the directory to find related files
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip files we can't access
		}
		
		// Skip directories
		if info.IsDir() {
			return nil
		}
		
		// Check if this file belongs to the log group
		if getMainLogPathFromFile(path) == mainPath {
			file := &LogGroupFile{
				Path:         path,
				Size:         info.Size(),
				IsCompressed: IsCompressedFile(path),
			}
			
			// Estimate lines
			ctx := context.Background()
			if lines, err := EstimateFileLines(ctx, path, info.Size(), file.IsCompressed); err == nil {
				file.EstimatedLines = lines
			}
			
			files = append(files, file)
		}
		
		return nil
	})
	
	if err != nil {
		return nil, err
	}
	
	return files, nil
}

// indexFile indexes a single file
func (rm *RebuildManager) indexFile(ctx context.Context, file *LogGroupFile, tracker *ProgressTracker) error {
	// Create a batch writer
	batch := NewBatchWriter(rm.indexer, rm.config.BatchSize)
	defer batch.Flush()
	
	// Open and process the file
	// This is simplified - in real implementation, you would:
	// 1. Open the file (handling compression)
	// 2. Parse log lines
	// 3. Create documents
	// 4. Add to batch
	// 5. Update progress
	
	// For now, return a placeholder implementation
	file.ProcessedLines = file.EstimatedLines
	file.DocumentCount = uint64(file.EstimatedLines)
	file.LastPosition = file.Size
	
	// Update progress periodically
	tracker.UpdateFileProgress(file.Path, file.ProcessedLines)
	
	return nil
}

// getAllLogGroups returns all unique log groups
func (rm *RebuildManager) getAllLogGroups() ([]string, error) {
	if rm.persistence == nil {
		return []string{}, nil
	}
	
	indexes, err := rm.persistence.GetAllLogIndexes()
	if err != nil {
		return nil, err
	}
	
	// Use map to get unique main log paths
	groups := make(map[string]struct{})
	for _, idx := range indexes {
		groups[idx.MainLogPath] = struct{}{}
	}
	
	// Convert to slice
	result := make([]string, 0, len(groups))
	for group := range groups {
		result = append(result, group)
	}
	
	return result, nil
}

// deleteAllIndexes deletes all existing indexes
func (rm *RebuildManager) deleteAllIndexes() error {
	// Get all shards
	shards := rm.shardManager.GetAllShards()
	
	// Delete each shard
	for i, shard := range shards {
		if shard != nil {
			if err := shard.Close(); err != nil {
				return fmt.Errorf("failed to close shard %d: %w", i, err)
			}
		}
	}
	
	// Recreate shards
	// This would typically be done by recreating the shard manager
	// For now, return nil as placeholder
	return nil
}

// deleteLogGroupIndex deletes index for a specific log group
func (rm *RebuildManager) deleteLogGroupIndex(logGroupPath string) error {
	// In a real implementation, this would:
	// 1. Find all documents for this log group
	// 2. Delete them from the appropriate shards
	// For now, return nil as placeholder
	return nil
}

// resetAllPersistenceRecords resets all persistence records
func (rm *RebuildManager) resetAllPersistenceRecords() error {
	if rm.persistence == nil {
		return nil
	}
	
	indexes, err := rm.persistence.GetAllLogIndexes()
	if err != nil {
		return err
	}
	
	for _, idx := range indexes {
		idx.Reset()
		if err := rm.persistence.SaveLogIndex(idx); err != nil {
			return fmt.Errorf("failed to reset index %s: %w", idx.Path, err)
		}
	}
	
	return nil
}

// resetLogGroupPersistence resets persistence for a log group
func (rm *RebuildManager) resetLogGroupPersistence(logGroupPath string) error {
	if rm.persistence == nil {
		return nil
	}
	
	indexes, err := rm.persistence.GetLogGroupIndexes(logGroupPath)
	if err != nil {
		return err
	}
	
	for _, idx := range indexes {
		idx.Reset()
		if err := rm.persistence.SaveLogIndex(idx); err != nil {
			return fmt.Errorf("failed to reset index %s: %w", idx.Path, err)
		}
	}
	
	return nil
}

// RebuildProgress tracks rebuild progress
type RebuildProgress struct {
	TotalGroups     int
	CompletedGroups int
	StartTime       time.Time
	CompletedTime   time.Time
	Duration        time.Duration
	CurrentGroup    string
	CurrentFile     string
	Errors          []error
}

// notification methods
func (rm *RebuildManager) notifyRebuildProgress(progress *RebuildProgress) {
	// Emit progress event
	// This would typically publish to an event bus
}

func (rm *RebuildManager) notifyRebuildComplete(progress *RebuildProgress) {
	// Emit completion event
}

func (rm *RebuildManager) notifySingleRebuildComplete(logGroupPath string, duration time.Duration) {
	// Emit single rebuild completion event
}

func (rm *RebuildManager) handleProgressNotification(logGroupPath string, pn ProgressNotification) {
	// Handle progress notification from tracker
}

func (rm *RebuildManager) handleCompletionNotification(logGroupPath string, cn CompletionNotification) {
	// Handle completion notification from tracker
}

// IsRebuilding returns true if rebuild is in progress
func (rm *RebuildManager) IsRebuilding() bool {
	return atomic.LoadInt32(&rm.rebuilding) == 1
}

// GetLastRebuildTime returns the time of the last rebuild
func (rm *RebuildManager) GetLastRebuildTime() time.Time {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	return rm.lastRebuildTime
}

// GetRebuildStats returns statistics about rebuild operations
type RebuildStats struct {
	IsRebuilding    bool      `json:"is_rebuilding"`
	LastRebuildTime time.Time `json:"last_rebuild_time"`
	Config          *RebuildConfig `json:"config"`
}

func (rm *RebuildManager) GetRebuildStats() *RebuildStats {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	
	return &RebuildStats{
		IsRebuilding:    rm.IsRebuilding(),
		LastRebuildTime: rm.lastRebuildTime,
		Config:          rm.config,
	}
}