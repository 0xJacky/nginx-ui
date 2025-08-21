package nginx_log

import ()

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
	Path           string `json:"path"`                      // Path to the log file
	Type           string `json:"type"`                      // Type of log: "access" or "error"
	Name           string `json:"name"`                      // Name of the log file
	ConfigFile     string `json:"config_file"`               // Path to the configuration file
	IndexStatus    string `json:"index_status"`              // Index status: indexed, indexing, not_indexed
	LastModified   int64  `json:"last_modified,omitempty"`   // Unix timestamp of last modification time
	LastSize       int64  `json:"last_size,omitempty"`       // Last known size of the file
	LastIndexed    int64  `json:"last_indexed,omitempty"`    // Unix timestamp when the file was last indexed
	IndexStartTime int64  `json:"index_start_time,omitempty"` // Unix timestamp when the last indexing operation started
	IndexDuration  int64  `json:"index_duration,omitempty"`  // Duration of last indexing operation in milliseconds
	IsCompressed   bool   `json:"is_compressed"`             // Whether the file is compressed
	HasTimeRange   bool   `json:"has_timerange"`             // Whether time range is available
	TimeRangeStart int64  `json:"timerange_start,omitempty"` // Unix timestamp of start of time range in the log
	TimeRangeEnd   int64  `json:"timerange_end,omitempty"`   // Unix timestamp of end of time range in the log
	DocumentCount  uint64 `json:"document_count,omitempty"`  // Number of indexed documents from this file
}