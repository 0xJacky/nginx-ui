package indexer

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/nginx_log/utils"
	"github.com/uozi-tech/cosy/logger"
)

// fileIndexFunc indexes one physical file and reports the documents written
// and their time range.
type fileIndexFunc func(filePath string, progressTracker *ProgressTracker) (uint64, *time.Time, *time.Time, error)

// sourceEndSetter is implemented by batches that record how far each source
// file has been read.
type sourceEndSetter interface {
	SetSourceEnd(filePath string, offset int64)
}

// batchAborter is implemented by batches that can undo what they wrote, so a
// failed read leaves no partial data behind.
type batchAborter interface {
	Abort() error
}

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

// indexFileInto parses one physical file and writes its documents through
// batch. Only one parse batch is held in memory at a time. recordPosition
// receives how far a plain file was read once everything is flushed.
func indexFileInto(filePath string, progressTracker *ProgressTracker, batch BatchWriterInterface, recordPosition func(int64)) (uint64, *time.Time, *time.Time, error) {
	if !utils.IsValidLogPath(filePath) {
		return 0, nil, nil, fmt.Errorf("invalid log path: %s", filePath)
	}

	file, err := os.Open(filePath)
	if err != nil {
		return 0, nil, nil, fmt.Errorf("failed to open log file %s: %w", filePath, err)
	}
	defer func() { _ = file.Close() }()

	fileInfo, err := file.Stat()
	if err != nil {
		return 0, nil, nil, fmt.Errorf("failed to get file info for %s: %w", filePath, err)
	}
	fileSize := fileInfo.Size()

	if progressTracker != nil {
		progressTracker.SetFileSize(filePath, fileSize)
		// Rough estimate: ~150 bytes per line.
		progressTracker.SetFileEstimate(filePath, max(fileSize/150, 100))
	}

	logger.Infof("Starting to process file: %s", filePath)

	// Gzip decompression is handled inside the streaming parser via the gzip
	// magic-header detection in createReaderForFile.
	isCompressed := strings.HasSuffix(filePath, ".gz")

	abort := func() {
		if a, ok := batch.(batchAborter); ok {
			if err := a.Abort(); err != nil {
				logger.Warnf("Failed to discard the partial index of %s: %v", filePath, err)
			}
		}
	}

	var (
		docCount          uint64
		minTime, maxTime  *time.Time
		invalidEntryCount int
		processedLines    int64
	)

	// Reuse one buffer for ID construction; the ID itself must be a copied
	// string because the batch may retain it.
	idBuf := make([]byte, 0, len(filePath)+16)

	processed, _, err := ParseLogStreamBatches(context.Background(), file, filePath, func(docs []*LogDocument) error {
		for _, doc := range docs {
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

			idBuf = append(idBuf[:0], filePath...)
			idBuf = append(idBuf, '-')
			idBuf = strconv.AppendInt(idBuf, int64(docCount), 10)

			if err := batch.Add(&Document{ID: string(idBuf), Fields: doc}); err != nil {
				return fmt.Errorf("failed to add document to batch for %s (auto-flush might have failed): %w", filePath, err)
			}
			docCount++
		}

		processedLines += int64(len(docs))
		if progressTracker != nil {
			if isCompressed {
				progressTracker.UpdateFileProgress(filePath, processedLines)
			} else {
				progressTracker.UpdateFileProgress(filePath, processedLines, min(processedLines*150, fileSize))
			}
		}
		return nil
	})
	if err != nil {
		abort()
		return docCount, minTime, maxTime, fmt.Errorf("failed to parse file %s: %w", filePath, err)
	}

	// The parser reads a plain file to EOF, so the descriptor offset is the
	// end of the indexed data, including lines appended while indexing ran.
	// Compressed files are always re-read in full, so they record nothing.
	position := int64(-1)
	if !isCompressed {
		if pos, err := file.Seek(0, io.SeekCurrent); err == nil {
			position = pos
			if setter, ok := batch.(sourceEndSetter); ok {
				setter.SetSourceEnd(filePath, pos)
			}
		} else {
			logger.Warnf("Failed to determine read position for %s: %v", filePath, err)
		}
	}

	if docCount > 0 {
		if _, err := batch.Flush(); err != nil {
			abort()
			return docCount, minTime, maxTime, fmt.Errorf("failed to flush batch for %s: %w", filePath, err)
		}
	}

	// Remember how far the file was read, so the metadata saved for it
	// records a LastPosition and the incremental job only reads what is
	// appended later.
	if position >= 0 && recordPosition != nil {
		recordPosition(position)
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
