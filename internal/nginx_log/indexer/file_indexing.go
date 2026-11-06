package indexer

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/nginx_log/utils"
	"github.com/uozi-tech/cosy/logger"
)

// fileIndexFunc indexes one physical file and reports the documents written
// and their time range.
type fileIndexFunc func(filePath string, progressTracker *ProgressTracker) (uint64, *time.Time, *time.Time, error)

// groupFiles lists the regular files of a log group: the base file and
// everything matching basePath*, such as rotated and compressed siblings.
func groupFiles(basePath string) ([]string, error) {
	matches, err := filepath.Glob(basePath + "*")
	if err != nil {
		return nil, fmt.Errorf("failed to glob for log files with base %s: %w", basePath, err)
	}
	// Glob does not match the base file itself when it has no extension.
	matches = append(matches, basePath)

	seen := make(map[string]struct{})
	files := make([]string, 0, len(matches))
	for _, match := range matches {
		if _, ok := seen[match]; ok || !utils.IsValidLogPath(match) {
			continue
		}
		if info, err := os.Stat(match); err == nil && info.Mode().IsRegular() {
			seen[match] = struct{}{}
			files = append(files, match)
		}
	}
	return files, nil
}

// indexLogGroupFiles indexes every file of a log group with bounded
// concurrency. A file that fails is logged and skipped.
func indexLogGroupFiles(basePath string, progressConfig *ProgressConfig, concurrency int, indexFile fileIndexFunc) (map[string]uint64, *time.Time, *time.Time, error) {
	var progressTracker *ProgressTracker
	if progressConfig != nil {
		progressTracker = NewProgressTracker(basePath, progressConfig)
	}

	files, err := groupFiles(basePath)
	if err != nil {
		if progressTracker != nil {
			progressTracker.Cancel(fmt.Sprintf("glob failed: %v", err))
		}
		return nil, nil, nil, err
	}
	if len(files) == 0 {
		logger.Warnf("No actual log file found for group: %s", basePath)
		if progressTracker != nil {
			progressTracker.Cancel("no files found")
		}
		return nil, nil, nil, nil
	}

	logger.Infof("Found %d file(s) for log group %s: %v", len(files), basePath, files)

	if progressTracker != nil {
		for _, filePath := range files {
			progressTracker.AddFile(filePath, IsCompressedFile(filePath))
			if stat, err := os.Stat(filePath); err == nil {
				progressTracker.SetFileSize(filePath, stat.Size())
			}
		}
	}

	var (
		mu                             sync.Mutex
		docsCountMap                   = make(map[string]uint64)
		overallMinTime, overallMaxTime *time.Time
		wg                             sync.WaitGroup
	)
	concurrency = max(concurrency, 1)
	semaphore := make(chan struct{}, concurrency)

	logger.Infof("Processing %d files in log group %s with concurrency=%d", len(files), basePath, concurrency)

	for _, filePath := range files {
		wg.Add(1)
		go func(fp string) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			if progressTracker != nil {
				progressTracker.StartFile(fp)
			}
			docsIndexed, minTime, maxTime, err := indexFile(fp, progressTracker)
			if err != nil {
				logger.Warnf("Failed to index file '%s' in group '%s', skipping: %v", fp, basePath, err)
				if progressTracker != nil {
					progressTracker.FailFile(fp, err.Error())
				}
				return
			}
			if progressTracker != nil {
				progressTracker.CompleteFile(fp, int64(docsIndexed))
			}

			mu.Lock()
			defer mu.Unlock()
			docsCountMap[fp] = docsIndexed
			if minTime != nil && (overallMinTime == nil || minTime.Before(*overallMinTime)) {
				overallMinTime = minTime
			}
			if maxTime != nil && (overallMaxTime == nil || maxTime.After(*overallMaxTime)) {
				overallMaxTime = maxTime
			}
		}(filePath)
	}
	wg.Wait()

	return docsCountMap, overallMinTime, overallMaxTime, nil
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
