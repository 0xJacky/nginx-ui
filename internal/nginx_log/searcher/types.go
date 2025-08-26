package searcher

import (
	"context"
	"time"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/search/query"
)

// SearcherConfig holds configuration for the searcher
type Config struct {
	MaxConcurrency     int           `json:"max_concurrency"`
	TimeoutDuration    time.Duration `json:"timeout_duration"`
	CacheSize          int           `json:"cache_size"`
	EnableCache        bool          `json:"enable_cache"`
	DefaultLimit       int           `json:"default_limit"`
	MaxLimit           int           `json:"max_limit"`
	EnableHighlighting bool          `json:"enable_highlighting"`
	EnableFaceting     bool          `json:"enable_faceting"`
	ShardTimeout       time.Duration `json:"shard_timeout"`
}

// DefaultSearcherConfig returns default searcher configuration
func DefaultSearcherConfig() *Config {
	return &Config{
		MaxConcurrency:     10,
		TimeoutDuration:    30 * time.Second,
		CacheSize:          1000,
		EnableCache:        true,
		DefaultLimit:       50,
		MaxLimit:           10000,
		EnableHighlighting: true,
		EnableFaceting:     true,
		ShardTimeout:       5 * time.Second,
	}
}

// SearchCache defines the interface for search result caching
type SearchCache interface {
	Get(req *SearchRequest) *SearchResult
	Put(req *SearchRequest, result *SearchResult, ttl time.Duration)
	Clear()
	GetStats() *CacheStats
	Close()
}

// SearchRequest represents a search query request
type SearchRequest struct {
	// Query parameters
	Query  string   `json:"query,omitempty"`
	Fields []string `json:"fields,omitempty"`

	// Filters
	LogPaths    []string `json:"log_paths,omitempty"`
	StartTime   *int64   `json:"start_time,omitempty"` // Unix timestamp
	EndTime     *int64   `json:"end_time,omitempty"`   // Unix timestamp
	IPAddresses []string `json:"ip_addresses,omitempty"`
	Methods     []string `json:"methods,omitempty"`
	StatusCodes []int    `json:"status_codes,omitempty"`
	Paths       []string `json:"paths,omitempty"`
	UserAgents  []string `json:"user_agents,omitempty"`
	Referers    []string `json:"referers,omitempty"`
	Countries   []string `json:"countries,omitempty"`
	Browsers    []string `json:"browsers,omitempty"`
	OSs         []string `json:"operating_systems,omitempty"`
	Devices     []string `json:"devices,omitempty"`

	// Range filters
	MinBytes   *int64   `json:"min_bytes,omitempty"`
	MaxBytes   *int64   `json:"max_bytes,omitempty"`
	MinReqTime *float64 `json:"min_request_time,omitempty"`
	MaxReqTime *float64 `json:"max_request_time,omitempty"`

	// Pagination
	Limit  int `json:"limit"`
	Offset int `json:"offset"`

	// Sorting
	SortBy    string `json:"sort_by,omitempty"`
	SortOrder string `json:"sort_order,omitempty"` // "asc" or "desc"

	// Additional options
	IncludeHighlighting bool     `json:"include_highlighting,omitempty"`
	IncludeFacets       bool     `json:"include_facets,omitempty"`
	FacetFields         []string `json:"facet_fields,omitempty"`
	FacetSize           int      `json:"facet_size,omitempty"` // Number of terms to return for each facet
	IncludeStats        bool     `json:"include_stats,omitempty"`

	// Performance options
	Timeout  time.Duration `json:"timeout,omitempty"`
	UseCache bool          `json:"use_cache,omitempty"`
	CacheKey string        `json:"cache_key,omitempty"`
}

// SearchResult represents the result of a search operation
type SearchResult struct {
	// Results
	Hits      []*SearchHit `json:"hits"`
	TotalHits uint64       `json:"total_hits"`
	MaxScore  float64      `json:"max_score"`

	// Metadata
	Duration     time.Duration  `json:"duration"`
	ShardResults []*ShardResult `json:"shard_results,omitempty"`

	// Aggregations
	Facets map[string]*Facet `json:"facets,omitempty"`
	Stats  *SearchStats      `json:"stats,omitempty"`

	// Cache info
	FromCache bool `json:"from_cache,omitempty"`
	CacheHit  bool `json:"cache_hit,omitempty"`
	
	// Warning message for deep pagination or other issues
	Warning string `json:"warning,omitempty"`
}

