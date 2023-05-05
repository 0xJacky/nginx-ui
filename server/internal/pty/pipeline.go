package pty

import (
	"encoding/json"
	"github.com/0xJacky/Nginx-UI/server/settings"
	"github.com/creack/pty"
	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
	"os"
	"os/exec"
	"time"
	"unicode/utf8"
)

type Pipeline struct {
	Pty *os.File
	ws  *websocket.Conn
}

type Message struct {
	Type MsgType
	Data json.RawMessage
}

const bufferSize = 2048

func NewPipeLine(conn *websocket.Conn) (p *Pipeline, err error) {
	c := exec.Command(settings.ServerSettings.StartCmd)

	ptmx, err := pty.StartWithSize(c, &pty.Winsize{Cols: 90, Rows: 60})
	if err != nil {
		return nil, errors.Wrap(err, "start pty error")
	}

	p = &Pipeline{
		Pty: ptmx,
		ws:  conn,
	}

	return
}

func (p *Pipeline) ReadWsAndWritePty(errorChan chan error) {
	for {
		msgType, payload, err := p.ws.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNoStatusReceived,
				websocket.CloseNormalClosure) {
				errorChan <- errors.Wrap(err, "Error ReadWsAndWritePty unexpected close")
				return
			}
			errorChan <- err
			return
		}
		if msgType != websocket.TextMessage {
			errorChan <- errors.Errorf("Error ReadWsAndWritePty Invalid msgType: %v", msgType)
			return
		}

		var msg Message
		err = json.Unmarshal(payload, &msg)
		if err != nil {
			errorChan <- errors.Wrap(err, "Error ReadWsAndWritePty json.Unmarshal")
			return
		}

		switch msg.Type {
		case TypeData:
			var data string
			err = json.Unmarshal(msg.Data, &data)
			if err != nil {
				errorChan <- errors.Wrap(err, "Error ReadWsAndWritePty json.Unmarshal msg.Data")
				return
			}

			_, err = p.Pty.Write([]byte(data))

			if err != nil {
				errorChan <- errors.Wrap(err, "Error ReadWsAndWritePty write pty")
				return
			}
		case TypeResize:
			var win struct {
				Cols uint16
				Rows uint16
			}

			err = json.Unmarshal(msg.Data, &win)
			if err != nil {
				errorChan <- errors.Wrap(err, "Error ReadSktAndWritePty Invalid resize message")
				return
			}
			err = pty.Setsize(p.Pty, &pty.Winsize{Rows: win.Rows, Cols: win.Cols})
			if err != nil {
				errorChan <- errors.Wrap(err, "Error ReadSktAndWritePty set pty size")
				return
			}
		case TypePing:
			err = p.ws.WriteControl(websocket.PongMessage, []byte{}, time.Now().Add(time.Second))
			if err != nil {
				errorChan <- errors.Wrap(err, "Error ReadSktAndWritePty write pong")
				return
			}
		default:
			errorChan <- errors.Errorf("Error ReadWsAndWritePty unknown msg.Type %v", msg.Type)
			return
		}
	}
}

func (p *Pipeline) ReadPtyAndWriteWs(errorChan chan error) {
	buf := make([]byte, bufferSize)
	for {
		n, err := p.Pty.Read(buf)
		if err != nil {
			errorChan <- errors.Wrap(err, "Error ReadPtyAndWriteWs read pty")
			return
		}
		processedOutput := validString(string(buf[:n]))
		err = p.ws.WriteMessage(websocket.TextMessage, []byte(processedOutput))
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseNormalClosure) {
				errorChan <- errors.Wrap(err, "Error ReadPtyAndWriteWs websocket write")
				return
			}
			errorChan <- err
			return
		}
	}
}

func validString(s string) string {
	if !utf8.ValidString(s) {
		v := make([]rune, 0, len(s))
		for i, r := range s {
			if r == utf8.RuneError {
				_, size := utf8.DecodeRuneInString(s[i:])
				if size == 1 {
					continue
				}
			}
			v = append(v, r)
		}
		s = string(v)
	}
	return s
}
