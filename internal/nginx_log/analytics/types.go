package analytics

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
	Path           string `json:"path"`
	LastModified   int64  `json:"last_modified"` // Unix timestamp
	LastSize       int64  `json:"last_size"`
	LastIndexed    int64  `json:"last_indexed"` // Unix timestamp
	IsCompressed   bool   `json:"is_compressed"`
	HasTimeRange   bool   `json:"has_timerange"`
	TimeRangeStart int64  `json:"timerange_start,omitzero"` // Unix timestamp
	TimeRangeEnd   int64  `json:"timerange_end,omitzero"`   // Unix timestamp
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

// PreflightResult represents the result of a preflight check
type PreflightResult struct {
	StartTime   int64  `json:"start_time,omitempty"` // Unix timestamp
	EndTime     int64  `json:"end_time,omitempty"`   // Unix timestamp
	Available   bool   `json:"available"`
	IndexStatus string `json:"index_status"`
}

// DashboardQueryRequest represents a request for dashboard analytics
type DashboardQueryRequest struct {
	LogPath   string   // The base log path for the group
	LogPaths  []string // The expanded list of physical file paths
	StartTime int64
	EndTime   int64
}

// DashboardAnalytics represents comprehensive dashboard analytics data
type DashboardAnalytics struct {
	HourlyStats      []HourlyAccessStats  `json:"hourly_stats"`
	DailyStats       []DailyAccessStats   `json:"daily_stats"`
	TopURLs          []URLAccessStats     `json:"top_urls"`
	Browsers         []BrowserAccessStats `json:"browsers"`
	OperatingSystems []OSAccessStats      `json:"operating_systems"`
	Devices          []DeviceAccessStats  `json:"devices"`
	Summary          DashboardSummary     `json:"summary"`
}

// DashboardSummary represents summary statistics for the dashboard
type DashboardSummary struct {
	TotalUV         int     `json:"total_uv"`
	TotalPV         int     `json:"total_pv"`
	AvgDailyUV      float64 `json:"avg_daily_uv"`
	AvgDailyPV      float64 `json:"avg_daily_pv"`
	PeakHour        int     `json:"peak_hour"`
	PeakHourTraffic int     `json:"peak_hour_traffic"`
}

// HourlyAccessStats represents hourly access statistics
type HourlyAccessStats struct {
	Hour      int   `json:"hour"`
	UV        int   `json:"uv"`
	PV        int   `json:"pv"`
	Timestamp int64 `json:"timestamp"`
}

// DailyAccessStats represents daily access statistics
type DailyAccessStats struct {
	Date      string `json:"date"`
	UV        int    `json:"uv"`
	PV        int    `json:"pv"`
	Timestamp int64  `json:"timestamp"`
}

// URLAccessStats represents URL access statistics
type URLAccessStats struct {
	URL     string  `json:"url"`
	Visits  int     `json:"visits"`
	Percent float64 `json:"percent"`
}

// BrowserAccessStats represents browser usage statistics
type BrowserAccessStats struct {
	Browser string  `json:"browser"`
	Count   int     `json:"count"`
	Percent float64 `json:"percent"`
}

// OSAccessStats represents operating system usage statistics
type OSAccessStats struct {
	OS      string  `json:"os"`
	Count   int     `json:"count"`
	Percent float64 `json:"percent"`
}

// DeviceAccessStats represents device type usage statistics
type DeviceAccessStats struct {
	Device  string  `json:"device"`
	Count   int     `json:"count"`
	Percent float64 `json:"percent"`
}

// EntriesStats represents statistics for log entries
type EntriesStats struct {
	TotalEntries      int64                   `json:"total_entries"`
	StatusCodeDist    map[string]int          `json:"status_code_distribution"`
	MethodDist        map[string]int          `json:"method_distribution"`
	TopPaths          []KeyValue              `json:"top_paths"`
	TopIPs            []KeyValue              `json:"top_ips"`
	TopUserAgents     []KeyValue              `json:"top_user_agents"`
	BytesStats        *BytesStatistics        `json:"bytes_stats"`
	ResponseTimeStats *ResponseTimeStatistics `json:"response_time_stats"`
}

// BytesStatistics represents byte-related statistics
type BytesStatistics struct {
	Total   int64   `json:"total"`
	Average float64 `json:"average"`
	Min     int64   `json:"min"`
	Max     int64   `json:"max"`
}

// ResponseTimeStatistics represents response time statistics
type ResponseTimeStatistics struct {
	Average float64 `json:"average"`
	Min     float64 `json:"min"`
	Max     float64 `json:"max"`
}

