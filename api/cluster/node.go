package cluster

import (
	"net/http"

	"github.com/0xJacky/Nginx-UI/internal/analytic"
	"github.com/0xJacky/Nginx-UI/internal/version"
	"github.com/gin-gonic/gin"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/uozi-tech/cosy"
)

func GetCurrentNode(c *gin.Context) {
	if _, ok := c.Get("Secret"); !ok {
		c.JSON(http.StatusNotAcceptable, gin.H{
			"message": "node secret not exist",
		})
		return
	}

	runtimeInfo, err := version.GetRuntimeInfo()
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}
	cpuInfo, _ := cpu.Info()
	memory, _ := analytic.GetMemoryStat()
	ver := version.GetVersionInfo()
	diskUsage, _ := analytic.GetDiskStat()

	nodeInfo := analytic.NodeInfo{
		NodeRuntimeInfo: runtimeInfo,
		CPUNum:          len(cpuInfo),
		MemoryTotal:     memory.Total,
		DiskTotal:       diskUsage.Total,
		Version:         ver.Version,
	}

	stat := analytic.GetNodeStat()

	c.JSON(http.StatusOK, analytic.Node{
		NodeInfo: nodeInfo,
		NodeStat: stat,
	})
}
