package settings

import "github.com/uozi-tech/cosy"

var (
	e                         = cosy.NewErrorScope("settings")
	ErrHTTP2RequiresHTTPS     = e.New(40001, "HTTP/2 requires HTTPS to be enabled")
	ErrHTTP3RequiresHTTPS     = e.New(40002, "HTTP/3 requires HTTPS to be enabled")
	ErrHTTP3OnUnixSocket      = e.New(40003, "HTTP/3 cannot be enabled while Nginx UI listens on a Unix socket")
	ErrUnknownSettingsSection = e.New(40004, "Unknown settings section: {0}")
)
