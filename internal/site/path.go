package site

import (
	"errors"
	"strings"

	"github.com/0xJacky/Nginx-UI/internal/config"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
)

// ErrInvalidSiteName is returned when a site name cannot be turned into a safe
// path element (empty, ".", or containing a path separator or "..").
var ErrInvalidSiteName = errors.New("invalid site name")

// validateSiteName rejects site names that could escape the sites-available /
// sites-enabled directories once joined into a path. It is intentionally
// self-contained so every path-building entry point in this package carries its
// own barrier, regardless of how ResolveAvailablePath/ResolveEnabledPath are
// implemented.
func validateSiteName(name string) error {
	if name == "" || name == "." || strings.ContainsAny(name, `/\`) || strings.Contains(name, "..") {
		return ErrInvalidSiteName
	}
	return nil
}

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

// ResolveEnabledMaintenancePath resolves the generated maintenance config in
// sites-enabled. It must go through the symlink helper, otherwise Windows setups
// that include sites-enabled/*.conf would never load the file.
func ResolveEnabledMaintenancePath(name string) (string, error) {
	return resolveEnabledSymlinkPath(name + MaintenanceSuffix)
}
