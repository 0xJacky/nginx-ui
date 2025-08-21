package nginx_log

import (
	"sync"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/event"
	"github.com/uozi-tech/cosy/logger"
)

// ProgressTracker manages progress tracking for log group indexing
type ProgressTracker struct {
	mu                 sync.RWMutex
	logGroupPath       string
	startTime          int64 // Unix timestamp
	files              map[string]*FileProgress
	totalEstimate      int64 // Total estimated lines across all files
	totalActual        int64 // Total actual lines processed
	isCompleted        bool
	completionNotified bool // Flag to prevent duplicate completion notifications
	lastNotify         int64 // Unix timestamp
}

// FileProgress tracks progress for individual files
type FileProgress struct {
	FilePath       string
	State          FileState
	EstimatedLines int64 // Estimated total lines in this file
	ProcessedLines int64 // Actually processed lines
	FileSize       int64 // Total file size in bytes (compressed size for .gz files)
	CurrentPos     int64 // Current reading position in bytes (for uncompressed files only)
	AvgLineSize    int64 // Dynamic average line size in bytes (for compressed files)
	SampleCount    int64 // Number of lines sampled for average calculation
	IsCompressed   bool
	StartTime      int64 // Unix timestamp
	CompletedTime  int64 // Unix timestamp
}

// FileState represents the current state of file processing
type FileState int

const (
	FileStatePending FileState = iota
	FileStateProcessing
	FileStateCompleted
)

func (fs FileState) String() string {
	switch fs {
	case FileStatePending:
		return "pending"
	case FileStateProcessing:
		return "processing"
	case FileStateCompleted:
		return "completed"
	default:
		return "unknown"
	}
}

// NewProgressTracker creates a new progress tracker for a log group
func NewProgressTracker(logGroupPath string) *ProgressTracker {
	return &ProgressTracker{
		logGroupPath:       logGroupPath,
		startTime:          time.Now().Unix(),
		files:              make(map[string]*FileProgress),
		completionNotified: false,
	}
}

// AddFile adds a file to the progress tracker
func (pt *ProgressTracker) AddFile(filePath string, isCompressed bool) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	pt.files[filePath] = &FileProgress{
		FilePath:     filePath,
		State:        FileStatePending,
		IsCompressed: isCompressed,
		AvgLineSize:  120, // Initial estimate: 120 bytes per line
		SampleCount:  0,
	}

	logger.Debugf("Added file to progress tracker: %s (compressed: %v)", filePath, isCompressed)
}

// SetFileEstimate sets the estimated line count for a file
func (pt *ProgressTracker) SetFileEstimate(filePath string, estimatedLines int64) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	if progress, exists := pt.files[filePath]; exists {
		oldEstimate := progress.EstimatedLines
		progress.EstimatedLines = estimatedLines

		// Update total estimate
		pt.totalEstimate = pt.totalEstimate - oldEstimate + estimatedLines

		logger.Debugf("Updated file estimate for %s: %d lines (total estimate: %d)",
			filePath, estimatedLines, pt.totalEstimate)
	}
}

// SetFileSize sets the file size for a file
func (pt *ProgressTracker) SetFileSize(filePath string, fileSize int64) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	if progress, exists := pt.files[filePath]; exists {
		progress.FileSize = fileSize
		logger.Debugf("Set file size for %s: %d bytes", filePath, fileSize)
	}
}

// UpdateFilePosition updates the current reading position for files
func (pt *ProgressTracker) UpdateFilePosition(filePath string, currentPos int64, linesProcessed int64) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	if progress, exists := pt.files[filePath]; exists {
		progress.ProcessedLines = linesProcessed

		if progress.IsCompressed {
			// For compressed files, update average line size dynamically
			if linesProcessed > 0 {
				// Use the first 1000 lines to establish a good average, then update less frequently
				if progress.SampleCount < 1000 || progress.SampleCount%100 == 0 {
					// Calculate current average line size based on processed data
					// For compressed files, we estimate based on processed lines and compression ratio
					estimatedUncompressedBytes := progress.FileSize * 3 // Assume 3:1 compression ratio
					newAvgLineSize := estimatedUncompressedBytes / linesProcessed
					if newAvgLineSize > 50 && newAvgLineSize < 5000 { // Sanity check: 50-5000 bytes per line
						// Smooth the average to avoid sudden jumps
						if progress.SampleCount > 0 {
							progress.AvgLineSize = (progress.AvgLineSize + newAvgLineSize) / 2
						} else {
							progress.AvgLineSize = newAvgLineSize
						}
					}
				}
				progress.SampleCount = linesProcessed
			}
		} else {
			// For uncompressed files, update current position
			progress.CurrentPos = currentPos
		}
	}
}

