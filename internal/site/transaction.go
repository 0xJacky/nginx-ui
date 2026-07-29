package site

import (
	"errors"
	"fmt"
	"os"

	"github.com/0xJacky/Nginx-UI/internal/nginx"
)

type configFileSnapshot struct {
	exists  bool
	content []byte
	mode    os.FileMode
}

func captureConfigFile(path string) (configFileSnapshot, error) {
	content, err := nginx.ReadFile(path)
	if os.IsNotExist(err) {
		return configFileSnapshot{}, nil
	}
	if err != nil {
		return configFileSnapshot{}, err
	}

	info, err := nginx.Stat(path)
	if err != nil {
		return configFileSnapshot{}, err
	}

	return configFileSnapshot{
		exists:  true,
		content: content,
		mode:    info.Mode().Perm(),
	}, nil
}

func (snapshot configFileSnapshot) restore(path string) error {
	if !snapshot.exists {
		if err := nginx.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}

	return writeConfigFile(path, snapshot.content, snapshot.mode)
}

func writeConfigFile(path string, content []byte, mode os.FileMode) error {
	// Write in place to preserve symlink targets and file-level bind mounts.
	// The caller keeps a snapshot and restores it when validation or reload fails.
	return nginx.WriteFile(path, content, mode)
}

func rollbackError(primary error, rollback func() error) error {
	if err := rollback(); err != nil {
		return errors.Join(primary, fmt.Errorf("rollback failed: %w", err))
	}
	return primary
}

func restoreConfigAndReload(path string, snapshot configFileSnapshot) error {
	if err := snapshot.restore(path); err != nil {
		return err
	}

	if result := nginx.Control(nginx.TestConfig); result.IsError() {
		return fmt.Errorf("test restored configuration: %w", result.GetError())
	}
	if result := nginx.Control(nginx.Reload); result.IsError() {
		return fmt.Errorf("reload restored configuration: %w", result.GetError())
	}

	return nil
}

func removeEnabledLink(path string) error {
	if err := nginx.Remove(path); err != nil && !os.IsNotExist(err) {
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
