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
	EnableCompression    bool          `json:"enable_compression"`
	MemoryQuota          int64         `json:"memory_quota"`           // Memory limit in bytes
	MaxSegmentSize       int64         `json:"max_segment_size"`       // Maximum segment size
	OptimizeInterval     time.Duration `json:"optimize_interval"`      // Auto-optimization interval
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

	return &Config{
		IndexPath:            "./log-index",
		ShardCount:           max(4, maxProcs/2), // Scale shards with CPU cores
		WorkerCount:          workerCount,        // One worker per logical CPU by default (min 2)
		BatchSize:            baseBatchSize,      // Dynamically scaled based on CPU cores
		FlushInterval:        5 * time.Second,
		MaxQueueSize:         baseBatchSize * 10, // Scale queue with batch size
		EnableCompression:    true,
		MemoryQuota:          1024 * 1024 * 1024,         // 1GB
		MaxSegmentSize:       64 * 1024 * 1024,           // 64MB
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
			base.MaxQueueSize = 50000
		} else if maxProcs >= 8 {
			base.BatchSize = 4000 // Large batches for 8+ cores
			base.MaxQueueSize = 40000
		} else {
			base.BatchSize = 3000 // Standard high-throughput batch size
			base.MaxQueueSize = 30000
		}
		base.FlushInterval = 10 * time.Second

	case "low_latency":
		// Minimize latency with reasonable throughput
		base.WorkerCount = int(float64(maxProcs) * 1.5)
		base.BatchSize = 500
		base.MaxQueueSize = 10000
		base.FlushInterval = 2 * time.Second

	case "balanced":
		// Balanced performance (same as default)
		// Already set by DefaultIndexerConfig()

	case "memory_constrained":
		// Reduce memory usage
		base.WorkerCount = max(2, maxProcs/2)
		base.BatchSize = 250
		base.MaxQueueSize = 5000
		base.MemoryQuota = 256 * 1024 * 1024 // 256MB

	case "cpu_intensive":
		// CPU-heavy workloads (parsing, etc.)
		// Optimized for maximum CPU utilization on multi-core systems
		base.WorkerCount = maxProcs * 4 // Even more workers for CPU-bound tasks
		if maxProcs >= 16 {
			base.BatchSize = 4500 // Large batches to keep all cores busy
			base.MaxQueueSize = 45000
		} else if maxProcs >= 8 {
			base.BatchSize = 3500
			base.MaxQueueSize = 35000
		} else {
			base.BatchSize = 2500
			base.MaxQueueSize = 25000
		}

	case "max_performance":
		// Maximum performance mode - uses all available resources
		// WARNING: This will consume significant CPU and memory
		base.WorkerCount = maxProcs * 5    // Maximum workers
		base.ShardCount = max(8, maxProcs) // More shards for parallelism
		if maxProcs >= 16 {
			base.BatchSize = 6000 // Very large batches for maximum throughput
			base.MaxQueueSize = 60000
			base.MemoryQuota = 2 * 1024 * 1024 * 1024 // 2GB
		} else if maxProcs >= 8 {
			base.BatchSize = 5000
			base.MaxQueueSize = 50000
			base.MemoryQuota = 1536 * 1024 * 1024 // 1.5GB
		} else {
			base.BatchSize = 4000
			base.MaxQueueSize = 40000
		}
		base.FlushInterval = 15 * time.Second   // Less frequent flushes for larger batches
		base.MaxSegmentSize = 128 * 1024 * 1024 // 128MB segments
	}

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
	Documents []*Document `json:"documents"`
	Priority  int         `json:"priority"`
	Callback  func(error) `json:"-"`
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
	QueueSize         int                `json:"queue_size"`    // Pending jobs
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
	IndexDocumentAsync(doc *Document, callback func(error))
	IndexDocumentsAsync(docs []*Document, callback func(error))

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

	// Configure text analyzer for better search
	indexMapping.DefaultAnalyzer = "standard"

	// Define document mapping
	docMapping := bleve.NewDocumentMapping()

	// Timestamp field - stored and indexed for range queries
	timestampMapping := bleve.NewNumericFieldMapping()
	timestampMapping.Store = true
	timestampMapping.Index = true
	docMapping.AddFieldMappingsAt("timestamp", timestampMapping)

	// IP field - keyword for exact matching
	ipMapping := bleve.NewTextFieldMapping()
	ipMapping.Store = true
	ipMapping.Index = true
	ipMapping.Analyzer = "keyword"
	ipMapping.DocValues = true // Enable for faceting performance
	docMapping.AddFieldMappingsAt("ip", ipMapping)

	// Geographic fields
	regionMapping := bleve.NewTextFieldMapping()
	regionMapping.Store = true
	regionMapping.Index = true
	regionMapping.Analyzer = "keyword"
	docMapping.AddFieldMappingsAt("region_code", regionMapping)
	docMapping.AddFieldMappingsAt("province", regionMapping)
	docMapping.AddFieldMappingsAt("city", regionMapping)

	// HTTP method - keyword
	methodMapping := bleve.NewTextFieldMapping()
	methodMapping.Store = true
	methodMapping.Index = true
	methodMapping.Analyzer = "keyword"
	docMapping.AddFieldMappingsAt("method", methodMapping)

	// Path field - both analyzed and keyword for different query types
	pathMapping := bleve.NewTextFieldMapping()
	pathMapping.Store = true
	pathMapping.Index = true
	pathMapping.Analyzer = "standard"
	docMapping.AddFieldMappingsAt("path", pathMapping)

	pathKeywordMapping := bleve.NewTextFieldMapping()
	pathKeywordMapping.Store = false
	pathKeywordMapping.Index = true
	pathKeywordMapping.Analyzer = "keyword"
	pathKeywordMapping.DocValues = true // Enable for faceting performance
	docMapping.AddFieldMappingsAt("path_exact", pathKeywordMapping)

	// Status code - numeric for range queries
	statusMapping := bleve.NewNumericFieldMapping()
	statusMapping.Store = true
	statusMapping.Index = true
	docMapping.AddFieldMappingsAt("status", statusMapping)

	// Bytes sent - numeric
	bytesMapping := bleve.NewNumericFieldMapping()
	bytesMapping.Store = true
	bytesMapping.Index = true
	docMapping.AddFieldMappingsAt("bytes_sent", bytesMapping)

	// Referer and User Agent - analyzed text
	textMapping := bleve.NewTextFieldMapping()
	textMapping.Store = true
	textMapping.Index = true
	textMapping.Analyzer = "standard"
	docMapping.AddFieldMappingsAt("referer", textMapping)
	docMapping.AddFieldMappingsAt("user_agent", textMapping)

	// Browser, OS, Device - keywords
	keywordMapping := bleve.NewTextFieldMapping()
	keywordMapping.Store = true
	keywordMapping.Index = true
	keywordMapping.Analyzer = "keyword"
	docMapping.AddFieldMappingsAt("browser", keywordMapping)
	docMapping.AddFieldMappingsAt("browser_version", keywordMapping)
	docMapping.AddFieldMappingsAt("os", keywordMapping)
	docMapping.AddFieldMappingsAt("os_version", keywordMapping)
	docMapping.AddFieldMappingsAt("device_type", keywordMapping)

	// Request and upstream time - numeric
	timeMapping := bleve.NewNumericFieldMapping()
	timeMapping.Store = true
	timeMapping.Index = true
	docMapping.AddFieldMappingsAt("request_time", timeMapping)
	docMapping.AddFieldMappingsAt("upstream_time", timeMapping)

	// Raw log line - stored but not indexed (for retrieval)
	rawMapping := bleve.NewTextFieldMapping()
	rawMapping.Store = true
	rawMapping.Index = false
	docMapping.AddFieldMappingsAt("raw", rawMapping)

	// File path - keyword for filtering by file
	fileMapping := bleve.NewTextFieldMapping()
	fileMapping.Store = true
	fileMapping.Index = true
	fileMapping.Analyzer = "keyword"
	docMapping.AddFieldMappingsAt("file_path", fileMapping)

	// Main log path - keyword for efficient log group filtering
	mainLogMapping := bleve.NewTextFieldMapping()
	mainLogMapping.Store = true
	mainLogMapping.Index = true
	mainLogMapping.Analyzer = "keyword"
	mainLogMapping.DocValues = true // Enable for efficient faceting and filtering
	docMapping.AddFieldMappingsAt("main_log_path", mainLogMapping)

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
