package event

// EventType represents the type of event
type EventType string

const (
	// Processing status events
	EventTypeIndexScanning      EventType = "index_scanning"
	EventTypeAutoCertProcessing EventType = "auto_cert_processing"
	EventTypeProcessingStatus   EventType = "processing_status"

	// Nginx log status events (for backward compatibility)
	EventTypeNginxLogStatus EventType = "nginx_log_status"

	// Nginx log indexing events
	EventTypeNginxLogIndexReady    EventType = "nginx_log_index_ready"
	EventTypeNginxLogIndexProgress EventType = "nginx_log_index_progress"
	EventTypeNginxLogIndexComplete EventType = "nginx_log_index_complete"

	// Notification events
	EventTypeNotification EventType = "notification"
)

// Event represents a generic event structure
type Event struct {
	Type EventType   `json:"type"`
	Data interface{} `json:"data"`
}

// ProcessingStatusData represents the data for processing status events
type ProcessingStatusData struct {
	IndexScanning      bool `json:"index_scanning"`
	AutoCertProcessing bool `json:"auto_cert_processing"`
	NginxLogIndexing   bool `json:"nginx_log_indexing"`
}

// NginxLogStatusData represents the data for nginx log status events (backward compatibility)
type NginxLogStatusData struct {
	Indexing bool `json:"indexing"`
}

// NginxLogIndexReadyData represents the data for nginx log index ready events
type NginxLogIndexReadyData struct {
	LogPath     string `json:"log_path"`
	StartTime   int64  `json:"start_time"`
	EndTime     int64  `json:"end_time"`
	Available   bool   `json:"available"`
	IndexStatus string `json:"index_status"`
}

// NginxLogIndexProgressData represents the data for nginx log index progress events
type NginxLogIndexProgressData struct {
	LogPath         string  `json:"log_path"`
	Progress        float64 `json:"progress"`        // 0-100 percentage
	Stage           string  `json:"stage"`           // "scanning", "indexing", "stats"
	Status          string  `json:"status"`          // "running", "completed", "error"
	ElapsedTime     int64   `json:"elapsed_time"`    // milliseconds
	EstimatedRemain int64   `json:"estimated_remain"` // milliseconds
}

// NginxLogIndexCompleteData represents the data for nginx log index complete events
type NginxLogIndexCompleteData struct {
	LogPath     string `json:"log_path"`
	Success     bool   `json:"success"`
	Duration    int64  `json:"duration"`    // milliseconds
	TotalLines  int64  `json:"total_lines"`
	IndexedSize int64  `json:"indexed_size"` // bytes
	Error       string `json:"error,omitempty"`
}
