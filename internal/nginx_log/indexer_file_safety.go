package nginx_log

import (
	"fmt"
	"os"
)

// safeGetFileInfo safely gets file information after validating the path
func (li *LogIndexer) safeGetFileInfo(filePath string) (os.FileInfo, error) {
	// Validate path is under whitelist before accessing
	if !IsLogPathUnderWhiteList(filePath) {
		return nil, fmt.Errorf("file path not under whitelist: %s", filePath)
	}

	// Additional validation using isValidLogPath
	if !isValidLogPath(filePath) {
		return nil, fmt.Errorf("invalid log path: %s", filePath)
	}

	return os.Stat(filePath)
}

// safeOpenFile safely opens a file after validating the path
func (li *LogIndexer) safeOpenFile(filePath string) (*os.File, error) {
	// Validate path is under whitelist before accessing
	if !IsLogPathUnderWhiteList(filePath) {
		return nil, fmt.Errorf("file path not under whitelist: %s", filePath)
	}

	// Additional validation using isValidLogPath
	if !isValidLogPath(filePath) {
		return nil, fmt.Errorf("invalid log path: %s", filePath)
	}

	return os.Open(filePath)
}

// safeReadDir safely reads a directory after validating the path
func (li *LogIndexer) safeReadDir(dirPath string) ([]os.DirEntry, error) {
	// Validate directory path is under whitelist before accessing
	if !IsLogPathUnderWhiteList(dirPath) {
		return nil, fmt.Errorf("directory path not under whitelist: %s", dirPath)
	}

	return os.ReadDir(dirPath)
}