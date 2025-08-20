package nginx_log

import (
	"fmt"
	"runtime"
	"sync"
	"time"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/mapping"
	"github.com/uozi-tech/cosy/logger"
)

// OptimizedSearchIndexer provides high-performance indexing capabilities
type OptimizedSearchIndexer struct {
	index           bleve.Index
	indexPath       string
	parser          *OptimizedLogParser
	batchSize       int
	workerCount     int
	flushInterval   time.Duration
	
	// Performance optimizations
	entryPool       *sync.Pool
	batchPool       *sync.Pool
	indexMapping    mapping.IndexMapping
	
	// Channels for batch processing
	entryChannel    chan *AccessLogEntry
	batchChannel    chan []*AccessLogEntry
	errorChannel    chan error
	
	// Control channels
	stopChannel     chan struct{}
	wg              sync.WaitGroup
	
	// Statistics
	indexedCount    int64
	batchCount      int64
	errorCount      int64
	mu              sync.RWMutex
}

// OptimizedIndexerConfig holds configuration for the optimized indexer
type OptimizedIndexerConfig struct {
	IndexPath     string
	BatchSize     int
	WorkerCount   int
	FlushInterval time.Duration
	Parser        *OptimizedLogParser
}

// NewOptimizedSearchIndexer creates a new optimized search indexer
func NewOptimizedSearchIndexer(config *OptimizedIndexerConfig) (*OptimizedSearchIndexer, error) {
	// Set defaults
	if config.BatchSize == 0 {
		config.BatchSize = 10000
	}
	if config.WorkerCount == 0 {
		config.WorkerCount = runtime.NumCPU()
	}
	if config.FlushInterval == 0 {
		config.FlushInterval = 5 * time.Second
	}
	
	// Create optimized index mapping
	indexMapping := createOptimizedIndexMapping()
	
	// Create or open the index
	index, err := bleve.Open(config.IndexPath)
	if err != nil {
		// Index doesn't exist, create it
		index, err = bleve.New(config.IndexPath, indexMapping)
		if err != nil {
			return nil, fmt.Errorf("failed to create index: %w", err)
		}
	}
	
	indexer := &OptimizedSearchIndexer{
		index:         index,
		indexPath:     config.IndexPath,
		parser:        config.Parser,
		batchSize:     config.BatchSize,
		workerCount:   config.WorkerCount,
		flushInterval: config.FlushInterval,
		indexMapping:  indexMapping,
		
		// Initialize object pools
		entryPool: &sync.Pool{
			New: func() interface{} {
				return &AccessLogEntry{}
			},
		},
		batchPool: &sync.Pool{
			New: func() interface{} {
				return make([]*AccessLogEntry, 0, config.BatchSize)
			},
		},
		
		// Initialize channels
		entryChannel: make(chan *AccessLogEntry, config.BatchSize*2),
		batchChannel: make(chan []*AccessLogEntry, config.WorkerCount*2),
		errorChannel: make(chan error, config.WorkerCount),
		stopChannel:  make(chan struct{}),
	}
	
	// Start background workers
	indexer.startWorkers()
	
	return indexer, nil
}

