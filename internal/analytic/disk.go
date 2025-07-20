package analytic

import (
	"fmt"

	"github.com/dustin/go-humanize"
	"github.com/pkg/errors"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/spf13/cast"
)

func GetDiskStat() (DiskStat, error) {
	// Get all partitions
	partitions, err := disk.Partitions(false)
	if err != nil {
		return DiskStat{}, errors.Wrap(err, "error analytic getDiskStat - getting partitions")
	}

	var totalSize uint64
	var totalUsed uint64
	var partitionStats []PartitionStat
	// Track partitions to avoid double counting same partition with multiple mount points
	partitionUsage := make(map[string]*disk.UsageStat)

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

		// Skip OS-specific paths that shouldn't be counted
		if shouldSkipPath(partition.Mountpoint, partition.Device) {
			continue
		}

		// Create partition stat for display purposes
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

		// Only count each partition device once for total calculation
		// This handles cases where same partition is mounted multiple times (e.g., bind mounts, overlayfs)
		if _, exists := partitionUsage[partition.Device]; !exists {
			partitionUsage[partition.Device] = usage
			totalSize += usage.Total
			totalUsed += usage.Used
		}
	}

	// Calculate overall percentage
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
	}, nil
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
