package api

import (
	"fmt"
	"github.com/0xJacky/Nginx-UI/server/internal/analytic"
	"github.com/0xJacky/Nginx-UI/server/internal/logger"
	"github.com/pkg/errors"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/load"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/net"
	"github.com/spf13/cast"
	"math"
	"net/http"
	"runtime"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

type CPUStat struct {
	User   float64 `json:"user"`
	System float64 `json:"system"`
	Idle   float64 `json:"idle"`
	Total  float64 `json:"total"`
}

type MemStat struct {
	Total       string  `json:"total"`
	Used        string  `json:"used"`
	Cached      string  `json:"cached"`
	Free        string  `json:"free"`
	SwapUsed    string  `json:"swap_used"`
	SwapTotal   string  `json:"swap_total"`
	SwapCached  string  `json:"swap_cached"`
	SwapPercent float64 `json:"swap_percent"`
	Pressure    float64 `json:"pressure"`
}

type DiskStat struct {
	Total      string         `json:"total"`
	Used       string         `json:"used"`
	Percentage float64        `json:"percentage"`
	Writes     analytic.Usage `json:"writes"`
	Reads      analytic.Usage `json:"reads"`
}

type Stat struct {
	Uptime  uint64             `json:"uptime"`
	LoadAvg *load.AvgStat      `json:"loadavg"`
	CPU     CPUStat            `json:"cpu"`
	Memory  MemStat            `json:"memory"`
	Disk    DiskStat           `json:"disk"`
	Network net.IOCountersStat `json:"network"`
}

func getMemoryStat() (MemStat, error) {
	memoryStat, err := mem.VirtualMemory()
	if err != nil {
		return MemStat{}, errors.Wrap(err, "error analytic getMemoryStat")
	}
	return MemStat{
		Total:      humanize.Bytes(memoryStat.Total),
		Used:       humanize.Bytes(memoryStat.Used),
		Cached:     humanize.Bytes(memoryStat.Cached),
		Free:       humanize.Bytes(memoryStat.Free),
		SwapUsed:   humanize.Bytes(memoryStat.SwapTotal - memoryStat.SwapFree),
		SwapTotal:  humanize.Bytes(memoryStat.SwapTotal),
		SwapCached: humanize.Bytes(memoryStat.SwapCached),
		SwapPercent: cast.ToFloat64(fmt.Sprintf("%.2f",
			100*float64(memoryStat.SwapTotal-memoryStat.SwapFree)/math.Max(float64(memoryStat.SwapTotal), 1))),
		Pressure: cast.ToFloat64(fmt.Sprintf("%.2f", memoryStat.UsedPercent)),
	}, nil
}

func getDiskStat() (DiskStat, error) {
	diskUsage, err := disk.Usage(".")

	if err != nil {
		return DiskStat{}, errors.Wrap(err, "error analytic getDiskStat")
	}

	return DiskStat{
		Used:       humanize.Bytes(diskUsage.Used),
		Total:      humanize.Bytes(diskUsage.Total),
		Percentage: cast.ToFloat64(fmt.Sprintf("%.2f", diskUsage.UsedPercent)),
		Writes:     analytic.DiskWriteRecord[len(analytic.DiskWriteRecord)-1],
		Reads:      analytic.DiskReadRecord[len(analytic.DiskReadRecord)-1],
	}, nil
}

func Analytic(c *gin.Context) {
	var upGrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
	// upgrade http to websocket
	ws, err := upGrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		logger.Error(err)
		return
	}

	defer ws.Close()

	var stat Stat

	for {
		stat.Memory, err = getMemoryStat()

		if err != nil {
			logger.Error(err)
			return
		}

		cpuTimesBefore, _ := cpu.Times(false)
		time.Sleep(1000 * time.Millisecond)
		cpuTimesAfter, _ := cpu.Times(false)
		threadNum := runtime.GOMAXPROCS(0)
		cpuUserUsage := (cpuTimesAfter[0].User - cpuTimesBefore[0].User) / (float64(1000*threadNum) / 1000)
		cpuSystemUsage := (cpuTimesAfter[0].System - cpuTimesBefore[0].System) / (float64(1000*threadNum) / 1000)

		stat.CPU = CPUStat{
			User:   cast.ToFloat64(fmt.Sprintf("%.2f", cpuUserUsage*100)),
			System: cast.ToFloat64(fmt.Sprintf("%.2f", cpuSystemUsage*100)),
			Idle:   cast.ToFloat64(fmt.Sprintf("%.2f", (1-cpuUserUsage-cpuSystemUsage)*100)),
			Total:  cast.ToFloat64(fmt.Sprintf("%.2f", (cpuUserUsage+cpuSystemUsage)*100)),
		}

		stat.Uptime, _ = host.Uptime()

		stat.LoadAvg, _ = load.Avg()

		stat.Disk, err = getDiskStat()

		if err != nil {
			logger.Error(err)
			return
		}

		network, _ := net.IOCounters(false)

		if len(network) > 0 {
			stat.Network = network[0]
		}

		// write
		err = ws.WriteJSON(stat)
		if err != nil {
			logger.Error(err)
			break
		}
		time.Sleep(800 * time.Microsecond)
	}

}

func GetAnalyticInit(c *gin.Context) {
	cpuInfo, _ := cpu.Info()
	network, _ := net.IOCounters(false)
	memory, err := getMemoryStat()

	if err != nil {
		logger.Error(err)
		return
	}

	diskStat, err := getDiskStat()

	if err != nil {
		logger.Error(err)
		return
	}

	var _net net.IOCountersStat
	if len(network) > 0 {
		_net = network[0]
	}
	hostInfo, _ := host.Info()

	switch hostInfo.Platform {
	case "ubuntu":
		hostInfo.Platform = "Ubuntu"
	case "centos":
		hostInfo.Platform = "CentOS"
	}

	loadAvg, _ := load.Avg()

	c.JSON(http.StatusOK, gin.H{
		"host": hostInfo,
		"cpu": gin.H{
			"info":  cpuInfo,
			"user":  analytic.CpuUserRecord,
			"total": analytic.CpuTotalRecord,
		},
		"network": gin.H{
			"init":      _net,
			"bytesRecv": analytic.NetRecvRecord,
			"bytesSent": analytic.NetSentRecord,
		},
		"disk_io": gin.H{
			"writes": analytic.DiskWriteRecord,
			"reads":  analytic.DiskReadRecord,
		},
		"memory":  memory,
		"disk":    diskStat,
		"loadavg": loadAvg,
	})
}
