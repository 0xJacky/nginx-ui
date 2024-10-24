package analytic

import (
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/net"
	"github.com/uozi-tech/cosy/logger"
	"runtime"
	"time"
)

func getTotalDiskIO() (read, write uint64) {
	diskIOCounters, err := disk.IOCounters()
	if err != nil {
		logger.Error(err.Error())
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
		logger.Error(err.Error())
		return
	}
	time.Sleep(1000 * time.Millisecond)
	cpuTimesAfter, err := cpu.Times(false)
	if err != nil {
		logger.Error(err.Error())
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
	network, err := net.IOCounters(false)

	if err != nil {
		logger.Error(err.Error())
		return
	}

	if len(network) == 0 {
		return
	}
	NetRecvRecord = append(NetRecvRecord, Usage[uint64]{
		Time:  now,
		Usage: network[0].BytesRecv - LastNetRecv,
	})
	NetSentRecord = append(NetSentRecord, Usage[uint64]{
		Time:  now,
		Usage: network[0].BytesSent - LastNetSent,
	})
	LastNetRecv = network[0].BytesRecv
	LastNetSent = network[0].BytesSent
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
