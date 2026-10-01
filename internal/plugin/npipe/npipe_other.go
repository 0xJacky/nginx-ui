//go:build !windows

package npipe

import (
	"context"
	"net"
)

// Dial reports ErrUnsupported: named pipes exist only on Windows.
func Dial(context.Context, string) (net.Conn, error) {
	return nil, ErrUnsupported
}
