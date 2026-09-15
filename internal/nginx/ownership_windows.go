//go:build windows

package nginx

import "os"

func localFileOwnership(os.FileInfo) (FileOwnership, bool) {
	return FileOwnership{}, false
}
