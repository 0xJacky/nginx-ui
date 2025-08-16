package nginx_log

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/blevesearch/bleve/v2"
	"github.com/uozi-tech/cosy/logger"
)

// IndexLogFileFull performs full reindexing of a log file and its related log group
func (li *LogIndexer) IndexLogFileFull(filePath string) error {
	logger.Infof("Starting full reindex of log file and related group: %s", filePath)

	// Determine main log path for group operations
	mainLogPath := li.getMainLogPath(filePath)
	logDir := filepath.Dir(mainLogPath)
	baseLogName := filepath.Base(mainLogPath)

	// Get or create progress tracker for this log group
	progressTracker := GetProgressTracker(mainLogPath)

	// Find all related log files in the group
	relatedFiles, err := li.findRelatedLogFiles(logDir, baseLogName)
	if err != nil {
		// Fallback to single file if related file discovery fails
		logger.Warnf("Failed to find related files for %s, processing single file: %v", filePath, err)
		relatedFiles = []string{filePath}
	}

	logger.Infof("Full reindexing log group %s with %d files: %v", mainLogPath, len(relatedFiles), relatedFiles)

	// Initialize progress tracker with all files
	for _, file := range relatedFiles {
		info, err := li.safeGetFileInfo(file)
		if err != nil {
			logger.Warnf("Failed to get file info for %s: %v", file, err)
			continue
		}

		isCompressed := strings.HasSuffix(file, ".gz") || strings.HasSuffix(file, ".bz2")
		progressTracker.AddFile(file, isCompressed)

		// Estimate lines in this file
		estimatedLines := EstimateFileLines(file, info.Size(), isCompressed)
		progressTracker.SetFileEstimate(file, estimatedLines)
	}

	// Delete existing index data for the entire log group
	if err := li.DeleteLogGroupFromIndex(mainLogPath); err != nil {
		logger.Warnf("Failed to delete existing index data for log group %s: %v", mainLogPath, err)
	}

	// Index all files in the group
	for _, file := range relatedFiles {
		if err := li.indexSingleFileForGroup(file, mainLogPath, progressTracker); err != nil {
			logger.Errorf("Failed to index file %s in group %s: %v", file, mainLogPath, err)
			// Continue with other files rather than failing completely
		} else {
			logger.Infof("Successfully indexed file %s in group %s", file, mainLogPath)
		}
	}

	// Clean up progress tracker
	RemoveProgressTracker(mainLogPath)

	// Clear indexing status for all files in the group
	for _, file := range relatedFiles {
		SetIndexingStatus(file, false)
	}

	logger.Infof("Completed full reindex of log group %s with %d files", mainLogPath, len(relatedFiles))
	return nil
}

// indexSingleFileForGroup indexes a single file as part of a log group
func (li *LogIndexer) indexSingleFileForGroup(filePath, mainLogPath string, progressTracker *ProgressTracker) error {
	// Get or create log index record
	logIndex, err := li.persistence.GetLogIndex(filePath)
	if err != nil {
		return fmt.Errorf("failed to get log index record: %w", err)
	}

	// Get current file info using safe method
	currentInfo, err := li.safeGetFileInfo(filePath)
	if err != nil {
		return fmt.Errorf("failed to safely stat file %s: %w", filePath, err)
	}

	logger.Infof("Indexing file in group: %s -> %s (size: %d, mod: %v)", filePath, mainLogPath, currentInfo.Size(), currentInfo.ModTime())

	// Start file processing in progress tracker
	progressTracker.StartFile(filePath)

	// Reset log index position for full reindex
	logIndex.Reset()

	// Index the entire file with specified mainLogPath for grouping
	return li.indexFileFromPositionWithMainLogPath(filePath, mainLogPath, 0, logIndex, progressTracker)
}

// DeleteLogGroupFromIndex removes all index entries for a given log group
func (li *LogIndexer) DeleteLogGroupFromIndex(mainLogPath string) error {
	logger.Infof("Deleting all index entries for log group: %s", mainLogPath)
	query := bleve.NewTermQuery(mainLogPath)
	query.SetField("file_path")
	searchReq := bleve.NewSearchRequest(query)
	searchReq.Size = 10000 // Process in batches

	for {
		searchResult, err := li.index.Search(searchReq)
		if err != nil {
			return fmt.Errorf("failed to search existing entries for log group %s: %w", mainLogPath, err)
		}

		if len(searchResult.Hits) == 0 {
			break
		}

		batch := li.index.NewBatch()
		for _, hit := range searchResult.Hits {
			batch.Delete(hit.ID)
		}

		if err := li.index.Batch(batch); err != nil {
			return fmt.Errorf("failed to delete entries for log group %s: %w", mainLogPath, err)
		}
		logger.Infof("Deleted %d entries for log group %s", len(searchResult.Hits), mainLogPath)
	}
	return nil
}