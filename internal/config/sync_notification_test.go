package config

import (
	"strings"
	"testing"

	"github.com/0xJacky/Nginx-UI/internal/notification"
	"github.com/0xJacky/Nginx-UI/model"
)

func TestSyncConfigNotificationContentIncludesActingUser(t *testing.T) {
	for name, content := range map[string]string{
		"success": syncConfigSuccessContent("alice"),
		"error":   syncConfigErrorContent("alice"),
	} {
		if !strings.Contains(content, "%{user_name}") {
			t.Fatalf("%s content = %q", name, content)
		}
	}
}

func TestSyncConfigNotificationContentKeepsLegacyMessageWithoutUser(t *testing.T) {
	if got := syncConfigSuccessContent(""); got != "Sync config %{config_name} to %{node_name} successfully" {
		t.Fatalf("success content = %q", got)
	}
	if got := syncConfigErrorContent(""); got != "Sync config %{config_name} to %{node_name} failed" {
		t.Fatalf("failure content = %q", got)
	}
}

func TestSyncConfigNotificationRendersActingUser(t *testing.T) {
	message := &notification.ExternalMessage{Notification: &model.Notification{
		Content: syncConfigSuccessContent("alice"),
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
