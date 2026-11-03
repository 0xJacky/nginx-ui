//go:build !windows

package helper

import "os"

// IsNonexistentPathError reports whether err from os.Stat or os.Lstat means
// that the path does not exist on the file system. See the Windows variant for
// the names that platform rejects instead of reporting them as missing.
func IsNonexistentPathError(err error) bool {
	return os.IsNotExist(err)
}
