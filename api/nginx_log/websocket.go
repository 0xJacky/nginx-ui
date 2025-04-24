package nginx_log

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"runtime"

	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/internal/nginx_log"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/nxadm/tail"
	"github.com/pkg/errors"
	"github.com/uozi-tech/cosy/logger"
)

// getLogPath resolves the log file path based on the provided control parameters
// It checks if the path is under the whitelist directories
func getLogPath(control *controlStruct) (logPath string, err error) {
	// If direct log path is provided, use it
	if control.LogPath != "" {
		logPath = control.LogPath
		// Check if logPath is under one of the paths in LogDirWhiteList
		if !nginx_log.IsLogPathUnderWhiteList(logPath) {
			return "", nginx_log.ErrLogPathIsNotUnderTheLogDirWhiteList
		}
		return
	}

	// Otherwise, use default log path based on type
	switch control.Type {
	case "error":
		path := nginx.GetErrorLogPath()

		if path == "" {
			err = nginx_log.ErrErrorLogPathIsEmpty
			return
		}

		logPath = path
	case "access":
		fallthrough
	default:
		path := nginx.GetAccessLogPath()

		if path == "" {
			err = nginx_log.ErrAccessLogPathIsEmpty
			return
		}

		logPath = path
	}

	// check if logPath is under one of the paths in LogDirWhiteList
	if !nginx_log.IsLogPathUnderWhiteList(logPath) {
		return "", nginx_log.ErrLogPathIsNotUnderTheLogDirWhiteList
	}
	return
}

// tailNginxLog tails the specified log file and sends each line to the websocket
func tailNginxLog(ws *websocket.Conn, controlChan chan controlStruct, errChan chan error) {
	defer func() {
		if err := recover(); err != nil {
			buf := make([]byte, 1024)
			runtime.Stack(buf, false)
			logger.Error(err)
			return
		}
	}()

	control := <-controlChan

	for {
		logPath, err := getLogPath(&control)

		if err != nil {
			errChan <- err
			return
		}

		seek := tail.SeekInfo{
			Offset: 0,
			Whence: io.SeekEnd,
		}

		stat, err := os.Stat(logPath)
		if os.IsNotExist(err) {
			errChan <- errors.New("[error] Log path does not exist: " + logPath)
			return
		}

		if !stat.Mode().IsRegular() {
			errChan <- errors.Errorf("[error] %s is not a regular file. If you are using nginx-ui in docker container, please refer to https://nginxui.com/zh_CN/guide/config-nginx-log.html for more information.", logPath)
			return
		}

		// Create a tail
		t, err := tail.TailFile(logPath, tail.Config{Follow: true,
			ReOpen: true, Location: &seek})
		if err != nil {
			errChan <- errors.Wrap(err, "error tailing log")
			return
		}

		for {
			var next = false
			select {
			case line := <-t.Lines:
				// Print the text of each received line
				if line == nil {
					continue
				}

				err = ws.WriteMessage(websocket.TextMessage, []byte(line.Text))
				if err != nil {
					if helper.IsUnexpectedWebsocketError(err) {
						errChan <- errors.Wrap(err, "error tailNginxLog write message")
					}
					return
				}
			case control = <-controlChan:
				next = true
				break
			}
			if next {
				break
			}
		}
	}
}

// handleLogControl processes websocket control messages
func handleLogControl(ws *websocket.Conn, controlChan chan controlStruct, errChan chan error) {
	defer func() {
		if err := recover(); err != nil {
			buf := make([]byte, 1024)
			runtime.Stack(buf, false)
			logger.Error(err)
			return
		}
	}()

	for {
		msgType, payload, err := ws.ReadMessage()
		if err != nil && websocket.IsUnexpectedCloseError(err, websocket.CloseNormalClosure) {
			errChan <- errors.Wrap(err, "error handleLogControl read message")
			return
		}

		if msgType != websocket.TextMessage {
			errChan <- errors.New("error handleLogControl message type")
			return
		}

		var msg controlStruct
		err = json.Unmarshal(payload, &msg)
		if err != nil {
			errChan <- errors.Wrap(err, "error ReadWsAndWritePty json.Unmarshal")
			return
		}
		controlChan <- msg
	}
}

// Log handles websocket connection for real-time log viewing
func Log(c *gin.Context) {
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

	errChan := make(chan error, 1)
	controlChan := make(chan controlStruct, 1)

	go tailNginxLog(ws, controlChan, errChan)
	go handleLogControl(ws, controlChan, errChan)

	if err = <-errChan; err != nil {
		logger.Error(err)
		_ = ws.WriteMessage(websocket.TextMessage, []byte(err.Error()))
		return
	}
}
