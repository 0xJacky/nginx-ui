package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/0xJacky/Nginx-UI/internal/nginx"
)

// TestSiteStatusMapBuilderDetectsPlatformMaintenanceSuffix reproduces the
// Windows-specific bug where the maintenance suffix check ran before the
// platform symlink suffix (".conf" on Windows) was stripped, so a maintenance
// config such as "example.com_nginx_ui_maintenance.conf" was reported as
// enabled instead of in maintenance. Building the enabled file name through
// nginx.GetConfSymlinkPath, exactly like production code does, keeps this test
// meaningful on every platform.
func TestSiteStatusMapBuilderDetectsPlatformMaintenanceSuffix(t *testing.T) {
	const maintenanceSuffix = "_nginx_ui_maintenance"
	dir := t.TempDir()

	availableName := "example.com"
	if err := os.WriteFile(filepath.Join(dir, availableName), []byte("server {}\n"), 0o644); err != nil {
		t.Fatalf("failed to write available config: %v", err)
	}

	enabledName := nginx.GetConfSymlinkPath(availableName + maintenanceSuffix)
	if err := os.WriteFile(filepath.Join(dir, enabledName), []byte("server {}\n"), 0o644); err != nil {
		t.Fatalf("failed to write enabled maintenance config: %v", err)
	}

	configFiles, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("failed to read available dir: %v", err)
	}
	enabledConfig, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("failed to read enabled dir: %v", err)
	}
	// Only the maintenance file represents "enabled" entries here.
	var maintenanceEntries []os.DirEntry
	for _, entry := range enabledConfig {
		if entry.Name() == enabledName {
			maintenanceEntries = append(maintenanceEntries, entry)
		}
	}

	statusMap := SiteStatusMapBuilder(maintenanceSuffix)(configFiles, maintenanceEntries)

	if got := statusMap[availableName]; got != StatusMaintenance {
		t.Fatalf("statusMap[%q] = %q, want %q", availableName, got, StatusMaintenance)
	}
}
