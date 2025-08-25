package indexer

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ProgressTracker manages progress tracking for indexing operations
type ProgressTracker struct {
	mu                 sync.RWMutex
	logGroupPath       string
	startTime          time.Time
	files              map[string]*FileProgress
	totalEstimate      int64 // Total estimated lines across all files
	totalActual        int64 // Total actual lines processed
	isCompleted        bool
	completionNotified bool // Flag to prevent duplicate completion notifications
	lastNotify         time.Time

	// Callback functions for notifications
	onProgress   func(ProgressNotification)
	onCompletion func(CompletionNotification)
}

// FileProgress tracks progress for individual files
type FileProgress struct {
	FilePath       string    `json:"file_path"`
	State          FileState `json:"state"`
	EstimatedLines int64     `json:"estimated_lines"` // Estimated total lines in this file
	ProcessedLines int64     `json:"processed_lines"` // Actually processed lines
	FileSize       int64     `json:"file_size"`       // Total file size in bytes
	CurrentPos     int64     `json:"current_pos"`     // Current reading position in bytes
	AvgLineSize    int64     `json:"avg_line_size"`   // Dynamic average line size in bytes
	SampleCount    int64     `json:"sample_count"`    // Number of lines sampled for average calculation
	IsCompressed   bool      `json:"is_compressed"`
	StartTime      time.Time `json:"start_time"`
	CompletedTime  time.Time `json:"completed_time"`
	ErrorMsg       string    `json:"error_msg,omitempty"` // Error message if processing failed
}

// FileState represents the current state of file processing
type FileState int

const (
	FileStatePending FileState = iota
	FileStateProcessing
	FileStateCompleted
	FileStateFailed
)

func (fs FileState) String() string {
	switch fs {
	case FileStatePending:
		return "pending"
	case FileStateProcessing:
		return "processing"
	case FileStateCompleted:
		return "completed"
	case FileStateFailed:
		return "failed"
	default:
		return "unknown"
	}
}

// ProgressNotification contains progress update information
type ProgressNotification struct {
	LogGroupPath    string        `json:"log_group_path"`
	Percentage      float64       `json:"percentage"`
	TotalFiles      int           `json:"total_files"`
	CompletedFiles  int           `json:"completed_files"`
	ProcessingFiles int           `json:"processing_files"`
	FailedFiles     int           `json:"failed_files"`
	ProcessedLines  int64         `json:"processed_lines"`
	EstimatedLines  int64         `json:"estimated_lines"`
	ElapsedTime     time.Duration `json:"elapsed_time"`
	EstimatedRemain time.Duration `json:"estimated_remain"`
	IsCompleted     bool          `json:"is_completed"`
}

// CompletionNotification contains completion information
type CompletionNotification struct {
	LogGroupPath string        `json:"log_group_path"`
	Success      bool          `json:"success"`
	Duration     time.Duration `json:"duration"`
	TotalLines   int64         `json:"total_lines"`
	TotalFiles   int           `json:"total_files"`
	FailedFiles  int           `json:"failed_files"`
	IndexedSize  int64         `json:"indexed_size"`
	Error        string        `json:"error,omitempty"`
}

// ProgressConfig contains configuration for progress tracking
type ProgressConfig struct {
	NotifyInterval time.Duration // Minimum time between progress notifications
	OnProgress     func(ProgressNotification)
	OnCompletion   func(CompletionNotification)
}

// NewProgressTracker creates a new progress tracker for indexing operations
func NewProgressTracker(logGroupPath string, config *ProgressConfig) *ProgressTracker {
	pt := &ProgressTracker{
		logGroupPath:       logGroupPath,
		startTime:          time.Now(),
		files:              make(map[string]*FileProgress),
		completionNotified: false,
	}

	if config != nil {
		if config.NotifyInterval == 0 {
			config.NotifyInterval = 2 * time.Second // Default notify interval
		}
		pt.onProgress = config.OnProgress
		pt.onCompletion = config.OnCompletion
	}

	return pt
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
	}
}

// SetFileSize sets the file size for a file
func (pt *ProgressTracker) SetFileSize(filePath string, fileSize int64) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	if progress, exists := pt.files[filePath]; exists {
		progress.FileSize = fileSize
	}
}

