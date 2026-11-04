package analytic

import (
	"errors"
	"math"
	"os"
	"runtime"
	"strconv"
	"strings"

	"github.com/shirou/gopsutil/v4/host"
)

const procUptimePath = "/proc/uptime"

// Uptime returns the system uptime in seconds. On Linux it reads /proc/uptime,
// which LXCFS virtualizes per container; the sysinfo syscall gopsutil uses
// reports the host kernel's uptime inside LXC.
func Uptime() (uint64, error) {
	return readUptime(runtime.GOOS, procUptimePath, host.Uptime)
}

func readUptime(goos, path string, fallback func() (uint64, error)) (uint64, error) {
	if goos == "linux" {
		if uptime, err := parseProcUptime(path); err == nil {
			return uptime, nil
		}
	}
	return fallback()
}

func parseProcUptime(path string) (uint64, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	fields := strings.Fields(string(data))
	if len(fields) == 0 {
		return 0, errors.New("empty uptime file")
	}
	seconds, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0, err
	}
	if seconds < 0 || math.IsInf(seconds, 0) || math.IsNaN(seconds) {
		return 0, errors.New("invalid uptime value")
	}
	return uint64(seconds), nil
}
