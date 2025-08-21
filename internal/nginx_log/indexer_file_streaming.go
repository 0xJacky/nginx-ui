package nginx_log

import (
	"bufio"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/uozi-tech/cosy/logger"
)

// indexFileFromPosition indexes a file starting from a specific byte position
func (li *LogIndexer) indexFileFromPosition(filePath string, startPosition int64, logIndex *model.NginxLogIndex) error {
	indexStartTime := time.Now()

	// Record the start time of indexing operation
	logIndex.SetIndexStartTime(indexStartTime)

	file, err := li.safeOpenFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to safely open file %s: %w", filePath, err)
	}
	defer file.Close()

	// Get file info
	fileInfo, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to get file info: %w", err)
	}

	// Seek to start position
	if startPosition > 0 {
		_, err = file.Seek(startPosition, io.SeekStart)
		if err != nil {
			return fmt.Errorf("failed to seek to position %d: %w", startPosition, err)
		}
	}

	var reader io.Reader = file

	// Handle compressed files (note: incremental indexing may not work well with compressed files)
	if strings.HasSuffix(filePath, ".gz") {
		if startPosition > 0 {
			return fmt.Errorf("incremental indexing not supported for compressed files")
		}
		gzReader, err := gzip.NewReader(file)
		if err != nil {
			return fmt.Errorf("failed to create gzip reader: %w", err)
		}
		defer gzReader.Close()
		reader = gzReader
	}

	scanner := bufio.NewScanner(reader)
	const maxCapacity = 1024 * 1024 // 1MB
	buf := make([]byte, maxCapacity)
	scanner.Buffer(buf, maxCapacity)

	// Use streaming processing to avoid loading all lines into memory
	return li.indexFileFromPositionStreaming(filePath, startPosition, logIndex, fileInfo, scanner, indexStartTime)
}

// indexFileFromPositionWithMainLogPath indexes a file starting from a specific byte position with specified main log path
func (li *LogIndexer) indexFileFromPositionWithMainLogPath(filePath, mainLogPath string, startPosition int64, logIndex *model.NginxLogIndex, progressTracker *ProgressTracker) error {
	indexStartTime := time.Now()

	// Record the start time of indexing operation
	logIndex.SetIndexStartTime(indexStartTime)

	file, err := li.safeOpenFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to safely open file %s: %w", filePath, err)
	}
	defer file.Close()

	// Get file info
	fileInfo, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to get file info: %w", err)
	}

	// Seek to start position
	if startPosition > 0 {
		_, err = file.Seek(startPosition, io.SeekStart)
		if err != nil {
			return fmt.Errorf("failed to seek to position %d: %w", startPosition, err)
		}
	}

	var reader io.Reader = file

	// Handle compressed files (note: incremental indexing may not work well with compressed files)
	if strings.HasSuffix(filePath, ".gz") {
		if startPosition > 0 {
			return fmt.Errorf("incremental indexing not supported for compressed files")
		}
		gzReader, err := gzip.NewReader(file)
		if err != nil {
			return fmt.Errorf("failed to create gzip reader: %w", err)
		}
		defer gzReader.Close()
		reader = gzReader
	}

	scanner := bufio.NewScanner(reader)
	const maxCapacity = 1024 * 1024 // 1MB
	buf := make([]byte, maxCapacity)
	scanner.Buffer(buf, maxCapacity)

	// Use streaming processing with specified main log path
	return li.indexFileFromPositionStreamingWithMainLogPath(filePath, mainLogPath, startPosition, logIndex, fileInfo, scanner, indexStartTime, progressTracker)
}

