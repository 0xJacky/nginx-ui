package indexer

import (
	"context"
	"runtime"
	"time"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/mapping"
)

// IndexStatus represents different states of log indexing
type IndexStatus string

// Index status constants
const (
	IndexStatusNotIndexed IndexStatus = "not_indexed" // File not indexed
	IndexStatusQueued     IndexStatus = "queued"      // Waiting in queue
	IndexStatusIndexing   IndexStatus = "indexing"    // Currently being indexed
	IndexStatusIndexed    IndexStatus = "indexed"     // Successfully indexed
	IndexStatusError      IndexStatus = "error"       // Index failed with error
)

// IndexStatusDetails contains detailed status information
type IndexStatusDetails struct {
	Status        IndexStatus    `json:"status"`
	Message       string         `json:"message,omitempty"`
	ErrorMessage  string         `json:"error_message,omitempty"`
	ErrorTime     *time.Time     `json:"error_time,omitempty"`
	RetryCount    int            `json:"retry_count,omitempty"`
	QueuePosition int            `json:"queue_position,omitempty"`
	Progress      *IndexProgress `json:"progress,omitempty"`
}

// IndexProgress contains indexing progress information
type IndexProgress struct {
	Percent        float64 `json:"percent"`
	ProcessedLines int64   `json:"processed_lines"`
	TotalLines     int64   `json:"total_lines"`
	ProcessedBytes int64   `json:"processed_bytes"`
	TotalBytes     int64   `json:"total_bytes"`
	Speed          int64   `json:"speed"` // lines per second
	ETA            int64   `json:"eta"`   // estimated time to completion in seconds
}

// IndexerConfig holds configuration for the indexer
type Config struct {
	IndexPath            string        `json:"index_path"`
	ShardCount           int           `json:"shard_count"`
	WorkerCount          int           `json:"worker_count"`
	BatchSize            int           `json:"batch_size"`
	FlushInterval        time.Duration `json:"flush_interval"`
	MaxQueueSize         int           `json:"max_queue_size"`
	EnableCompression    bool          `json:"enable_compression"` // Deprecated: Scorch/Zap compression is always enabled
	MemoryQuota          int64         `json:"memory_quota"`       // Maximum estimated bytes retained by queued and active jobs
	MaxSegmentSize       int64         `json:"max_segment_size"`   // Maximum Scorch in-memory merge input per persister worker
	OptimizeInterval     time.Duration `json:"optimize_interval"`  // Auto-optimization interval
	EnableMetrics        bool          `json:"enable_metrics"`
	FileGroupConcurrency int           `json:"file_group_concurrency"` // Max concurrent files within a log group (0 = use WorkerCount)
}

// DefaultIndexerConfig returns default indexer configuration with processor optimization
func DefaultIndexerConfig() *Config {
	maxProcs := runtime.GOMAXPROCS(0)

	// Dynamically scale batch size based on CPU cores
	// Significantly increased batch sizes to maximize frontend indexing throughput
	baseBatchSize := 15000
	if maxProcs >= 16 {
		baseBatchSize = 25000 // High-core systems (16+ cores) - maximum throughput
	} else if maxProcs >= 8 {
		baseBatchSize = 20000 // Mid-range systems (8-15 cores) - high throughput
	} else if maxProcs >= 4 {
		baseBatchSize = 18000 // Standard systems (4-7 cores) - good throughput
	}

	// Derive conservative, CPU-aware defaults to avoid oversubscribing small machines.
	// Treat GOMAXPROCS as the upper bound for CPU-bound worker concurrency.
	workerCount := maxProcs
	if workerCount < 2 {
		workerCount = 2
	}

	// Limit file-level concurrency to at most half of the logical CPUs by default.
	fileGroupConcurrency := maxProcs / 2
	if fileGroupConcurrency < 2 {
		fileGroupConcurrency = 2
	}
	shardCount := 1
	if maxProcs >= 8 {
		shardCount = 2
	}

	return &Config{
		IndexPath:            "./log-index",
		ShardCount:           shardCount,
		WorkerCount:          workerCount,   // One worker per logical CPU by default (min 2)
		BatchSize:            baseBatchSize, // Dynamically scaled based on CPU cores
		FlushInterval:        5 * time.Second,
		MaxQueueSize:         max(4, workerCount*2),
		EnableCompression:    true,
		MemoryQuota:          1024 * 1024 * 1024, // 1GB
		MaxSegmentSize:       64 * 1024 * 1024,   // 64MB
		OptimizeInterval:     30 * time.Minute,
		EnableMetrics:        true,
		FileGroupConcurrency: fileGroupConcurrency, // Default: up to 50% of logical CPUs for file-level parallelism
	}
}

