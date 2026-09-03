//go:build windows

package setup

import "os"

// dirAccessible has no access(2) equivalent that honours mount options on
// Windows. A Windows build cannot bind-mount a Linux host directory anyway, so
// the mounted-only check reports itself as unsupported instead of guessing.
func dirAccessible(string, bool) error {
	return errMountedChecksUnsupported
}

// localInode has no portable inode source on Windows: os.FileInfo.Sys() is a
// Win32FileAttributeData without a file index, so the shared-path check
// degrades to a warning there.
func localInode(os.FileInfo) (uint64, error) {
	return 0, errMountedChecksUnsupported
}
