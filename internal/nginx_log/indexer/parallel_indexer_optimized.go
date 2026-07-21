package indexer

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/nginx_log/utils"
	"github.com/uozi-tech/cosy/logger"
)

// IndexLogFile reads and indexes a single log file using the streaming
// pipeline. Kept as a thin compatibility wrapper around IndexSingleFile.
func (pi *ParallelIndexer) IndexLogFile(filePath string) error {
	_, _, _, err := pi.IndexSingleFile(filePath)
	return err
}

// IndexSingleFile contains the logic to process one physical log file.
// It returns the number of documents indexed from the file, and the min/max timestamps.
func (pi *ParallelIndexer) IndexSingleFile(filePath string) (uint64, *time.Time, *time.Time, error) {
	return pi.IndexSingleFileWithProgress(filePath, nil)
}

// IndexSingleFileWithProgress processes a file with progress tracking integration.
// The file is parsed, converted, validated and flushed to the index in bounded
// batches: only one parse batch is held in memory at a time, so peak memory no
// longer scales with file size.
func (pi *ParallelIndexer) IndexSingleFileWithProgress(filePath string, progressTracker *ProgressTracker) (uint64, *time.Time, *time.Time, error) {
	// Validate log path before accessing it
	if !utils.IsValidLogPath(filePath) {
		return 0, nil, nil, fmt.Errorf("invalid log path: %s", filePath)
	}

	file, err := os.Open(filePath)
	if err != nil {
		return 0, nil, nil, fmt.Errorf("failed to open log file %s: %w", filePath, err)
	}
	defer file.Close()

	// Get file info for progress tracking
	fileInfo, err := file.Stat()
	if err != nil {
		return 0, nil, nil, fmt.Errorf("failed to get file info for %s: %w", filePath, err)
	}
	fileSize := fileInfo.Size()

	// Initialize progress tracking if provided
	if progressTracker != nil {
		// Set file size for progress calculation
		progressTracker.SetFileSize(filePath, fileSize)

		// Estimate line count for progress tracking (rough estimate: ~150 bytes per line)
		estimatedLines := fileSize / 150
		if estimatedLines < 100 {
			estimatedLines = 100 // Minimum estimate
		}
		progressTracker.SetFileEstimate(filePath, estimatedLines)
	}

	logger.Infof("Starting to process file: %s", filePath)

	// Gzip decompression is handled inside the streaming parser via the gzip
	// magic-header detection in createReaderForFile.
	ctx := context.Background()
	isCompressed := strings.HasSuffix(filePath, ".gz")

	batch := pi.StartBatch()

	var docCount uint64
	var minTime, maxTime *time.Time
	var invalidEntryCount int
	var processedLines int64

	// Reuse one buffer for ID construction; the ID itself must be a copied
	// string because Bleve retains it beyond this loop.
	idBuf := make([]byte, 0, len(filePath)+16)

	processed, _, err := ParseLogStreamBatches(ctx, file, filePath, func(docs []*LogDocument) error {
		for _, doc := range docs {
			// Validate and filter out obviously incorrect parsed entries
			if !isValidLogEntry(doc) {
				invalidEntryCount++
				continue
			}

			ts := time.Unix(doc.Timestamp, 0)
			if minTime == nil || ts.Before(*minTime) {
				tsCopy := ts
				minTime = &tsCopy
			}
			if maxTime == nil || ts.After(*maxTime) {
				tsCopy := ts
				maxTime = &tsCopy
			}

			idBuf = idBuf[:0]
			idBuf = append(idBuf, filePath...)
			idBuf = append(idBuf, '-')
			idBuf = strconv.AppendInt(idBuf, int64(docCount), 10)

			if err := batch.Add(&Document{ID: string(idBuf), Fields: doc}); err != nil {
				// This indicates an auto-flush occurred and failed.
				return fmt.Errorf("failed to add document to batch for %s (auto-flush might have failed): %w", filePath, err)
			}
			docCount++
		}

		// Real incremental progress per parsed batch
		processedLines += int64(len(docs))
		if progressTracker != nil {
			if isCompressed {
				// For compressed files, we can't track byte position accurately
				progressTracker.UpdateFileProgress(filePath, processedLines)
			} else {
				// Estimate position based on processed line count
				estimatedPos := processedLines * 150 // Assume ~150 bytes per line
				if estimatedPos > fileSize {
					estimatedPos = fileSize
				}
				progressTracker.UpdateFileProgress(filePath, processedLines, estimatedPos)
			}
		}
		return nil
	})
	if err != nil {
		return docCount, minTime, maxTime, fmt.Errorf("failed to parse file %s: %w", filePath, err)
	}

	// Flush any remaining documents in the batch
	if docCount > 0 {
		if _, err := batch.Flush(); err != nil {
			return docCount, minTime, maxTime, fmt.Errorf("failed to flush batch for %s: %w", filePath, err)
		}
	}

	if invalidEntryCount > 0 {
		logger.Warnf("File %s: Filtered out %d invalid entries out of %d total (possible parsing issue)",
			filePath, invalidEntryCount, processed)
	}

	if minTime != nil && maxTime != nil {
		logger.Debugf("Calculated time range for %s: %v to %v", filePath, minTime, maxTime)
	} else if docCount == 0 && processed > 0 {
		logger.Errorf("All %d entries in file %s were invalid - possible format issue", processed, filePath)
	}

	logger.Infof("Finished processing file: %s. Total documents indexed: %d", filePath, docCount)

	return docCount, minTime, maxTime, nil
}

// isValidLogEntry validates if a parsed log entry is correct
func isValidLogEntry(doc *LogDocument) bool {
	if doc == nil {
		return false
	}

	// Check IP address - should be a valid IP format
	// Allow empty IP for now but reject obvious non-IP strings
	if doc.IP != "" && doc.IP != "-" {
		// Simple check: IP shouldn't contain URLs, paths, or binary data
		if strings.Contains(doc.IP, "http") ||
			strings.Contains(doc.IP, "/") ||
			strings.Contains(doc.IP, "\\x") ||
			strings.Contains(doc.IP, "%") ||
			len(doc.IP) > 45 { // Max IPv6 length is 45 chars
			return false
		}
	}

	// Check timestamp - should be reasonable (not 0, not in far future)
	now := time.Now().Unix()
	if doc.Timestamp <= 0 || doc.Timestamp > now+86400 { // Allow up to 1 day in future
		return false
	}

	// Check HTTP method if present
	if doc.Method != "" && !validHTTPMethods[doc.Method] {
		return false
	}

	// Check status code - should be in valid HTTP range
	if doc.Status != 0 && (doc.Status < 100 || doc.Status > 599) {
		return false
	}

	// Check for binary data in path
	if strings.Contains(doc.Path, "\\x") {
		return false
	}

	// If raw log line contains obvious binary data, reject it
	if strings.Contains(doc.Raw, "\\x16\\x03") || // SSL/TLS handshake
		strings.Contains(doc.Raw, "\\xFF\\xD8") { // JPEG header
		return false
	}

	return true
}

// validHTTPMethods contains the standard HTTP and WebDAV (RFC 4918) methods
// accepted during document validation. Package-level so the per-document
// validation loop does not allocate a map per call.
var validHTTPMethods = map[string]bool{
	"GET": true, "POST": true, "PUT": true, "DELETE": true,
	"HEAD": true, "OPTIONS": true, "PATCH": true, "CONNECT": true, "TRACE": true,
	"PROPFIND": true, "PROPPATCH": true, "MKCOL": true,
	"COPY": true, "MOVE": true, "LOCK": true, "UNLOCK": true,
}
