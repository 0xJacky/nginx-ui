package system

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/0xJacky/Nginx-UI/settings"
	cSettings "github.com/uozi-tech/cosy/settings"
)

const (
	InstallSecretHeaderName = "X-Install-Secret"
	InstallSecretQueryKey   = "install_secret"
	InstallSecretFileName   = ".install_secret"
	InstallWindow           = 10 * time.Minute
	installSecretBytes      = 32
)

var startupTime = time.Now()

// SetInstallStartupTimeForTest updates the setup window start time for tests.
func SetInstallStartupTimeForTest(start time.Time) {
	startupTime = start
}

// InstallLockStatus checks if the system is installed.
func InstallLockStatus() bool {
	return settings.NodeSettings.SkipInstallation || cSettings.AppSettings.JwtSecret != ""
}

// IsInstallTimeoutExceeded checks if installation time limit is exceeded.
func IsInstallTimeoutExceeded() bool {
	if time.Since(startupTime) <= InstallWindow {
		return false
	}

	_ = CleanupInstallSecret()
	return true
}

// InstallSecretPath returns the hidden install secret file path in the config directory.
func InstallSecretPath() string {
	return filepath.Join(filepath.Dir(cSettings.ConfPath), InstallSecretFileName)
}

// EnsureInstallSecret refreshes the install secret for a fresh, uninstalled instance.
func EnsureInstallSecret() error {
	if InstallLockStatus() || settings.NodeSettings.SkipInstallation || IsInstallTimeoutExceeded() {
		return CleanupInstallSecret()
	}

	secret, err := generateInstallSecret()
	if err != nil {
		return err
	}

	return writeInstallSecret(secret)
}

// CleanupInstallSecret removes the hidden install secret file if present.
func CleanupInstallSecret() error {
	err := os.Remove(InstallSecretPath())
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}

	return nil
}

// ConsumeInstallSecret removes the current install secret after successful setup.
func ConsumeInstallSecret() error {
	return CleanupInstallSecret()
}

// ValidateInstallSecret validates the provided setup token against the hidden file.
func ValidateInstallSecret(secret string) error {
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return ErrInstallSecretRequired
	}

	if IsInstallTimeoutExceeded() {
		return ErrInstallSecretExpired
	}

	expected, err := readInstallSecret()
	if err != nil {
		return ErrInstallSecretInvalid
	}

	if subtle.ConstantTimeCompare([]byte(secret), []byte(expected)) != 1 {
		return ErrInstallSecretInvalid
	}

	return nil
}

func generateInstallSecret() (string, error) {
	secret := make([]byte, installSecretBytes)
	if _, err := rand.Read(secret); err != nil {
		return "", err
	}

	return hex.EncodeToString(secret), nil
}

func readInstallSecret() (string, error) {
	data, err := os.ReadFile(InstallSecretPath())
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(data)), nil
}

func writeInstallSecret(secret string) error {
	secretPath := InstallSecretPath()
	if err := os.MkdirAll(filepath.Dir(secretPath), 0755); err != nil {
		return err
	}

	tempFile, err := os.CreateTemp(filepath.Dir(secretPath), InstallSecretFileName+".tmp.*")
	if err != nil {
		return err
	}

	tempPath := tempFile.Name()
	defer func() {
		_ = tempFile.Close()
		_ = os.Remove(tempPath)
	}()

	if _, err := tempFile.WriteString(secret + "\n"); err != nil {
		return err
	}

	if err := tempFile.Chmod(0600); err != nil {
		return err
	}

	if err := tempFile.Close(); err != nil {
		return err
	}

	return os.Rename(tempPath, secretPath)
}