// indexFileFromPositionStreamingWithMainLogPath processes file content using streaming approach with specified main log path
func (li *LogIndexer) indexFileFromPositionStreamingWithMainLogPath(filePath, mainLogPath string, startPosition int64, logIndex *model.NginxLogIndex, fileInfo os.FileInfo, scanner *bufio.Scanner, startTime time.Time, progressTracker *ProgressTracker) error {
	// Record index start time
	logIndex.SetIndexStartTime(startTime)
	var currentPosition int64 = startPosition
	lineCount := 0
	entryCount := 0
	batch := li.index.NewBatch()
	var newTimeStart, newTimeEnd int64

	logger.Infof("Starting index for file %s -> %s (size: %d bytes)", filePath, mainLogPath, fileInfo.Size())

	// Set file size in progress tracker
	if progressTracker != nil {
		progressTracker.SetFileSize(filePath, fileInfo.Size())
	}

	// Determine compression status
	isCompressed := strings.HasSuffix(filePath, ".gz") || strings.HasSuffix(filePath, ".bz2")

	// Line buffer for batch processing
	const batchLines = 10000 // Process 10000 lines at a time for better performance
	var lineBuffer []string

	// If starting from middle of file, skip partial line
	if startPosition > 0 {
		if scanner.Scan() {
			// Skip the first (potentially partial) line
			line := scanner.Text()
			currentPosition += int64(len(line)) + 1 // +1 for newline
		}
	}

	// Process lines in batches
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			lineBuffer = append(lineBuffer, line)
			lineCount++
			currentPosition += int64(len(scanner.Text())) + 1 // +1 for newline

			// Process batch when buffer is full
			if len(lineBuffer) >= batchLines {
				if err := li.processBatchStreaming(lineBuffer, filePath, mainLogPath, startPosition, &batch, &entryCount, &newTimeStart, &newTimeEnd); err != nil {
					return err
				}
				// Clear buffer
				lineBuffer = lineBuffer[:0]
			}

			// Update progress tracker periodically
			if lineCount%5000 == 0 {
				progressTracker.UpdateFileProgress(filePath, int64(lineCount), currentPosition)
			}
		}
	}

	// Process remaining lines in buffer
	if len(lineBuffer) > 0 {
		if err := li.processBatchStreaming(lineBuffer, filePath, mainLogPath, startPosition, &batch, &entryCount, &newTimeStart, &newTimeEnd); err != nil {
			return err
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("failed to scan file %s: %w", filePath, err)
	}

	// Execute final batch
	if batch.Size() > 0 {
		if err := li.index.Batch(batch); err != nil {
			return fmt.Errorf("failed to execute final batch: %w", err)
		}
		logger.Infof("Final batch executed: %d entries indexed for %s", batch.Size(), filePath)
	}

	// For compressed files, mark as fully indexed
	if isCompressed {
		currentPosition = fileInfo.Size()
	}

	// Update persistence with final status
	var newTimeStartPtr, newTimeEndPtr *time.Time
	if newTimeStart != 0 {
		t := time.Unix(newTimeStart, 0)
		newTimeStartPtr = &t
	}
	if newTimeEnd != 0 {
		t := time.Unix(newTimeEnd, 0)
		newTimeEndPtr = &t
	}
	logIndex.UpdateProgress(fileInfo.ModTime(), fileInfo.Size(), currentPosition, uint64(entryCount), newTimeStartPtr, newTimeEndPtr)
	logIndex.SetIndexDuration(startTime)

	// Save the updated log index
	if err := li.persistence.SaveLogIndex(logIndex); err != nil {
		logger.Warnf("Failed to save log index for %s: %v", filePath, err)
	}

	// Final position update for progress tracker
	if progressTracker != nil {
		progressTracker.UpdateFileProgress(filePath, int64(lineCount), currentPosition)
	}

	// Complete file in progress tracker
	if progressTracker != nil {
		progressTracker.CompleteFile(filePath, int64(lineCount))
	}

	duration := time.Since(startTime)
	logger.Infof("Completed indexing of %s: %d lines processed, %d entries indexed in %v", filePath, lineCount, entryCount, duration)

	return nil
}

