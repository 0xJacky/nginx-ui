package cert

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/translation"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/gorilla/websocket"
	"github.com/uozi-tech/cosy/logger"
)

type Logger struct {
	buffer []string
	cert   *model.Cert
	ws     *websocket.Conn
	trans  *translation.Container
	mu     sync.Mutex
	msgCh  chan []byte
	done   chan struct{}
}

func NewLogger() *Logger {
	l := &Logger{
		msgCh: make(chan []byte, 100),
		done:  make(chan struct{}),
	}
	go l.processMessages()
	return l
}

func (t *Logger) processMessages() {
	for {
		select {
		case msg := <-t.msgCh:
			t.mu.Lock()
			if t.ws != nil {
				_ = t.ws.WriteMessage(websocket.TextMessage, msg)
			}
			t.mu.Unlock()
		case <-t.done:
			return
		}
	}
}

func (t *Logger) SetCertModel(cert *model.Cert) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.cert = cert
}

func (t *Logger) SetWebSocket(ws *websocket.Conn) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.ws = ws
}

func (t *Logger) Info(c *translation.Container) {
	result, err := c.ToJSON()
	if err != nil {
		return
	}

	t.mu.Lock()
	t.buffer = append(t.buffer, string(result))
	t.mu.Unlock()

	logger.Info("AutoCert", c.ToString())

	t.msgCh <- result
}

func (t *Logger) Error(err error) {
	t.mu.Lock()
	t.buffer = append(t.buffer, fmt.Sprintf("%s [Error] %s",
		time.Now().Format(time.DateTime),
		strings.TrimSpace(err.Error()),
	))
	t.mu.Unlock()

	logger.Error("AutoCert", err)
}

func (t *Logger) Close() {
	t.mu.Lock()
	defer t.mu.Unlock()

	close(t.msgCh)
	close(t.done)

	if t.cert == nil {
		return
	}

	_ = t.cert.Updates(&model.Cert{
		Log: t.ToString(),
	})
}

func (t *Logger) ToString() (content string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	content = strings.Join(t.buffer, "\n")
	return
}
