package nginx_log

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/mapping"
	"github.com/dgraph-io/ristretto/v2"
	"github.com/fsnotify/fsnotify"
	"github.com/uozi-tech/cosy/logger"
	cosysettings "github.com/uozi-tech/cosy/settings"
)

const (
	// MinIndexInterval is the minimum interval between two index operations for the same file
	MinIndexInterval = 30 * time.Second
)

// LogIndexer provides high-performance log indexing and querying capabilities
type LogIndexer struct {
	indexPath  string
	index      bleve.Index
	cache      *ristretto.Cache[string, *CachedSearchResult]
	statsCache *ristretto.Cache[string, *CachedStatsResult]
	parser     *LogParser
	watcher    *fsnotify.Watcher
	logPaths   map[string]*LogFileInfo
	mu         sync.RWMutex

	// Background processing
	ctx          context.Context
	cancel       context.CancelFunc
	indexQueue   chan *IndexTask
	indexingLock sync.Map // map[string]*sync.Mutex for per-file locking

	// File debouncing
	debounceTimers sync.Map // map[string]*time.Timer for per-file debouncing
	lastIndexTime  sync.Map // map[string]time.Time for tracking last index time

	// Progress event deduplication
	lastProgressNotify sync.Map // map[string]time.Time for tracking last progress notification per log group

	// Log group completion tracking to prevent duplicate notifications
	logGroupCompletionSent sync.Map // map[string]bool for tracking completion notifications per log group

	// Persistence
	persistence *PersistenceManager

	// Configuration
	maxCacheSize int64
	indexBatch   int
}

