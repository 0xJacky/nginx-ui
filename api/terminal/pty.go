package terminal

import (
	"github.com/0xJacky/Nginx-UI/internal/middleware"
	"github.com/0xJacky/Nginx-UI/internal/pty"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/uozi-tech/cosy"
	"github.com/uozi-tech/cosy/logger"
)

func Pty(c *gin.Context) {
	// Refuse before the upgrade. Once the connection is hijacked no HTTP status
	// can be written and the browser sees nothing but an opaque close. A demo
	// node exposes no PTY at all; its frontend renders a simulated shell that
	// never leaves the browser.
	if settings.NodeSettings.Demo {
		cosy.ErrHandler(c, middleware.ErrDisabledInDemo)
		return
	}

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

	p, err := pty.NewPipeLine(ws)
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
