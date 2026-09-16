package notification

import (
	"strings"

	"github.com/0xJacky/Nginx-UI/internal/event"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/uozi-tech/cosy/logger"
)

var notificationURLByTitle = map[string]string{
	"Reload Remote Nginx Error":               "#/nodes",
	"Reload Remote Nginx Success":             "#/nodes",
	"Restart Remote Nginx Error":              "#/nodes",
	"Restart Remote Nginx Success":            "#/nodes",
	"Auto Backup Configuration Error":         "#/backup/auto-backup",
	"Auto Backup Failed":                      "#/backup/auto-backup",
	"Auto Backup Storage Failed":              "#/backup/auto-backup",
	"Auto Backup Completed":                   "#/backup/auto-backup",
	"Renew Certificate Success":               "#/certificates/list",
	"Renew Certificate Error":                 "#/certificates/list",
	"Certificate Expired":                     "#/certificates/list",
	"Certificate Expiration Notice":           "#/certificates/list",
	"Certificate Expiring Soon":               "#/certificates/list",
	"Sync Certificate Error":                  "#/certificates/list",
	"Sync Certificate Success":                "#/certificates/list",
	"Sync Config Error":                       "#/config",
	"Sync Config Success":                     "#/config",
	"Rename Remote Config Error":              "#/config",
	"Rename Remote Config Success":            "#/config",
	"Delete Remote Config Error":              "#/config",
	"Delete Remote Config Success":            "#/config",
	"Auto Sync Namespace Error":               "#/namespaces",
	"External Notification Test":              "#/preference",
	"Certificate Paths Migrated":              "#/certificates/list",
	"Certificate Configuration Mismatch":      "#/certificates/list",
	"Certificate Path Migration Failed":       "#/certificates/list",
	"Delete Remote Site Error":                "#/sites/list",
	"Delete Remote Site Success":              "#/sites/list",
	"Disable Remote Site Error":               "#/sites/list",
	"Disable Remote Site Success":             "#/sites/list",
	"Enable Remote Site Error":                "#/sites/list",
	"Enable Remote Site Success":              "#/sites/list",
	"Enable Remote Site Maintenance Error":    "#/sites/list",
	"Enable Remote Site Maintenance Success":  "#/sites/list",
	"Disable Remote Site Maintenance Error":   "#/sites/list",
	"Disable Remote Site Maintenance Success": "#/sites/list",
	"Rename Remote Site Error":                "#/sites/list",
	"Rename Remote Site Success":              "#/sites/list",
	"Save Remote Site Error":                  "#/sites/list",
	"Save Remote Site Success":                "#/sites/list",
	"Site Health Check Failed":                "#/sites/list",
	"Site Health Check Recovered":             "#/sites/list",
	"Delete Remote Stream Error":              "#/streams",
	"Delete Remote Stream Success":            "#/streams",
	"Disable Remote Stream Error":             "#/streams",
	"Disable Remote Stream Success":           "#/streams",
	"Enable Remote Stream Error":              "#/streams",
	"Enable Remote Stream Success":            "#/streams",
	"Rename Remote Stream Error":              "#/streams",
	"Rename Remote Stream Success":            "#/streams",
	"Save Remote Stream Error":                "#/streams",
	"Save Remote Stream Success":              "#/streams",
	"All Recovery Codes Have Been Used":       "#/profile",
}

func resolveNotificationURL(title string, details any) string {
	payload, ok := details.(map[string]any)
	if !ok {
		return notificationURLByTitle[title]
	}

	url, ok := payload["url"].(string)
	if !ok {
		return notificationURLByTitle[title]
	}

	url = strings.TrimSpace(url)
	if url == "" {
		return notificationURLByTitle[title]
	}

	return url
}

func push(nType model.NotificationType, title string, content string, details any) {
	pushWithExternalTargets(nType, title, content, details, nil, false)
}

func pushTo(nType model.NotificationType, title string, content string, details any, externalNotifyIDs []uint64) {
	pushWithExternalTargets(nType, title, content, details, externalNotifyIDs, true)
}

func pushWithExternalTargets(nType model.NotificationType, title string, content string, details any, externalNotifyIDs []uint64, targeted bool) {
	n := query.Notification

	data := &model.Notification{
		Type:    nType,
		Title:   title,
		Content: content,
		URL:     resolveNotificationURL(title, details),
		Details: details,
	}

	err := n.Create(data)
	if err != nil {
		logger.Error(err)
		return
	}

	// Use event system instead of direct broadcast
	event.Publish(event.Event{
		Type: event.TypeNotification,
		Data: data,
	})

	// Keep the old broadcast for backward compatibility
	broadcast(data)

	extNotify := &ExternalMessage{data}
	if targeted {
		extNotify.SendTo(externalNotifyIDs)
	} else {
		extNotify.Send()
	}
}
