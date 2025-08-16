package nginx_log

import (
	"sync"
)

var (
	// indexingFiles tracks which files are currently being indexed
	indexingFiles = make(map[string]bool)
	indexingMutex sync.RWMutex
)

// SetIndexingStatus updates the indexing status for a file
func SetIndexingStatus(path string, isIndexing bool) {
	indexingMutex.Lock()
	defer indexingMutex.Unlock()

	if isIndexing {
		indexingFiles[path] = true
	} else {
		delete(indexingFiles, path)
	}
}

// IsFileIndexing checks if a file is currently being indexed
func IsFileIndexing(path string) bool {
	indexingMutex.RLock()
	defer indexingMutex.RUnlock()

	return indexingFiles[path]
}

// GetIndexingFiles returns all files currently being indexed
func GetIndexingFiles() []string {
	indexingMutex.RLock()
	defer indexingMutex.RUnlock()

	files := make([]string, 0, len(indexingFiles))
	for path := range indexingFiles {
		files = append(files, path)
	}
	return files
}