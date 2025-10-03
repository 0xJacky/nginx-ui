package notification

import "github.com/uozi-tech/cosy"

var (
	e                         = cosy.NewErrorScope("notification")
	ErrNotifierNotFound       = e.New(404001, "notifier not found")
	ErrInvalidNotifierConfig  = e.New(400001, "invalid notifier config")
	ErrInvalidNotificationID  = e.New(400002, "invalid notification ID")
	ErrExternalNotifyNotFound = e.New(404002, "external notification configuration not found")
	ErrTelegramChatIDZero     = e.New(400003, "invalid Telegram Chat ID: cannot be zero")
)
