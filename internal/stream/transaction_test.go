package stream

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	appsettings "github.com/0xJacky/Nginx-UI/settings"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnableRollsBackLinkWhenConfigTestFails(t *testing.T) {
	confDir, _ := setupStreamMutationTest(t)
	availablePath := filepath.Join(confDir, "streams-available", "tcp_proxy")
	enabledPath := filepath.Join(confDir, "streams-enabled", "tcp_proxy")
	require.NoError(t, os.WriteFile(availablePath, []byte("server {\n    listen 8080;\n}\n"), 0o644))
	appsettings.NginxSettings.TestConfigCmd = "false"

	err := Enable("tcp_proxy")

	require.Error(t, err)
	_, statErr := os.Lstat(enabledPath)
	assert.ErrorIs(t, statErr, os.ErrNotExist)
}

func TestEnableRestoresDisabledStateWhenReloadFails(t *testing.T) {
	confDir, _ := setupStreamMutationTest(t)
	availablePath := filepath.Join(confDir, "streams-available", "tcp_proxy")
	enabledPath := filepath.Join(confDir, "streams-enabled", "tcp_proxy")
	require.NoError(t, os.WriteFile(availablePath, []byte("server {\n    listen 8080;\n}\n"), 0o644))

	// Fail the first reload only, so the rollback reload still succeeds.
	reloadMarker := filepath.Join(t.TempDir(), "reload-attempted")
	appsettings.NginxSettings.ReloadCmd = fmt.Sprintf(
		"if [ ! -e %q ]; then touch %q; exit 1; fi", reloadMarker, reloadMarker)

	err := Enable("tcp_proxy")

	require.Error(t, err)
	_, statErr := os.Lstat(enabledPath)
	assert.ErrorIs(t, statErr, os.ErrNotExist)
	_, markerErr := os.Stat(reloadMarker)
	assert.NoError(t, markerErr)
}