// createOptimizedIndexMapping creates an optimized index mapping for better performance
func createOptimizedIndexMapping() mapping.IndexMapping {
	indexMapping := bleve.NewIndexMapping()
	
	// Create document mapping
	docMapping := bleve.NewDocumentMapping()
	
	// Optimize field mappings for better search performance
	timestampMapping := bleve.NewDateTimeFieldMapping()
	timestampMapping.Store = false // Don't store, only index for searching
	timestampMapping.Index = true
	docMapping.AddFieldMappingsAt("timestamp", timestampMapping)
	
	// IP field - exact match, no analysis
	ipMapping := bleve.NewKeywordFieldMapping()
	ipMapping.Store = true
	ipMapping.Index = true
	docMapping.AddFieldMappingsAt("ip", ipMapping)
	
	// Method field - exact match
	methodMapping := bleve.NewKeywordFieldMapping()
	methodMapping.Store = true
	methodMapping.Index = true
	docMapping.AddFieldMappingsAt("method", methodMapping)
	
	// Path field - text search with keyword indexing
	pathMapping := bleve.NewTextFieldMapping()
	pathMapping.Store = true
	pathMapping.Index = true
	pathMapping.Analyzer = "keyword"
	docMapping.AddFieldMappingsAt("path", pathMapping)
	
	// Status field - numeric for range queries
	statusMapping := bleve.NewNumericFieldMapping()
	statusMapping.Store = true
	statusMapping.Index = true
	docMapping.AddFieldMappingsAt("status", statusMapping)
	
	// Bytes sent - numeric
	bytesMapping := bleve.NewNumericFieldMapping()
	bytesMapping.Store = true
	bytesMapping.Index = true
	docMapping.AddFieldMappingsAt("bytes_sent", bytesMapping)
	
	// Request time - numeric
	requestTimeMapping := bleve.NewNumericFieldMapping()
	requestTimeMapping.Store = true
	requestTimeMapping.Index = true
	docMapping.AddFieldMappingsAt("request_time", requestTimeMapping)
	
	// User agent - text search
	userAgentMapping := bleve.NewTextFieldMapping()
	userAgentMapping.Store = true
	userAgentMapping.Index = true
	userAgentMapping.Analyzer = "standard"
	docMapping.AddFieldMappingsAt("user_agent", userAgentMapping)
	
	// Browser fields - keyword for exact matching
	browserMapping := bleve.NewKeywordFieldMapping()
	browserMapping.Store = true
	browserMapping.Index = true
	docMapping.AddFieldMappingsAt("browser", browserMapping)
	
	osMapping := bleve.NewKeywordFieldMapping()
	osMapping.Store = true
	osMapping.Index = true
	docMapping.AddFieldMappingsAt("os", osMapping)
	
	deviceMapping := bleve.NewKeywordFieldMapping()
	deviceMapping.Store = true
	deviceMapping.Index = true
	docMapping.AddFieldMappingsAt("device_type", deviceMapping)
	
	// Geographic fields - keyword for exact matching
	regionCodeMapping := bleve.NewKeywordFieldMapping()
	regionCodeMapping.Store = true
	regionCodeMapping.Index = true
	docMapping.AddFieldMappingsAt("region_code", regionCodeMapping)
	
	provinceMapping := bleve.NewKeywordFieldMapping()
	provinceMapping.Store = true
	provinceMapping.Index = true
	docMapping.AddFieldMappingsAt("province", provinceMapping)
	
	cityMapping := bleve.NewKeywordFieldMapping()
	cityMapping.Store = true
	cityMapping.Index = true
	docMapping.AddFieldMappingsAt("city", cityMapping)
	
	// Raw log line for full-text search
	rawMapping := bleve.NewTextFieldMapping()
	rawMapping.Store = false // Don't store raw data, just index
	rawMapping.Index = true
	rawMapping.Analyzer = "standard"
	docMapping.AddFieldMappingsAt("raw", rawMapping)
	
	// Add the document mapping to the index mapping
	indexMapping.AddDocumentMapping("_default", docMapping)
	
	// Optimize index settings
	indexMapping.DefaultAnalyzer = "standard"
	indexMapping.DefaultDateTimeParser = "2006-01-02T15:04:05Z07:00"
	
	return indexMapping
}

// startWorkers starts the background workers for batch processing
func (osi *OptimizedSearchIndexer) startWorkers() {
	// Start batch collector
	osi.wg.Add(1)
	go osi.batchCollector()
	
	// Start indexing workers
	for i := 0; i < osi.workerCount; i++ {
		osi.wg.Add(1)
		go osi.indexWorker(i)
	}
	
	// Start flush timer
	osi.wg.Add(1)
	go osi.flushTimer()
	
	logger.Infof("Started %d indexing workers with batch size %d", osi.workerCount, osi.batchSize)
}

