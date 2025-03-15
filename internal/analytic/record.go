package analytic

import (
	"runtime"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
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
	// Get network statistics using GetNetworkStat which includes Ethernet interfaces
	networkStats, err := GetNetworkStat()
	if err != nil {
		logger.Error(err)
		return
	}

	// Calculate usage since last record
	bytesRecv := networkStats.BytesRecv - LastNetRecv
	bytesSent := networkStats.BytesSent - LastNetSent

	// Update records
	NetRecvRecord = append(NetRecvRecord, Usage[uint64]{
		Time:  now,
		Usage: bytesRecv,
	})
	NetSentRecord = append(NetSentRecord, Usage[uint64]{
		Time:  now,
		Usage: bytesSent,
	})

	// Update last values
	LastNetRecv = networkStats.BytesRecv
	LastNetSent = networkStats.BytesSent

	// Limit record size
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
