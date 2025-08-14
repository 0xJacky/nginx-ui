package stream

import "github.com/uozi-tech/cosy"

var (
	e                    = cosy.NewErrorScope("stream")
	ErrStreamNotFound    = e.New(40401, "stream not found")
	ErrDstFileExists     = e.New(50001, "destination file already exists")
	ErrStreamIsEnabled   = e.New(50002, "stream is enabled")
	ErrNginxTestFailed   = e.New(50003, "nginx test failed: {0}")
	ErrNginxReloadFailed = e.New(50004, "nginx reload failed: {0}")
	ErrReadDirFailed     = e.New(50005, "read dir failed: {0}")
)
