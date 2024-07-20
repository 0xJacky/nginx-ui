package helper

import (
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"syscall"
	"testing"
)

func TestIsUnexpectedWebsocketError(t *testing.T) {
	var tests = []struct {
		input  error
		output bool
	}{
		{nil, false},
		{input: &websocket.CloseError{
			Code: websocket.CloseGoingAway,
		}, output: false},
		{input: &websocket.CloseError{
			Code: websocket.CloseNoStatusReceived,
		}, output: false},
		{input: &websocket.CloseError{
			Code: websocket.CloseNormalClosure,
		}, output: false},
		{input: &websocket.CloseError{
			Code: websocket.CloseInternalServerErr,
		}, output: true},
		{
			input:  syscall.EPIPE,
			output: false,
		},
	}
	for _, test := range tests {
		if !assert.Equal(t, test.output, IsUnexpectedWebsocketError(test.input)) {
			t.Log(test.input)
		}
	}
}