// SearchHit represents a single search result
type SearchHit struct {
	ID           string                 `json:"id"`
	Score        float64                `json:"score"`
	Fields       map[string]interface{} `json:"fields"`
	Highlighting map[string][]string    `json:"highlighting,omitempty"`
	Index        string                 `json:"index,omitempty"` // Shard identifier
}

// ShardResult represents results from a single shard
type ShardResult struct {
	ShardID   int           `json:"shard_id"`
	Hits      uint64        `json:"hits"`
	Duration  time.Duration `json:"duration"`
	Error     string        `json:"error,omitempty"`
	Succeeded bool          `json:"succeeded"`
}

// Facet represents aggregated data for a field
type Facet struct {
	Field   string       `json:"field"`
	Total   int          `json:"total"`
	Missing int          `json:"missing"`
	Other   int          `json:"other"`
	Terms   []*FacetTerm `json:"terms"`
}

// FacetTerm represents a single term in a facet
type FacetTerm struct {
	Term  string `json:"term"`
	Count int    `json:"count"`
}

// SearchStats provides statistical information about search results
type SearchStats struct {
	TotalBytes     int64          `json:"total_bytes"`
	AvgBytes       float64        `json:"avg_bytes"`
	MinBytes       int64          `json:"min_bytes"`
	MaxBytes       int64          `json:"max_bytes"`
	TotalReqTime   float64        `json:"total_request_time"`
	AvgReqTime     float64        `json:"avg_request_time"`
	MinReqTime     float64        `json:"min_request_time"`
	MaxReqTime     float64        `json:"max_request_time"`
	UniqueIPs      int            `json:"unique_ips"`
	UniquePaths    int            `json:"unique_paths"`
	StatusCodeDist map[string]int `json:"status_code_distribution"`
	MethodDist     map[string]int `json:"method_distribution"`
}

// AggregationRequest represents a request for aggregated data
type AggregationRequest struct {
	Field      string            `json:"field"`
	Type       AggregationType   `json:"type"`
	Size       int               `json:"size,omitempty"`
	Interval   string            `json:"interval,omitempty"`    // For date histograms
	DateFormat string            `json:"date_format,omitempty"` // For date formatting
	Filters    map[string]string `json:"filters,omitempty"`
}

// AggregationType defines the type of aggregation
type AggregationType string

const (
	AggregationTerms         AggregationType = "terms"
	AggregationHistogram     AggregationType = "histogram"
	AggregationDateHistogram AggregationType = "date_histogram"
	AggregationStats         AggregationType = "stats"
	AggregationCardinality   AggregationType = "cardinality"
)

// CacheEntry represents a cached search result
type CacheEntry struct {
	Result    *SearchResult `json:"result"`
	CreatedAt time.Time     `json:"created_at"`
	ExpiresAt time.Time     `json:"expires_at"`
	HitCount  int64         `json:"hit_count"`
	Size      int64         `json:"size"` // Estimated memory size in bytes
}

// ShardSearcher defines the interface for searching individual shards
type ShardSearcher interface {
	Search(ctx context.Context, shardID int, req *SearchRequest) (*SearchResult, error)
	GetShardInfo(shardID int) (*ShardInfo, error)
	IsShardHealthy(shardID int) bool
}

// Searcher defines the main search interface
type Searcher interface {
	Search(ctx context.Context, req *SearchRequest) (*SearchResult, error)
	SearchAsync(ctx context.Context, req *SearchRequest) (<-chan *SearchResult, <-chan error)

	Aggregate(ctx context.Context, req *AggregationRequest) (*AggregationResult, error)

	Suggest(ctx context.Context, text string, field string, size int) ([]*Suggestion, error)

	Analyze(ctx context.Context, text string, analyzer string) ([]string, error)

	ClearCache() error
	GetCacheStats() *CacheStats

	IsHealthy() bool
	GetStats() *Stats
	GetConfig() *Config
	Stop() error
}

