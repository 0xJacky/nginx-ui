package utils

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/0xJacky/Nginx-UI/internal/cache"
	"github.com/0xJacky/Nginx-UI/settings"
)

func TestIsValidLogPathRejectsSymlinkTargetOutsideWhitelist(t *testing.T) {
	cache.InitInMemoryCache()
	t.Cleanup(cache.Shutdown)

	originalWhitelist := settings.NginxSettings.LogDirWhiteList
	settings.NginxSettings.LogDirWhiteList = nil
	t.Cleanup(func() {
		settings.NginxSettings.LogDirWhiteList = originalWhitelist
	})

	whitelistDir := t.TempDir()
	outsideDir := t.TempDir()
	outsideFile := filepath.Join(outsideDir, "secret.log")

	err := os.WriteFile(outsideFile, []byte("secret"), 0o644)
	if err != nil {
		t.Fatalf("failed to create outside file: %v", err)
	}

	settings.NginxSettings.LogDirWhiteList = []string{whitelistDir}

	linkPath := filepath.Join(whitelistDir, "access.log")
	err = os.Symlink(outsideFile, linkPath)
	if err != nil {
		t.Fatalf("failed to create symlink: %v", err)
	}

	if IsValidLogPath(linkPath) {
		t.Fatalf("expected symlink path %q to be rejected", linkPath)
	}
}
