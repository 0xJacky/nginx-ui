package tool

import (
	"github.com/shirou/gopsutil/v3/cpu"
	"runtime"
	"time"
)

type cpuUsage struct {
	Time  time.Time `json:"x"`
	Usage float64   `json:"y"`
}

var CpuUserBuffer []cpuUsage
var CpuTotalBuffer []cpuUsage

func RecordCpuUsage() {
	for {
		cpuTimesBefore, _ := cpu.Times(false)
		time.Sleep(1000 * time.Millisecond)
		cpuTimesAfter, _ := cpu.Times(false)
		threadNum := runtime.GOMAXPROCS(0)

		cpuUserUsage := (cpuTimesAfter[0].User - cpuTimesBefore[0].User) / (float64(1000*threadNum) / 1000)
		cpuSystemUsage := (cpuTimesAfter[0].System - cpuTimesBefore[0].System) / (float64(1000*threadNum) / 1000)
		now := time.Now()
		u := cpuUsage{
			Time:  now,
			Usage: cpuUserUsage,
		}
		CpuUserBuffer = append(CpuUserBuffer, u)
		s := cpuUsage{
			Time:  now,
			Usage: cpuUserUsage + cpuSystemUsage,
		}
		CpuTotalBuffer = append(CpuTotalBuffer, s)
		if len(CpuUserBuffer) > 200 {
			CpuUserBuffer = CpuUserBuffer[1:]
		}
		if len(CpuTotalBuffer) > 200 {
			CpuTotalBuffer = CpuTotalBuffer[1:]
		}
		// time.Sleep(1 * time.Second)
	}
}