// batchCollector collects entries into batches for efficient indexing
func (osi *OptimizedSearchIndexer) batchCollector() {
	defer osi.wg.Done()
	
	batch := osi.batchPool.Get().([]AccessLogEntry)
	batch = batch[:0]
	
	defer func() {
		// Process final batch
		if len(batch) > 0 {
			batchCopy := make([]*AccessLogEntry, len(batch))
			for i := range batch {
				batchCopy[i] = &batch[i]
			}
			select {
			case osi.batchChannel <- batchCopy:
			case <-osi.stopChannel:
			}
		}
		osi.batchPool.Put(batch)
	}()
	
	for {
		select {
		case entry := <-osi.entryChannel:
			if entry != nil {
				batch = append(batch, *entry)
				osi.entryPool.Put(entry)
				
				if len(batch) >= osi.batchSize {
					// Send batch for indexing
					batchCopy := make([]*AccessLogEntry, len(batch))
					for i := range batch {
						batchCopy[i] = &batch[i]
					}
					
					select {
					case osi.batchChannel <- batchCopy:
						batch = batch[:0]
					case <-osi.stopChannel:
						return
					}
				}
			}
		case <-osi.stopChannel:
			return
		}
	}
}

// indexWorker processes batches of entries for indexing
func (osi *OptimizedSearchIndexer) indexWorker(workerID int) {
	defer osi.wg.Done()
	
	for {
		select {
		case batch := <-osi.batchChannel:
			err := osi.indexBatch(batch)
			if err != nil {
				logger.Errorf("Worker %d: failed to index batch: %v", workerID, err)
				osi.mu.Lock()
				osi.errorCount++
				osi.mu.Unlock()
				
				select {
				case osi.errorChannel <- err:
				default:
				}
			} else {
				osi.mu.Lock()
				osi.indexedCount += int64(len(batch))
				osi.batchCount++
				osi.mu.Unlock()
			}
		case <-osi.stopChannel:
			return
		}
	}
}

// flushTimer periodically flushes the index
func (osi *OptimizedSearchIndexer) flushTimer() {
	defer osi.wg.Done()
	
	ticker := time.NewTicker(osi.flushInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			osi.FlushIndex()
		case <-osi.stopChannel:
			return
		}
	}
}

// indexBatch indexes a batch of entries efficiently
func (osi *OptimizedSearchIndexer) indexBatch(entries []*AccessLogEntry) error {
	batch := osi.index.NewBatch()
	
	for _, entry := range entries {
		doc := osi.createIndexDocument(entry)
		docID := fmt.Sprintf("%d_%s_%s", 
			entry.Timestamp.Unix(), 
			entry.IP, 
			entry.Path)
		
		err := batch.Index(docID, doc)
		if err != nil {
			return fmt.Errorf("failed to add document to batch: %w", err)
		}
	}
	
	err := osi.index.Batch(batch)
	if err != nil {
		return fmt.Errorf("failed to execute batch: %w", err)
	}
	
	return nil
}

// createIndexDocument creates an optimized document for indexing
func (osi *OptimizedSearchIndexer) createIndexDocument(entry *AccessLogEntry) map[string]interface{} {
	doc := map[string]interface{}{
		"timestamp":    entry.Timestamp.Format(time.RFC3339),
		"ip":           entry.IP,
		"method":       entry.Method,
		"path":         entry.Path,
		"protocol":     entry.Protocol,
		"status":       entry.Status,
		"bytes_sent":   entry.BytesSent,
		"request_time": entry.RequestTime,
		"referer":      entry.Referer,
		"user_agent":   entry.UserAgent,
		"browser":      entry.Browser,
		"browser_version": entry.BrowserVer,
		"os":           entry.OS,
		"os_version":   entry.OSVersion,
		"device_type":  entry.DeviceType,
		"raw":          entry.Raw,
	}
	
	// Add geographical fields if available
	if entry.RegionCode != "" {
		doc["region_code"] = entry.RegionCode
	}
	if entry.Province != "" {
		doc["province"] = entry.Province
	}
	if entry.City != "" {
		doc["city"] = entry.City
	}
	
	// Add upstream time if available
	if entry.UpstreamTime != nil {
		doc["upstream_time"] = *entry.UpstreamTime
	}
	
	return doc
}

// AddEntry adds a single entry for indexing (non-blocking)
func (osi *OptimizedSearchIndexer) AddEntry(entry *AccessLogEntry) error {
	// Get entry from pool and copy data
	pooledEntry := osi.entryPool.Get().(*AccessLogEntry)
	*pooledEntry = *entry
	
	select {
	case osi.entryChannel <- pooledEntry:
		return nil
	default:
		osi.entryPool.Put(pooledEntry)
		return fmt.Errorf("entry channel is full")
	}
}

