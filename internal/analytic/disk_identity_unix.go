//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package analytic

import (
	"fmt"

	"github.com/shirou/gopsutil/v4/disk"
	"golang.org/x/sys/unix"
)

func getFilesystemKey(partition disk.PartitionStat, _ *disk.UsageStat) (string, error) {
	var stat unix.Stat_t
	if err := unix.Stat(partition.Mountpoint, &stat); err != nil {
		return "", err
	}

	return fmt.Sprintf("dev:%d", stat.Dev), nil
}
