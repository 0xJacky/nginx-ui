//go:build windows

package backup

// isDeviceDifferent always returns false on Windows
// Windows mount points work differently and are not a concern for this use case
func isDeviceDifferent(path string) bool {
	return false
}

// isInMountTable always returns false on Windows
// /proc/mounts doesn't exist on Windows
func isInMountTable(path string) bool {
	return false
}

