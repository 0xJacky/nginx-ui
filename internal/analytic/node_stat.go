package analytic

import (
	"github.com/0xJacky/Nginx-UI/internal/upstream"
	"github.com/shirou/gopsutil/v4/load"
	"github.com/uozi-tech/cosy/logger"
)

func GetNodeStat() (data NodeStat) {
	memory, err := GetMemoryStat()

	if err != nil {
		logger.Error(err)
		return
	}

	_, cpuUsage, err := GetCPUUsage()
	if err != nil {
		logger.Error(err)
		return
	}

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
	upstreamStatusMap := upstreamService.GetAvailabilityMap()

	return NodeStat{
		AvgLoad:           loadAvg,
		CPUPercent:        cpuUsage,
		MemoryPercent:     memory.Pressure,
		DiskPercent:       diskStat.Percentage,
		Network:           *network,
		UpstreamStatusMap: upstreamStatusMap,
	}
}
