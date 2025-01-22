package helper

import (
	"strings"
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
	// client closed error: *net.OpErr
	if strings.Contains(err.Error(), "An existing connection was forcibly closed by the remote host") {
		return true
	}

	return websocket.IsUnexpectedCloseError(err,
		websocket.CloseGoingAway,
		websocket.CloseNoStatusReceived,
		websocket.CloseNormalClosure)
}