// GetConfig returns configuration optimized for specific scenarios
func GetConfig(scenario string) *Config {
	base := DefaultIndexerConfig()
	maxProcs := runtime.GOMAXPROCS(0)

	switch scenario {
	case "high_throughput":
		// Maximize throughput at cost of higher latency
		// Aggressively utilize multi-core CPUs
		base.WorkerCount = maxProcs * 4 // Aggressive worker scaling for I/O-bound operations
		if maxProcs >= 16 {
			base.BatchSize = 5000 // Very large batches for 16+ cores
		} else if maxProcs >= 8 {
			base.BatchSize = 4000 // Large batches for 8+ cores
		} else {
			base.BatchSize = 3000 // Standard high-throughput batch size
		}
		base.FlushInterval = 10 * time.Second

	case "low_latency":
		// Minimize latency with reasonable throughput
		base.WorkerCount = int(float64(maxProcs) * 1.5)
		base.BatchSize = 500
		base.FlushInterval = 2 * time.Second

	case "balanced":
		// Balanced performance (same as default)
		// Already set by DefaultIndexerConfig()

	case "memory_constrained":
		// Reduce memory usage
		base.WorkerCount = max(2, maxProcs/2)
		base.BatchSize = 250
		base.MemoryQuota = 256 * 1024 * 1024 // 256MB

	case "cpu_intensive":
		// CPU-heavy workloads (parsing, etc.)
		// Optimized for maximum CPU utilization on multi-core systems
		base.WorkerCount = maxProcs * 4 // Even more workers for CPU-bound tasks
		if maxProcs >= 16 {
			base.BatchSize = 4500 // Large batches to keep all cores busy
		} else if maxProcs >= 8 {
			base.BatchSize = 3500
		} else {
			base.BatchSize = 2500
		}

	case "max_performance":
		// Maximum performance mode - uses all available resources
		// WARNING: This will consume significant CPU and memory
		base.WorkerCount = maxProcs * 5    // Maximum workers
		base.ShardCount = max(8, maxProcs) // More shards for parallelism
		if maxProcs >= 16 {
			base.BatchSize = 6000                     // Very large batches for maximum throughput
			base.MemoryQuota = 2 * 1024 * 1024 * 1024 // 2GB
		} else if maxProcs >= 8 {
			base.BatchSize = 5000
			base.MemoryQuota = 1536 * 1024 * 1024 // 1.5GB
		} else {
			base.BatchSize = 4000
		}
		base.FlushInterval = 15 * time.Second   // Less frequent flushes for larger batches
		base.MaxSegmentSize = 128 * 1024 * 1024 // 128MB segments
	}

	// IndexDocuments waits for completion, so retaining thousands of whole
	// batches cannot improve worker throughput and only expands peak memory.
	base.MaxQueueSize = max(4, base.WorkerCount*2)

	return base
}

// Document represents a document to be indexed
type Document struct {
	ID     string       `json:"id"`
	Fields *LogDocument `json:"fields"`
}

