package system

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/stretchr/testify/require"
	cSettings "github.com/uozi-tech/cosy/settings"
)

func setupInstallSecretTest(t *testing.T) string {
	t.Helper()

	confDir := t.TempDir()
	confPath := filepath.Join(confDir, "app.ini")
	require.NoError(t, os.WriteFile(confPath, []byte("[app]\n"), 0644))

	cSettings.ConfPath = confPath
	cSettings.AppSettings.JwtSecret = ""
	settings.NodeSettings.SkipInstallation = false
	settings.NodeSettings.Secret = ""
	startupTime = time.Now()

	t.Cleanup(func() {
		_ = CleanupInstallSecret()
		cSettings.AppSettings.JwtSecret = ""
		settings.NodeSettings.SkipInstallation = false
		settings.NodeSettings.Secret = ""
		startupTime = time.Now()
	})

	return confDir
}

func TestEnsureInstallSecretCreatesHiddenFile(t *testing.T) {
	confDir := setupInstallSecretTest(t)

	require.NoError(t, EnsureInstallSecret())

	secretPath := filepath.Join(confDir, InstallSecretFileName)
	data, err := os.ReadFile(secretPath)
	require.NoError(t, err)
	require.NotEmpty(t, data)

	info, err := os.Stat(secretPath)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0600), info.Mode().Perm())
}

func TestEnsureInstallSecretRemovesStaleFileWhenInstalled(t *testing.T) {
	confDir := setupInstallSecretTest(t)
	secretPath := filepath.Join(confDir, InstallSecretFileName)
	require.NoError(t, os.WriteFile(secretPath, []byte("stale"), 0600))

	cSettings.AppSettings.JwtSecret = "installed"
	require.NoError(t, EnsureInstallSecret())

	_, err := os.Stat(secretPath)
	require.True(t, errors.Is(err, os.ErrNotExist))
}

func TestValidateInstallSecret(t *testing.T) {
	setupInstallSecretTest(t)
	require.NoError(t, EnsureInstallSecret())

	data, err := os.ReadFile(InstallSecretPath())
	require.NoError(t, err)
	secret := string(data)

	require.ErrorIs(t, ValidateInstallSecret(""), ErrInstallSecretRequired)
	require.ErrorIs(t, ValidateInstallSecret("wrong-secret"), ErrInstallSecretInvalid)
	require.NoError(t, ValidateInstallSecret(secret))
}

func TestInstallSecretExpiresAndCleansUp(t *testing.T) {
	confDir := setupInstallSecretTest(t)
	require.NoError(t, EnsureInstallSecret())

	startupTime = time.Now().Add(-InstallWindow - time.Second)
	require.True(t, IsInstallTimeoutExceeded())
	require.ErrorIs(t, ValidateInstallSecret("whatever"), ErrInstallSecretExpired)

	_, err := os.Stat(filepath.Join(confDir, InstallSecretFileName))
	require.True(t, errors.Is(err, os.ErrNotExist))
}
