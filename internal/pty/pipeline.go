package pty

import (
	"encoding/json"
	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
	"github.com/uozi-tech/cosy/logger"
	"io"
	"sync"
	"time"
	"unicode/utf8"
)

type Pipeline struct {
	Pty       terminal
	ws        *websocket.Conn
	closeOnce sync.Once
}

type terminal interface {
	io.ReadWriteCloser
	Resize(cols, rows uint16) error
}

type Message struct {
	Type MsgType
	Data json.RawMessage
}

const bufferSize = 2048

func NewPipeLine(conn *websocket.Conn) (p Runner, err error) {
	ptmx, err := startTerminal(settings.TerminalSettings.StartCmd)
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
	defer signalError(errorChan, nil)
	for {
		msgType, payload, err := p.ws.ReadMessage()
		if err != nil {
			if helper.IsUnexpectedWebsocketError(err) {
				signalError(errorChan, errors.Wrap(err, "Error ReadWsAndWritePty unexpected close"))
			}
			return
		}
		if msgType != websocket.TextMessage {
			signalError(errorChan, errors.Errorf("Error ReadWsAndWritePty Invalid msgType: %v", msgType))
			return
		}

		var msg Message
		err = json.Unmarshal(payload, &msg)
		if err != nil {
			signalError(errorChan, errors.Wrap(err, "Error ReadWsAndWritePty json.Unmarshal"))
			return
		}

		switch msg.Type {
		case TypeData:
			var data string
			err = json.Unmarshal(msg.Data, &data)
			if err != nil {
				signalError(errorChan, errors.Wrap(err, "Error ReadWsAndWritePty json.Unmarshal msg.Data"))
				return
			}

			_, err = p.Pty.Write([]byte(data))

			if err != nil {
				signalError(errorChan, errors.Wrap(err, "Error ReadWsAndWritePty write pty"))
				return
			}
		case TypeResize:
			var win struct {
				Cols uint16
				Rows uint16
			}

			err = json.Unmarshal(msg.Data, &win)
			if err != nil {
				signalError(errorChan, errors.Wrap(err, "Error ReadSktAndWritePty Invalid resize message"))
				return
			}
			err = p.Pty.Resize(win.Cols, win.Rows)
			if err != nil {
				signalError(errorChan, errors.Wrap(err, "Error ReadSktAndWritePty set pty size"))
				return
			}
		case TypePing:
			err = p.ws.WriteControl(websocket.PongMessage, []byte{}, time.Now().Add(time.Second))
			if err != nil {
				signalError(errorChan, errors.Wrap(err, "Error ReadSktAndWritePty write pong"))
				return
			}
		default:
			signalError(errorChan, errors.Errorf("Error ReadWsAndWritePty unknown msg.Type %v", msg.Type))
			return
		}
	}
}

func (p *Pipeline) ReadPtyAndWriteWs(errorChan chan error) {
	defer signalError(errorChan, nil)
	buf := make([]byte, bufferSize)
	for {
		n, err := p.Pty.Read(buf)
		if err != nil {
			signalError(errorChan, errors.Wrap(err, "Error ReadPtyAndWriteWs read pty"))
			return
		}
		processedOutput := validString(string(buf[:n]))
		err = p.ws.WriteMessage(websocket.TextMessage, []byte(processedOutput))
		if err != nil {
			if helper.IsUnexpectedWebsocketError(err) {
				signalError(errorChan, errors.Wrap(err, "Error ReadPtyAndWriteWs websocket write"))
			}
			return
		}
	}
}

func (p *Pipeline) Close() {
	p.closeOnce.Do(func() {
		// Unblock both pumps before releasing the terminal and its child process.
		_ = p.ws.Close()
		if err := p.Pty.Close(); err != nil {
			logger.Error(err)
		}
	})
}

// Both pumps may stop together; reporting must not strand the second goroutine.
func signalError(ch chan error, err error) {
	select {
	case ch <- err:
	default:
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
