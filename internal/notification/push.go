package notification

import (
	"github.com/0xJacky/Nginx-UI/internal/event"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/uozi-tech/cosy/logger"
)

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
