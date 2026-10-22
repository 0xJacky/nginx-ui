package analytic

import (
	"fmt"
	"math"
	"net/http"
	"runtime"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/analytic"
	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/internal/kernel"
	"github.com/0xJacky/Nginx-UI/internal/middleware"
	"github.com/0xJacky/Nginx-UI/internal/version"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/load"
	"github.com/uozi-tech/cosy/logger"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

func Analytic(c *gin.Context) {
	serveAnalytic(c, analytic.GetCPUUsage)
}

func serveAnalytic(c *gin.Context, sampleCPU func() (float64, float64, error)) {
	var upGrader = websocket.Upgrader{
		CheckOrigin: middleware.CheckWebSocketOrigin,
	}
	// upgrade http to websocket
	ws, err := upGrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		logger.Error(err)
		return
	}

	defer ws.Close()

	// This handler only writes. The keepalive owns the reader so pongs are
	// processed, and Done fires when the peer stops answering pings.
	keepalive := helper.StartWebSocketKeepaliveReader(ws)
	defer keepalive.Stop()
	peerGone := keepalive.Done()

	var stat Stat

	// waitNext throttles the loop and reports whether it should keep running.
	//
	// Every error path has to go through it. A bare `continue` would spin this
	// goroutine at full speed while flooding the log, and - because the
	// cancellation check lives at the bottom of the loop - it would also never
	// notice that the peer disconnected or that the process is shutting down,
	// leaking one hot goroutine per dashboard connection.
	waitNext := func() bool {
		select {
		case <-kernel.Context.Done():
			logger.Debug("Analytic: Context cancelled, closing WebSocket")
			return false
		case <-peerGone:
			logger.Debug("Analytic: peer disconnected, closing WebSocket")
			return false
		case <-time.After(1 * time.Second):
			return true
		}
	}

	for {
		stat.Memory, err = analytic.GetMemoryStat()
		if err != nil {
			logger.Error(err)
			if !waitNext() {
				return
			}
			continue
		}

		cpuUserUsage, cpuTotalUsage, err := sampleCPU()
		if err != nil {
			logger.Error(err)
			if !waitNext() {
				return
			}
			continue
		}

		cpuUserUsage = math.Round(cpuUserUsage*100) / 100
		cpuTotalUsage = math.Round(cpuTotalUsage*100) / 100
		stat.CPU = CPUStat{
			User: cpuUserUsage,
			// Clients derive total usage from User + System, so include all
			// remaining busy modes here (nice, IRQ and steal as well as system).
			System: math.Round((cpuTotalUsage-cpuUserUsage)*100) / 100,
			Idle:   math.Round((100-cpuTotalUsage)*100) / 100,
			Total:  cpuTotalUsage,
		}

		stat.Uptime, err = host.Uptime()
		if err != nil {
			logger.Error(err)
			if !waitNext() {
				return
			}
			continue
		}

		stat.LoadAvg, err = load.Avg()
		if err != nil {
			logger.Error(err)
			if !waitNext() {
				return
			}
			continue
		}

		stat.Disk, err = analytic.GetDiskStat()
		if err != nil {
			logger.Error(err)
			if !waitNext() {
				return
			}
			continue
		}

		network, err := analytic.GetNetworkStat()
		if err != nil {
			logger.Error(err)
			if !waitNext() {
				return
			}
			continue
		}

		stat.Network = *network
		stat.SampledAt = time.Now().UnixMilli()

		// write
		_ = ws.SetWriteDeadline(time.Now().Add(helper.WebSocketWriteWait))
		err = ws.WriteJSON(stat)
		if err != nil {
			if helper.IsUnexpectedWebsocketError(err) {
				logger.Error(err)
			}
			break
		}

		if !waitNext() {
			return
		}
	}
}

func GetAnalyticInit(c *gin.Context) {
	cpuInfo, err := cpu.Info()
	if err != nil {
		logger.Error(err)
	}

	network, err := analytic.GetNetworkStat()
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

	ipAddresses, err := analytic.GetHostIPAddresses()
	if err != nil {
		logger.Error(err)
	}

	c.JSON(http.StatusOK, InitResp{
		Host:        hostInfo,
		IPAddresses: ipAddresses,
		CPU: CPURecords{
			Info:  cpuInfo,
			User:  analytic.CpuUserRecord,
			Total: analytic.CpuTotalRecord,
		},
		Network: NetworkRecords{
			Init:      *network,
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

func GetNode(c *gin.Context) {
	cpuInfo, err := cpu.Info()
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

	runtimeInfo, err := version.GetRuntimeInfo()
	if err != nil {
		logger.Error("Failed to get runtime info:", err)
		runtimeInfo = version.RuntimeInfo{
			OS:   fmt.Sprintf("%s %s", hostInfo.Platform, hostInfo.PlatformVersion),
			Arch: runtime.GOARCH,
		}
	}

	ver := version.GetVersionInfo()

	nodeInfo := analytic.NodeInfo{
		NodeRuntimeInfo: runtimeInfo,
		Version:         ver.Version,
		CPUNum:          len(cpuInfo),
		MemoryTotal:     memory.Total,
		DiskTotal:       diskStat.Total,
	}

	c.JSON(http.StatusOK, nodeInfo)
}
