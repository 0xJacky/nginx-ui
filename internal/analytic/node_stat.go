package analytic

import (
	"math"
	"runtime"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/upstream"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/load"
	"github.com/uozi-tech/cosy/logger"
)

func GetNodeStat() (data NodeStat) {
	memory, err := GetMemoryStat()

	if err != nil {
		logger.Error(err)
		return
	}

	cpuTimesBefore, _ := cpu.Times(false)
	time.Sleep(1000 * time.Millisecond)
	cpuTimesAfter, _ := cpu.Times(false)
	threadNum := runtime.GOMAXPROCS(0)
	cpuUserUsage := (cpuTimesAfter[0].User - cpuTimesBefore[0].User) / (float64(1000*threadNum) / 1000)
	cpuSystemUsage := (cpuTimesAfter[0].System - cpuTimesBefore[0].System) / (float64(1000*threadNum) / 1000)

	loadAvg, err := load.Avg()
	if err != nil {
		logger.Error(err)
		return
	}

	diskStat, err := GetDiskStat()
	if err != nil {
		logger.Error(err)
		return
	}

	network, err := GetNetworkStat()
	if err != nil {
		logger.Error(err)
		return
	}

	// Get upstream status for current node
	upstreamService := upstream.GetUpstreamService()
	
	// Ensure upstream availability test is performed if targets exist
	if upstreamService.GetTargetCount() > 0 {
		upstreamService.PerformAvailabilityTest()
	}
	
	upstreamStatusMap := upstreamService.GetAvailabilityMap()

	return NodeStat{
		AvgLoad:           loadAvg,
		CPUPercent:        math.Min((cpuUserUsage+cpuSystemUsage)*100, 100),
		MemoryPercent:     memory.Pressure,
		DiskPercent:       diskStat.Percentage,
		Network:           *network,
		UpstreamStatusMap: upstreamStatusMap,
	}
}
