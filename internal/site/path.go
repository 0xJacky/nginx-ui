package site

import (
	"github.com/0xJacky/Nginx-UI/internal/config"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
)

func ResolveAvailablePath(name string) (string, error) {
	return config.ResolveConfPathInDir("sites-available", name)
}

func ResolveEnabledPath(name string) (string, error) {
	return config.ResolveConfPathInDirPreserveLeaf("sites-enabled", name)
}

func resolveEnabledSymlinkPath(name string) (string, error) {
	enabledPath, err := ResolveEnabledPath(name)
	if err != nil {
		return "", err
	}

	return nginx.GetConfSymlinkPath(enabledPath), nil
}
