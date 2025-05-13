//go:build windows

package nginx

import "strings"

// fix #1046
// nginx.conf include sites-enabled/*.conf
// sites-enabled/example.com.conf -> example.com.conf.conf

// GetConfSymlinkPath returns the path of the symlink file
func GetConfSymlinkPath(path string) string {
	return path + ".conf"
}

// GetConfNameBySymlinkName returns the name of the symlink file
func GetConfNameBySymlinkName(name string) string {
	return strings.TrimSuffix(name, ".conf")
}
