package backup

import (
	"io"
	"os"
	"path/filepath"

	"github.com/uozi-tech/cosy"
)

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	// Open source file
	source, err := os.Open(src)
	if err != nil {
		return err
	}
	defer source.Close()

	// Create destination file
	destination, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destination.Close()

	// Copy content
	_, err = io.Copy(destination, source)
	return err
}

// copyDirectory copies a directory recursively from src to dst
func copyDirectory(src, dst string) error {
	// Check if source is a directory
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}
	if !srcInfo.IsDir() {
		return cosy.WrapErrorWithParams(ErrCopyNginxConfigDir, "%s is not a directory", src)
	}

	// Create destination directory
	if err := os.MkdirAll(dst, srcInfo.Mode()); err != nil {
		return err
	}

	// Walk through source directory
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Calculate relative path
		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if relPath == "." {
			return nil
		}

		// Create target path
		targetPath := filepath.Join(dst, relPath)

		// Check if it's a symlink
		if info.Mode()&os.ModeSymlink != 0 {
			// Read the link
			linkTarget, err := os.Readlink(path)
			if err != nil {
				return err
			}
			// Create symlink at target path
			return os.Symlink(linkTarget, targetPath)
		}

		// If it's a directory, create it
		if info.IsDir() {
			return os.MkdirAll(targetPath, info.Mode())
		}

		// If it's a file, copy it
		return copyFile(path, targetPath)
	})
}