// indexFileFromPositionStreaming processes file content using streaming approach
func (li *LogIndexer) indexFileFromPositionStreaming(filePath string, startPosition int64, logIndex *model.NginxLogIndex, fileInfo os.FileInfo, scanner *bufio.Scanner, startTime time.Time) error {
	// Record index start time
	logIndex.SetIndexStartTime(startTime)
	var currentPosition int64 = startPosition
	lineCount := 0
	entryCount := 0
	batch := li.index.NewBatch()
	var newTimeStart, newTimeEnd int64

	// Get main log path first (for statistics grouping)
	mainLogPath := li.getMainLogPath(filePath)

	// Note: Stats calculation removed - using Bleve aggregations instead

	// For compressed files, we can't use position-based progress accurately
	// Fall back to line-based estimation for compressed files
	isCompressed := strings.HasSuffix(filePath, ".gz") || strings.HasSuffix(filePath, ".bz2")

	var totalFileSize int64

	if isCompressed {
		// For compressed files, estimate uncompressed size using compression ratio
		// Use 3:1 compression ratio as a reasonable estimate for most log files
		estimatedUncompressedSize := fileInfo.Size() * 3
		if estimatedUncompressedSize < 1 {
			estimatedUncompressedSize = 1
		}
		totalFileSize = estimatedUncompressedSize
		logger.Infof("Starting index for compressed file %s: compressed size %d, estimated uncompressed size %d", filePath, fileInfo.Size(), estimatedUncompressedSize)
	} else {
		// For uncompressed files, use actual file size
		totalFileSize = fileInfo.Size()
		if totalFileSize < 1 {
			totalFileSize = 1
		}
		logger.Infof("Starting index from position %d of %d bytes for %s", startPosition, totalFileSize, filePath)
	}

	logger.Debugf("Starting indexing: filePath=%s, mainLogPath=%s", filePath, mainLogPath)

	// Line buffer for batch processing
	const batchLines = 10000 // Process 10000 lines at a time for better performance
	var lineBuffer []string

	// If starting from middle of file, skip partial line
	if startPosition > 0 {
		if scanner.Scan() {
			// Skip the first (potentially partial) line
			line := scanner.Text()
			currentPosition += int64(len(line)) + 1 // +1 for newline
		}
	}

	// Process lines in batches
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			lineBuffer = append(lineBuffer, line)
			lineCount++
			currentPosition += int64(len(scanner.Text())) + 1 // +1 for newline

			// Process batch when buffer is full
			if len(lineBuffer) >= batchLines {
				if err := li.processBatchStreaming(lineBuffer, filePath, mainLogPath, startPosition, &batch, &entryCount, &newTimeStart, &newTimeEnd); err != nil {
					return err
				}
				// Clear buffer
				lineBuffer = lineBuffer[:0]
			}

			// Progress reporting for large files
			if lineCount%10000 == 0 {
				// Calculate current file progress percentage
				var currentFileProgress float64
				if isCompressed {
					logger.Debugf("Processed %d lines, indexed %d entries from compressed file %s...",
						lineCount, entryCount, filePath)
					// For compressed files, estimate progress based on current position vs total estimated size
					currentFileProgress = float64(currentPosition) / float64(totalFileSize) * 100
				} else {
					logger.Debugf("Processed %d lines, indexed %d entries from %s... (position: %d/%d bytes)",
						lineCount, entryCount, filePath, currentPosition, totalFileSize)
					// For uncompressed files, use byte position
					currentFileProgress = float64(currentPosition) / float64(totalFileSize) * 100
				}

				// Ensure file progress doesn't exceed 100%
				if currentFileProgress > 100 {
					currentFileProgress = 100
				}

				// Log progress (simplified for incremental indexing)
				logger.Debugf("Processed %d lines for incremental indexing of %s", lineCount, filePath)
			}
		}
	}

	// Process remaining lines in buffer
	if len(lineBuffer) > 0 {
		if err := li.processBatchStreaming(lineBuffer, filePath, mainLogPath, startPosition, &batch, &entryCount, &newTimeStart, &newTimeEnd); err != nil {
			return err
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("failed to scan file %s: %w", filePath, err)
	}

	// Execute final batch
	if batch.Size() > 0 {
		if err := li.index.Batch(batch); err != nil {
			return fmt.Errorf("failed to execute final batch: %w", err)
		}
	}

	logger.Debugf("Processed %d lines, indexed %d entries from %s (position: %d->%d)", lineCount, entryCount, filePath, startPosition, currentPosition)

	// Note: Log group aggregator removed - using Bleve aggregations instead

	if entryCount == 0 {
		logger.Debugf("No new entries to index in %s", filePath)
		// Still update the position and modification time with total index size
		totalSize := li.calculateRelatedLogFilesSize(filePath)
		// Record index completion time and duration
		logIndex.SetIndexDuration(startTime)
		logIndex.UpdateProgress(fileInfo.ModTime(), totalSize, fileInfo.Size(), logIndex.DocumentCount, logIndex.TimeRangeStart, logIndex.TimeRangeEnd)
		return li.persistence.SaveLogIndex(logIndex)
	}

	// Update time range in log index
	var timeRangeStart, timeRangeEnd *time.Time
	if logIndex.TimeRangeStart != nil {
		timeRangeStart = logIndex.TimeRangeStart
	} else if newTimeStart != 0 {
		t := time.Unix(newTimeStart, 0)
		timeRangeStart = &t
	}
	if logIndex.TimeRangeEnd != nil {
		timeRangeEnd = logIndex.TimeRangeEnd
	} else if newTimeEnd != 0 {
		t := time.Unix(newTimeEnd, 0)
		timeRangeEnd = &t
	}

	// Expand time range if needed
	if newTimeStart != 0 && (timeRangeStart == nil || time.Unix(newTimeStart, 0).Before(*timeRangeStart)) {
		t := time.Unix(newTimeStart, 0)
		timeRangeStart = &t
	}
	if newTimeEnd != 0 && (timeRangeEnd == nil || time.Unix(newTimeEnd, 0).After(*timeRangeEnd)) {
		t := time.Unix(newTimeEnd, 0)
		timeRangeEnd = &t
	}

	// Calculate total index size of related log files for this log group
	totalSize := li.calculateRelatedLogFilesSize(filePath)

	// Record index completion time and duration
	logIndex.SetIndexDuration(startTime)

	// Update persistence record with total index size
	logIndex.UpdateProgress(fileInfo.ModTime(), totalSize, currentPosition, logIndex.DocumentCount+uint64(entryCount), timeRangeStart, timeRangeEnd)
	if err := li.persistence.SaveLogIndex(logIndex); err != nil {
		logger.Warnf("Failed to save log index: %v", err)
	}

	// Update in-memory file info for compatibility
	li.mu.Lock()
	if fileInfo, exists := li.logPaths[filePath]; exists {
		fileInfo.LastModified = logIndex.LastModified.Unix()
		fileInfo.LastSize = logIndex.LastSize
		fileInfo.LastIndexed = logIndex.LastIndexed.Unix()
		if timeRangeStart != nil && timeRangeEnd != nil {
			fileInfo.TimeRange = &TimeRange{Start: timeRangeStart.Unix(), End: timeRangeEnd.Unix()}
		}
	}
	li.mu.Unlock()

	// Invalidate statistics cache since data has changed
	li.invalidateStatsCache()

	// Clear indexing status for this file
	SetIndexingStatus(filePath, false)
	statusManager := GetIndexingStatusManager()
	statusManager.UpdateIndexingStatus()

	indexDuration := time.Since(startTime)
	logger.Infof("Indexed %d new entries from %s in %v (position: %d->%d, index_size: %d bytes)",
		entryCount, filePath, indexDuration, startPosition, currentPosition, totalSize)

	// Send completion event
	// duration := time.Since(startTime).Milliseconds()
	if isCompressed {
		logger.Infof("Indexing completed for compressed file %s: processed %d lines (estimated %d total)", filePath, lineCount, totalFileSize)
	} else {
		logger.Infof("Indexing completed for %s: processed %d lines, position %d/%d bytes", filePath, lineCount, currentPosition, totalFileSize)
	}
	// Note: Index complete notification will be sent with the log group ready notification

	// Note: Log group ready notification is now handled centrally after all files complete

	return nil
}
