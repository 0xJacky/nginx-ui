package notification

import (
	"context"
	"errors"
	"testing"

	"github.com/0xJacky/Nginx-UI/model"
)

func TestBarkNotifierRejectsEmptyDeviceKey(t *testing.T) {
	// A config with a custom server URL but no device key must be rejected
	// immediately, before any HTTP request is attempted.
	msg := &ExternalMessage{Notification: &model.Notification{
		Title:   "Test",
		Content: "Test",
	}}
	err := msg.SendWithConfigContext(context.Background(), "bark", "en", map[string]string{
		"device_key": "",
		"server_url": "https://example.invalid",
	})
	if !errors.Is(err, ErrInvalidNotifierConfig) {
		t.Fatalf("SendWithConfigContext() error = %v, want ErrInvalidNotifierConfig", err)
	}
}
