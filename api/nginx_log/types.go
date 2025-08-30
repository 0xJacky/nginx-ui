package nginx_log

import "github.com/0xJacky/Nginx-UI/internal/translation"

const (
	// PageSize defines the size of log chunks returned by the API
	PageSize = 128 * 1024
)

// controlStruct represents the request parameters for getting log content
type controlStruct struct {
	Type string `json:"type"` // Type of log: "access" or "error"
	Path string `json:"path"` // Path to the log file
}

// nginxLogPageResp represents the response format for log content
type nginxLogPageResp struct {
	Content string                 `json:"content"`         // Log content
	Page    int64                  `json:"page"`            // Current page number
	Error   *translation.Container `json:"error,omitempty"` // Error message if any
}

// FileInfo represents basic file information
type FileInfo struct {
	Exists        bool  `json:"exists"`
	Readable      bool  `json:"readable"`
	Size          int64 `json:"size,omitempty"`
	LastModified  int64 `json:"last_modified,omitempty"`
}

// TimeRange represents a time range for log data
type TimeRange struct {
	Start int64 `json:"start"`
	End   int64 `json:"end"`
}

// PreflightResponse represents the response from preflight checks
type PreflightResponse struct {
	Available   bool       `json:"available"`
	IndexStatus string     `json:"index_status"`
	Message     string     `json:"message,omitempty"`
	TimeRange   *TimeRange `json:"time_range,omitempty"`
	FileInfo    *FileInfo  `json:"file_info,omitempty"`
}

// AnalyticsResponse represents the response for analytics endpoints
type AnalyticsResponse struct {
	Entries []map[string]interface{} `json:"entries"`
	Count   int                      `json:"count"`
}

// GeoDataResponse represents the response for geographic data
type GeoDataResponse struct {
	Data []GeoDataItem `json:"data"`
}

// GeoRegionResponse represents the response for geographic region data
type GeoRegionResponse struct {
	Data []GeoRegionItem `json:"data"`
}

// GeoStatsResponse represents the response for geographic statistics
type GeoStatsResponse struct {
	Stats []interface{} `json:"stats"`
}

// ErrorResponse represents a standard error response
type ErrorResponse struct {
	Error string `json:"error"`
}

// IndexRebuildResponse represents the response for index rebuild operations
type IndexRebuildResponse struct {
	Message string `json:"message"`
	Status  string `json:"status"`
}

// LogListSummary represents summary statistics for log list
type LogListSummary struct {
	TotalFiles    int `json:"total_files"`
	IndexedFiles  int `json:"indexed_files"`
	IndexingFiles int `json:"indexing_files"`
	DocumentCount int `json:"document_count"`
}

// LogListResponse represents the response for log list endpoint
type LogListResponse struct {
	Data    interface{}    `json:"data"`
	Summary LogListSummary `json:"summary"`
}