// NewLogIndexer creates a new log indexer instance
func NewLogIndexer() (*LogIndexer, error) {
	// Use nginx-ui config directory for index storage
	configDir := filepath.Dir(cosysettings.ConfPath)
	if configDir == "" {
		return nil, fmt.Errorf("nginx-ui config directory not found")
	}

	indexPath := filepath.Join(configDir, "log-index")

	// Create index directory if it doesn't exist
	if err := os.MkdirAll(indexPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create index directory: %w", err)
	}

	// Create or open Bleve index
	index, err := createOrOpenIndex(indexPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create/open index: %w", err)
	}

	// Initialize cache with 100MB capacity
	cache, err := ristretto.NewCache(&ristretto.Config[string, *CachedSearchResult]{
		NumCounters: 1e7,     // number of keys to track frequency of (10M)
		MaxCost:     1 << 27, // maximum cost of cache (128MB)
		BufferItems: 64,      // number of keys per Get buffer
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create cache: %w", err)
	}

	// Initialize statistics cache with 50MB capacity
	statsCache, err := ristretto.NewCache(&ristretto.Config[string, *CachedStatsResult]{
		NumCounters: 1e5,     // number of keys to track frequency of (100K)
		MaxCost:     1 << 26, // maximum cost of cache (64MB)
		BufferItems: 64,      // number of keys per Get buffer
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create stats cache: %w", err)
	}

	// Create user agent parser
	userAgent := NewSimpleUserAgentParser()
	parser := NewLogParser(userAgent)

	// Initialize file system watcher
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		logger.Warnf("Failed to create file watcher: %v", err)
		// Continue without watcher - manual indexing will still work
	}

	// Create context for background processing
	ctx, cancel := context.WithCancel(context.Background())

	// Create persistence manager
	persistence := NewPersistenceManager()

	indexer := &LogIndexer{
		indexPath:      indexPath,
		index:          index,
		cache:          cache,
		statsCache:     statsCache,
		parser:         parser,
		watcher:        watcher,
		logPaths:       make(map[string]*LogFileInfo),
		ctx:            ctx,
		cancel:         cancel,
		indexQueue:     make(chan *IndexTask, 1000),
		persistence:    persistence,
		maxCacheSize:   128 * 1024 * 1024, // 128MB
		indexBatch:     1000,               // Process 1000 entries per batch
	}

	// Start background processing
	go indexer.processIndexQueue()
	if watcher != nil {
		go indexer.watchFiles()
	}

	logger.Info("Log indexer initialized successfully")
	return indexer, nil
}

// createOrOpenIndex creates or opens a Bleve index
func createOrOpenIndex(indexPath string) (bleve.Index, error) {
	// Try to open existing index first
	index, err := bleve.Open(indexPath)
	if err == nil {
		logger.Infof("Opened existing Bleve index at %s", indexPath)
		return index, nil
	}

	// If opening failed, create a new index
	logger.Infof("Creating new Bleve index at %s", indexPath)
	indexMapping := createIndexMapping()
	index, err = bleve.New(indexPath, indexMapping)
	if err != nil {
		return nil, fmt.Errorf("failed to create new index: %w", err)
	}

	logger.Infof("Created new Bleve index at %s", indexPath)
	return index, nil
}

// createIndexMapping creates the mapping for log entries
func createIndexMapping() mapping.IndexMapping {
	// Create a mapping with keyword analyzer for file_path to enable exact matching
	indexMapping := bleve.NewIndexMapping()

	// Create a document mapping for log entries
	logMapping := bleve.NewDocumentMapping()

	// Create field mappings
	textFieldMapping := bleve.NewTextFieldMapping()
	textFieldMapping.Store = true
	textFieldMapping.Index = true
	textFieldMapping.IncludeTermVectors = false
	textFieldMapping.IncludeInAll = false

	// For file_path, use keyword analyzer to enable exact matching
	filePathFieldMapping := bleve.NewTextFieldMapping()
	filePathFieldMapping.Store = true
	filePathFieldMapping.Index = true
	filePathFieldMapping.Analyzer = "keyword" // Use keyword analyzer for exact matching
	filePathFieldMapping.IncludeTermVectors = false
	filePathFieldMapping.IncludeInAll = false

	dateFieldMapping := bleve.NewDateTimeFieldMapping()
	dateFieldMapping.Store = true
	dateFieldMapping.Index = true
	dateFieldMapping.IncludeInAll = false

	numericFieldMapping := bleve.NewNumericFieldMapping()
	numericFieldMapping.Store = true
	numericFieldMapping.Index = true
	numericFieldMapping.IncludeInAll = false

	// Map fields to their types
	logMapping.AddFieldMappingsAt("file_path", filePathFieldMapping) // Use keyword analyzer for exact matching
	logMapping.AddFieldMappingsAt("timestamp", dateFieldMapping)
	logMapping.AddFieldMappingsAt("ip", textFieldMapping)
	logMapping.AddFieldMappingsAt("location", textFieldMapping)
	logMapping.AddFieldMappingsAt("method", textFieldMapping)
	logMapping.AddFieldMappingsAt("path", textFieldMapping)
	logMapping.AddFieldMappingsAt("protocol", textFieldMapping)
	logMapping.AddFieldMappingsAt("status", numericFieldMapping)
	logMapping.AddFieldMappingsAt("bytes_sent", numericFieldMapping)
	logMapping.AddFieldMappingsAt("referer", textFieldMapping)
	logMapping.AddFieldMappingsAt("user_agent", textFieldMapping)
	logMapping.AddFieldMappingsAt("browser", textFieldMapping)
	logMapping.AddFieldMappingsAt("browser_ver", textFieldMapping)
	logMapping.AddFieldMappingsAt("os", textFieldMapping)
	logMapping.AddFieldMappingsAt("os_version", textFieldMapping)
	logMapping.AddFieldMappingsAt("device_type", textFieldMapping)
	logMapping.AddFieldMappingsAt("request_time", numericFieldMapping)
	logMapping.AddFieldMappingsAt("upstream_time", numericFieldMapping)
	logMapping.AddFieldMappingsAt("raw", textFieldMapping)

	// Set the default mapping
	indexMapping.DefaultMapping = logMapping

	// Enable the _all field for general text search
	indexMapping.DefaultAnalyzer = "standard"

	return indexMapping
}

// Close closes the indexer and cleans up resources
func (li *LogIndexer) Close() error {
	logger.Info("Closing log indexer...")

	// Cancel context to stop background processing
	if li.cancel != nil {
		li.cancel()
	}

	// Close the index queue
	if li.indexQueue != nil {
		close(li.indexQueue)
	}

	// Close file watcher
	if li.watcher != nil {
		if err := li.watcher.Close(); err != nil {
			logger.Warnf("Failed to close file watcher: %v", err)
		}
	}

	// Close persistence manager
	if li.persistence != nil {
		if err := li.persistence.Close(); err != nil {
			logger.Warnf("Failed to close persistence manager: %v", err)
		}
	}

	// Close cache
	if li.cache != nil {
		li.cache.Close()
	}

	if li.statsCache != nil {
		li.statsCache.Close()
	}

	// Close Bleve index
	if li.index != nil {
		if err := li.index.Close(); err != nil {
			logger.Errorf("Failed to close Bleve index: %v", err)
			return err
		}
	}

	logger.Info("Log indexer closed successfully")
	return nil
}