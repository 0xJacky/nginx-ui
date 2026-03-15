package helper

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/uozi-tech/cosy/logger"
)

func resolvePathWithExistingSymlinks(path string) (string, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}

	currentPath := filepath.Clean(absPath)
	tail := make([]string, 0)

	for {
		_, err = os.Lstat(currentPath)
		if err == nil {
			resolvedPath, err := filepath.EvalSymlinks(currentPath)
			if err != nil {
				return "", err
			}

			for i := len(tail) - 1; i >= 0; i-- {
				resolvedPath = filepath.Join(resolvedPath, tail[i])
			}

			return filepath.Clean(resolvedPath), nil
		}

		if !os.IsNotExist(err) {
			return "", err
		}

		parentPath := filepath.Dir(currentPath)
		if parentPath == currentPath {
			return currentPath, nil
		}

		tail = append(tail, filepath.Base(currentPath))
		currentPath = parentPath
	}
}

// IsUnderDirectory checks if the path is under the directory
func IsUnderDirectory(path, directory string) bool {
	resolvedPath, err := resolvePathWithExistingSymlinks(path)
	if err != nil {
		logger.Error(err)
		return false
	}

	resolvedDirectory, err := resolvePathWithExistingSymlinks(directory)
	if err != nil {
		logger.Error(err)
		return false
	}

	resolvedPath = filepath.Clean(resolvedPath)
	resolvedDirectory = filepath.Clean(resolvedDirectory)

	// Check if path is exactly the directory or under it
	if resolvedPath == resolvedDirectory {
		return true
	}

	resolvedDirectory = resolvedDirectory + string(filepath.Separator)
	return strings.HasPrefix(resolvedPath, resolvedDirectory)
}
