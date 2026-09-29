package protocol

// EventNotification is the payload of the events.on notification. A fired
// cron entry reuses it as the params of a request to the entry's own method,
// with Type set to the cron entry id and no Data.
type EventNotification struct {
	Type string `json:"type"`
	Data any    `json:"data,omitempty"`
	TS   int64  `json:"ts"` // unix seconds
}

// Event types a plugin may subscribe to through Manifest.Events.
const (
	EventCertIssued        = "cert.issued"
	EventCertRenewed       = "cert.renewed"
	EventCertExpiring      = "cert.expiring"
	EventSiteSaved         = "site.saved"
	EventSiteEnabled       = "site.enabled"
	EventSiteDisabled      = "site.disabled"
	EventNginxReloaded     = "nginx.reloaded"
	EventNginxReloadFailed = "nginx.reload_failed"
	EventNodeStatusChanged = "node.status_changed"
	EventNodeJoined        = "node.joined"
	EventBackupCompleted   = "backup.completed"
	EventAuthLoginFailed   = "auth.login_failed"
	EventPluginChanged     = "plugin.changed"
	// EventLogPathsChanged is sent only to a plugin holding the log.files
	// permission.
	EventLogPathsChanged = "log.paths_changed"
)
