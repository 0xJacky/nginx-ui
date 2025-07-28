package notification

import (
	"time"

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

func Define(title string, content string, details any) *model.Notification {
	return &model.Notification{
		Type:    model.NotificationInfo,
		Title:   title,
		Content: content,
		Details: details,
	}
}

// SendTestMessage sends a test message with direct parameters
func SendTestMessage(notifyType, language string, config map[string]string) error {
	timestamp := time.Now().Format(time.DateTime)

	data := Define("External Notification Test", "This is a test message sent at %{timestamp} from Nginx UI.", map[string]any{
		"timestamp": timestamp,
	})

	// Create external message and send with direct parameters
	extNotify := &ExternalMessage{data}
	err := extNotify.SendWithConfig(notifyType, language, config)
	if err != nil {
		return err
	}
	return nil
}
