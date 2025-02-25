package notification

import (
	"github.com/0xJacky/Nginx-UI/model"
)

func Info(title string, content string, details any) {
	push(model.NotificationInfo, title, content, details)
}

func Error(title string, content string, details any) {
	push(model.NotificationError, title, content, details)
}

func Warning(title string, content string, details any) {
	push(model.NotificationWarning, title, content, details)
}

func Success(title string, content string, details any) {
	push(model.NotificationSuccess, title, content, details)
}
