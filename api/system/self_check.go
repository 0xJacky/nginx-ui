package system

import (
	"github.com/0xJacky/Nginx-UI/internal/middleware"
	"github.com/gorilla/websocket"
	"github.com/uozi-tech/cosy/logger"
	"net/http"

	"time"

	"github.com/0xJacky/Nginx-UI/internal/self_check"
	"github.com/gin-gonic/gin"
)

func SelfCheck(c *gin.Context) {
	report := self_check.Run()
	c.JSON(http.StatusOK, report)
}

func SelfCheckFix(c *gin.Context) {
	result := self_check.AttemptFix(c.Param("name"))
	c.JSON(http.StatusOK, result)
}

func CheckWebSocket(c *gin.Context) {
	var upgrader = websocket.Upgrader{
		CheckOrigin: middleware.CheckWebSocketOrigin,
	}
	ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		logger.Error(err)
		return
	}
	defer ws.Close()
	err = ws.WriteJSON(gin.H{
		"message": "ok",
	})
	if err != nil {
		logger.Error(err)
		return
	}

	// Wait for the client to close after receiving the probe.
	// Closing immediately after writing creates a race: the connection may be
	// torn down before the browser has processed the open/message events.
	ws.SetReadDeadline(time.Now().Add(5 * time.Second))
	for {
		_, _, err := ws.ReadMessage()
		if err != nil {
			break
		}
	}
}

func TimeoutCheck(c *gin.Context) {
	time.Sleep(time.Minute)
	c.JSON(http.StatusOK, gin.H{
		"message": "ok",
	})
}