// AggregationResult represents the result of an aggregation
type AggregationResult struct {
	Field    string          `json:"field"`
	Type     AggregationType `json:"type"`
	Total    int             `json:"total"`
	Data     interface{}     `json:"data"`
	Duration time.Duration   `json:"duration"`
}

// Suggestion represents a search suggestion
type Suggestion struct {
	Text  string  `json:"text"`
	Score float64 `json:"score"`
	Freq  int64   `json:"freq"`
}

// Stats provides comprehensive search statistics
type Stats struct {
	TotalSearches      int64               `json:"total_searches"`
	SuccessfulSearches int64               `json:"successful_searches"`
	FailedSearches     int64               `json:"failed_searches"`
	AverageLatency     time.Duration       `json:"average_latency"`
	MinLatency         time.Duration       `json:"min_latency"`
	MaxLatency         time.Duration       `json:"max_latency"`
	CacheStats         *CacheStats         `json:"cache_stats"`
	ShardStats         []*ShardSearchStats `json:"shard_stats"`
	ActiveSearches     int32               `json:"active_searches"`
	QueuedSearches     int                 `json:"queued_searches"`
}

// ShardSearchStats provides per-shard search statistics
type ShardSearchStats struct {
	ShardID        int           `json:"shard_id"`
	SearchCount    int64         `json:"search_count"`
	ErrorCount     int64         `json:"error_count"`
	AverageLatency time.Duration `json:"average_latency"`
	LastSearchTime time.Time     `json:"last_search_time"`
	IsHealthy      bool          `json:"is_healthy"`
}

// ShardInfo represents information about a shard for searching
type ShardInfo struct {
	ID            int    `json:"id"`
	DocumentCount uint64 `json:"document_count"`
	Size          int64  `json:"size"`
	IsHealthy     bool   `json:"is_healthy"`
	LastUpdated   int64  `json:"last_updated"`
}

// Query builder types for complex queries
type QueryBuilder interface {
	Query() query.Query
}

// BoolQueryBuilder builds boolean queries
type BoolQueryBuilder struct {
	must    []query.Query
	mustNot []query.Query
	should  []query.Query
}

// NewBoolQueryBuilder creates a new boolean query builder
func NewBoolQueryBuilder() *BoolQueryBuilder {
	return &BoolQueryBuilder{}
}

// Must adds a must clause
func (b *BoolQueryBuilder) Must(q query.Query) *BoolQueryBuilder {
	b.must = append(b.must, q)
	return b
}

// MustNot adds a must not clause
func (b *BoolQueryBuilder) MustNot(q query.Query) *BoolQueryBuilder {
	b.mustNot = append(b.mustNot, q)
	return b
}

// Should adds a should clause
func (b *BoolQueryBuilder) Should(q query.Query) *BoolQueryBuilder {
	b.should = append(b.should, q)
	return b
}

// Query builds the final boolean query
func (b *BoolQueryBuilder) Query() query.Query {
	boolQuery := bleve.NewBooleanQuery()

	for _, q := range b.must {
		boolQuery.AddMust(q)
	}

	for _, q := range b.mustNot {
		boolQuery.AddMustNot(q)
	}

	for _, q := range b.should {
		boolQuery.AddShould(q)
	}

	return boolQuery
}

// Search operation constants
const (
	DefaultSortField = "_score"
	SortOrderAsc     = "asc"
	SortOrderDesc    = "desc"

	// Cache constants
	DefaultCacheTTL = 5 * time.Minute
	MaxCacheSize    = 10000

	// Facet constants
	DefaultFacetSize = 10
	MaxFacetSize     = 1000
)

// Error types for search operations
var (
	ErrSearchTimeout    = "search timeout"
	ErrInvalidQuery     = "invalid query"
	ErrShardUnavailable = "shard unavailable"
	ErrCacheMiss        = "cache miss"
	ErrTooManyResults   = "too many results"
)