// LogDocument represents the structured data for a log entry
type LogDocument struct {
	Timestamp    int64    `json:"timestamp"`
	IP           string   `json:"ip"`
	RegionCode   string   `json:"region_code,omitempty"`
	Province     string   `json:"province,omitempty"`
	City         string   `json:"city,omitempty"`
	Method       string   `json:"method"`
	Path         string   `json:"path"`
	PathExact    string   `json:"path_exact"`
	Protocol     string   `json:"protocol,omitempty"`
	Status       int      `json:"status"`
	BytesSent    int64    `json:"bytes_sent"`
	Referer      string   `json:"referer,omitempty"`
	UserAgent    string   `json:"user_agent,omitempty"`
	Browser      string   `json:"browser,omitempty"`
	BrowserVer   string   `json:"browser_version,omitempty"`
	OS           string   `json:"os,omitempty"`
	OSVersion    string   `json:"os_version,omitempty"`
	DeviceType   string   `json:"device_type,omitempty"`
	RequestTime  float64  `json:"request_time,omitempty"`
	UpstreamTime *float64 `json:"upstream_time,omitempty"`
	FilePath     string   `json:"file_path"`     // Actual physical file path (e.g., /var/log/nginx/access.log.1.gz)
	MainLogPath  string   `json:"main_log_path"` // Main log group path (e.g., /var/log/nginx/access.log)
	Raw          string   `json:"raw"`
}

// IndexJob represents a single indexing job
type IndexJob struct {
	Documents   []*Document `json:"documents"`
	Priority    int         `json:"priority"`
	Callback    func(error) `json:"-"`
	memoryBytes int64
}

// IndexResult represents the result of an indexing operation
type IndexResult struct {
	Processed  int           `json:"processed"`
	Succeeded  int           `json:"succeeded"`
	Failed     int           `json:"failed"`
	Duration   time.Duration `json:"duration"`
	ErrorRate  float64       `json:"error_rate"`
	Throughput float64       `json:"throughput"` // Documents per second
}

// ShardInfo contains information about a single shard
type ShardInfo struct {
	ID            int    `json:"id"`
	Path          string `json:"path"`
	DocumentCount uint64 `json:"document_count"`
	Size          int64  `json:"size"`
	LastUpdated   int64  `json:"last_updated"`
}

// IndexStats provides comprehensive indexing statistics
type IndexStats struct {
	TotalDocuments    uint64             `json:"total_documents"`
	TotalSize         int64              `json:"total_size"`
	ShardCount        int                `json:"shard_count"`
	Shards            []*ShardInfo       `json:"shards"`
	IndexingRate      float64            `json:"indexing_rate"` // Docs per second
	MemoryUsage       int64              `json:"memory_usage"`  // Bytes
	QueueMemoryUsage  int64              `json:"queue_memory_usage"`
	QueueSize         int                `json:"queue_size"` // Pending jobs
	WorkerStats       []*WorkerStats     `json:"worker_stats"`
	LastOptimized     int64              `json:"last_optimized"` // Unix timestamp
	OptimizationStats *OptimizationStats `json:"optimization_stats,omitempty"`
}

// WorkerStats tracks individual worker performance
type WorkerStats struct {
	ID             int           `json:"id"`
	ProcessedJobs  int64         `json:"processed_jobs"`
	ProcessedDocs  int64         `json:"processed_docs"`
	ErrorCount     int64         `json:"error_count"`
	LastActive     int64         `json:"last_active"`
	AverageLatency time.Duration `json:"average_latency"`
	Status         string        `json:"status"` // idle, busy, error
}

// OptimizationStats tracks optimization operations
type OptimizationStats struct {
	LastRun        int64         `json:"last_run"`
	Duration       time.Duration `json:"duration"`
	SegmentsBefore int           `json:"segments_before"`
	SegmentsAfter  int           `json:"segments_after"`
	SizeReduction  int64         `json:"size_reduction"`
	Success        bool          `json:"success"`
}

