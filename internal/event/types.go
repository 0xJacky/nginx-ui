package event

// EventType represents the type of event
type Type string

const (
	TypeIndexScanning      Type = "index_scanning"
	TypeAutoCertProcessing Type = "auto_cert_processing"
	TypeProcessingStatus   Type = "processing_status"

	TypeNginxLogStatus Type = "nginx_log_status"

	TypeNginxLogIndexReady    Type = "nginx_log_index_ready"
	TypeNginxLogIndexProgress Type = "nginx_log_index_progress"
	TypeNginxLogIndexComplete Type = "nginx_log_index_complete"

	TypeNotification Type = "notification"

	// Domain events that plugins may subscribe to. Values match the plugin
	// protocol event names.
	TypeCertIssued        Type = "cert.issued"
	TypeCertRenewed       Type = "cert.renewed"
	TypeCertExpiring      Type = "cert.expiring"
	TypeSiteSaved         Type = "site.saved"
	TypeSiteEnabled       Type = "site.enabled"
	TypeSiteDisabled      Type = "site.disabled"
	TypeNginxReloaded     Type = "nginx.reloaded"
	TypeNginxReloadFailed Type = "nginx.reload_failed"
	TypeNodeStatusChanged Type = "node.status_changed"
	TypeNodeJoined        Type = "node.joined"
	TypeBackupCompleted   Type = "backup.completed"
	TypeAuthLoginFailed   Type = "auth.login_failed"
	TypePluginChanged     Type = "plugin.changed"
	TypeLogPathsChanged   Type = "log.paths_changed"
)

// Event represents a generic event structure
type Event struct {
	Type Type        `json:"type"`
	Data interface{} `json:"data"`
}

// ProcessingStatusData represents the data for processing status events
type ProcessingStatusData struct {
	IndexScanning      bool `json:"index_scanning"`
	AutoCertProcessing bool `json:"auto_cert_processing"`
	NginxLogIndexing   bool `json:"nginx_log_indexing"`
	// Plugins lists the background work plugins reported. Never nil.
	Plugins []PluginActivity `json:"plugins"`
}

// PluginActivity is one processing entry a plugin shows in the indicator.
type PluginActivity struct {
	PluginID string `json:"plugin_id"`
	Key      string `json:"key"`
	// Label is an English source string the browser translates.
	Label string `json:"label"`
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
	Progress        float64 `json:"progress"`         // 0-100 percentage
	Stage           string  `json:"stage"`            // "scanning", "indexing", "stats"
	Status          string  `json:"status"`           // "running", "completed", "error"
	ElapsedTime     int64   `json:"elapsed_time"`     // milliseconds
	EstimatedRemain int64   `json:"estimated_remain"` // milliseconds
}

// NginxLogIndexCompleteData represents the data for nginx log index complete events
type NginxLogIndexCompleteData struct {
	LogPath     string `json:"log_path"`
	Success     bool   `json:"success"`
	Duration    int64  `json:"duration"` // milliseconds
	TotalLines  int64  `json:"total_lines"`
	IndexedSize int64  `json:"indexed_size"` // bytes
	Error       string `json:"error,omitempty"`
}
