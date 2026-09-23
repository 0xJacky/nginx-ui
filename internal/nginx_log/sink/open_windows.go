//go:build windows

package sink

import (
	"os"

	"github.com/nxadm/tail/winfile"
)

// openLog opens a log for reading with FILE_SHARE_DELETE, so the open handle
// does not keep nginx or a log rotation from renaming or deleting the file.
func openLog(path string) (*os.File, error) {
	return winfile.OpenFile(path, os.O_RDONLY, 0)
}
