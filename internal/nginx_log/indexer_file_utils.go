package nginx_log

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/blevesearch/bleve/v2"
	"github.com/uozi-tech/cosy/logger"
)

// findRelatedLogFiles finds all log files related to a base log name in a directory
func (li *LogIndexer) findRelatedLogFiles(logDir string, baseLogName string) ([]string, error) {
	entries, err := li.safeReadDir(logDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read log directory %s: %w", logDir, err)
	}

	var logFiles []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if isLogrotateFile(name, baseLogName) {
			fullPath := filepath.Join(logDir, name)
			logFiles = append(logFiles, fullPath)
		}
	}
	return logFiles, nil
}

// Note: isLogrotateFile is now defined in date_patterns.go as a common utility

// RepairFileMetadata repairs file metadata by scanning existing index data
func (li *LogIndexer) RepairFileMetadata() error {
	logger.Infof("Starting file metadata repair...")

	li.mu.Lock()
	defer li.mu.Unlock()

	for filePath, fileInfo := range li.logPaths {
		logger.Infof("Repairing metadata for: %s", filePath)

		// Check if file exists and get current info
		currentInfo, err := os.Stat(filePath)
		if err != nil {
			logger.Warnf("Failed to stat file %s: %v", filePath, err)
			continue
		}

		// Query index for entries from this file to determine time range
		query := bleve.NewTermQuery(filePath)
		query.SetField("file_path")

		searchReq := bleve.NewSearchRequest(query)
		searchReq.Size = 1000 // Get a sample to determine time range
		searchReq.Fields = []string{"timestamp"}
		searchReq.SortBy([]string{"timestamp"}) // Sort by timestamp

		searchResult, err := li.index.Search(searchReq)
		if err != nil {
			logger.Warnf("Failed to search index for file %s: %v", filePath, err)
			continue
		}

		if searchResult.Total == 0 {
			logger.Warnf("No indexed entries found for file %s", filePath)
			continue
		}

		// Get time range from search results
		var timeRange *TimeRange
		for _, hit := range searchResult.Hits {
			if timestampField, ok := hit.Fields["timestamp"]; ok {
				var timestamp int64
				switch v := timestampField.(type) {
				case float64:
					timestamp = int64(v)
				case int64:
					timestamp = v
				default:
					continue
				}

				if timeRange == nil {
					timeRange = &TimeRange{Start: timestamp, End: timestamp}
				} else {
					if timestamp < timeRange.Start {
						timeRange.Start = timestamp
					}
					if timestamp > timeRange.End {
						timeRange.End = timestamp
					}
				}
			}
		}

		// Update file info
		fileInfo.LastModified = currentInfo.ModTime().Unix()
		fileInfo.LastSize = currentInfo.Size()
		fileInfo.LastIndexed = time.Now().Unix()
		fileInfo.TimeRange = timeRange

		if timeRange != nil {
			logger.Infof("Repaired metadata for %s: TimeRange %v to %v, Total entries: %d",
				filePath, timeRange.Start, timeRange.End, searchResult.Total)
		} else {
			logger.Warnf("Could not determine time range for %s", filePath)
		}
	}

	logger.Infof("File metadata repair completed")
	return nil
}

// DiscoverLogFiles discovers log files in a directory, including compressed ones
// This function now primarily adds paths to the indexer for tracking.
// The actual indexing is queued by AddLogPath.
func (li *LogIndexer) DiscoverLogFiles(logDir string, baseLogName string) error {
	logger.Infof("Auto-discovering log files in %s with base name %s", logDir, baseLogName)

	logFiles, err := li.findRelatedLogFiles(logDir, baseLogName)
	if err != nil {
		return err
	}

	if len(logFiles) == 0 {
		logger.Warnf("No log files found matching pattern %s in directory %s", baseLogName, logDir)
		return fmt.Errorf("no log files found matching pattern %s", baseLogName)
	}

	// Sort files to process them in order (newest first for current log)
	sort.Slice(logFiles, func(i, j int) bool {
		// Current log file should be processed first
		if !strings.Contains(logFiles[i], ".") && strings.Contains(logFiles[j], ".") {
			return true
		}
		return logFiles[i] < logFiles[j]
	})

	logger.Infof("Found %d log files to process: %v", len(logFiles), logFiles)

	// Add all discovered log files for tracking
	var addedCount int
	for _, logFile := range logFiles {
		if err := li.AddLogPath(logFile); err != nil {
			logger.Warnf("Failed to add log path %s: %v", logFile, err)
			continue
		}
		addedCount++
	}

	logger.Infof("Discovered and added %d log files in %s for tracking", addedCount, logDir)
	return nil
}

