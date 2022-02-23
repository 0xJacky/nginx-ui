package tool

import (
	"github.com/0xJacky/Nginx-UI/server/settings"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/net"
	"runtime"
	"time"
)

type usage struct {
	Time  time.Time `json:"x"`
	Usage float64   `json:"y"`
}

var CpuUserBuffer []usage
var CpuTotalBuffer []usage
var NetRecvBuffer []usage
var NetSentBuffer []usage
var DiskWriteBuffer []usage
var DiskReadBuffer []usage

var LastDiskWrites uint64
var LastDiskReads uint64

var LastNetRecv uint64
var LastNetSent uint64

func RecordServerAnalytic() {
	network, _ := net.IOCounters(false)
	diskIOCounters, _ := disk.IOCounters(settings.ServerSettings.DiskName)
	diskIO, ok := diskIOCounters[settings.ServerSettings.DiskName]

	if ok {
		LastDiskWrites = diskIO.WriteCount
		LastDiskReads = diskIO.ReadCount
	}

	if len(network) > 0 {
		LastNetRecv = network[0].BytesRecv
		LastNetSent = network[0].BytesSent
	}

	now := time.Now()
	// 初始化记录数组
	for i := 100; i > 0; i-- {
		u := usage{Time: now.Add(time.Duration(-i) * time.Second)}
		CpuUserBuffer = append(CpuUserBuffer, u)
		CpuTotalBuffer = append(CpuTotalBuffer, u)
		NetRecvBuffer = append(NetRecvBuffer, u)
		NetSentBuffer = append(NetSentBuffer, u)
		DiskWriteBuffer = append(DiskWriteBuffer, u)
		DiskReadBuffer = append(DiskReadBuffer, u)
	}
	for {
		cpuTimesBefore, _ := cpu.Times(false)
		time.Sleep(1000 * time.Millisecond)
		cpuTimesAfter, _ := cpu.Times(false)
		threadNum := runtime.GOMAXPROCS(0)

		cpuUserUsage := (cpuTimesAfter[0].User - cpuTimesBefore[0].User) / (float64(1000*threadNum) / 1000)
		cpuUserUsage *= 100
		cpuSystemUsage := (cpuTimesAfter[0].System - cpuTimesBefore[0].System) / (float64(1000*threadNum) / 1000)
		cpuSystemUsage *= 100
		now := time.Now()
		u := usage{
			Time:  now,
			Usage: cpuUserUsage,
		}
		CpuUserBuffer = append(CpuUserBuffer, u)
		s := usage{
			Time:  now,
			Usage: cpuUserUsage + cpuSystemUsage,
		}
		CpuTotalBuffer = append(CpuTotalBuffer, s)
		if len(CpuUserBuffer) > 100 {
			CpuUserBuffer = CpuUserBuffer[1:]
		}
		if len(CpuTotalBuffer) > 100 {
			CpuTotalBuffer = CpuTotalBuffer[1:]
		}
		network, _ = net.IOCounters(false)
		if len(network) == 0 {
			continue
		}
		NetRecvBuffer = append(NetRecvBuffer, usage{
			Time:  now,
			Usage: float64(network[0].BytesRecv - LastNetRecv),
		})
		NetSentBuffer = append(NetRecvBuffer, usage{
			Time:  now,
			Usage: float64(network[0].BytesSent - LastNetSent),
		})
		LastNetRecv = network[0].BytesRecv
		LastNetSent = network[0].BytesSent
		if len(NetRecvBuffer) > 100 {
			NetRecvBuffer = NetRecvBuffer[1:]
		}
		if len(NetSentBuffer) > 100 {
			NetSentBuffer = NetSentBuffer[1:]
		}
		diskIOCounters, _ = disk.IOCounters(settings.ServerSettings.DiskName)
		diskIO, ok = diskIOCounters[settings.ServerSettings.DiskName]
		if ok {
			DiskReadBuffer = append(DiskReadBuffer, usage{
				Time:  now,
				Usage: float64(diskIO.ReadCount - LastDiskReads),
			})
			DiskWriteBuffer = append(DiskWriteBuffer, usage{
				Time:  now,
				Usage: float64(diskIO.WriteCount - LastDiskWrites),
			})
			if len(DiskReadBuffer) > 100 {
				DiskReadBuffer = DiskReadBuffer[1:]
			}
			if len(DiskWriteBuffer) > 100 {
				DiskWriteBuffer = DiskWriteBuffer[1:]
			}
			LastDiskWrites = diskIO.WriteCount
			LastDiskReads = diskIO.ReadCount
		}
	}
}
