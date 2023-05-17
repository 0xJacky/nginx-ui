package api

import (
	"github.com/0xJacky/Nginx-UI/server/service"
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

	runtimeInfo, err := service.GetRuntimeInfo()
	if err != nil {
		ErrHandler(c, err)
		return
	}

	cpuInfo, _ := cpu.Info()
	memory, _ := getMemoryStat()
	ver, _ := service.GetCurrentVersion()
	diskUsage, _ := disk.Usage(".")

	c.JSON(http.StatusOK, gin.H{
		"request_node_secret": c.MustGet("NodeSecret"),
		"node_runtime_info":   runtimeInfo,
		"cpu_num":             len(cpuInfo),
		"memory_total":        memory.Total,
		"disk_total":          humanize.Bytes(diskUsage.Total),
		"version":             ver.Version,
	})
}
