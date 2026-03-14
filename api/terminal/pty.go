package terminal

import (
	"github.com/0xJacky/Nginx-UI/internal/middleware"
	"github.com/0xJacky/Nginx-UI/internal/pty"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/uozi-tech/cosy/logger"
)

func Pty(c *gin.Context) {
	var upGrader = websocket.Upgrader{
		CheckOrigin: middleware.CheckWebSocketOrigin,
	}
	// upgrade http to websocket
	ws, err := upGrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		logger.Error(err)
		return
	}

	defer ws.Close()

	var p pty.Runner
	if settings.NodeSettings.Demo {
		p, err = pty.NewRestrictedPipeline(ws)
	} else {
		p, err = pty.NewPipeLine(ws)
	}

	if err != nil {
		logger.Error(err)
		return
	}

	defer p.Close()

	errorChan := make(chan error, 1)
	go p.ReadPtyAndWriteWs(errorChan)
	go p.ReadWsAndWritePty(errorChan)

	err = <-errorChan

	if err != nil {
		logger.Error(err)
	}
}
