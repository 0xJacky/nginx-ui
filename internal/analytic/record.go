package analytic

import (
	stdnet "net"
	"runtime"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/net"
	"github.com/uozi-tech/cosy/logger"
)

func getTotalDiskIO() (read, write uint64) {
	diskIOCounters, err := disk.IOCounters()
	if err != nil {
		logger.Error(err)
		return
	}
	for _, v := range diskIOCounters {
		write += v.WriteCount
		read += v.ReadCount
	}
	return
}

func recordCpu(now time.Time) {
	cpuTimesBefore, err := cpu.Times(false)
	if err != nil {
		logger.Error(err)
		return
	}

	time.Sleep(1000 * time.Millisecond)

	cpuTimesAfter, err := cpu.Times(false)
	if err != nil {
		logger.Error(err)
		return
	}

	threadNum := runtime.GOMAXPROCS(0)

	cpuUserUsage := (cpuTimesAfter[0].User - cpuTimesBefore[0].User) / (float64(1000*threadNum) / 1000)
	cpuUserUsage *= 100
	cpuSystemUsage := (cpuTimesAfter[0].System - cpuTimesBefore[0].System) / (float64(1000*threadNum) / 1000)
	cpuSystemUsage *= 100

	u := Usage[float64]{
		Time:  now,
		Usage: cpuUserUsage,
	}

	CpuUserRecord = append(CpuUserRecord, u)

	s := Usage[float64]{
		Time:  now,
		Usage: cpuUserUsage + cpuSystemUsage,
	}

	CpuTotalRecord = append(CpuTotalRecord, s)

	if len(CpuUserRecord) > 100 {
		CpuUserRecord = CpuUserRecord[1:]
	}

	if len(CpuTotalRecord) > 100 {
		CpuTotalRecord = CpuTotalRecord[1:]
	}
}

func recordNetwork(now time.Time) {
	// Get separate statistics for each interface
	networkStats, err := net.IOCounters(true)
	if err != nil {
		logger.Error(err)
		return
	}

	if len(networkStats) == 0 {
		return
	}

	// Get all network interfaces
	interfaces, err := stdnet.Interfaces()
	if err != nil {
		logger.Error(err)
		return
	}

	var totalBytesRecv uint64
	var totalBytesSent uint64
	var externalInterfaceFound bool

	// Iterate through all interfaces to find external ones
	for _, iface := range interfaces {
		// Skip interfaces that are down
		if iface.Flags&stdnet.FlagUp == 0 {
			continue
		}

		// Get IP addresses for the interface
		addrs, err := iface.Addrs()
		if err != nil {
			logger.Error(err)
			continue
		}

		// Check if this is an external interface
		for _, addr := range addrs {
			if ipNet, ok := addr.(*stdnet.IPNet); ok {
				// Exclude loopback addresses and private IPs
				if !ipNet.IP.IsLoopback() {
					// Found external interface, accumulate its statistics
					for _, stat := range networkStats {
						if stat.Name == iface.Name {
							totalBytesRecv += stat.BytesRecv
							totalBytesSent += stat.BytesSent
							externalInterfaceFound = true
							break
						}
					}
					break
				}
			}
		}
	}

	// If no external interface is found, use fallback option
	if !externalInterfaceFound {
		// Fallback: use all non-loopback interfaces
		for _, iface := range interfaces {
			if iface.Flags&stdnet.FlagLoopback == 0 && iface.Flags&stdnet.FlagUp != 0 {
				for _, stat := range networkStats {
					if stat.Name == iface.Name {
						totalBytesRecv += stat.BytesRecv
						totalBytesSent += stat.BytesSent
						break
					}
				}
			}
		}
	}

	LastNetRecv = totalBytesRecv
	LastNetSent = totalBytesSent

	NetRecvRecord = append(NetRecvRecord, Usage[uint64]{
		Time:  now,
		Usage: totalBytesRecv - LastNetRecv,
	})
	NetSentRecord = append(NetSentRecord, Usage[uint64]{
		Time:  now,
		Usage: totalBytesSent - LastNetSent,
	})

	if len(NetRecvRecord) > 100 {
		NetRecvRecord = NetRecvRecord[1:]
	}
	if len(NetSentRecord) > 100 {
		NetSentRecord = NetSentRecord[1:]
	}
}

func recordDiskIO(now time.Time) {
	readCount, writeCount := getTotalDiskIO()

	DiskReadRecord = append(DiskReadRecord, Usage[uint64]{
		Time:  now,
		Usage: readCount - LastDiskReads,
	})
	DiskWriteRecord = append(DiskWriteRecord, Usage[uint64]{
		Time:  now,
		Usage: writeCount - LastDiskWrites,
	})
	if len(DiskReadRecord) > 100 {
		DiskReadRecord = DiskReadRecord[1:]
	}
	if len(DiskWriteRecord) > 100 {
		DiskWriteRecord = DiskWriteRecord[1:]
	}
	LastDiskWrites = writeCount
	LastDiskReads = readCount
}
