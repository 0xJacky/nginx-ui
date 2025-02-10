package nginx

import "github.com/uozi-tech/cosy"

var (
	e             = cosy.NewErrorScope("nginx")
	ErrBlockIsNil = e.New(50001, "block is nil")
)
