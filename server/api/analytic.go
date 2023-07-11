package api

import (
    "fmt"
    "github.com/0xJacky/Nginx-UI/server/internal/analytic"
    "github.com/0xJacky/Nginx-UI/server/internal/logger"
    "github.com/shirou/gopsutil/v3/cpu"
    "github.com/shirou/gopsutil/v3/host"
    "github.com/shirou/gopsutil/v3/load"
    "github.com/shirou/gopsutil/v3/net"
    "github.com/spf13/cast"
    "net/http"
    "runtime"
    "time"

    "github.com/gin-gonic/gin"
    "github.com/gorilla/websocket"
)

type CPUStat struct {
    User   float64 `json:"user"`
    System float64 `json:"system"`
    Idle   float64 `json:"idle"`
    Total  float64 `json:"total"`
}

type Stat struct {
    Uptime  uint64             `json:"uptime"`
    LoadAvg *load.AvgStat      `json:"loadavg"`
    CPU     CPUStat            `json:"cpu"`
    Memory  analytic.MemStat   `json:"memory"`
    Disk    analytic.DiskStat  `json:"disk"`
    Network net.IOCountersStat `json:"network"`
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
        stat.Memory, err = analytic.GetMemoryStat()

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

        stat.Disk, err = analytic.GetDiskStat()

        if err != nil {
            logger.Error(err)
            return
        }

        network, _ := net.IOCounters(false)

        if len(network) > 0 {
            stat.Network = network[0]
        }

        // write
        if err != nil || websocket.IsUnexpectedCloseError(err,
            websocket.CloseGoingAway,
            websocket.CloseNoStatusReceived,
            websocket.CloseNormalClosure) {
            logger.Error(err)
            break
        }
        time.Sleep(800 * time.Microsecond)
    }
}

func GetAnalyticInit(c *gin.Context) {
    cpuInfo, _ := cpu.Info()
    network, _ := net.IOCounters(false)
    memory, err := analytic.GetMemoryStat()

    if err != nil {
        logger.Error(err)
        return
    }

    diskStat, err := analytic.GetDiskStat()

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

func GetNodeStat(c *gin.Context) {
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

    for {
        // write
        if err != nil || websocket.IsUnexpectedCloseError(err,
            websocket.CloseGoingAway,
            websocket.CloseNoStatusReceived,
            websocket.CloseNormalClosure) {
            logger.Error(err)
            break
        }

        time.Sleep(10 * time.Second)
    }
}

func GetNodesAnalytic(c *gin.Context) {
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

    for {
        // write
        if err != nil || websocket.IsUnexpectedCloseError(err,
            websocket.CloseGoingAway,
            websocket.CloseNoStatusReceived,
            websocket.CloseNormalClosure) {
            logger.Error(err)
            break
        }

        time.Sleep(10 * time.Second)
    }
}
