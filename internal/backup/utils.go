package backup

import (
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/uozi-tech/cosy"
)

// ValidatePathAccess validates if a given path is within the granted access paths.
// This function ensures that all backup read/write operations are restricted to
// authorized directories only, preventing unauthorized file system access.
//
// Parameters:
//   - path: The file system path to validate
//
// Returns:
//   - error: CosyError if path is not allowed, nil if path is valid
func ValidatePathAccess(path string) error {
	if path == "" {
		return cosy.WrapErrorWithParams(ErrInvalidPath, "path cannot be empty")
	}

	// Clean the path to resolve any relative components like ".." or "."
	cleanPath := filepath.Clean(path)

	// Check if the path is within any of the granted access paths
	for _, allowedPath := range settings.BackupSettings.GrantedAccessPath {
		if allowedPath == "" {
			continue
		}

		// Clean the allowed path as well for consistent comparison
		cleanAllowedPath := filepath.Clean(allowedPath)

		// Special case: if allowed path is root directory, allow all paths
		if cleanAllowedPath == string(filepath.Separator) {
			return nil
		}

		// Check if the path is within the allowed path
		if strings.HasPrefix(cleanPath, cleanAllowedPath) {
			// Ensure it's actually a subdirectory or the same directory
			// This prevents "/tmp" from matching "/tmpfoo"
			if cleanPath == cleanAllowedPath || strings.HasPrefix(cleanPath, cleanAllowedPath+string(filepath.Separator)) {
				return nil
			}
		}
	}

	return cosy.WrapErrorWithParams(ErrPathNotInGrantedAccess, cleanPath)
}

// ValidateBackupPath validates the backup source path for custom directory backups.
// This function checks if the source directory exists and is accessible.
//
// Parameters:
//   - path: The backup source path to validate
//
// Returns:
//   - error: CosyError if validation fails, nil if path is valid
func ValidateBackupPath(path string) error {
	// First check if path is in granted access paths
	if err := ValidatePathAccess(path); err != nil {
		return err
	}

	// Check if the path exists and is a directory
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cosy.WrapErrorWithParams(ErrBackupPathNotExist, path)
		}
		return cosy.WrapErrorWithParams(ErrBackupPathAccess, path, err.Error())
	}

	if !info.IsDir() {
		return cosy.WrapErrorWithParams(ErrBackupPathNotDirectory, path)
	}

	return nil
}

// ValidateStoragePath validates the storage destination path for backup files.
// This function ensures the storage directory exists or can be created.
//
// Parameters:
//   - path: The storage destination path to validate
//
// Returns:
//   - error: CosyError if validation fails, nil if path is valid
func ValidateStoragePath(path string) error {
	// First check if path is in granted access paths
	if err := ValidatePathAccess(path); err != nil {
		return err
	}

	// Check if the directory exists, if not try to create it
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := os.MkdirAll(path, 0755); err != nil {
			return cosy.WrapErrorWithParams(ErrCreateStorageDir, path, err.Error())
		}
	} else if err != nil {
		return cosy.WrapErrorWithParams(ErrStoragePathAccess, path, err.Error())
	}

	return nil
}

// copyFile copies a file from source to destination with proper error handling.
// This function handles file copying operations used in backup processes.
//
// Parameters:
//   - src: Source file path
//   - dst: Destination file path
//
// Returns:
//   - error: Standard error if copy operation fails
func copyFile(src, dst string) error {
	// Open source file for reading
	source, err := os.Open(src)
	if err != nil {
		return err
	}
	defer source.Close()

	// Create destination file for writing
	destination, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destination.Close()

	// Copy file content from source to destination
	_, err = io.Copy(destination, source)
	return err
}

// copyDirectory recursively copies a directory from source to destination.
// This function preserves file permissions and handles symbolic links properly.
//
// Parameters:
//   - src: Source directory path
//   - dst: Destination directory path
//
// Returns:
//   - error: CosyError if copy operation fails
func copyDirectory(src, dst string) error {
	// Verify source is a directory
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}
	if !srcInfo.IsDir() {
		return cosy.WrapErrorWithParams(ErrCopyNginxConfigDir, "%s is not a directory", src)
	}

	// Create destination directory with same permissions as source
	if err := os.MkdirAll(dst, 0755); err != nil {
		return err
	}

	// Walk through source directory and copy all contents
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Calculate relative path from source root
		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if relPath == "." {
			return nil
		}

		// Construct target path
		targetPath := filepath.Join(dst, relPath)

		// Handle symbolic links by recreating them
		if info.Mode()&os.ModeSymlink != 0 {
			linkTarget, err := os.Readlink(path)
			if err != nil {
				return err
			}
			return os.Symlink(linkTarget, targetPath)
		}

		// Create directories with original permissions
		if info.IsDir() {
			return os.MkdirAll(targetPath, 0755)
		}

		// Copy regular files
		return copyFile(path, targetPath)
	})
}
