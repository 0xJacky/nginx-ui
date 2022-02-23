package api

import (
	"encoding/json"
	"fmt"
	"github.com/0xJacky/Nginx-UI/server/tool"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/load"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/net"
	"math"
	"net/http"
	"runtime"
	"strconv"
	"time"

	"github.com/dustin/go-humanize"
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
		return
	}

	defer ws.Close()

	response := make(gin.H)

	for {
		// read
		mt, message, err := ws.ReadMessage()
		if err != nil {
			break
		}
		for {

			memoryStat, err := mem.VirtualMemory()
			if err != nil {
				fmt.Println(err)
				return
			}

			response["memory_total"] = humanize.Bytes(memoryStat.Total)
			response["memory_used"] = humanize.Bytes(memoryStat.Used)
			response["memory_cached"] = humanize.Bytes(memoryStat.Cached)
			response["memory_free"] = humanize.Bytes(memoryStat.Free)
			response["memory_swap_used"] = humanize.Bytes(memoryStat.SwapTotal - memoryStat.SwapFree)
			response["memory_swap_total"] = humanize.Bytes(memoryStat.SwapTotal)
			response["memory_swap_cached"] = humanize.Bytes(memoryStat.SwapCached)
			response["memory_swap_percent"] = float64(memoryStat.SwapFree) / math.Max(float64(memoryStat.SwapTotal), 1)

			response["memory_pressure"], _ = strconv.ParseFloat(fmt.Sprintf("%.2f", memoryStat.UsedPercent), 64)

			cpuTimesBefore, _ := cpu.Times(false)
			time.Sleep(1000 * time.Millisecond)
			cpuTimesAfter, _ := cpu.Times(false)
			threadNum := runtime.GOMAXPROCS(0)

			cpuUserUsage := (cpuTimesAfter[0].User - cpuTimesBefore[0].User) / (float64(1000*threadNum) / 1000)
			cpuSystemUsage := (cpuTimesAfter[0].System - cpuTimesBefore[0].System) / (float64(1000*threadNum) / 1000)

			response["cpu_user"], _ = strconv.ParseFloat(fmt.Sprintf("%.2f",
				cpuUserUsage*100), 64)

			response["cpu_system"], _ = strconv.ParseFloat(fmt.Sprintf("%.2f",
				cpuSystemUsage*100), 64)

			response["cpu_idle"], _ = strconv.ParseFloat(fmt.Sprintf("%.2f",
				(1-cpuUserUsage+cpuSystemUsage)*100), 64)

			response["uptime"], _ = host.Uptime()
			response["loadavg"], _ = load.Avg()

			diskUsage, _ := disk.Usage(".")

			response["disk_used"] = humanize.Bytes(diskUsage.Used)
			response["disk_total"] = humanize.Bytes(diskUsage.Total)
			response["disk_percentage"], _ = strconv.ParseFloat(fmt.Sprintf("%.2f", diskUsage.UsedPercent), 64)

			response["diskIO"] = gin.H{
				"writes": tool.DiskWriteBuffer[len(tool.DiskWriteBuffer)-1],
				"reads":  tool.DiskReadBuffer[len(tool.DiskReadBuffer)-1],
			}

			network, _ := net.IOCounters(false)

			if len(network) > 0 {
				response["network"] = network[0]
			}

			m, _ := json.Marshal(response)
			message = m

			// write
			err = ws.WriteMessage(mt, message)
			if err != nil {
				break
			}
			time.Sleep(800 * time.Microsecond)
		}
	}
}

func GetAnalyticInit(c *gin.Context) {
	cpuInfo, _ := cpu.Info()
	network, _ := net.IOCounters(false)
	var _net net.IOCountersStat
	if len(network) > 0 {
		_net = network[0]
	}
	hostInfo, _ := host.Info()

	c.JSON(http.StatusOK, gin.H{
		"host": hostInfo,
		"cpu": gin.H{
			"info":  cpuInfo,
			"user":  tool.CpuUserBuffer,
			"total": tool.CpuTotalBuffer,
		},
		"network": gin.H{
			"init":      _net,
			"bytesRecv": tool.NetRecvBuffer,
			"bytesSent": tool.NetSentBuffer,
		},
		"diskIO": gin.H{
			"writes": tool.DiskWriteBuffer,
			"reads":  tool.DiskReadBuffer,
		},
	})
}
