package analytic

import (
	"github.com/0xJacky/Nginx-UI/internal/analytic"
	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/uozi-tech/cosy/logger"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"net/http"
	"time"
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

	for {
		// write
		err = ws.WriteJSON(analytic.GetNodeStat())
		if helper.IsUnexpectedWebsocketError(err) {
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
		err = ws.WriteJSON(analytic.NodeMap)
		if helper.IsUnexpectedWebsocketError(err) {
			logger.Error(err)
			break
		}

		time.Sleep(10 * time.Second)
	}
}
