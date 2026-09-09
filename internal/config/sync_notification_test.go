package config

import (
	"strings"
	"testing"
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
