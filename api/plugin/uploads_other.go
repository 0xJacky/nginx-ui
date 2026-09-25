//go:build !unix

package plugin

import "os"

// isPrivateDir skips the owner check where unix permissions do not apply.
func isPrivateDir(os.FileInfo) bool {
	return true
}
