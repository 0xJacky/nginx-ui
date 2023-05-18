package api

import (
	"github.com/0xJacky/Nginx-UI/server/internal/analytic"
	"github.com/0xJacky/Nginx-UI/server/internal/upgrader"
	"github.com/dustin/go-humanize"
	"github.com/gin-gonic/gin"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"net/http"
)

func GetCurrentNode(c *gin.Context) {
	if _, ok := c.Get("NodeSecret"); !ok {
		c.JSON(http.StatusNotAcceptable, gin.H{
			"message": "node secret not exist",
		})
		return
	}

	runtimeInfo, err := upgrader.GetRuntimeInfo()
	if err != nil {
		ErrHandler(c, err)
		return
	}
	cpuInfo, _ := cpu.Info()
	memory, _ := analytic.GetMemoryStat()
	ver, _ := upgrader.GetCurrentVersion()
	diskUsage, _ := disk.Usage(".")

	nodeInfo := analytic.NodeInfo{
		NodeRuntimeInfo: runtimeInfo,
		CPUNum:          len(cpuInfo),
		MemoryTotal:     memory.Total,
		DiskTotal:       humanize.Bytes(diskUsage.Total),
		Version:         ver.Version,
	}

	stat := analytic.GetNodeStat()

	c.JSON(http.StatusOK, analytic.Node{
		NodeInfo: nodeInfo,
		NodeStat: stat,
	})
}
