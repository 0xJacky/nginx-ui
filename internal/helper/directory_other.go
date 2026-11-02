//go:build !windows

package helper

import "os"

// isNonexistentPathError reports whether err from os.Lstat means that the path
// does not exist on the file system.
func isNonexistentPathError(err error) bool {
	return os.IsNotExist(err)
}
