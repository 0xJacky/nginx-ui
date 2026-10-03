package plugin

import (
	"path/filepath"
)

// ownMemoryLimitBytes returns the smallest memory.max on the cgroup v2 path
// of this process up to the root, 0 when none is set.
func ownMemoryLimitBytes() int64 {
	return memoryLimitAt("/sys/fs/cgroup", readFileString("/proc/self/cgroup"))
}

// memoryLimitAt walks from the cgroup of the process to the root below root.
func memoryLimitAt(root, procCgroup string) int64 {
	path, ok := cgroupV2Path(procCgroup)
	if !ok {
		return 0
	}
	var limit int64
	for dir := filepath.Clean("/" + path); ; dir = filepath.Dir(dir) {
		v := parseMemoryMax(readFileString(filepath.Join(root, dir, "memory.max")))
		if v > 0 && (limit == 0 || v < limit) {
			limit = v
		}
		if dir == "/" {
			break
		}
	}
	return limit
}
