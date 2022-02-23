package analytic

import (
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/net"
	"runtime"
	"time"
)

func getTotalDiskIO() (read, write uint64) {
	diskIOCounters, _ := disk.IOCounters()
	for _, v := range diskIOCounters {
		write += v.WriteCount
		read += v.ReadCount
	}
	return
}

func recordCpu(now time.Time) {
	cpuTimesBefore, _ := cpu.Times(false)
	time.Sleep(1000 * time.Millisecond)
	cpuTimesAfter, _ := cpu.Times(false)
	threadNum := runtime.GOMAXPROCS(0)

	cpuUserUsage := (cpuTimesAfter[0].User - cpuTimesBefore[0].User) / (float64(1000*threadNum) / 1000)
	cpuUserUsage *= 100
	cpuSystemUsage := (cpuTimesAfter[0].System - cpuTimesBefore[0].System) / (float64(1000*threadNum) / 1000)
	cpuSystemUsage *= 100

	u := usage{
		Time:  now,
		Usage: cpuUserUsage,
	}

	CpuUserRecord = append(CpuUserRecord, u)

	s := usage{
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
	network, _ := net.IOCounters(false)
	if len(network) == 0 {
		return
	}
	NetRecvRecord = append(NetRecvRecord, usage{
		Time:  now,
		Usage: network[0].BytesRecv - LastNetRecv,
	})
	NetSentRecord = append(NetSentRecord, usage{
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

	DiskReadRecord = append(DiskReadRecord, usage{
		Time:  now,
		Usage: readCount - LastDiskReads,
	})
	DiskWriteRecord = append(DiskWriteRecord, usage{
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
