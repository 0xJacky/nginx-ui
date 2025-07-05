//go:build !darwin

package analytic

// otherOSVirtualFilesystems contains additional virtual filesystem types for non-macOS systems
var otherOSVirtualFilesystems = map[string]bool{
	// Empty map for other systems - all virtual filesystems are already in the common list
	// Could add Linux-specific virtual filesystems here if needed:
	// "snap":      true,  // Snap package mounts
	// "squashfs":  true,  // SquashFS (used by snap)
	// "overlay":   true,  // Docker overlay filesystems (already in common list)
}

// shouldSkipPath checks if a path should be skipped from disk calculation on non-macOS systems
func shouldSkipPath(mountpoint, device string) bool {
	// For non-macOS systems, we only do basic filtering
	// Most filtering is handled by the virtual filesystem check

	// Could add Linux-specific logic here if needed
	// For example: skip snap mounts, docker overlay filesystems, etc.

	return false
}

// getAdditionalVirtualFilesystems returns additional virtual filesystem types for non-macOS systems
func getAdditionalVirtualFilesystems() map[string]bool {
	return otherOSVirtualFilesystems
}
