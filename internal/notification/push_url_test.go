package notification

import (
	"testing"

	"github.com/0xJacky/Nginx-UI/settings"
)

func TestResolveNotificationURLUsesTitleFallback(t *testing.T) {
	originalOrigins := settings.WebAuthnSettings.RPOrigins
	settings.WebAuthnSettings.RPOrigins = []string{"https://ui.example.com"}
	t.Cleanup(func() {
		settings.WebAuthnSettings.RPOrigins = originalOrigins
	})

	url := resolveNotificationURL("Auto Backup Completed", nil)
	if url != "https://ui.example.com/#/backup/auto-backup" {
		t.Fatalf("expected backup url, got %q", url)
	}
}

func TestResolveNotificationURLUsesDetailsOverride(t *testing.T) {
	originalOrigins := settings.WebAuthnSettings.RPOrigins
	settings.WebAuthnSettings.RPOrigins = []string{"https://ui.example.com"}
	t.Cleanup(func() {
		settings.WebAuthnSettings.RPOrigins = originalOrigins
	})

	url := resolveNotificationURL("Auto Backup Completed", map[string]any{
		"url": "#/custom/page",
	})
	if url != "https://ui.example.com/#/custom/page" {
		t.Fatalf("expected custom url override, got %q", url)
	}
}

func TestResolveNotificationURLFallsBackWhenDetailsURLIsBlank(t *testing.T) {
	originalOrigins := settings.WebAuthnSettings.RPOrigins
	settings.WebAuthnSettings.RPOrigins = []string{"https://ui.example.com"}
	t.Cleanup(func() {
		settings.WebAuthnSettings.RPOrigins = originalOrigins
	})

	url := resolveNotificationURL("Auto Backup Completed", map[string]any{
		"url": "   ",
	})
	if url != "https://ui.example.com/#/backup/auto-backup" {
		t.Fatalf("expected fallback backup url, got %q", url)
	}
}

func TestResolveNotificationURLForSiteHealthRecovered(t *testing.T) {
	originalOrigins := settings.WebAuthnSettings.RPOrigins
	settings.WebAuthnSettings.RPOrigins = []string{"https://ui.example.com"}
	t.Cleanup(func() {
		settings.WebAuthnSettings.RPOrigins = originalOrigins
	})

	url := resolveNotificationURL("Site Health Check Recovered", nil)
	if url != "https://ui.example.com/#/sites/list" {
		t.Fatalf("expected sites list url, got %q", url)
	}
}

func TestResolveNotificationURLKeepsShortPathWhenRPOriginsMissing(t *testing.T) {
	originalOrigins := settings.WebAuthnSettings.RPOrigins
	settings.WebAuthnSettings.RPOrigins = nil
	t.Cleanup(func() {
		settings.WebAuthnSettings.RPOrigins = originalOrigins
	})

	url := resolveNotificationURL("Auto Backup Completed", nil)
	if url != "#/backup/auto-backup" {
		t.Fatalf("expected short url fallback, got %q", url)
	}
}
