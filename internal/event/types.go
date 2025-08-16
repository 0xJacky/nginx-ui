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
	EventTypeNginxLogIndexReady EventType = "nginx_log_index_ready"

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
	StartTime   string `json:"start_time"`
	EndTime     string `json:"end_time"`
	Available   bool   `json:"available"`
	IndexStatus string `json:"index_status"`
}
