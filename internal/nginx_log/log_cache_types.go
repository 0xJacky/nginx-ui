package nginx_log

import (
	"time"
)

// IndexStatus constants
const (
	IndexStatusIndexed    = "indexed"
	IndexStatusIndexing   = "indexing" 
	IndexStatusNotIndexed = "not_indexed"
)

// NginxLogCache represents a cached log entry from nginx configuration
type NginxLogCache struct {
	Path       string `json:"path"`        // Path to the log file
	Type       string `json:"type"`        // Type of log: "access" or "error"
	Name       string `json:"name"`        // Name of the log file
	ConfigFile string `json:"config_file"` // Path to the configuration file that contains this log directive
}

// NginxLogWithIndex represents a log file with its index status information
type NginxLogWithIndex struct {
	Path           string     `json:"path"`                      // Path to the log file
	Type           string     `json:"type"`                      // Type of log: "access" or "error"
	Name           string     `json:"name"`                      // Name of the log file
	ConfigFile     string     `json:"config_file"`               // Path to the configuration file
	IndexStatus    string     `json:"index_status"`              // Index status: indexed, indexing, not_indexed
	LastModified   *time.Time `json:"last_modified,omitempty"`   // Last modification time of the file
	LastSize       int64      `json:"last_size,omitempty"`       // Last known size of the file
	LastIndexed    *time.Time `json:"last_indexed,omitempty"`    // When the file was last indexed
	IndexStartTime *time.Time `json:"index_start_time,omitempty"` // When the last indexing operation started
	IndexDuration  *int64     `json:"index_duration,omitempty"`  // Duration of last indexing operation in milliseconds
	IsCompressed   bool       `json:"is_compressed"`             // Whether the file is compressed
	HasTimeRange   bool       `json:"has_timerange"`             // Whether time range is available
	TimeRangeStart *time.Time `json:"timerange_start,omitempty"` // Start of time range in the log
	TimeRangeEnd   *time.Time `json:"timerange_end,omitempty"`   // End of time range in the log
	DocumentCount  uint64     `json:"document_count,omitempty"`  // Number of indexed documents from this file
}