// UpdateFileProgress updates the processed line count and position for a file
func (pt *ProgressTracker) UpdateFileProgress(filePath string, processedLines int64, currentPos ...int64) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	if progress, exists := pt.files[filePath]; exists {
		// Update total actual processed, ensuring not to double-count on completion
		if progress.State != FileStateCompleted {
			pt.totalActual = pt.totalActual - progress.ProcessedLines + processedLines
		}
		progress.ProcessedLines = processedLines

		// Update position if provided
		if len(currentPos) > 0 && !progress.IsCompressed {
			progress.CurrentPos = currentPos[0]
		}

		// Update average line size for compressed files
		if progress.IsCompressed && processedLines > 0 && currentPos != nil && len(currentPos) > 0 {
			progress.SampleCount++
			if progress.SampleCount > 0 {
				progress.AvgLineSize = currentPos[0] / processedLines
			}
		}

		// Notify progress if enough time has passed
		pt.notifyProgressLocked()
	}
}

// StartFile marks a file as started processing
func (pt *ProgressTracker) StartFile(filePath string) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	if progress, exists := pt.files[filePath]; exists {
		progress.State = FileStateProcessing
		progress.StartTime = time.Now()
		pt.notifyProgressLocked()
	}
}

// CompleteFile marks a file as completed successfully
func (pt *ProgressTracker) CompleteFile(filePath string, finalProcessedLines int64) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	if progress, exists := pt.files[filePath]; exists {
		// Prevent marking as completed multiple times
		if progress.State == FileStateCompleted || progress.State == FileStateFailed {
			return
		}

		// Ensure final processed lines are correctly accounted for in total
		pt.totalActual = pt.totalActual - progress.ProcessedLines + finalProcessedLines

		progress.ProcessedLines = finalProcessedLines
		progress.State = FileStateCompleted
		progress.CompletedTime = time.Now()

		pt.checkCompletionLocked()
	}
}

// FailFile marks a file as failed processing
func (pt *ProgressTracker) FailFile(filePath string, errorMsg string) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	if progress, exists := pt.files[filePath]; exists {
		// Prevent marking as failed multiple times
		if progress.State == FileStateCompleted || progress.State == FileStateFailed {
			return
		}

		progress.State = FileStateFailed
		progress.ErrorMsg = errorMsg
		progress.CompletedTime = time.Now()

		pt.checkCompletionLocked()
	}
}

// GetProgress returns the current progress percentage and stats
func (pt *ProgressTracker) GetProgress() ProgressNotification {
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	return pt.getProgressLocked()
}

// GetFileProgress returns progress for a specific file
func (pt *ProgressTracker) GetFileProgress(filePath string) (*FileProgress, bool) {
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	progress, exists := pt.files[filePath]
	if !exists {
		return nil, false
	}

	// Return a copy to avoid race conditions
	copy := *progress
	return &copy, true
}

// GetAllFiles returns all file progress information
func (pt *ProgressTracker) GetAllFiles() map[string]*FileProgress {
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	result := make(map[string]*FileProgress)
	for path, progress := range pt.files {
		copy := *progress
		result[path] = &copy
	}
	return result
}

// IsCompleted returns whether all files have been processed
func (pt *ProgressTracker) IsCompleted() bool {
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	return pt.isCompleted
}

// Cancel marks the tracker as cancelled and stops processing
func (pt *ProgressTracker) Cancel(reason string) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	for _, progress := range pt.files {
		if progress.State == FileStateProcessing || progress.State == FileStatePending {
			progress.State = FileStateFailed
			progress.ErrorMsg = "cancelled: " + reason
			progress.CompletedTime = time.Now()
		}
	}

	pt.isCompleted = true
	pt.notifyCompletionLocked()
}

// checkCompletionLocked checks if all files are completed and notifies if so
func (pt *ProgressTracker) checkCompletionLocked() {
	if pt.completionNotified {
		return
	}

	allCompleted := true
	for _, fp := range pt.files {
		if fp.State != FileStateCompleted && fp.State != FileStateFailed {
			allCompleted = false
			break
		}
	}

	if allCompleted {
		pt.isCompleted = true
		pt.completionNotified = true
		pt.notifyCompletionLocked()
	} else {
		pt.notifyProgressLocked()
	}
}

