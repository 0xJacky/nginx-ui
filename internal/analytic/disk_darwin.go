//go:build darwin

package analytic

import "strings"

// macOSVirtualFilesystems contains macOS-specific virtual filesystem types
var macOSVirtualFilesystems = map[string]bool{
	"devtmpfs":      true,
	"kernfs":        true,
	"fdesc":         true,
	"map":           true,
	"synthfs":       true,
	"volfs":         true,
	"ctlfs":         true,
	"objfs":         true,
	"procfs":        true,
	"lifs":          true,
	"mtab":          true,
	"nullfs":        true,
	"unionfs":       true,
	"osxfuse":       true,
	"macfuse":       true,
	"fuse":          true,
	"bindfs":        true,
	"autofs_nowait": true,
}

// shouldSkipPath checks if a macOS path should be skipped from disk calculation
func shouldSkipPath(mountpoint, device string) bool {
	// Skip Time Machine snapshots and system snapshots
	if strings.Contains(mountpoint, ".timemachine") ||
		strings.Contains(mountpoint, ".Snapshot") ||
		strings.Contains(mountpoint, "/.vol/") ||
		strings.Contains(device, "@") { // APFS snapshots contain @
		return true
	}

	// Skip read-only system volumes (including root partition on macOS Catalina+)
	// The root "/" partition is read-only and shares space with "/System/Volumes/Data"
	if strings.HasPrefix(mountpoint, "/System/Volumes/") &&
		!strings.HasPrefix(mountpoint, "/System/Volumes/Data") {
		return true
	}

	// Skip root partition "/" on macOS Catalina+ to avoid double counting with Data volume
	// In modern macOS, "/" and "/System/Volumes/Data" are the same APFS container
	if mountpoint == "/" {
		return true
	}

	// Skip preboot and recovery volumes
	if strings.Contains(mountpoint, "Preboot") ||
		strings.Contains(mountpoint, "Recovery") ||
		strings.Contains(mountpoint, "Update") ||
		strings.Contains(mountpoint, "VM") {
		return true
	}

	// Skip network mounts
	if strings.HasPrefix(device, "//") ||
		strings.HasPrefix(device, "afp://") ||
		strings.HasPrefix(device, "smb://") ||
		strings.HasPrefix(device, "nfs://") {
		return true
	}

	// Skip virtual disk images
	if strings.Contains(device, ".dmg") ||
		strings.Contains(device, ".sparsebundle") ||
		strings.Contains(device, ".sparseimage") {
		return true
	}

	return false
}

// getAdditionalVirtualFilesystems returns macOS-specific virtual filesystem types
func getAdditionalVirtualFilesystems() map[string]bool {
	return macOSVirtualFilesystems
}
