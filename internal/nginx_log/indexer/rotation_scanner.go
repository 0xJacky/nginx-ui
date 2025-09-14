package indexer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/uozi-tech/cosy/logger"
)

// RotationScanner efficiently scans and prioritizes rotation logs for indexing
type RotationScanner struct {
	config         *RotationScanConfig
	scanResults    map[string]*LogGroupScanResult
	resultsMutex   sync.RWMutex
	priorityQueue  []*RotationLogFileInfo
	queueMutex     sync.Mutex
	scanInProgress bool
	progressMutex  sync.Mutex
}

// RotationScanConfig configures the rotation log scanner
type RotationScanConfig struct {
	// Scanning parameters
	MaxConcurrentScans int           `json:"max_concurrent_scans"`
	ScanTimeout        time.Duration `json:"scan_timeout"`
	MinFileSize        int64         `json:"min_file_size"`
	MaxFileAge         time.Duration `json:"max_file_age"`

	// Throughput optimization
	PrioritizeBySize    bool     `json:"prioritize_by_size"`
	PrioritizeByAge     bool     `json:"prioritize_by_age"`
	PreferredExtensions []string `json:"preferred_extensions"`
	ExcludePatterns     []string `json:"exclude_patterns"`

	// Performance settings
	EnableParallelScan bool `json:"enable_parallel_scan"`
	ScanBatchSize      int  `json:"scan_batch_size"`
}

// LogGroupScanResult contains the scan results for a log group
type LogGroupScanResult struct {
	BasePath       string                 `json:"base_path"`
	Files          []*RotationLogFileInfo `json:"files"`
	TotalSize      int64                  `json:"total_size"`
	TotalFiles     int                    `json:"total_files"`
	ScanTime       time.Time              `json:"scan_time"`
	ScanDuration   time.Duration          `json:"scan_duration"`
	EstimatedLines int64                  `json:"estimated_lines"`
}

// LogFileInfo contains detailed information about a log file
type RotationLogFileInfo struct {
	Path           string    `json:"path"`
	Size           int64     `json:"size"`
	ModTime        time.Time `json:"mod_time"`
	IsCompressed   bool      `json:"is_compressed"`
	RotationIndex  int       `json:"rotation_index"`
	EstimatedLines int64     `json:"estimated_lines"`
	Priority       int       `json:"priority"`
	MainLogPath    string    `json:"main_log_path"`
}

// NewRotationScanner creates a new rotation log scanner
func NewRotationScanner(config *RotationScanConfig) *RotationScanner {
	if config == nil {
		config = DefaultRotationScanConfig()
	}

	return &RotationScanner{
		config:        config,
		scanResults:   make(map[string]*LogGroupScanResult),
		priorityQueue: make([]*RotationLogFileInfo, 0),
	}
}

// DefaultRotationScanConfig returns default configuration for rotation scanning
func DefaultRotationScanConfig() *RotationScanConfig {
	return &RotationScanConfig{
		MaxConcurrentScans:  8,
		ScanTimeout:         30 * time.Second,
		MinFileSize:         1024,                // 1KB minimum
		MaxFileAge:          30 * 24 * time.Hour, // 30 days
		PrioritizeBySize:    true,
		PrioritizeByAge:     true,
		PreferredExtensions: []string{".log", ".gz"},
		ExcludePatterns:     []string{"*.tmp", "*.lock", "*.swap"},
		EnableParallelScan:  true,
		ScanBatchSize:       50,
	}
}

// ScanLogGroups scans multiple log groups and builds a prioritized queue
func (rs *RotationScanner) ScanLogGroups(ctx context.Context, basePaths []string) error {
	rs.progressMutex.Lock()
	if rs.scanInProgress {
		rs.progressMutex.Unlock()
		return fmt.Errorf("scan already in progress")
	}
	rs.scanInProgress = true
	rs.progressMutex.Unlock()

	defer func() {
		rs.progressMutex.Lock()
		rs.scanInProgress = false
		rs.progressMutex.Unlock()
	}()

	logger.Infof("🔍 Starting rotation log scan for %d log groups", len(basePaths))

	if rs.config.EnableParallelScan {
		return rs.scanLogGroupsParallel(ctx, basePaths)
	} else {
		return rs.scanLogGroupsSequential(ctx, basePaths)
	}
}

