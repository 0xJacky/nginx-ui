package analytic

import (
	"errors"
	"math"
	"runtime"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
)

// GetCPUUsage samples host-wide user and total busy CPU percentages over one second.
// The node summary, history and live WebSocket must use the same normalization.
func GetCPUUsage() (user, total float64, err error) {
	return sampleCPUUsage(time.Second, cpu.Times, time.Sleep)
}

// calculateCPUUsage normalizes aggregate CPU counters by their own elapsed time.
// GOMAXPROCS may reflect a container quota and cannot normalize host counters.
func calculateCPUUsage(before, after cpu.TimesStat) (user, total float64) {
	beforeTotal, afterTotal := before.Total(), after.Total()
	if runtime.GOOS == "linux" {
		// Linux already includes guest time in User and Nice (as in gopsutil).
		beforeTotal -= before.Guest + before.GuestNice
		afterTotal -= after.Guest + after.GuestNice
	}
	elapsed := afterTotal - beforeTotal
	busy := (afterTotal - after.Idle - after.Iowait) - (beforeTotal - before.Idle - before.Iowait)
	if elapsed <= 0 || busy <= 0 {
		return 0, 0
	}
	user = math.Min(100, math.Max(0, (after.User-before.User)/elapsed*100))
	total = math.Min(100, math.Max(0, busy/elapsed*100))
	return user, total
}

func sampleCPUUsage(interval time.Duration, read func(bool) ([]cpu.TimesStat, error), sleep func(time.Duration)) (user, total float64, err error) {
	before, err := read(false)
	if err != nil {
		return 0, 0, err
	}
	if len(before) == 0 {
		return 0, 0, errors.New("empty aggregate CPU statistics")
	}
	sleep(interval)
	after, err := read(false)
	if err != nil {
		return 0, 0, err
	}
	if len(after) == 0 {
		return 0, 0, errors.New("empty aggregate CPU statistics")
	}
	user, total = calculateCPUUsage(before[0], after[0])
	return user, total, nil
}
