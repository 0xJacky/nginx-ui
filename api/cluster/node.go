package cluster

import (
	"github.com/0xJacky/Nginx-UI/api"
	analytic2 "github.com/0xJacky/Nginx-UI/internal/analytic"
	"github.com/0xJacky/Nginx-UI/internal/upgrader"
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
		api.ErrHandler(c, err)
		return
	}
	cpuInfo, _ := cpu.Info()
	memory, _ := analytic2.GetMemoryStat()
	ver, _ := upgrader.GetCurrentVersion()
	diskUsage, _ := disk.Usage(".")

	nodeInfo := analytic2.NodeInfo{
		NodeRuntimeInfo: runtimeInfo,
		CPUNum:          len(cpuInfo),
		MemoryTotal:     memory.Total,
		DiskTotal:       humanize.Bytes(diskUsage.Total),
		Version:         ver.Version,
	}

	stat := analytic2.GetNodeStat()

	c.JSON(http.StatusOK, analytic2.Node{
		NodeInfo: nodeInfo,
		NodeStat: stat,
	})
}
