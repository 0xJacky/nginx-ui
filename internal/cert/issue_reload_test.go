package cert

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/config"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The reload after an issuance waits for a configuration mutation in
// progress, such as an ingress dry run with its files still on disk.
func TestReloadAfterIssueWaitsForTheApplyLock(t *testing.T) {
	dir := t.TempDir()
	original := *settings.NginxSettings
	t.Cleanup(func() { *settings.NginxSettings = original })
	settings.NginxSettings.ConfigDir = dir
	settings.NginxSettings.PIDPath = filepath.Join(dir, "nginx.pid")
	marker := filepath.Join(dir, "reloaded")
	settings.NginxSettings.ReloadCmd = "touch " + marker
	require.NoError(t, os.WriteFile(settings.NginxSettings.PIDPath, []byte(strconv.Itoa(os.Getpid())), 0o644))

	release := config.LockApply()
	done := make(chan struct{})
	go func() {
		defer close(done)
		reloadAfterIssue()
	}()

	select {
	case <-done:
		release()
		t.Fatal("reloaded while the apply lock was held")
	case <-time.After(100 * time.Millisecond):
	}
	_, err := os.Stat(marker)
	assert.True(t, os.IsNotExist(err))

	release()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("reload did not run after the lock was released")
	}
	_, err = os.Stat(marker)
	assert.NoError(t, err)
}
