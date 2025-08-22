package nginx_log

import (
	"os"
	"path/filepath"
)

// ExpandLogGroupPath finds all physical files belonging to a log group using filesystem globbing.
func ExpandLogGroupPath(basePath string) ([]string, error) {
	// Find all files belonging to this log group by globbing
	globPath := basePath + "*"
	matches, err := filepath.Glob(globPath)
	if err != nil {
		return nil, err
	}

	// filepath.Glob might not match the base file itself if it has no extension,
	// so we check for it explicitly and add it to the list.
	info, err := os.Stat(basePath)
	if err == nil && info.Mode().IsRegular() {
		matches = append(matches, basePath)
	}

	// Deduplicate file list
	seen := make(map[string]struct{})
	uniqueFiles := make([]string, 0)
	for _, match := range matches {
		if _, ok := seen[match]; !ok {
			// Further check if it's a file, not a directory. Glob can match dirs.
			info, err := os.Stat(match)
			if err == nil && info.Mode().IsRegular() {
				seen[match] = struct{}{}
				uniqueFiles = append(uniqueFiles, match)
			}
		}
	}

	return uniqueFiles, nil
}
