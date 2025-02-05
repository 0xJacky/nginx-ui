package helper

import (
	"errors"
	"github.com/gorilla/websocket"
	"strings"
	"syscall"
)

// IsUnexpectedWebsocketError checks if the error is an unexpected websocket error
func IsUnexpectedWebsocketError(err error) bool {
	// ignore: write: broken pipe
	if errors.Is(err, syscall.EPIPE) {
		return false
	}
	// client closed error: *net.OpErr
	if strings.Contains(err.Error(), "An existing connection was forcibly closed by the remote host") {
		return false
	}

	return websocket.IsUnexpectedCloseError(err,
		websocket.CloseGoingAway,
		websocket.CloseNoStatusReceived,
		websocket.CloseNormalClosure)
}