// calculateRelatedLogFilesSize calculates the total processing units for all related log files
// For uncompressed files, returns bytes; for compressed files, estimates equivalent processing units
func (li *LogIndexer) calculateRelatedLogFilesSize(filePath string) int64 {
	// Get the main log path for this file to find all related files in the group
	mainLogPath := li.getMainLogPath(filePath)
	logDir := filepath.Dir(mainLogPath)
	baseLogName := filepath.Base(mainLogPath)

	entries, err := li.safeReadDir(logDir)
	if err != nil {
		logger.Warnf("Failed to read log directory %s: %v", logDir, err)
		return 0
	}

	var totalSize int64

	var foundFiles []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if isLogrotateFile(name, baseLogName) {
			fullPath := filepath.Join(logDir, name)
			foundFiles = append(foundFiles, name)
			// Use safe method to get file info for related log files
			if info, err := li.safeGetFileInfo(fullPath); err == nil {
				fileSize := info.Size()

				// For compressed files, use estimated processing units based on compression ratio
				if strings.HasSuffix(fullPath, ".gz") || strings.HasSuffix(fullPath, ".bz2") {
					// Estimate uncompressed size using 3:1 compression ratio for progress calculation
					// This provides a more consistent progress measurement across file types
					estimatedUncompressedSize := fileSize * 3
					totalSize += estimatedUncompressedSize
				} else {
					// For uncompressed files, use actual size
					totalSize += fileSize
				}
			}
		}
	}

	return totalSize
}

// getMainLogPath extracts the main log path from a file (including rotated files)
func (li *LogIndexer) getMainLogPath(filePath string) string {
	dir := filepath.Dir(filePath)
	filename := filepath.Base(filePath)

	// Remove .gz compression suffix if present
	filename = strings.TrimSuffix(filename, ".gz")

	// Handle numbered rotation (access.log.1, access.log.2, etc.)
	// Use a more specific pattern to avoid matching date patterns like "20231201"
	if match := regexp.MustCompile(`^(.+)\.(\d{1,3})$`).FindStringSubmatch(filename); len(match) > 1 {
		// Only match if the number is reasonable for rotation (1-999)
		baseFilename := match[1]
		return filepath.Join(dir, baseFilename)
	}

	// Handle date-based rotation (access.20231201, access.2023-12-01, etc.)
	datePatterns := []string{
		`^\d{8}$`,               // YYYYMMDD
		`^\d{4}-\d{2}-\d{2}$`,   // YYYY-MM-DD
		`^\d{4}\.\d{2}\.\d{2}$`, // YYYY.MM.DD
		`^\d{4}_\d{2}_\d{2}$`,   // YYYY_MM_DD
	}

	// Check if filename itself contains date patterns that we should strip
	// Example: access.2023-12-01 -> access.log, access.20231201 -> access.log
	parts := strings.Split(filename, ".")
	if len(parts) >= 2 {
		lastPart := parts[len(parts)-1]
		for _, pattern := range datePatterns {
			if matched, _ := regexp.MatchString(pattern, lastPart); matched {
				baseFilename := strings.Join(parts[:len(parts)-1], ".")
				// If the base doesn't end with .log, add it
				if !strings.HasSuffix(baseFilename, ".log") {
					baseFilename += ".log"
				}
				return filepath.Join(dir, baseFilename)
			}
		}
	}

	// No rotation pattern found, return as-is
	return filePath
}

// clearLogGroupCompletionFlag clears the completion flag for a log group (used during reindex)
func (li *LogIndexer) clearLogGroupCompletionFlag(logGroupPath string) {
	li.logGroupCompletionSent.Delete(logGroupPath)
}