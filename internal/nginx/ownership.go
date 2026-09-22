package nginx

import (
	"os"

	"github.com/pkg/sftp"
)

// FileOwnership identifies the numeric owner and group of a file on the nginx
// target filesystem.
type FileOwnership struct {
	UID int
	GID int
}

// Ownership returns numeric ownership when the target filesystem exposes it.
func Ownership(info os.FileInfo) (FileOwnership, bool) {
	if stat, ok := info.Sys().(*sftp.FileStat); ok {
		return FileOwnership{UID: int(stat.UID), GID: int(stat.GID)}, true
	}
	return localFileOwnership(info)
}
