package nginx_log

import (
	"sync"
	"time"
)

// LogFileInfo holds metadata about a log file
type LogFileInfo struct {
	Path         string
	LastModified time.Time
	LastSize     int64
	LastIndexed  time.Time
	IsCompressed bool
	TimeRange    *TimeRange
}

// TimeRange represents a time range for log entries
type TimeRange struct {
	Start time.Time
	End   time.Time
}

// CachedSearchResult represents a cached search result with total count
type CachedSearchResult struct {
	Entries []*AccessLogEntry
	Total   int
}

// CachedStatsResult represents cached statistics for a query with metadata
type CachedStatsResult struct {
	Stats          *SummaryStats `json:"stats"`
	QueryHash      string        `json:"query_hash"`      // Hash of the query parameters
	LastCalculated time.Time     `json:"last_calculated"` // When stats were calculated
	FilesModTime   time.Time     `json:"files_mod_time"`  // Latest modification time of all log files
	DocCount       uint64        `json:"doc_count"`       // Document count when stats were calculated
}

// IndexTask represents a background indexing task
type IndexTask struct {
	FilePath    string
	Priority    int // Higher priority = process first
	FullReindex bool
	Wg          *sync.WaitGroup // Add WaitGroup for synchronization
}

// IndexedLogEntry represents a log entry stored in the index
type IndexedLogEntry struct {
	ID           string    `json:"id"`
	FilePath     string    `json:"file_path"`
	Timestamp    time.Time `json:"timestamp"`
	IP           string    `json:"ip"`
	RegionCode   string    `json:"region_code"`
	Province     string    `json:"province"`
	City         string    `json:"city"`
	ISP          string    `json:"isp"`
	Method       string    `json:"method"`
	Path         string    `json:"path"`
	Protocol     string    `json:"protocol"`
	Status       int       `json:"status"`
	BytesSent    int64     `json:"bytes_sent"`
	Referer      string    `json:"referer"`
	UserAgent    string    `json:"user_agent"`
	Browser      string    `json:"browser"`
	BrowserVer   string    `json:"browser_version"`
	OS           string    `json:"os"`
	OSVersion    string    `json:"os_version"`
	DeviceType   string    `json:"device_type"`
	RequestTime  float64   `json:"request_time"`
	UpstreamTime *float64  `json:"upstream_time,omitempty"`
	Raw          string    `json:"raw"`
}

// QueryRequest represents a search query for logs
type QueryRequest struct {
	StartTime      time.Time `json:"start_time"`
	EndTime        time.Time `json:"end_time"`
	Query          string    `json:"query,omitempty"`
	IP             string    `json:"ip,omitempty"`
	Method         string    `json:"method,omitempty"`
	Status         []int     `json:"status,omitempty"`
	Path           string    `json:"path,omitempty"`
	UserAgent      string    `json:"user_agent,omitempty"`
	Referer        string    `json:"referer,omitempty"`
	Browser        string    `json:"browser,omitempty"`
	OS             string    `json:"os,omitempty"`
	Device         string    `json:"device,omitempty"`
	Limit          int       `json:"limit"`
	Offset         int       `json:"offset"`
	SortBy         string    `json:"sort_by"`
	SortOrder      string    `json:"sort_order"`
	LogPath        string    `json:"log_path,omitempty"`
	IncludeSummary bool      `json:"include_summary,omitempty"`
}

// SummaryStats represents the summary statistics for log entries
type SummaryStats struct {
	UV              int     `json:"uv"`                 // Unique Visitors (unique IPs)
	PV              int     `json:"pv"`                 // Page Views (total requests)
	TotalTraffic    int64   `json:"total_traffic"`      // Total bytes sent
	UniquePages     int     `json:"unique_pages"`       // Unique pages visited
	AvgTrafficPerPV float64 `json:"avg_traffic_per_pv"` // Average traffic per page view
}

// QueryResult represents the result of a search query
type QueryResult struct {
	Entries      []*AccessLogEntry `json:"entries"`
	Total        int               `json:"total"`
	Took         time.Duration     `json:"took"`
	Aggregations map[string]int    `json:"aggregations,omitempty"`
	Summary      *SummaryStats     `json:"summary,omitempty"`
	FromCache    bool              `json:"from_cache,omitempty"`
}