// StartFile marks a file as started processing
func (pt *ProgressTracker) StartFile(filePath string) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	if progress, exists := pt.files[filePath]; exists {
		progress.State = FileStateProcessing
		progress.StartTime = time.Now().Unix()

		logger.Debugf("Started processing file: %s", filePath)
		pt.notifyProgressLocked()
	}
}

// UpdateFileProgress updates the processed line count for a file
func (pt *ProgressTracker) UpdateFileProgress(filePath string, processedLines int64) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	if progress, exists := pt.files[filePath]; exists {
		oldProcessed := progress.ProcessedLines
		progress.ProcessedLines = processedLines

		// Update total actual processed
		pt.totalActual = pt.totalActual - oldProcessed + processedLines

		// Notify progress if enough time has passed
		pt.notifyProgressLocked()
	}
}

// CompleteFile marks a file as completed
func (pt *ProgressTracker) CompleteFile(filePath string, finalProcessedLines int64) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	if progress, exists := pt.files[filePath]; exists {
		oldProcessed := progress.ProcessedLines
		progress.ProcessedLines = finalProcessedLines
		progress.State = FileStateCompleted
		progress.CompletedTime = time.Now().Unix()

		// Update total actual processed
		pt.totalActual = pt.totalActual - oldProcessed + finalProcessedLines

		logger.Debugf("Completed processing file: %s (%d lines)", filePath, finalProcessedLines)

		// Check if all files are completed and we haven't notified yet
		if !pt.completionNotified {
			allCompleted := true
			for _, fp := range pt.files {
				if fp.State != FileStateCompleted {
					allCompleted = false
					break
				}
			}

			if allCompleted {
				pt.isCompleted = true
				pt.completionNotified = true // Mark as notified to prevent duplicates
				pt.notifyCompletionLocked()
			} else {
				pt.notifyProgressLocked()
			}
		}
	}
}

// GetProgress returns the current progress percentage and stats
func (pt *ProgressTracker) GetProgress() (percentage float64, stats ProgressStats) {
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	stats = ProgressStats{
		LogGroupPath:   pt.logGroupPath,
		TotalFiles:     len(pt.files),
		ProcessedLines: pt.totalActual,
		EstimatedLines: pt.totalEstimate,
		StartTime:      pt.startTime,
		IsCompleted:    pt.isCompleted,
	}

	// Count completed files
	for _, fp := range pt.files {
		switch fp.State {
		case FileStateCompleted:
			stats.CompletedFiles++
		case FileStateProcessing:
			stats.ProcessingFiles++
		}
	}

	// Calculate progress percentage
	if pt.totalEstimate > 0 {
		percentage = float64(pt.totalActual) / float64(pt.totalEstimate) * 100
	} else if stats.TotalFiles > 0 {
		// Fallback to file-based progress if no line estimates
		percentage = float64(stats.CompletedFiles) / float64(stats.TotalFiles) * 100
	}

	// Cap at 100%
	if percentage > 100 {
		percentage = 100
	}

	return percentage, stats
}

// ProgressStats contains progress statistics
type ProgressStats struct {
	LogGroupPath    string
	TotalFiles      int
	CompletedFiles  int
	ProcessingFiles int
	ProcessedLines  int64
	EstimatedLines  int64
	StartTime       int64 // Unix timestamp
	IsCompleted     bool
}

// notifyProgressLocked sends progress notification (must be called with lock held)
func (pt *ProgressTracker) notifyProgressLocked() {
	// Throttle notifications to avoid spam
	now := time.Now().Unix()
	if now-pt.lastNotify < 2 {
		return
	}
	pt.lastNotify = now

	percentage, stats := pt.getProgressLocked()

	elapsed := (time.Now().Unix() - pt.startTime) * 1000 // Convert to milliseconds
	var estimatedRemain int64

	if percentage > 0 && percentage < 100 {
		avgTimePerPercent := float64(elapsed) / percentage
		remainingPercent := 100.0 - percentage
		estimatedRemain = int64(avgTimePerPercent * remainingPercent)
	}

	eventData := event.NginxLogIndexProgressData{
		LogPath:         pt.logGroupPath,
		Progress:        percentage,
		Stage:           "indexing",
		Status:          "running",
		ElapsedTime:     elapsed,
		EstimatedRemain: estimatedRemain,
	}

	logger.Debugf("Progress update for %s: %.1f%% (%d/%d files, %d/%d lines)",
		pt.logGroupPath, percentage, stats.CompletedFiles, stats.TotalFiles,
		stats.ProcessedLines, stats.EstimatedLines)

	event.Publish(event.Event{
		Type: event.EventTypeNginxLogIndexProgress,
		Data: eventData,
	})
}

