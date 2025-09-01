package llm

import (
	"net/http"
	"sync"
	"time"

	"github.com/0xJacky/Nginx-UI/api"
	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/internal/llm"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/uozi-tech/cosy"
	"github.com/uozi-tech/cosy/logger"
)

var mutex sync.Mutex

// CodeCompletion handles code completion requests
func CodeCompletion(c *gin.Context) {
	if !settings.OpenAISettings.EnableCodeCompletion {
		cosy.ErrHandler(c, llm.ErrCodeCompletionNotEnabled)
		return
	}

	var upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
	ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}
	defer ws.Close()

	for {
		var codeCompletionRequest llm.CodeCompletionRequest
		err := ws.ReadJSON(&codeCompletionRequest)
		if err != nil {
			if helper.IsUnexpectedWebsocketError(err) {
				logger.Errorf("Error reading JSON: %v", err)
			}
			return
		}

		codeCompletionRequest.UserID = api.CurrentUser(c).ID

		go func() {
			start := time.Now()
			completedCode, err := codeCompletionRequest.Send()
			if err != nil {
				logger.Errorf("Error sending code completion request: %v", err)
				return
			}
			elapsed := time.Since(start)

			mutex.Lock()
			defer mutex.Unlock()

			err = ws.WriteJSON(gin.H{
				"code":          completedCode,
				"request_id":    codeCompletionRequest.RequestID,
				"completion_ms": elapsed.Milliseconds(),
			})
			if err != nil {
				if helper.IsUnexpectedWebsocketError(err) {
					logger.Errorf("Error writing JSON: %v", err)
				}
				return
			}
		}()
	}
}

func GetCodeCompletionEnabledStatus(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"enabled": settings.OpenAISettings.EnableCodeCompletion,
	})
}
