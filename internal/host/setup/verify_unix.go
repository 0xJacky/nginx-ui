//go:build unix

package setup

import (
	"os"
	"syscall"

	"golang.org/x/sys/unix"
)

// dirAccessible reports whether the calling process may read (and, when
// writable is set, write) the directory at path. access(2) evaluates the real
// uid and the mount options, so a read-only bind mount is caught here.
func dirAccessible(path string, writable bool) error {
	mode := unix.W_OK | unix.R_OK
	if !writable {
		mode = unix.R_OK
	}
	return unix.Access(path, uint32(mode))
}

// localInode returns the inode of a stat result. A bind mount exposes the host
// inode unchanged, so it is the evidence the shared-path check compares.
func localInode(info os.FileInfo) (uint64, error) {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, errInodeUnavailable
	}
	return uint64(stat.Ino), nil
}