// Indexer interface defines the contract for all indexer implementations
type Indexer interface {
	IndexDocument(ctx context.Context, doc *Document) error
	IndexDocuments(ctx context.Context, docs []*Document) error

	StartBatch() BatchWriterInterface
	FlushAll() error

	Optimize() error
	GetStats() *IndexStats
	GetShardInfo(shardID int) (*ShardInfo, error)

	Start(ctx context.Context) error
	Stop() error
	IsHealthy() bool

	GetConfig() *Config
	UpdateConfig(config *Config) error
}

// BatchWriterInterface provides efficient batch writing capabilities
type BatchWriterInterface interface {
	Add(doc *Document) error
	Flush() (*IndexResult, error)
	Size() int
	Reset()
}

// ShardManager manages multiple index shards
type ShardManager interface {
	Initialize() error
	GetShard(key string) (bleve.Index, int, error)
	// GetShardForDocument routes by main log group and key; required for grouped manager
	// mainLogPath must be non-empty
	GetShardForDocument(mainLogPath string, key string) (bleve.Index, int, error)
	GetShardByID(id int) (bleve.Index, error)
	GetAllShards() []bleve.Index
	GetShardStats() []*ShardInfo
	CreateShard(id int, path string) error
	CloseShard(id int) error
	OptimizeShard(id int) error
	HealthCheck() error
	Close() error // Close all shards and cleanup resources
}

// MetricsCollector collects and reports indexing metrics
type MetricsCollector interface {
	RecordIndexOperation(docs int, duration time.Duration, success bool)
	RecordBatchOperation(batchSize int, duration time.Duration)
	RecordOptimization(duration time.Duration, success bool)
	GetMetrics() *Metrics
	Reset()
}

// Metrics represents comprehensive indexing metrics
type Metrics struct {
	TotalOperations    int64   `json:"total_operations"`
	SuccessOperations  int64   `json:"success_operations"`
	FailedOperations   int64   `json:"failed_operations"`
	TotalDocuments     int64   `json:"total_documents"`
	TotalBatches       int64   `json:"total_batches"`
	OptimizationCount  int64   `json:"optimization_count"`
	IndexingRate       float64 `json:"indexing_rate"` // docs per second
	SuccessRate        float64 `json:"success_rate"`
	AverageLatencyMS   float64 `json:"average_latency_ms"`
	MinLatencyMS       float64 `json:"min_latency_ms"`
	MaxLatencyMS       float64 `json:"max_latency_ms"`
	AverageThroughput  float64 `json:"average_throughput"` // docs per second
	AverageBatchTimeMS float64 `json:"average_batch_time_ms"`
	AverageOptTimeS    float64 `json:"average_optimization_time_s"`
}

