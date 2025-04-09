package notification

import "github.com/uozi-tech/cosy"

var (
	e                        = cosy.NewErrorScope("notification")
	ErrNotifierNotFound      = e.New(404001, "notifier not found")
	ErrInvalidNotifierConfig = e.New(400001, "invalid notifier config")
)
