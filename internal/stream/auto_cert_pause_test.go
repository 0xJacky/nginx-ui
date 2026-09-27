package stream

import (
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// waitForQuietDB registers a cleanup that waits until the database has been
// idle for a while. Disable and Enable start sync goroutines that still read
// the nginx settings the setup cleanup restores; their queries are the last
// thing they do, and the atomic counter orders those reads before the restore.
func waitForQuietDB(t *testing.T) {
	t.Helper()
	var queries atomic.Int64
	require.NoError(t, model.UseDB().Callback().Query().After("gorm:query").
		Register("test:auto_cert_pause_queries", func(*gorm.DB) { queries.Add(1) }))
	t.Cleanup(func() {
		last := queries.Load()
		stableSince := time.Now()
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			time.Sleep(20 * time.Millisecond)
			if current := queries.Load(); current != last {
				last = current
				stableSince = time.Now()
				continue
			}
			if time.Since(stableSince) >= 300*time.Millisecond {
				return
			}
		}
	})
}

func TestDisablePausesAndEnableResumesStreamAutoCert(t *testing.T) {
	confDir, _ := setupStreamMutationTest(t)
	waitForQuietDB(t)
	name := "tcp-proxy"
	availablePath := filepath.Join(confDir, "streams-available", name)
	enabledPath := filepath.Join(confDir, "streams-enabled", name)
	require.NoError(t, os.WriteFile(availablePath, []byte("server {\n    listen 9000;\n}\n"), 0o644))
	require.NoError(t, os.Symlink(availablePath, enabledPath))

	certModel := &model.Cert{Name: name, Filename: name, AutoCert: model.AutoCertEnabled}
	require.NoError(t, model.UseDB().Create(certModel).Error)

	require.NoError(t, Disable(name))

	var paused model.Cert
	require.NoError(t, model.UseDB().First(&paused, certModel.ID).Error)
	assert.Equal(t, model.AutoCertPaused, paused.AutoCert)

	require.NoError(t, Enable(name))

	var resumed model.Cert
	require.NoError(t, model.UseDB().First(&resumed, certModel.ID).Error)
	assert.Equal(t, model.AutoCertEnabled, resumed.AutoCert)
}
