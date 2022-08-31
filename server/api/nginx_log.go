package api

import (
	"encoding/json"
	"github.com/0xJacky/Nginx-UI/server/pkg/nginx"
	"github.com/0xJacky/Nginx-UI/server/settings"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/hpcloud/tail"
	"github.com/pkg/errors"
	"io"
	"log"
	"net/http"
	"path/filepath"
)

type controlStruct struct {
	Fetch        string `json:"fetch"`
	Type         string `json:"type"`
	ConfName     string `json:"conf_name"`
	ServerIdx    int    `json:"server_idx"`
	DirectiveIdx int    `json:"directive_idx"`
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
		var logPath string
		switch control.Type {
		case "site":
			path := filepath.Join(nginx.GetNginxConfPath("sites-available"), control.ConfName)
			config, err := nginx.ParseNgxConfig(path)
			if err != nil {
				errChan <- errors.Wrap(err, "error parsing ngx config")
				return
			}

			if control.ServerIdx >= len(config.Servers) {
				errChan <- errors.New("serverIdx out of range")
				return
			}
			if control.DirectiveIdx >= len(config.Servers[control.ServerIdx].Directives) {
				errChan <- errors.New("DirectiveIdx out of range")
				return
			}
			directive := config.Servers[control.ServerIdx].Directives[control.DirectiveIdx]

			switch directive.Directive {
			case "access_log", "error_log":
				// ok
			default:
				errChan <- errors.New("directive.Params neither access_log nor error_log")
				return
			}

			logPath = directive.Params

		case "error":
			logPath = settings.NginxLogSettings.ErrorLogPath
		default:
			logPath = settings.NginxLogSettings.AccessLogPath
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
		_ = ws.WriteMessage(websocket.TextMessage, []byte(err.Error()))
		return
	}
}
