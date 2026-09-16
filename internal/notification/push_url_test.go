package notification

import "testing"

func TestResolveNotificationURLUsesTitleFallback(t *testing.T) {
	url := resolveNotificationURL("Auto Backup Completed", nil)
	if url != "#/backup/auto-backup" {
		t.Fatalf("expected backup url, got %q", url)
	}
}

func TestResolveNotificationURLUsesDetailsOverride(t *testing.T) {
	url := resolveNotificationURL("Auto Backup Completed", map[string]any{
		"url": "#/custom/page",
	})
	if url != "#/custom/page" {
		t.Fatalf("expected custom url override, got %q", url)
	}
}

func TestResolveNotificationURLFallsBackWhenDetailsURLIsBlank(t *testing.T) {
	url := resolveNotificationURL("Auto Backup Completed", map[string]any{
		"url": "   ",
	})
	if url != "#/backup/auto-backup" {
		t.Fatalf("expected fallback backup url, got %q", url)
	}
}

func TestResolveNotificationURLForSiteHealthRecovered(t *testing.T) {
	url := resolveNotificationURL("Site Health Check Recovered", nil)
	if url != "#/sites/list" {
		t.Fatalf("expected sites list url, got %q", url)
	}
}