// scanLogGroupsParallel scans log groups in parallel for maximum throughput
func (rs *RotationScanner) scanLogGroupsParallel(ctx context.Context, basePaths []string) error {
	semaphore := make(chan struct{}, rs.config.MaxConcurrentScans)
	var wg sync.WaitGroup
	errors := make(chan error, len(basePaths))

	for _, basePath := range basePaths {
		wg.Add(1)
		go func(path string) {
			defer wg.Done()

			select {
			case semaphore <- struct{}{}:
				defer func() { <-semaphore }()

				if err := rs.scanSingleLogGroup(ctx, path); err != nil {
					errors <- fmt.Errorf("failed to scan %s: %w", path, err)
				}
			case <-ctx.Done():
				errors <- ctx.Err()
				return
			}
		}(basePath)
	}

	wg.Wait()
	close(errors)

	// Collect any errors
	var scanErrors []error
	for err := range errors {
		if err != nil {
			scanErrors = append(scanErrors, err)
		}
	}

	if len(scanErrors) > 0 {
		logger.Warnf("Encountered %d errors during parallel scan: %v", len(scanErrors), scanErrors)
	}

	// Build priority queue from all scan results
	rs.buildPriorityQueue()

	logger.Infof("✅ Rotation log scan completed: %d log groups, %d total files",
		len(rs.scanResults), len(rs.priorityQueue))

	return nil
}

// scanLogGroupsSequential scans log groups sequentially
func (rs *RotationScanner) scanLogGroupsSequential(ctx context.Context, basePaths []string) error {
	for _, basePath := range basePaths {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			if err := rs.scanSingleLogGroup(ctx, basePath); err != nil {
				logger.Warnf("Failed to scan log group %s: %v", basePath, err)
				// Continue with other groups
			}
		}
	}

	rs.buildPriorityQueue()
	return nil
}

// scanSingleLogGroup scans a single log group for rotation logs
func (rs *RotationScanner) scanSingleLogGroup(ctx context.Context, basePath string) error {
	start := time.Now()

	// Use optimized glob pattern for rotation logs
	patterns := []string{
		basePath,        // Main log file
		basePath + ".*", // access.log.1, access.log.2.gz, etc.
		basePath + "-*", // access.log-20240101, etc.
		strings.TrimSuffix(basePath, ".log") + ".*.log*", // For patterns like access.20240101.log.gz
	}

	var allFiles []*RotationLogFileInfo
	seen := make(map[string]struct{})

	for _, pattern := range patterns {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			continue // Skip problematic patterns
		}

		for _, match := range matches {
			if _, exists := seen[match]; exists {
				continue
			}
			seen[match] = struct{}{}

			info, err := os.Stat(match)
			if err != nil || !info.Mode().IsRegular() {
				continue
			}

			// Apply file filters
			if info.Size() < rs.config.MinFileSize {
				continue
			}

			if time.Since(info.ModTime()) > rs.config.MaxFileAge {
				continue
			}

			if rs.shouldExcludeFile(match) {
				continue
			}

			logFile := &RotationLogFileInfo{
				Path:           match,
				Size:           info.Size(),
				ModTime:        info.ModTime(),
				IsCompressed:   strings.HasSuffix(match, ".gz"),
				RotationIndex:  rs.extractRotationIndex(match, basePath),
				EstimatedLines: rs.estimateLineCount(info.Size(), strings.HasSuffix(match, ".gz")),
				MainLogPath:    basePath,
			}

			logFile.Priority = rs.calculatePriority(logFile)
			allFiles = append(allFiles, logFile)
		}
	}

	// Sort files by priority and rotation index
	sort.Slice(allFiles, func(i, j int) bool {
		if allFiles[i].Priority != allFiles[j].Priority {
			return allFiles[i].Priority > allFiles[j].Priority // Higher priority first
		}
		return allFiles[i].RotationIndex < allFiles[j].RotationIndex // Newer files first
	})

	// Calculate totals
	var totalSize int64
	var estimatedLines int64
	for _, file := range allFiles {
		totalSize += file.Size
		estimatedLines += file.EstimatedLines
	}

	result := &LogGroupScanResult{
		BasePath:       basePath,
		Files:          allFiles,
		TotalSize:      totalSize,
		TotalFiles:     len(allFiles),
		ScanTime:       start,
		ScanDuration:   time.Since(start),
		EstimatedLines: estimatedLines,
	}

	rs.resultsMutex.Lock()
	rs.scanResults[basePath] = result
	rs.resultsMutex.Unlock()

	logger.Debugf("📊 Scanned log group %s: %d files, %s total, %d estimated lines",
		basePath, len(allFiles), formatSize(totalSize), estimatedLines)

	return nil
}

