package analytic

import (
	"net/http"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/analytic"
	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
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

	for {
		// write
		err = ws.WriteJSON(analytic.GetNodeStat())
		if err != nil {
			if helper.IsUnexpectedWebsocketError(err) {
				logger.Error(err)
			}
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
		if err != nil {
			if helper.IsUnexpectedWebsocketError(err) {
				logger.Error(err)
			}
			break
		}

		time.Sleep(10 * time.Second)
	}
}
