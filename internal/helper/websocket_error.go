package helper

import (
	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
	"syscall"
)

func IsUnexpectedWebsocketError(err error) bool {
	// nil error is an expected error
	if err == nil {
		return false
	}
	// ignore: write: broken pipe
	if errors.Is(err, syscall.EPIPE) {
		return false
	}
	return websocket.IsUnexpectedCloseError(err,
		websocket.CloseGoingAway,
		websocket.CloseNoStatusReceived,
		websocket.CloseNormalClosure)
}
