package serverstate

import "github.com/uozi-tech/cosy"

var (
	e                      = cosy.NewErrorScope("upstream")
	ErrInvalidServerToggle = e.New(40016, "an upstream name, a config file and a server address are required")
	ErrUpstreamNotInConfig = e.New(40402, "upstream {0} is not defined in {1}")
	ErrServerNotFound      = e.New(40403, "server {0} was not found in upstream {1}")
)
