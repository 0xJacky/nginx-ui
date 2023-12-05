package notification

import (
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
)

func Info(title string, details string) {
	push(model.NotificationInfo, title, details)
}

func Error(title string, details string) {
	push(model.NotificationError, title, details)
}

func Warning(title string, details string) {
	push(model.NotificationWarning, title, details)
}

func Success(title string, details string) {
	push(model.NotificationSuccess, title, details)
}

func push(nType model.NotificationType, title string, details string) {
	n := query.Notification

	_ = n.Create(&model.Notification{
		Type:    nType,
		Title:   title,
		Details: details,
	})
}
