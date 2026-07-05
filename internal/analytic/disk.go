package analytic

import (
	"fmt"

	"github.com/dustin/go-humanize"
	"github.com/pkg/errors"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/spf13/cast"
)

func getVisiblePartitions() ([]disk.PartitionStat, error) {
	partitions, err := disk.Partitions(false)
	if err != nil {
		return nil, err
	}

	if len(partitions) > 0 {
		return partitions, nil
	}

	partitions, err = disk.Partitions(true)
	if err != nil {
		return nil, err
	}

	return partitions, nil
}

func GetDiskStat() (DiskStat, error) {
	partitions, err := getVisiblePartitions()
	if err != nil {
		return DiskStat{}, errors.Wrap(err, "error analytic getDiskStat - getting partitions")
	}

	return buildDiskStat(partitions, disk.Usage, getFilesystemKey), nil
}

type diskUsageFunc func(path string) (*disk.UsageStat, error)
type filesystemKeyFunc func(partition disk.PartitionStat, usage *disk.UsageStat) (string, error)

func buildDiskStat(partitions []disk.PartitionStat, getUsage diskUsageFunc, getKey filesystemKeyFunc) DiskStat {
	var totalSize uint64
	var totalUsed uint64
	var partitionStats []PartitionStat
	seenFilesystems := make(map[string]struct{})

	for _, partition := range partitions {
		usage, err := getUsage(partition.Mountpoint)
		if err != nil {
			continue
		}

		if isVirtualFilesystem(partition.Fstype) {
			continue
		}

		if shouldSkipPath(partition.Mountpoint, partition.Device) {
			continue
		}

		partitionStat := PartitionStat{
			Mountpoint: partition.Mountpoint,
			Device:     partition.Device,
			Fstype:     partition.Fstype,
			Total:      humanize.IBytes(usage.Total),
			Used:       humanize.IBytes(usage.Used),
			Free:       humanize.IBytes(usage.Free),
			Percentage: cast.ToFloat64(fmt.Sprintf("%.2f", usage.UsedPercent)),
		}
		partitionStats = append(partitionStats, partitionStat)

		key, err := getKey(partition, usage)
		if err != nil || key == "" {
			key = fallbackFilesystemKey(partition)
		}
		if _, exists := seenFilesystems[key]; !exists {
			seenFilesystems[key] = struct{}{}
			totalSize += usage.Total
			totalUsed += usage.Used
		}
	}

	var overallPercentage float64
	if totalSize > 0 {
		overallPercentage = cast.ToFloat64(fmt.Sprintf("%.2f", float64(totalUsed)/float64(totalSize)*100))
	}

	return DiskStat{
		Used:       humanize.IBytes(totalUsed),
		Total:      humanize.IBytes(totalSize),
		Percentage: overallPercentage,
		Writes:     DiskWriteRecord[len(DiskWriteRecord)-1],
		Reads:      DiskReadRecord[len(DiskReadRecord)-1],
		Partitions: partitionStats,
	}
}

func fallbackFilesystemKey(partition disk.PartitionStat) string {
	if partition.Device != "" {
		return "device:" + partition.Device
	}

	return "mountpoint:" + partition.Mountpoint
}

// isVirtualFilesystem checks if the filesystem type is virtual
func isVirtualFilesystem(fstype string) bool {
	virtualFSTypes := map[string]bool{
		// Common virtual filesystems
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

		// Network filesystems (should be excluded from total disk calculation)
		"nfs":    true,
		"nfs4":   true,
		"cifs":   true,
		"smb":    true,
		"smbfs":  true,
		"afpfs":  true,
		"webdav": true,
		"ftpfs":  true,
	}

	// Check common virtual filesystems first
	if virtualFSTypes[fstype] {
		return true
	}

	// Check OS-specific additional virtual filesystems
	additionalFS := getAdditionalVirtualFilesystems()
	return additionalFS[fstype]
}
