package notification

import (
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/uozi-tech/cosy/logger"
)

func push(nType model.NotificationType, title string, content string, details any) {
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
	broadcast(data)

	extNotify := &ExternalMessage{data}
	extNotify.Send()
}