// notifyProgressLocked sends progress notification (must be called with lock held)
func (pt *ProgressTracker) notifyProgressLocked() {
	// Throttle notifications to avoid spam
	now := time.Now()
	if now.Sub(pt.lastNotify) < 2*time.Second {
		return
	}
	pt.lastNotify = now

	if pt.onProgress != nil {
		notification := pt.getProgressLocked()
		go pt.onProgress(notification) // Non-blocking notification
	}
}

// notifyCompletionLocked sends completion notification (must be called with lock held)
func (pt *ProgressTracker) notifyCompletionLocked() {
	if pt.onCompletion == nil {
		return
	}

	elapsed := time.Since(pt.startTime)

	// Calculate total size processed
	var totalSize int64
	var failedFiles int
	for _, fp := range pt.files {
		if fp.State == FileStateFailed {
			failedFiles++
			continue
		}

		if fp.IsCompressed {
			// For compressed files, use dynamic average line size
			totalSize += fp.ProcessedLines * fp.AvgLineSize
		} else {
			// For uncompressed files, use actual bytes processed if available
			if fp.CurrentPos > 0 {
				totalSize += fp.CurrentPos
			} else {
				// Fallback to line-based estimation
				totalSize += fp.ProcessedLines * 150
			}
		}
	}

	notification := CompletionNotification{
		LogGroupPath: pt.logGroupPath,
		Success:      failedFiles == 0,
		Duration:     elapsed,
		TotalLines:   pt.totalActual,
		TotalFiles:   len(pt.files),
		FailedFiles:  failedFiles,
		IndexedSize:  totalSize,
	}

	if failedFiles > 0 {
		notification.Error = "some files failed to process"
	}

	go pt.onCompletion(notification) // Non-blocking notification
}

// getProgressLocked returns progress without notification (must be called with lock held)
func (pt *ProgressTracker) getProgressLocked() ProgressNotification {
	var completedFiles, processingFiles, failedFiles int

	// Count files by state
	for _, fp := range pt.files {
		switch fp.State {
		case FileStateCompleted:
			completedFiles++
		case FileStateProcessing:
			processingFiles++
		case FileStateFailed:
			failedFiles++
		}
	}

	// Calculate progress percentage
	var percentage float64
	if pt.totalEstimate > 0 {
		percentage = float64(pt.totalActual) / float64(pt.totalEstimate) * 100
	} else if len(pt.files) > 0 {
		// Fallback to file-based progress if no line estimates
		percentage = float64(completedFiles) / float64(len(pt.files)) * 100
	}

	// Cap at 100%
	if percentage > 100 {
		percentage = 100
	}

	elapsed := time.Since(pt.startTime)
	var estimatedRemain time.Duration

	if percentage > 0 && percentage < 100 {
		avgTimePerPercent := float64(elapsed.Nanoseconds()) / percentage
		remainingPercent := 100.0 - percentage
		estimatedRemain = time.Duration(int64(avgTimePerPercent * remainingPercent))
	}

	return ProgressNotification{
		LogGroupPath:    pt.logGroupPath,
		Percentage:      percentage,
		TotalFiles:      len(pt.files),
		CompletedFiles:  completedFiles,
		ProcessingFiles: processingFiles,
		FailedFiles:     failedFiles,
		ProcessedLines:  pt.totalActual,
		EstimatedLines:  pt.totalEstimate,
		ElapsedTime:     elapsed,
		EstimatedRemain: estimatedRemain,
		IsCompleted:     pt.isCompleted,
	}
}

