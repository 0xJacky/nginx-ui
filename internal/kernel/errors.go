package kernel

import (
	"context"
	"errors"
	"net"
)

// IsUnknownServerListenError checks if the error is an unknown server listen error
func IsUnknownServerListenError(err error) bool {
	return !errors.Is(err, context.DeadlineExceeded) &&
		!errors.Is(err, context.Canceled) &&
		!errors.Is(err, net.ErrClosed)
}
