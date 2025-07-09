package analytic

import (
	"fmt"
	"math"

	"github.com/dustin/go-humanize"
	"github.com/pkg/errors"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/spf13/cast"
)

func GetMemoryStat() (MemStat, error) {
	memoryStat, err := mem.VirtualMemory()
	if err != nil {
		return MemStat{}, errors.Wrap(err, "error analytic getMemoryStat")
	}
	return MemStat{
		Total:      humanize.IBytes(memoryStat.Total),
		Used:       humanize.IBytes(memoryStat.Used),
		Cached:     humanize.IBytes(memoryStat.Cached),
		Free:       humanize.IBytes(memoryStat.Free),
		SwapUsed:   humanize.IBytes(memoryStat.SwapTotal - memoryStat.SwapFree),
		SwapTotal:  humanize.IBytes(memoryStat.SwapTotal),
		SwapCached: humanize.IBytes(memoryStat.SwapCached),
		SwapPercent: cast.ToFloat64(fmt.Sprintf("%.2f",
			100*float64(memoryStat.SwapTotal-memoryStat.SwapFree)/math.Max(float64(memoryStat.SwapTotal), 1))),
		Pressure: cast.ToFloat64(fmt.Sprintf("%.2f", memoryStat.UsedPercent)),
	}, nil
}
