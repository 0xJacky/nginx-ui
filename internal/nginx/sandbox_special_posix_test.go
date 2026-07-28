//go:build !windows

package nginx

import (
	"path/filepath"
	"syscall"
)

func createSandboxSpecialEntry(confDir string) (string, error) {
	const name = "control.fifo"
	return name, syscall.Mkfifo(filepath.Join(confDir, name), 0600)
}