// GeoQueryRequest represents a request for geographical data
type GeoQueryRequest struct {
	LogPath   string
	LogPaths  []string
	StartTime int64
	EndTime   int64
	Limit     int
}

// GeoDistribution represents geographical distribution of requests
type GeoDistribution struct {
	Countries map[string]int
}

// CountryStats represents statistics for a country
type CountryStats struct {
	Country  string
	Requests int
}

// CityStats represents statistics for a city
type CityStats struct {
	City        string  `json:"city"`
	Country     string  `json:"country"`
	CountryCode string  `json:"country_code"`
	Count       int     `json:"count"`
	Percent     float64 `json:"percent"`
}

// TimelineRequest represents a request for timeline data
type TimelineRequest struct {
	StartTime int64  `json:"start_time"` // Unix timestamp
	EndTime   int64  `json:"end_time"`   // Unix timestamp
	LogPath   string `json:"log_path,omitempty"`
	Interval  string `json:"interval"` // "hour", "day", "week", "month"
}

// TimelineData represents time series data
type TimelineData struct {
	Interval string          `json:"interval"`
	Data     []TimelinePoint `json:"data"`
}

// TimelinePoint represents a single point in time series
type TimelinePoint struct {
	Timestamp int64 `json:"timestamp"`
	Requests  int   `json:"requests"`
	UniqueIPs int   `json:"unique_ips"`
	Bytes     int64 `json:"bytes"`
}

// TopListRequest represents a request for top-N lists (e.g., top IPs, top paths)
type TopListRequest struct {
	StartTime int64
	EndTime   int64
	LogPath   string
	Limit     int
	Field     string
}

type TrafficStatsRequest struct {
	StartTime int64
	EndTime   int64
	LogPath   string
	LogPaths  []string
}

type VisitorsByTimeRequest struct {
	StartTime       int64
	EndTime         int64
	LogPath         string
	LogPaths        []string
	IntervalSeconds int
}

type VisitorsByCountryRequest struct {
	StartTime int64
	EndTime   int64
	LogPath   string
	LogPaths  []string
}

type TopRequestsRequest struct {
	StartTime int64
	EndTime   int64
	LogPath   string
	LogPaths  []string
	Limit     int
	SortBy    string
	SortOrder string
}

type ErrorDistributionRequest struct {
	StartTime int64
	EndTime   int64
	LogPath   string
	LogPaths  []string
}

type RequestRateRequest struct {
	StartTime int64
	EndTime   int64
	LogPath   string
	LogPaths  []string
}

type BandwidthUsageRequest struct {
	StartTime int64
	EndTime   int64
	LogPath   string
	LogPaths  []string
}

type VisitorPathsRequest struct {
	StartTime int64
	EndTime   int64
	LogPath   string
	LogPaths  []string
	IP        string
	Limit     int
	Offset    int
}

// Response structs for analytics calculations

type TrafficStats struct {
	TotalRequests int
	TotalBytes    int64
}

type VisitorsByTime struct {
	Data []TimeValue
}

type VisitorsByCountry struct {
	Data map[string]int
}

type TopRequests struct {
	Total    int
	Requests []RequestInfo
}

type RequestInfo struct {
	// Define fields based on what you need to show in the UI
}

type ErrorDistribution struct {
	Data map[string]int
}

type RequestRate struct {
	TotalRequests int
	Rate          float64 // requests per second
}

type BandwidthUsage struct {
	TotalRequests int
	TotalBytes    int64
}

type VisitorPaths struct {
	Total int
	Paths []VisitorPath
}

type VisitorPath struct {
	Path      string
	Timestamp int64
}

type TimeValue struct {
	Timestamp int64
	Value     int
}

// Constants for index status
const (
	IndexStatusReady = "ready" // Different from internal status - used for API
)

// Constants for timeline intervals
const (
	IntervalHour  = "hour"
	IntervalDay   = "day"
	IntervalWeek  = "week"
	IntervalMonth = "month"
)

// Constants for top list fields
const (
	FieldPath      = "path"
	FieldIP        = "ip"
	FieldUserAgent = "user_agent"
	FieldReferer   = "referer"
	FieldCountry   = "country"
	FieldBrowser   = "browser"
	FieldOS        = "os"
	FieldDevice    = "device"
)

// Default values
const (
	DefaultLimit        = 10
	MaxLimit            = 1000
	DefaultCacheTTL     = 5 * time.Minute
	MinTimelineInterval = time.Hour
)
