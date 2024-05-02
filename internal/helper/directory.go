package helper

import (
	"github.com/0xJacky/Nginx-UI/internal/logger"
	"path/filepath"
	"strings"
)

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

	absDirectory = filepath.Clean(absDirectory) + string(filepath.Separator)

	return strings.HasPrefix(absPath, absDirectory)
}
