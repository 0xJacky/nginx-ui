package analytic

import (
	"fmt"
	"github.com/dustin/go-humanize"
	"github.com/pkg/errors"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/spf13/cast"
	"math"
)

type MemStat struct {
	Total       string  `json:"total"`
	Used        string  `json:"used"`
	Cached      string  `json:"cached"`
	Free        string  `json:"free"`
	SwapUsed    string  `json:"swap_used"`
	SwapTotal   string  `json:"swap_total"`
	SwapCached  string  `json:"swap_cached"`
	SwapPercent float64 `json:"swap_percent"`
	Pressure    float64 `json:"pressure"`
}

type DiskStat struct {
	Total      string        `json:"total"`
	Used       string        `json:"used"`
	Percentage float64       `json:"percentage"`
	Writes     Usage[uint64] `json:"writes"`
	Reads      Usage[uint64] `json:"reads"`
}

func GetMemoryStat() (MemStat, error) {
	memoryStat, err := mem.VirtualMemory()
	if err != nil {
		return MemStat{}, errors.Wrap(err, "error analytic getMemoryStat")
	}
	return MemStat{
		Total:      humanize.Bytes(memoryStat.Total),
		Used:       humanize.Bytes(memoryStat.Used),
		Cached:     humanize.Bytes(memoryStat.Cached),
		Free:       humanize.Bytes(memoryStat.Free),
		SwapUsed:   humanize.Bytes(memoryStat.SwapTotal - memoryStat.SwapFree),
		SwapTotal:  humanize.Bytes(memoryStat.SwapTotal),
		SwapCached: humanize.Bytes(memoryStat.SwapCached),
		SwapPercent: cast.ToFloat64(fmt.Sprintf("%.2f",
			100*float64(memoryStat.SwapTotal-memoryStat.SwapFree)/math.Max(float64(memoryStat.SwapTotal), 1))),
		Pressure: cast.ToFloat64(fmt.Sprintf("%.2f", memoryStat.UsedPercent)),
	}, nil
}

func GetDiskStat() (DiskStat, error) {
	diskUsage, err := disk.Usage(".")

	if err != nil {
		return DiskStat{}, errors.Wrap(err, "error analytic getDiskStat")
	}

	return DiskStat{
		Used:       humanize.Bytes(diskUsage.Used),
		Total:      humanize.Bytes(diskUsage.Total),
		Percentage: cast.ToFloat64(fmt.Sprintf("%.2f", diskUsage.UsedPercent)),
		Writes:     DiskWriteRecord[len(DiskWriteRecord)-1],
		Reads:      DiskReadRecord[len(DiskReadRecord)-1],
	}, nil
}
