package site

import (
	"os"

	"github.com/0xJacky/Nginx-UI/internal/config"
)

// The site editor and the configuration editor share the same rollback rules,
// so both go through the transaction helpers of the config package. Those
// helpers address the nginx target filesystem, so they also work when the
// configuration lives on an SSH host reached over SFTP.
type configFileSnapshot = config.FileSnapshot

func captureConfigFile(path string) (configFileSnapshot, error) {
	return config.CaptureFile(path)
}

func writeConfigFile(path string, content []byte, mode os.FileMode) error {
	return config.WriteFile(path, content, mode)
}

func rollbackError(primary error, rollback func() error) error {
	return config.RollbackError(primary, rollback)
}

func restoreConfigAndReload(path string, snapshot configFileSnapshot) error {
	return config.RestoreAndReload(path, snapshot)
}

func removeEnabledLink(path string) error {
	return config.RemoveEnabledLink(path)
}

func removeEnabledLinkAndReload(path string) error {
	return config.RemoveEnabledLinkAndReload(path)
}
