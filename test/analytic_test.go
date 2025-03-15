package test

import (
	"fmt"
	"runtime"
	"testing"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/analytic"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/load"
	"github.com/shirou/gopsutil/v4/mem"
)

func TestGoPsutil(t *testing.T) {
	fmt.Println("os:", runtime.GOOS)
	fmt.Println("threads:", runtime.GOMAXPROCS(0))

	v, _ := mem.VirtualMemory()

	loadAvg, _ := load.Avg()

	fmt.Println("loadavg", loadAvg.String())

	fmt.Printf("Total: %v, Free:%v, UsedPercent:%f%%\n", v.Total, v.Free, v.UsedPercent)
	cpuTimesBefore, _ := cpu.Times(false)
	time.Sleep(1000 * time.Millisecond)
	cpuTimesAfter, _ := cpu.Times(false)
	threadNum := runtime.GOMAXPROCS(0)
	fmt.Println(cpuTimesBefore[0].String(), "\n", cpuTimesAfter[0].String())
	cpuUserUsage := (cpuTimesAfter[0].User - cpuTimesBefore[0].User) / (float64(1000*threadNum) / 1000)
	cpuSystemUsage := (cpuTimesAfter[0].System - cpuTimesBefore[0].System) / (float64(1000*threadNum) / 1000)
	fmt.Printf("%.2f, %.2f\n", cpuUserUsage*100, cpuSystemUsage*100)

	diskUsage, _ := disk.Usage(".")
	fmt.Println(diskUsage.String())

	network, _ := analytic.GetNetworkStat()
	fmt.Println(network)
	time.Sleep(time.Second)
	network, _ = analytic.GetNetworkStat()
	fmt.Println(network)
}
