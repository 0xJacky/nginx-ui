package upstream

import (
	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/internal/upstream"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/uozi-tech/cosy/logger"
	"net/http"
	"time"
)

func AvailabilityTest(c *gin.Context) {
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

	var body []string

	err = ws.ReadJSON(&body)

	if err != nil {
		logger.Error(err)
		return
	}

	for {
		err = ws.WriteJSON(upstream.AvailabilityTest(body))
		if helper.IsUnexpectedWebsocketError(err) {
			logger.Error(err)
			break
		}

		time.Sleep(10 * time.Second)
	}
}
