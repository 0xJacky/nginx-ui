package nginx_log

import (
	"fmt"
	"time"

	"github.com/blevesearch/bleve/v2"
)

// processBatchStreaming processes a batch of lines using parallel parsing
func (li *LogIndexer) processBatchStreaming(lines []string, filePath string, mainLogPath string, startPosition int64, batch **bleve.Batch, entryCount *int, newTimeStart, newTimeEnd **time.Time) error {
	if len(lines) == 0 {
		return nil
	}

	// Parse lines in parallel
	entries := li.parser.ParseLines(lines)

	if len(entries) == 0 {
		return nil // No valid entries in this batch
	}

	// Index entries
	for i, entry := range entries {
		// Track time range for new entries
		if *newTimeStart == nil || entry.Timestamp.Before(**newTimeStart) {
			*newTimeStart = &entry.Timestamp
		}
		if *newTimeEnd == nil || entry.Timestamp.After(**newTimeEnd) {
			*newTimeEnd = &entry.Timestamp
		}

		// Note: Real-time stats processing removed - using Bleve aggregations instead

		// Create indexed entry with unique ID
		// Use actual file path in ID to avoid conflicts, but mainLogPath for grouping queries
		indexedEntry := &IndexedLogEntry{
			ID:           fmt.Sprintf("%s_%d_%d", filePath, startPosition, *entryCount+i),
			FilePath:     mainLogPath, // Use main log path for queries
			Timestamp:    entry.Timestamp,
			IP:           entry.IP,
			Location:     entry.Location,
			Method:       entry.Method,
			Path:         entry.Path,
			Protocol:     entry.Protocol,
			Status:       entry.Status,
			BytesSent:    entry.BytesSent,
			Referer:      entry.Referer,
			UserAgent:    entry.UserAgent,
			Browser:      entry.Browser,
			BrowserVer:   entry.BrowserVer,
			OS:           entry.OS,
			OSVersion:    entry.OSVersion,
			DeviceType:   entry.DeviceType,
			RequestTime:  entry.RequestTime,
			UpstreamTime: entry.UpstreamTime,
			Raw:          entry.Raw,
		}

		(*batch).Index(indexedEntry.ID, indexedEntry)

		// Execute batch when it reaches the limit
		if (*batch).Size() >= li.indexBatch {
			if err := li.index.Batch(*batch); err != nil {
				return fmt.Errorf("failed to execute batch: %w", err)
			}
			*batch = li.index.NewBatch()
		}
	}

	*entryCount += len(entries)
	return nil
}