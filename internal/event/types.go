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
}

// NginxLogStatusData represents the data for nginx log status events (backward compatibility)
type NginxLogStatusData struct {
	Scanning bool `json:"scanning"`
}
