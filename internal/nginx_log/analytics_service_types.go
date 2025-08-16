package nginx_log

import (
	"time"
)

// KeyValue represents a key-value pair for analytics
type KeyValue struct {
	Key   string `json:"key"`
	Value int    `json:"value"`
}

// FileStatus represents the status of a log file
type FileStatus struct {
	Path           string    `json:"path"`
	LastModified   time.Time `json:"last_modified"`
	LastSize       int64     `json:"last_size"`
	LastIndexed    time.Time `json:"last_indexed"`
	IsCompressed   bool      `json:"is_compressed"`
	HasTimeRange   bool      `json:"has_timerange"`
	TimeRangeStart time.Time `json:"timerange_start,omitzero"`
	TimeRangeEnd   time.Time `json:"timerange_end,omitzero"`
}

// IndexStatus represents comprehensive index status and statistics
type IndexStatus struct {
	DocumentCount uint64       `json:"document_count"`
	LogPaths      []string     `json:"log_paths"`
	LogPathsCount int          `json:"log_paths_count"`
	TotalFiles    int          `json:"total_files"`
	Files         []FileStatus `json:"files"`
}

// LogAnalytics represents comprehensive log analytics
type LogAnalytics struct {
	TotalRequests       int            `json:"total_requests"`
	UniqueIPCount       int            `json:"unique_ip_count"`
	AverageResponseTime float64        `json:"average_response_time"`
	ErrorRate           float64        `json:"error_rate"`
	UniqueIPs           map[string]int `json:"-"` // Internal use only
	StatusCodes         map[int]int    `json:"status_codes"`
	TopPaths            map[string]int `json:"-"` // Internal use only
	TopIPs              map[string]int `json:"-"` // Internal use only
	Countries           map[string]int `json:"-"` // Internal use only
	Browsers            map[string]int `json:"-"` // Internal use only
	OperatingSystems    map[string]int `json:"-"` // Internal use only
	Devices             map[string]int `json:"-"` // Internal use only
	TopPathsList        []KeyValue     `json:"top_paths"`
	TopIPsList          []KeyValue     `json:"top_ips"`
	CountriesList       []KeyValue     `json:"countries"`
	BrowsersList        []KeyValue     `json:"browsers"`
	OSList              []KeyValue     `json:"operating_systems"`
	DeviceTypesList     []KeyValue     `json:"device_types"`
}

// Index status constants for API responses
const (
	IndexStatusReady = "ready" // Different from internal status - used for API
)

// PreflightResult represents the result of a preflight check
type PreflightResult struct {
	StartTime   *time.Time `json:"start_time,omitempty"`
	EndTime     *time.Time `json:"end_time,omitempty"`
	Available   bool       `json:"available"`
	IndexStatus string     `json:"index_status"`
}