package nginx

import "github.com/uozi-tech/cosy"

var (
	e               = cosy.NewErrorScope("nginx")
	ErrNginx        = e.New(50000, "nginx error: {0}")
	ErrBlockIsNil   = e.New(50001, "block is nil")
	ErrReloadFailed = e.New(50002, "reload nginx failed: {0}")
)
