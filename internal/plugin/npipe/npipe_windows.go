//go:build windows

package npipe

import (
	"context"
	"net"

	"github.com/Microsoft/go-winio"
)

// Dial connects to a local named pipe. The server may not impersonate the
// caller.
func Dial(ctx context.Context, name string) (net.Conn, error) {
	if !Valid(name) {
		return nil, ErrInvalidName
	}
	return winio.DialPipeContext(ctx, name)
}
