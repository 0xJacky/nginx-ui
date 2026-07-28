package site

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/0xJacky/Nginx-UI/model"
	appsettings "github.com/0xJacky/Nginx-UI/settings"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteConfigFilePreservesSymlink(t *testing.T) {
	targetPath := filepath.Join(t.TempDir(), "target.conf")
	linkPath := filepath.Join(t.TempDir(), "site.conf")
	require.NoError(t, os.WriteFile(targetPath, []byte("old\n"), 0o640))
	require.NoError(t, os.Symlink(targetPath, linkPath))

	require.NoError(t, writeConfigFile(linkPath, []byte("new\n"), 0o644))

	linkTarget, err := os.Readlink(linkPath)
	require.NoError(t, err)
	assert.Equal(t, targetPath, linkTarget)
	content, err := os.ReadFile(targetPath)
	require.NoError(t, err)
	assert.Equal(t, "new\n", string(content))
}

func TestEnableRollsBackLinkWhenConfigTestFails(t *testing.T) {
	confDir, _ := setupSiteMutationTest(t)
	availablePath := filepath.Join(confDir, "sites-available", "example.com")
	enabledPath := filepath.Join(confDir, "sites-enabled", "example.com")
	require.NoError(t, os.WriteFile(availablePath, []byte("server {}\n"), 0o644))
	appsettings.NginxSettings.TestConfigCmd = "false"

	err := Enable("example.com")

	require.Error(t, err)
	_, statErr := os.Lstat(enabledPath)
	assert.ErrorIs(t, statErr, os.ErrNotExist)
}

func TestEnableRestoresDisabledStateWhenReloadFails(t *testing.T) {
	confDir, _ := setupSiteMutationTest(t)
	availablePath := filepath.Join(confDir, "sites-available", "example.com")
	enabledPath := filepath.Join(confDir, "sites-enabled", "example.com")
	require.NoError(t, os.WriteFile(availablePath, []byte("server {}\n"), 0o644))

	reloadMarker := filepath.Join(t.TempDir(), "reload-attempted")
	appsettings.NginxSettings.ReloadCmd = fmt.Sprintf(
		"if [ ! -e %q ]; then touch %q; exit 1; fi", reloadMarker, reloadMarker)

	err := Enable("example.com")

	require.Error(t, err)
	_, statErr := os.Lstat(enabledPath)
	assert.ErrorIs(t, statErr, os.ErrNotExist)
	_, markerErr := os.Stat(reloadMarker)
	assert.NoError(t, markerErr)
}

func TestSaveRestoresEnabledConfigWhenTestFails(t *testing.T) {
	confDir, _ := setupSiteMutationTest(t)
	availablePath := filepath.Join(confDir, "sites-available", "example.com")
	enabledPath := filepath.Join(confDir, "sites-enabled", "example.com")
	oldContent := []byte("server { listen 80; }\n")
	require.NoError(t, os.WriteFile(availablePath, oldContent, 0o640))
	require.NoError(t, os.Symlink(availablePath, enabledPath))
	appsettings.NginxSettings.TestConfigCmd = "false"

	err := Save("example.com", "server { listen 81; }\n", true, 0, nil, "")

	require.Error(t, err)
	content, readErr := os.ReadFile(availablePath)
	require.NoError(t, readErr)
	assert.Equal(t, oldContent, content)
	info, statErr := os.Stat(availablePath)
	require.NoError(t, statErr)
	assert.Equal(t, os.FileMode(0o640), info.Mode().Perm())
}

func TestSaveRestoresAndReloadsPreviousConfigWhenReloadFails(t *testing.T) {
	confDir, _ := setupSiteMutationTest(t)
	availablePath := filepath.Join(confDir, "sites-available", "example.com")
	enabledPath := filepath.Join(confDir, "sites-enabled", "example.com")
	oldContent := []byte("server { listen 80; }\n")
	require.NoError(t, os.WriteFile(availablePath, oldContent, 0o644))
	require.NoError(t, os.Symlink(availablePath, enabledPath))

	reloadMarker := filepath.Join(t.TempDir(), "reload-attempted")
	appsettings.NginxSettings.ReloadCmd = fmt.Sprintf(
		"if [ ! -e %q ]; then touch %q; exit 1; fi", reloadMarker, reloadMarker)

	err := Save("example.com", "server { listen 81; }\n", true, 0, nil, model.PostSyncActionReloadNginx)

	require.Error(t, err)
	content, readErr := os.ReadFile(availablePath)
	require.NoError(t, readErr)
	assert.Equal(t, oldContent, content)
	_, markerErr := os.Stat(reloadMarker)
	assert.NoError(t, markerErr)
}
