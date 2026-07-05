//go:build !(aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris)

package analytic

import (
	"strings"

	"github.com/shirou/gopsutil/v4/disk"
)

func getFilesystemKey(partition disk.PartitionStat, _ *disk.UsageStat) (string, error) {
	if partition.Device != "" {
		return "device:" + strings.ToLower(partition.Device), nil
	}

	return "mountpoint:" + partition.Mountpoint, nil
}
