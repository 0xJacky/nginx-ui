// Package cgroup exposes the CPU and memory budget the current process is
// actually allowed to consume.
//
// Go sizes runtime.NumCPU/GOMAXPROCS from the CPU affinity mask, and
// /proc/meminfo reports whatever the kernel exposes. Inside a cgroup-limited
// container - Docker with --cpus, Kubernetes limits, and in particular an LXC
// container on Proxmox - neither reflects the real budget: the affinity mask
// still lists every host CPU while the CPU bandwidth controller throttles the
// container to a fraction of one. Sizing worker pools or memory budgets from
// the host numbers therefore oversubscribes the container by an order of
// magnitude.
//
// The helpers here read the cgroup v2 and v1 controller files directly and
// fall back to "unlimited" whenever the information is unavailable, so callers
// can clamp their own defaults without special-casing the platform.
package cgroup

import (
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/shirou/gopsutil/v4/mem"
)

// cgroupRoot is the mount point of the cgroup filesystem. It is a variable so
// tests can point the readers at a fixture directory.
var cgroupRoot = "/sys/fs/cgroup"

// maxReasonableMemoryLimit filters the sentinel values some kernels use to mean
// "no limit" (for example math.MaxInt64 rounded down to the page size).
const maxReasonableMemoryLimit = int64(1) << 60

// CPUQuota returns the number of CPUs the cgroup bandwidth controller allows
// this process to use. The second return value is false when no quota is
// configured, when the platform has no cgroup filesystem, or when the values
// cannot be parsed.
func CPUQuota() (float64, bool) {
	// cgroup v2: "<quota> <period>" or "max <period>".
	if raw, err := os.ReadFile(filepath.Join(cgroupRoot, "cpu.max")); err == nil {
		fields := strings.Fields(string(raw))
		if len(fields) >= 2 && fields[0] != "max" {
			quota, quotaErr := strconv.ParseInt(fields[0], 10, 64)
			period, periodErr := strconv.ParseInt(fields[1], 10, 64)
			if quotaErr == nil && periodErr == nil && quota > 0 && period > 0 {
				return float64(quota) / float64(period), true
			}
		}
	}

	// cgroup v1: quota and period live in separate files, quota == -1 means no limit.
	quota, quotaOK := readInt64(filepath.Join(cgroupRoot, "cpu", "cpu.cfs_quota_us"))
	period, periodOK := readInt64(filepath.Join(cgroupRoot, "cpu", "cpu.cfs_period_us"))
	if quotaOK && periodOK && quota > 0 && period > 0 {
		return float64(quota) / float64(period), true
	}

	return 0, false
}

// MemoryLimit returns the cgroup memory limit in bytes. The second return value
// is false when the cgroup does not cap memory.
func MemoryLimit() (int64, bool) {
	candidates := []string{
		filepath.Join(cgroupRoot, "memory.max"),                      // cgroup v2
		filepath.Join(cgroupRoot, "memory", "memory.limit_in_bytes"), // cgroup v1
	}

	for _, path := range candidates {
		raw, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		value := strings.TrimSpace(string(raw))
		if value == "" || value == "max" {
			continue
		}
		limit, err := strconv.ParseInt(value, 10, 64)
		if err != nil || limit <= 0 || limit >= maxReasonableMemoryLimit {
			continue
		}
		return limit, true
	}

	return 0, false
}

// readInt64 parses a cgroup file holding a single integer.
func readInt64(path string) (int64, bool) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return 0, false
	}
	value, err := strconv.ParseInt(strings.TrimSpace(string(raw)), 10, 64)
	if err != nil {
		return 0, false
	}
	return value, true
}

// totalMemory is indirected so tests can simulate a host without touching the
// real /proc/meminfo.
var totalMemory = func() (uint64, error) {
	stat, err := mem.VirtualMemory()
	if err != nil {
		return 0, err
	}
	return stat.Total, nil
}

// AvailableMemory reports the memory budget this process should size itself
// against: the cgroup limit when one is set, otherwise the total system memory.
// The second return value is false when neither number is available.
func AvailableMemory() (int64, bool) {
	limit, hasLimit := MemoryLimit()

	total, err := totalMemory()
	if err != nil || total == 0 || total > uint64(maxReasonableMemoryLimit) {
		return limit, hasLimit
	}

	if hasLimit && limit < int64(total) {
		return limit, true
	}
	return int64(total), true
}

// AvailableCPUs reports how many CPUs may be used for sizing worker pools.
//
// It is the smaller of GOMAXPROCS and the cgroup CPU quota, and never less
// than 1. Callers should use this instead of runtime.GOMAXPROCS/NumCPU when the
// value decides how many goroutines will compete for the CPU: exceeding the
// cgroup quota does not add throughput, it only multiplies peak memory and
// causes the scheduler to thrash against CFS throttling.
func AvailableCPUs() int {
	procs := runtime.GOMAXPROCS(0)
	if procs < 1 {
		procs = 1
	}

	quota, ok := CPUQuota()
	if !ok {
		return procs
	}

	limited := int(math.Ceil(quota))
	if limited < 1 {
		limited = 1
	}
	if limited < procs {
		return limited
	}
	return procs
}
