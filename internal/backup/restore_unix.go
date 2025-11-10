//go:build unix

package backup

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

// isDeviceDifferent checks if path is on a different device than its parent
func isDeviceDifferent(path string) bool {
	var pathStat, parentStat syscall.Stat_t
	
	if syscall.Stat(path, &pathStat) != nil {
		return false
	}
	
	if syscall.Stat(filepath.Dir(path), &parentStat) != nil {
		return false
	}
	
	return pathStat.Dev != parentStat.Dev
}

// isInMountTable checks if path is listed in /proc/mounts
func isInMountTable(path string) bool {
	file, err := os.Open("/proc/mounts")
	if err != nil {
		return false
	}
	defer file.Close()

	cleanPath := filepath.Clean(path)
	scanner := bufio.NewScanner(file)
	
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) >= 2 && unescapeOctal(fields[1]) == cleanPath {
			return true
		}
	}

	return false
}

