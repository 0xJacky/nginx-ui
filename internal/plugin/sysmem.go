package plugin

import (
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/shirou/gopsutil/v4/mem"
)

// systemMemoryMB is the memory nginx-ui runs with in MiB, 0 when unknown.
// It is the memory limit of the container when one is set, else total RAM.
var systemMemoryMB = sync.OnceValue(func() int {
	total := 0
	if stat, err := mem.VirtualMemory(); err == nil {
		total = int(stat.Total >> 20)
	}
	return effectiveMemoryMB(total, ownMemoryLimitBytes())
})

// SystemMemoryMB returns the memory this host runs with in MiB, 0 when unknown.
func SystemMemoryMB() int {
	return systemMemoryMB()
}

// effectiveMemoryMB picks the smaller of the RAM total and a positive limit.
func effectiveMemoryMB(totalMB int, limitBytes int64) int {
	if limitBytes <= 0 {
		return max(totalMB, 0)
	}
	limitMB := int(limitBytes >> 20)
	if limitMB < 1 {
		limitMB = 1
	}
	if totalMB <= 0 || limitMB < totalMB {
		return limitMB
	}
	return totalMB
}

// parseMemoryMax reads a memory.max value. 0 means no limit.
func parseMemoryMax(raw string) int64 {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "max" {
		return 0
	}
	v, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || v <= 0 {
		return 0
	}
	return v
}

// cgroupV2Path extracts the unified hierarchy path from /proc/self/cgroup.
func cgroupV2Path(procCgroup string) (string, bool) {
	for line := range strings.SplitSeq(procCgroup, "\n") {
		if rest, ok := strings.CutPrefix(line, "0::"); ok {
			return strings.TrimSpace(rest), true
		}
	}
	return "", false
}

// readFileString returns the file content or an empty string.
func readFileString(path string) string {
	raw, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(raw)
}