// notifyCompletionLocked sends completion notification (must be called with lock held)
func (pt *ProgressTracker) notifyCompletionLocked() {
	elapsed := (time.Now().Unix() - pt.startTime) * 1000 // Convert to milliseconds

	// Calculate total size processed using improved estimation
	var totalSize int64
	for _, fp := range pt.files {
		if fp.IsCompressed {
			// For compressed files, use dynamic average line size
			totalSize += fp.ProcessedLines * fp.AvgLineSize
		} else {
			// For uncompressed files, use actual bytes processed if available, otherwise estimate
			if fp.CurrentPos > 0 {
				totalSize += fp.CurrentPos
			} else {
				// Fallback to line-based estimation with improved calculation
				totalSize += fp.ProcessedLines * 150
			}
		}
	}

	completeEventData := event.NginxLogIndexCompleteData{
		LogPath:     pt.logGroupPath,
		Success:     true,
		Duration:    elapsed,
		TotalLines:  pt.totalActual,
		IndexedSize: totalSize,
		Error:       "",
	}

	event.Publish(event.Event{
		Type: event.EventTypeNginxLogIndexComplete,
		Data: completeEventData,
	})

	// Also publish index ready event for table refresh
	event.Publish(event.Event{
		Type: event.EventTypeNginxLogIndexReady,
		Data: map[string]interface{}{
			"log_path": pt.logGroupPath,
			"success":  true,
		},
	})

	logger.Infof("Log group indexing completed for %s: %d files, %d lines processed in %dms (SINGLE NOTIFICATION)",
		pt.logGroupPath, len(pt.files), pt.totalActual, elapsed)
}

// getProgressLocked returns progress without notification (must be called with lock held)
func (pt *ProgressTracker) getProgressLocked() (float64, ProgressStats) {
	stats := ProgressStats{
		LogGroupPath:   pt.logGroupPath,
		TotalFiles:     len(pt.files),
		ProcessedLines: pt.totalActual,
		EstimatedLines: pt.totalEstimate,
		StartTime:      pt.startTime,
		IsCompleted:    pt.isCompleted,
	}

	// Count completed files
	for _, fp := range pt.files {
		switch fp.State {
		case FileStateCompleted:
			stats.CompletedFiles++
		case FileStateProcessing:
			stats.ProcessingFiles++
		}
	}

	// Calculate progress percentage
	var percentage float64
	if pt.totalEstimate > 0 {
		percentage = float64(pt.totalActual) / float64(pt.totalEstimate) * 100
	} else if stats.TotalFiles > 0 {
		// Fallback to file-based progress if no line estimates
		percentage = float64(stats.CompletedFiles) / float64(stats.TotalFiles) * 100
	}

	// Cap at 100%
	if percentage > 100 {
		percentage = 100
	}

	return percentage, stats
}

// GlobalProgressManager manages all progress trackers
type GlobalProgressManager struct {
	mu       sync.RWMutex
	trackers map[string]*ProgressTracker
}

var globalProgressManager = &GlobalProgressManager{
	trackers: make(map[string]*ProgressTracker),
}

// GetProgressTracker gets or creates a progress tracker for a log group
func GetProgressTracker(logGroupPath string) *ProgressTracker {
	globalProgressManager.mu.Lock()
	defer globalProgressManager.mu.Unlock()

	if tracker, exists := globalProgressManager.trackers[logGroupPath]; exists {
		return tracker
	}

	tracker := NewProgressTracker(logGroupPath)
	globalProgressManager.trackers[logGroupPath] = tracker
	return tracker
}

// RemoveProgressTracker removes a progress tracker (called when indexing is complete)
func RemoveProgressTracker(logGroupPath string) {
	globalProgressManager.mu.Lock()
	defer globalProgressManager.mu.Unlock()

	delete(globalProgressManager.trackers, logGroupPath)
	logger.Debugf("Removed progress tracker for log group: %s", logGroupPath)
}

// EstimateFileLines estimates the number of lines in a file based on sampling
func EstimateFileLines(filePath string, fileSize int64, isCompressed bool) int64 {
	if isCompressed {
		// For compressed files, estimate based on compression ratio and average line size
		// Assume 3:1 compression ratio and 100 bytes average per line
		estimatedUncompressedSize := fileSize * 3
		return estimatedUncompressedSize / 100
	}

	// For uncompressed files, assume average 100 bytes per line
	if fileSize == 0 {
		return 0
	}

	return fileSize / 100 // Rough estimate
}