// buildPriorityQueue builds a global priority queue from all scan results
func (rs *RotationScanner) buildPriorityQueue() {
	rs.queueMutex.Lock()
	defer rs.queueMutex.Unlock()

	rs.priorityQueue = rs.priorityQueue[:0] // Clear existing queue

	rs.resultsMutex.RLock()
	for _, result := range rs.scanResults {
		rs.priorityQueue = append(rs.priorityQueue, result.Files...)
	}
	rs.resultsMutex.RUnlock()

	// Sort by priority
	sort.Slice(rs.priorityQueue, func(i, j int) bool {
		if rs.priorityQueue[i].Priority != rs.priorityQueue[j].Priority {
			return rs.priorityQueue[i].Priority > rs.priorityQueue[j].Priority
		}
		// Secondary sort by size for throughput
		return rs.priorityQueue[i].Size > rs.priorityQueue[j].Size
	})

	logger.Infof("🚀 Built priority queue with %d files for optimized indexing", len(rs.priorityQueue))
}

// GetNextBatch returns the next batch of files to index, prioritized for throughput
func (rs *RotationScanner) GetNextBatch(batchSize int) []*RotationLogFileInfo {
	rs.queueMutex.Lock()
	defer rs.queueMutex.Unlock()

	if len(rs.priorityQueue) == 0 {
		return []*RotationLogFileInfo{} // Return empty slice instead of nil
	}

	end := minInt(batchSize, len(rs.priorityQueue))
	batch := make([]*RotationLogFileInfo, end)
	copy(batch, rs.priorityQueue[:end])

	// Remove processed files from queue
	rs.priorityQueue = rs.priorityQueue[end:]

	return batch
}

// extractRotationIndex extracts rotation index from file path
func (rs *RotationScanner) extractRotationIndex(filePath, basePath string) int {
	if filePath == basePath {
		return 0 // Main log file
	}

	// Extract numeric suffix (e.g., access.log.1 -> 1)
	suffix := strings.TrimPrefix(filePath, basePath)

	// Handle various rotation formats
	if strings.HasPrefix(suffix, ".") {
		suffix = strings.TrimPrefix(suffix, ".")
		// Always trim optional ".gz" suffix
		suffix = strings.TrimSuffix(suffix, ".gz")

		// Try to parse as integer
		var index int
		if n, err := fmt.Sscanf(suffix, "%d", &index); n == 1 && err == nil {
			return index
		}
	}

	// Default to high index for unknown formats
	return 999
}

// calculatePriority calculates file priority based on configuration
func (rs *RotationScanner) calculatePriority(file *RotationLogFileInfo) int {
	priority := 100 // Base priority

	if rs.config.PrioritizeBySize {
		// Larger files get higher priority (better throughput)
		sizeMB := file.Size / (1024 * 1024)
		priority += int(sizeMB/10) * 10 // +10 points per 10MB
	}

	if rs.config.PrioritizeByAge {
		// Newer files get higher priority
		hoursOld := int(time.Since(file.ModTime).Hours())
		if hoursOld < 24 {
			priority += 50 // Recent files
		} else if hoursOld < 168 { // 1 week
			priority += 20
		}
	}

	// Main log file gets highest priority
	if file.RotationIndex == 0 {
		priority += 100
	}

	// Compressed files get slightly lower priority (decompression overhead)
	if file.IsCompressed {
		priority -= 10
	}

	return priority
}

// estimateLineCount estimates the number of log lines in a file
func (rs *RotationScanner) estimateLineCount(size int64, isCompressed bool) int64 {
	avgLineSize := int64(200) // Average nginx log line size in bytes

	if isCompressed {
		// Assume 3x compression ratio for text logs
		size = size * 3
	}

	return size / avgLineSize
}

// shouldExcludeFile checks if file should be excluded based on patterns
func (rs *RotationScanner) shouldExcludeFile(path string) bool {
	filename := filepath.Base(path)

	for _, pattern := range rs.config.ExcludePatterns {
		if matched, _ := filepath.Match(pattern, filename); matched {
			return true
		}
	}

	return false
}

// GetScanResults returns the current scan results
func (rs *RotationScanner) GetScanResults() map[string]*LogGroupScanResult {
	rs.resultsMutex.RLock()
	defer rs.resultsMutex.RUnlock()

	results := make(map[string]*LogGroupScanResult)
	for k, v := range rs.scanResults {
		results[k] = v
	}

	return results
}

// GetQueueSize returns the current number of files in the priority queue
func (rs *RotationScanner) GetQueueSize() int {
	rs.queueMutex.Lock()
	defer rs.queueMutex.Unlock()

	return len(rs.priorityQueue)
}

// IsScanning returns true if a scan is currently in progress
func (rs *RotationScanner) IsScanning() bool {
	rs.progressMutex.Lock()
	defer rs.progressMutex.Unlock()

	return rs.scanInProgress
}

// formatSize formats byte size for display
func formatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
