package indexer

import (
	"strings"
	"time"
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
	return indexFileInto(filePath, progressTracker, pi.StartBatch(), func(position int64) {
		pi.recordReadPosition(filePath, position)
	})
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