// EstimateFileLines estimates the number of lines in a file based on sampling
func EstimateFileLines(ctx context.Context, filePath string, fileSize int64, isCompressed bool) (int64, error) {
	if fileSize == 0 {
		return 0, nil
	}

	file, err := os.Open(filePath)
	if err != nil {
		// Return fallback estimate instead of error
		return fileSize / 150, nil // Fallback: ~150 bytes per line
	}
	defer file.Close()

	var reader io.Reader = file

	// Handle compressed files
	if isCompressed {
		gzReader, err := gzip.NewReader(file)
		if err != nil {
			return (fileSize * 3) / 150, nil // Fallback for compressed: 3:1 ratio
		}
		defer gzReader.Close()
		reader = gzReader
	}

	// Sample the first 1MB of the file content (decompressed if necessary)
	sampleSize := int64(1024 * 1024)
	buf := make([]byte, sampleSize)

	// Check for context cancellation
	select {
	case <-ctx.Done():
		return 0, ctx.Err()
	default:
	}

	bytesRead, err := io.ReadFull(reader, buf)
	if err != nil && err != io.EOF && !errors.Is(err, io.ErrUnexpectedEOF) {
		return fileSize / 150, nil // Fallback on read error
	}

	if bytesRead == 0 {
		return 0, nil // Empty file
	}

	// Count lines in the sample
	lineCount := bytes.Count(buf[:bytesRead], []byte{'\n'})

	if lineCount == 0 {
		// Avoid division by zero, fallback to rough estimate
		return fileSize / 150, nil
	}

	// Calculate average line size from the sample
	avgLineSize := float64(bytesRead) / float64(lineCount)
	if avgLineSize == 0 {
		return fileSize / 150, nil // Fallback
	}

	// Estimate total lines
	var estimatedLines int64
	if isCompressed {
		// For compressed files, use a compression ratio estimate
		estimatedUncompressedSize := fileSize * 5 // Conservative 5:1 compression ratio
		estimatedLines = int64(float64(estimatedUncompressedSize) / avgLineSize)
	} else {
		estimatedLines = int64(float64(fileSize) / avgLineSize)
	}

	return estimatedLines, nil
}

// IsCompressedFile determines if a file is compressed based on its extension
func IsCompressedFile(filePath string) bool {
	ext := strings.ToLower(filepath.Ext(filePath))
	return ext == ".gz" || ext == ".bz2" || ext == ".xz" || ext == ".lz4"
}

// IsRotationLogFile determines if a file is a rotation log file
func IsRotationLogFile(filePath string) bool {
	base := filepath.Base(filePath)

	// Common nginx rotation patterns:
	// access.log, access.log.1, access.log.2.gz
	// access.1.log, access.2.log.gz
	// error.log, error.log.1, error.log.2.gz

	// Remove compression extensions first
	if IsCompressedFile(base) {
		base = strings.TrimSuffix(base, filepath.Ext(base))
	}

	// Check for basic .log files
	if strings.HasSuffix(base, ".log") {
		return true
	}

	// Check for numbered rotation patterns: access.log.1, error.log.10, etc.
	parts := strings.Split(base, ".")
	if len(parts) >= 3 {
		// Pattern: name.log.number (e.g., access.log.1)
		if parts[len(parts)-2] == "log" && isNumeric(parts[len(parts)-1]) {
			return true
		}

		// Pattern: name.number.log (e.g., access.1.log)
		if parts[len(parts)-1] == "log" {
			for i := 1; i < len(parts)-1; i++ {
				if isNumeric(parts[i]) {
					return true
				}
			}
		}
	}

	return false
}

// isNumeric checks if a string represents a number
func isNumeric(s string) bool {
	if len(s) == 0 {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// AddRotationFiles automatically detects and adds rotation log files with appropriate compression detection
func (pt *ProgressTracker) AddRotationFiles(filePaths ...string) {
	for _, filePath := range filePaths {
		isCompressed := IsCompressedFile(filePath)
		pt.AddFile(filePath, isCompressed)
	}
}

// ProgressManager manages multiple progress trackers
type ProgressManager struct {
	mu       sync.RWMutex
	trackers map[string]*ProgressTracker
}

// NewProgressManager creates a new progress manager
func NewProgressManager() *ProgressManager {
	return &ProgressManager{
		trackers: make(map[string]*ProgressTracker),
	}
}

// GetTracker gets or creates a progress tracker for a log group
func (pm *ProgressManager) GetTracker(logGroupPath string, config *ProgressConfig) *ProgressTracker {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if tracker, exists := pm.trackers[logGroupPath]; exists {
		return tracker
	}

	tracker := NewProgressTracker(logGroupPath, config)
	pm.trackers[logGroupPath] = tracker
	return tracker
}

// RemoveTracker removes a progress tracker
func (pm *ProgressManager) RemoveTracker(logGroupPath string) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	delete(pm.trackers, logGroupPath)
}

// GetAllTrackers returns all current trackers
func (pm *ProgressManager) GetAllTrackers() map[string]*ProgressTracker {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	result := make(map[string]*ProgressTracker)
	for path, tracker := range pm.trackers {
		result[path] = tracker
	}
	return result
}

// Cleanup removes completed or failed trackers
func (pm *ProgressManager) Cleanup() {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	for path, tracker := range pm.trackers {
		if tracker.IsCompleted() {
			delete(pm.trackers, path)
		}
	}
}
