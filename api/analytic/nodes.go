package analytic

import (
	"net/http"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/analytic"
	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/internal/kernel"
	"github.com/0xJacky/Nginx-UI/internal/version"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/uozi-tech/cosy/logger"
)

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

	// Counter to track iterations for periodic full info update
	counter := 0
	const fullInfoInterval = 6 // Send full info every 6 iterations (every minute if interval is 10s)

	for {
		var data interface{}

		// Every fullInfoInterval iterations, send complete node information including version
		if counter%fullInfoInterval == 0 {
			// Get complete node information including version
			runtimeInfo, err := version.GetRuntimeInfo()
			if err != nil {
				logger.Error("Failed to get runtime info:", err)
				// Fallback to stat only
				data = analytic.GetNodeStat()
			} else {
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

				// Send complete node information
				data = analytic.Node{
					NodeInfo: nodeInfo,
					NodeStat: stat,
				}

				logger.Debugf("Sending complete node info including version: %s", ver.Version)
			}
		} else {
			// Send only stat information for performance
			data = analytic.GetNodeStat()
		}

		// write
		err = ws.WriteJSON(data)
		if err != nil {
			if helper.IsUnexpectedWebsocketError(err) {
				logger.Error(err)
			}
			break
		}

		counter++

		select {
		case <-kernel.Context.Done():
			return
		case <-time.After(10 * time.Second):
		}
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
		err = ws.WriteJSON(analytic.NodeMap)
		if err != nil {
			if helper.IsUnexpectedWebsocketError(err) {
				logger.Error(err)
			}
			break
		}

		select {
		case <-kernel.Context.Done():
			return
		case <-time.After(10 * time.Second):
		}
	}
}
