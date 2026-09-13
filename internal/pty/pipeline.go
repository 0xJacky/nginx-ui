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
	errorChan <- p.readWsAndWritePty()
}

func (p *Pipeline) readWsAndWritePty() error {
	for {
		msgType, payload, err := p.ws.ReadMessage()
		if err != nil {
			if helper.IsUnexpectedWebsocketError(err) {
				return errors.Wrap(err, "Error ReadWsAndWritePty unexpected close")
			}
			return nil
		}
		if msgType != websocket.TextMessage {
			return errors.Errorf("Error ReadWsAndWritePty Invalid msgType: %v", msgType)
		}

		var msg Message
		err = json.Unmarshal(payload, &msg)
		if err != nil {
			return errors.Wrap(err, "Error ReadWsAndWritePty json.Unmarshal")
		}

		switch msg.Type {
		case TypeData:
			var data string
			err = json.Unmarshal(msg.Data, &data)
			if err != nil {
				return errors.Wrap(err, "Error ReadWsAndWritePty json.Unmarshal msg.Data")
			}

			_, err = p.Pty.Write([]byte(data))

			if err != nil {
				return errors.Wrap(err, "Error ReadWsAndWritePty write pty")
			}
		case TypeResize:
			var win struct {
				Cols uint16
				Rows uint16
			}

			err = json.Unmarshal(msg.Data, &win)
			if err != nil {
				return errors.Wrap(err, "Error ReadSktAndWritePty Invalid resize message")
			}
			err = p.Pty.Resize(win.Cols, win.Rows)
			if err != nil {
				return errors.Wrap(err, "Error ReadSktAndWritePty set pty size")
			}
		case TypePing:
			err = p.ws.WriteControl(websocket.PongMessage, []byte{}, time.Now().Add(time.Second))
			if err != nil {
				return errors.Wrap(err, "Error ReadSktAndWritePty write pong")
			}
		default:
			return errors.Errorf("Error ReadWsAndWritePty unknown msg.Type %v", msg.Type)
		}
	}
}

func (p *Pipeline) ReadPtyAndWriteWs(errorChan chan error) {
	errorChan <- p.readPtyAndWriteWs()
}

func (p *Pipeline) readPtyAndWriteWs() error {
	buf := make([]byte, bufferSize)
	pending := make([]byte, 0, bufferSize+utf8.UTFMax)
	for {
		n, readErr := p.Pty.Read(buf)
		pending = append(pending, buf[:n]...)
		complete := 0
		for complete < len(pending) && utf8.FullRune(pending[complete:]) {
			_, size := utf8.DecodeRune(pending[complete:])
			complete += size
		}
		// Keep an incomplete trailing rune for the next PTY read. Process
		// complete output even when Read returns both data and an error.
		processedOutput := validString(string(pending[:complete]))
		if len(processedOutput) > 0 {
			if err := p.ws.WriteMessage(websocket.TextMessage, []byte(processedOutput)); err != nil {
				if helper.IsUnexpectedWebsocketError(err) {
					return errors.Wrap(err, "Error ReadPtyAndWriteWs websocket write")
				}
				return nil
			}
		}
		pending = append(pending[:0], pending[complete:]...)
		if readErr == io.EOF {
			return nil
		}
		if readErr != nil {
			return errors.Wrap(readErr, "Error ReadPtyAndWriteWs read pty")
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
