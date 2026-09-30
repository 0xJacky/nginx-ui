//go:build !linux

package plugin

// ownMemoryLimitBytes has no cgroup to read outside Linux.
func ownMemoryLimitBytes() int64 {
	return 0
}
