package event

// EventType represents the type of event
type Type string

const (
	TypeIndexScanning      Type = "index_scanning"
	TypeAutoCertProcessing Type = "auto_cert_processing"
	TypeProcessingStatus   Type = "processing_status"

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
