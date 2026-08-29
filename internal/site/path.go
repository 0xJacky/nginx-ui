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

// resolveEnabledMaintenancePath resolves the generated maintenance config in
// sites-enabled. It must go through the symlink helper, otherwise Windows setups
// that include sites-enabled/*.conf would never load the file.
func resolveEnabledMaintenancePath(name string) (string, error) {
	return resolveEnabledSymlinkPath(name + MaintenanceSuffix)
}
