//go:build !windows

package sink

import "os"

// openLog opens a log for reading.
func openLog(path string) (*os.File, error) {
	return os.Open(path)
}
