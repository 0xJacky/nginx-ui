//go:build unix

package plugin

import (
	"os"
	"syscall"
)

// isPrivateDir reports whether dir belongs to this user and is closed to others.
func isPrivateDir(info os.FileInfo) bool {
	stat, ok := info.Sys().(*syscall.Stat_t)
	return ok && int(stat.Uid) == os.Geteuid() && info.Mode().Perm()&0o077 == 0
}