// CreateLogIndexMapping creates optimized index mapping for log entries
func CreateLogIndexMapping() mapping.IndexMapping {
	indexMapping := bleve.NewIndexMapping()
	indexMapping.DefaultAnalyzer = "standard"
	indexMapping.DefaultField = "raw"
	indexMapping.IndexDynamic = false
	indexMapping.StoreDynamic = false
	indexMapping.DocValuesDynamic = false

	docMapping := bleve.NewDocumentStaticMapping()

	type fieldOptions struct {
		store              bool
		index              bool
		includeTermVectors bool
		docValues          bool
	}

	addTextField := func(name, analyzer string, options fieldOptions) {
		fieldMapping := bleve.NewTextFieldMapping()
		fieldMapping.Analyzer = analyzer
		fieldMapping.Store = options.store
		fieldMapping.Index = options.index
		fieldMapping.IncludeTermVectors = options.includeTermVectors
		fieldMapping.IncludeInAll = false
		fieldMapping.DocValues = options.docValues
		docMapping.AddFieldMappingsAt(name, fieldMapping)
	}
	addNumericField := func(name string, options fieldOptions) {
		fieldMapping := bleve.NewNumericFieldMapping()
		fieldMapping.Store = options.store
		fieldMapping.Index = options.index
		fieldMapping.IncludeInAll = false
		fieldMapping.DocValues = options.docValues
		docMapping.AddFieldMappingsAt(name, fieldMapping)
	}

	storedAndIndexed := fieldOptions{store: true, index: true}
	storedIndexedAndSortable := fieldOptions{store: true, index: true, docValues: true}
	storedPhraseSearchable := fieldOptions{store: true, index: true, includeTermVectors: true}

	// Keep doc values only on fields used by sorting, faceting, or cardinality queries.
	addNumericField("timestamp", storedIndexedAndSortable)
	addTextField("ip", "keyword", storedIndexedAndSortable)
	addTextField("region_code", "keyword", storedIndexedAndSortable)
	addTextField("province", "keyword", storedIndexedAndSortable)
	addTextField("city", "keyword", storedAndIndexed)
	addTextField("method", "keyword", storedIndexedAndSortable)
	addTextField("path", "standard", storedPhraseSearchable)
	addTextField("path_exact", "keyword", fieldOptions{index: true, docValues: true})
	addTextField("protocol", "keyword", fieldOptions{store: true})
	addNumericField("status", storedIndexedAndSortable)
	addNumericField("bytes_sent", storedIndexedAndSortable)
	addTextField("referer", "standard", storedPhraseSearchable)
	addTextField("user_agent", "standard", fieldOptions{
		store: true, index: true, includeTermVectors: true, docValues: true,
	})
	addTextField("browser", "keyword", storedIndexedAndSortable)
	addTextField("browser_version", "keyword", storedAndIndexed)
	addTextField("os", "keyword", storedIndexedAndSortable)
	addTextField("os_version", "keyword", storedAndIndexed)
	addTextField("device_type", "keyword", storedIndexedAndSortable)
	addNumericField("request_time", storedAndIndexed)
	addNumericField("upstream_time", storedAndIndexed)

	// Index the original line once for default full-text search instead of
	// duplicating every field into Bleve's composite _all field.
	addTextField("raw", "standard", storedAndIndexed)
	addTextField("file_path", "keyword", storedAndIndexed)
	addTextField("main_log_path", "keyword", storedAndIndexed)

	indexMapping.AddDocumentMapping("_default", docMapping)

	return indexMapping
}

// Priority levels for indexing jobs
const (
	PriorityLow      = 0
	PriorityNormal   = 50
	PriorityHigh     = 100
	PriorityCritical = 150
)

// Worker status constants
const (
	WorkerStatusIdle    = "idle"
	WorkerStatusBusy    = "busy"
	WorkerStatusError   = "error"
	WorkerStatusStopped = "stopped"
)

// Error types for indexer operations
var (
	ErrIndexerNotStarted  = "indexer not started"
	ErrIndexerStopped     = "indexer stopped"
	ErrShardNotFound      = "shard not found"
	ErrQueueFull          = "queue is full"
	ErrInvalidDocument    = "invalid document"
	ErrOptimizationFailed = "optimization failed"
)

// MetadataManager defines the interface for managing log index metadata
type MetadataManager interface {
	// SaveIndexMetadata saves metadata for a log group after indexing
	SaveIndexMetadata(basePath string, documentCount uint64, startTime time.Time, duration time.Duration, minTime *time.Time, maxTime *time.Time) error
	// DeleteIndexMetadataByGroup deletes all database records for a log group
	DeleteIndexMetadataByGroup(basePath string) error
	// DeleteAllIndexMetadata deletes all index metadata from the database
	DeleteAllIndexMetadata() error
	// GetFilePathsForGroup returns all physical file paths for a given log group
	GetFilePathsForGroup(basePath string) ([]string, error)
}

// GroupFileProvider defines the interface for getting file paths for a log group
type GroupFileProvider interface {
	// GetFilePathsForGroup returns all physical file paths for a given log group
	GetFilePathsForGroup(basePath string) ([]string, error)
}

// FlushableIndexer defines the interface for indexers that can be flushed
type FlushableIndexer interface {
	// FlushAll flushes all pending operations
	FlushAll() error
}

// RestartableIndexer defines the interface for indexers that can be restarted
type RestartableIndexer interface {
	// Start begins the indexer operation
	Start(context.Context) error
}
