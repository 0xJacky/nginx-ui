package analytic

import (
	"fmt"
	"math"

	"github.com/dustin/go-humanize"
	"github.com/pkg/errors"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/spf13/cast"
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

type PartitionStat struct {
	Mountpoint string  `json:"mountpoint"`
	Device     string  `json:"device"`
	Fstype     string  `json:"fstype"`
	Total      string  `json:"total"`
	Used       string  `json:"used"`
	Free       string  `json:"free"`
	Percentage float64 `json:"percentage"`
}

type DiskStat struct {
	Total      string          `json:"total"`
	Used       string          `json:"used"`
	Percentage float64         `json:"percentage"`
	Writes     Usage[uint64]   `json:"writes"`
	Reads      Usage[uint64]   `json:"reads"`
	Partitions []PartitionStat `json:"partitions"`
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
	// Get all partitions
	partitions, err := disk.Partitions(false)
	if err != nil {
		return DiskStat{}, errors.Wrap(err, "error analytic getDiskStat - getting partitions")
	}

	var totalSize uint64
	var totalUsed uint64
	var partitionStats []PartitionStat

	// Get usage for each partition
	for _, partition := range partitions {
		usage, err := disk.Usage(partition.Mountpoint)
		if err != nil {
			// Skip partitions that can't be accessed
			continue
		}

		// Skip virtual filesystems and special filesystems
		if isVirtualFilesystem(partition.Fstype) {
			continue
		}

		partitionStat := PartitionStat{
			Mountpoint: partition.Mountpoint,
			Device:     partition.Device,
			Fstype:     partition.Fstype,
			Total:      humanize.Bytes(usage.Total),
			Used:       humanize.Bytes(usage.Used),
			Free:       humanize.Bytes(usage.Free),
			Percentage: cast.ToFloat64(fmt.Sprintf("%.2f", usage.UsedPercent)),
		}

		partitionStats = append(partitionStats, partitionStat)
		totalSize += usage.Total
		totalUsed += usage.Used
	}

	// Calculate overall percentage
	var overallPercentage float64
	if totalSize > 0 {
		overallPercentage = cast.ToFloat64(fmt.Sprintf("%.2f", float64(totalUsed)/float64(totalSize)*100))
	}

	return DiskStat{
		Used:       humanize.Bytes(totalUsed),
		Total:      humanize.Bytes(totalSize),
		Percentage: overallPercentage,
		Writes:     DiskWriteRecord[len(DiskWriteRecord)-1],
		Reads:      DiskReadRecord[len(DiskReadRecord)-1],
		Partitions: partitionStats,
	}, nil
}

// isVirtualFilesystem checks if the filesystem type is virtual
func isVirtualFilesystem(fstype string) bool {
	virtualFSTypes := map[string]bool{
		"proc":        true,
		"sysfs":       true,
		"devfs":       true,
		"devpts":      true,
		"tmpfs":       true,
		"debugfs":     true,
		"securityfs":  true,
		"cgroup":      true,
		"cgroup2":     true,
		"pstore":      true,
		"bpf":         true,
		"tracefs":     true,
		"hugetlbfs":   true,
		"mqueue":      true,
		"overlay":     true,
		"autofs":      true,
		"binfmt_misc": true,
		"configfs":    true,
		"fusectl":     true,
		"rpc_pipefs":  true,
		"selinuxfs":   true,
		"systemd-1":   true,
		"none":        true,
	}

	return virtualFSTypes[fstype]
}
