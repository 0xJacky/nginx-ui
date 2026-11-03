//go:build windows

package helper

import (
	"errors"
	"os"

	"golang.org/x/sys/windows"
)

// IsNonexistentPathError reports whether err from os.Stat or os.Lstat means
// that the path does not exist on the file system.
//
// Windows rejects names holding characters such as "*" with
// ERROR_INVALID_NAME instead of reporting them as missing, so a wildcard
// certificate path like "ssl\*.example.com_EC256" failed the containment check
// and silently fell back to the nginx configuration directory, and checking
// for "sites-enabled\*.example.com.conf" logged an error (issue #2067).
//
// Treating it as missing does not weaken the symlink check in
// IsUnderDirectory: a name the file system refuses can neither exist nor be a
// symlink, so nothing at or below it can redirect the path. The walk still
// resolves every existing ancestor with EvalSymlinks and only appends the
// impossible components lexically.
func IsNonexistentPathError(err error) bool {
	return os.IsNotExist(err) || errors.Is(err, windows.ERROR_INVALID_NAME)
}
