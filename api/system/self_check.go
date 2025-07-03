package system

import (
	"net/http"

	"github.com/gorilla/websocket"
	"github.com/uozi-tech/cosy/logger"

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
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
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
}

func TimeoutCheck(c *gin.Context) {
	time.Sleep(time.Minute)
	c.JSON(http.StatusOK, gin.H{
		"message": "ok",
	})
}
