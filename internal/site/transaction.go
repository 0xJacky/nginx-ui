package site

import (
	"fmt"
	"os"

	"github.com/0xJacky/Nginx-UI/internal/config"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
)

// The site editor and the configuration editor share the same rollback rules,
// so both go through the transaction helpers of the config package.
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
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func removeEnabledLinkAndReload(path string) error {
	if err := removeEnabledLink(path); err != nil {
		return err
	}

	if result := nginx.Control(nginx.TestConfig); result.IsError() {
		return fmt.Errorf("test configuration after removing enabled link: %w", result.GetError())
	}
	if result := nginx.Control(nginx.Reload); result.IsError() {
		return fmt.Errorf("reload configuration after removing enabled link: %w", result.GetError())
	}

	return nil
}
