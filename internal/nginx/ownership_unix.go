//go:build unix

package nginx

import (
	"os"
	"syscall"
)

func localFileOwnership(info os.FileInfo) (FileOwnership, bool) {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return FileOwnership{}, false
	}
	return FileOwnership{UID: int(stat.Uid), GID: int(stat.Gid)}, true
}
