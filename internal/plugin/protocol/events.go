package protocol

// EventNotification is the payload of the events.on notification and of cron
// invocations (where Method is the cron entry's method and Data is nil).
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
)
