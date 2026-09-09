package config

import (
	"strings"
	"testing"

	"github.com/0xJacky/Nginx-UI/internal/notification"
	"github.com/0xJacky/Nginx-UI/model"
)

func TestSyncConfigNotificationContentIncludesActingUser(t *testing.T) {
	for _, success := range []bool{true, false} {
		content := syncConfigNotificationContent("alice", success)
		if !strings.Contains(content, "%{user_name}") {
			t.Fatalf("content = %q", content)
		}
	}
}

func TestSyncConfigNotificationContentKeepsLegacyMessageWithoutUser(t *testing.T) {
	if got := syncConfigNotificationContent("", true); got != "Sync config %{config_name} to %{node_name} successfully" {
		t.Fatalf("success content = %q", got)
	}
	if got := syncConfigNotificationContent("", false); got != "Sync config %{config_name} to %{node_name} failed" {
		t.Fatalf("failure content = %q", got)
	}
}

func TestSyncConfigNotificationRendersActingUser(t *testing.T) {
	message := &notification.ExternalMessage{Notification: &model.Notification{
		Content: syncConfigNotificationContent("alice", true),
		Details: &SyncNotificationPayload{
			ConfigName: "nginx.conf",
			NodeName:   "edge-1",
			UserName:   "alice",
		},
	}}

	got := message.GetContent("en")
	want := "User alice synced config nginx.conf to edge-1 successfully"
	if got != want {
		t.Fatalf("GetContent() = %q, want %q", got, want)
	}
}
