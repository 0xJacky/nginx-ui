package analytic

import (
	"fmt"
	"net/http"
	"runtime"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/analytic"
	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/load"
	"github.com/shirou/gopsutil/v4/net"
	"github.com/spf13/cast"
	"github.com/uozi-tech/cosy/logger"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

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
		stat.Memory, err = analytic.GetMemoryStat()
		if err != nil {
			logger.Error(err)
			continue
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

		stat.Uptime, err = host.Uptime()
		if err != nil {
			logger.Error(err)
			continue
		}

		stat.LoadAvg, err = load.Avg()
		if err != nil {
			logger.Error(err)
			continue
		}

		stat.Disk, err = analytic.GetDiskStat()
		if err != nil {
			logger.Error(err)
			continue
		}

		network, err := net.IOCounters(false)
		if err != nil {
			logger.Error(err)
			continue
		}

		if len(network) > 0 {
			stat.Network = network[0]
		}

		// write
		err = ws.WriteJSON(stat)
		if err != nil {
			if helper.IsUnexpectedWebsocketError(err) {
				logger.Error(err)
			}
			break
		}

		time.Sleep(1 * time.Second)
	}
}

func GetAnalyticInit(c *gin.Context) {
	cpuInfo, err := cpu.Info()
	if err != nil {
		logger.Error(err)
	}

	network, err := net.IOCounters(false)
	if err != nil {
		logger.Error(err)
	}

	memory, err := analytic.GetMemoryStat()
	if err != nil {
		logger.Error(err)
	}

	diskStat, err := analytic.GetDiskStat()
	if err != nil {
		logger.Error(err)
	}

	var _net net.IOCountersStat
	if len(network) > 0 {
		_net = network[0]
	}

	hostInfo, err := host.Info()
	if err != nil {
		logger.Error(err)
		hostInfo = &host.InfoStat{}
	}

	switch hostInfo.Platform {
	case "ubuntu":
		hostInfo.Platform = "Ubuntu"
	case "centos":
		hostInfo.Platform = "CentOS"
	}

	loadAvg, err := load.Avg()
	if err != nil {
		logger.Error(err)
		loadAvg = &load.AvgStat{}
	}

	c.JSON(http.StatusOK, InitResp{
		Host: hostInfo,
		CPU: CPURecords{
			Info:  cpuInfo,
			User:  analytic.CpuUserRecord,
			Total: analytic.CpuTotalRecord,
		},
		Network: NetworkRecords{
			Init:      _net,
			BytesRecv: analytic.NetRecvRecord,
			BytesSent: analytic.NetSentRecord,
		},
		DiskIO: DiskIORecords{
			Writes: analytic.DiskWriteRecord,
			Reads:  analytic.DiskReadRecord,
		},
		Memory:  memory,
		Disk:    diskStat,
		LoadAvg: loadAvg,
	})
}
