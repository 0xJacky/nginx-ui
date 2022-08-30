package api

import (
	"encoding/json"
	"github.com/0xJacky/Nginx-UI/server/settings"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/hpcloud/tail"
	"github.com/pkg/errors"
	"io"
	"log"
	"net/http"
)

type controlStruct struct {
	Fetch string `json:"fetch"`
	Type  string `json:"type"`
}

func tailNginxLog(ws *websocket.Conn, controlChan chan controlStruct, errChan chan error) {
	defer func() {
		if err := recover(); err != nil {
			log.Println("tailNginxLog recovery", err)
			return
		}
	}()

	var control controlStruct

	for {
		var seek tail.SeekInfo
		if control.Fetch != "all" {
			seek.Offset = 0
			seek.Whence = io.SeekEnd
		}

		logPath := settings.NginxLogSettings.AccessLogPath

		if control.Type == "error" {
			logPath = settings.NginxLogSettings.ErrorLogPath
		}

		// Create a tail
		t, err := tail.TailFile(logPath, tail.Config{Follow: true,
			ReOpen: true, Location: &seek})

		if err != nil {
			errChan <- errors.Wrap(err, "error NginxAccessLog Tail")
			return
		}

		for {
			var next = false
			select {
			case line := <-t.Lines:
				// Print the text of each received line
				err = ws.WriteMessage(websocket.TextMessage, []byte(line.Text))

				if err != nil {
					errChan <- errors.Wrap(err, "error NginxAccessLog write message")
					return
				}
			case control = <-controlChan:
				log.Println("control change")
				next = true
				break
			}
			if next {
				break
			}
		}
	}
}

func handleLogControl(ws *websocket.Conn, controlChan chan controlStruct, errChan chan error) {
	defer func() {
		if err := recover(); err != nil {
			log.Println("tailNginxLog recovery", err)
			return
		}
	}()

	for {
		msgType, payload, err := ws.ReadMessage()
		if err != nil {
			errChan <- errors.Wrap(err, "error NginxAccessLog read message")
			return
		}

		if msgType != websocket.TextMessage {
			errChan <- errors.New("error NginxAccessLog message type")
			return
		}

		var msg controlStruct
		err = json.Unmarshal(payload, &msg)
		if err != nil {
			errChan <- errors.Wrap(err, "Error ReadWsAndWritePty json.Unmarshal")
			return
		}
		controlChan <- msg
	}
}

func NginxLog(c *gin.Context) {
	var upGrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
	// upgrade http to websocket
	ws, err := upGrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Println("[Error] NginxAccessLog Upgrade", err)
		return
	}

	defer ws.Close()

	errChan := make(chan error, 1)
	controlChan := make(chan controlStruct, 1)

	go tailNginxLog(ws, controlChan, errChan)
	go handleLogControl(ws, controlChan, errChan)

	if err = <-errChan; err != nil {
		log.Println(err)
		return
	}
}
