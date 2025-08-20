package helper

import (
	"github.com/uozi-tech/cosy/logger"
	"path/filepath"
	"strings"
)

// IsUnderDirectory checks if the path is under the directory
func IsUnderDirectory(path, directory string) bool {
	absPath, err := filepath.Abs(path)
	if err != nil {
		logger.Error(err)
		return false
	}

	absDirectory, err := filepath.Abs(directory)
	if err != nil {
		logger.Error(err)
		return false
	}

	absPath = filepath.Clean(absPath)
	absDirectory = filepath.Clean(absDirectory)

	// Check if path is exactly the directory or under it
	if absPath == absDirectory {
		return true
	}

	absDirectory = absDirectory + string(filepath.Separator)
	return strings.HasPrefix(absPath, absDirectory)
}