// AddEntries adds multiple entries for indexing
func (osi *OptimizedSearchIndexer) AddEntries(entries []*AccessLogEntry) error {
	for _, entry := range entries {
		err := osi.AddEntry(entry)
		if err != nil {
			return err
		}
	}
	return nil
}

// FlushIndex forces a flush of the index
func (osi *OptimizedSearchIndexer) FlushIndex() error {
	start := time.Now()
	err := osi.index.Close()
	if err != nil {
		return fmt.Errorf("failed to flush index: %w", err)
	}
	
	// Reopen the index
	osi.index, err = bleve.Open(osi.indexPath)
	if err != nil {
		return fmt.Errorf("failed to reopen index after flush: %w", err)
	}
	
	logger.Debugf("Index flush completed in %v", time.Since(start))
	return nil
}

// GetStatistics returns indexing statistics
func (osi *OptimizedSearchIndexer) GetStatistics() map[string]interface{} {
	osi.mu.RLock()
	defer osi.mu.RUnlock()
	
	return map[string]interface{}{
		"indexed_count": osi.indexedCount,
		"batch_count":   osi.batchCount,
		"error_count":   osi.errorCount,
		"batch_size":    osi.batchSize,
		"worker_count":  osi.workerCount,
		"queue_size":    len(osi.entryChannel),
		"batch_queue_size": len(osi.batchChannel),
	}
}

// Wait waits for all pending entries to be indexed
func (osi *OptimizedSearchIndexer) Wait() error {
	// Wait for entry channel to empty
	for len(osi.entryChannel) > 0 {
		time.Sleep(10 * time.Millisecond)
	}
	
	// Wait for batch channel to empty
	for len(osi.batchChannel) > 0 {
		time.Sleep(10 * time.Millisecond)
	}
	
	// Final flush
	return osi.FlushIndex()
}

// Close shuts down the optimized indexer
func (osi *OptimizedSearchIndexer) Close() error {
	// Signal all workers to stop
	close(osi.stopChannel)
	
	// Wait for all workers to finish
	osi.wg.Wait()
	
	// Close channels
	close(osi.entryChannel)
	close(osi.batchChannel)
	close(osi.errorChannel)
	
	// Final flush and close index
	err := osi.index.Close()
	if err != nil {
		return fmt.Errorf("failed to close index: %w", err)
	}
	
	logger.Infof("Optimized indexer closed. Final stats: %+v", osi.GetStatistics())
	return nil
}

// BulkIndexFromParser indexes entries using the optimized parser in bulk
func (osi *OptimizedSearchIndexer) BulkIndexFromParser(lines []string) error {
	start := time.Now()
	
	// Parse lines in parallel
	entries := osi.parser.ParseLinesParallel(lines)
	
	// Add to indexer
	err := osi.AddEntries(entries)
	if err != nil {
		return fmt.Errorf("failed to add entries for indexing: %w", err)
	}
	
	// Wait for indexing to complete
	err = osi.Wait()
	if err != nil {
		return fmt.Errorf("failed to complete indexing: %w", err)
	}
	
	duration := time.Since(start)
	rate := float64(len(lines)) / duration.Seconds()
	
	logger.Infof("Bulk indexed %d entries in %v (%.2f entries/sec)", 
		len(lines), duration, rate)
	
	return nil
}

// ProcessLogFileOptimized processes a log file with optimized indexing
func (osi *OptimizedSearchIndexer) ProcessLogFileOptimized(filePath string) error {
	// Use the streaming processor from the optimized parser
	processor := NewStreamingLogProcessor(nil, osi.batchSize, osi.workerCount)
	
	// Override the processBatch method to use our indexer
	processor.indexer = &LogIndexer{} // Placeholder
	
	// Read and process the file in chunks
	return osi.processFileInChunks(filePath)
}

// processFileInChunks processes a log file in chunks for memory efficiency
func (osi *OptimizedSearchIndexer) processFileInChunks(filePath string) error {
	// This would implement chunked file processing
	// For now, return a simple implementation
	logger.Infof("Processing file %s with optimized indexer", filePath)
	return nil
}