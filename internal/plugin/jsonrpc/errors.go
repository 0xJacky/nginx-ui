package jsonrpc

import (
	"context"
	"errors"

	"github.com/0xJacky/Nginx-UI/internal/plugin/protocol"
)

var (
	// ErrClosed is returned by Call and Notify once the connection is gone and
	// fails every call that was still waiting for a response.
	ErrClosed = errors.New("jsonrpc: connection closed")
	// ErrMessageTooLarge is returned when a frame exceeds MaxMessageSize in
	// either direction. Incoming oversize frames also close the connection
	// because the stream can no longer be resynchronised.
	ErrMessageTooLarge = errors.New("jsonrpc: message exceeds the size limit")
)

// Caller is the outbound half of a Conn. The supervisor hands this interface
// to callers so they cannot accidentally reconfigure the connection.
type Caller interface {
	Call(ctx context.Context, method string, params any, result any) error
	Notify(ctx context.Context, method string, params any) error
}

// AsProtocolError extracts the JSON-RPC error object an error carries.
func AsProtocolError(err error) (*protocol.Error, bool) {
	var perr *protocol.Error
	if errors.As(err, &perr) {
		return perr, true
	}
	return nil, false
}

// IsMethodNotFound reports whether the peer does not know the method at all.
func IsMethodNotFound(err error) bool {
	perr, ok := AsProtocolError(err)
	return ok && perr.Code == protocol.CodeMethodNotFound
}

// IsUnsupported reports whether the peer knows the method but does not
// implement the capability behind it. Peers that simply never registered the
// method answer with CodeMethodNotFound instead, so capability probes usually
// want both helpers.
func IsUnsupported(err error) bool {
	perr, ok := AsProtocolError(err)
	return ok && perr.Code == protocol.CodeUnsupported
}

// Errorf builds a *protocol.Error a Handler can return to control the code and
// message sent back to the peer.
func Errorf(code int, message string) *protocol.Error {
	return &protocol.Error{Code: code, Message: message}
}
