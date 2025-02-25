package stream

import "github.com/uozi-tech/cosy"

var (
	e                = cosy.NewErrorScope("stream")
	ErrStreamNotFound  = e.New(40401, "stream not found")
	ErrDstFileExists = e.New(50001, "destination file already exists")
	ErrStreamIsEnabled = e.New(50002, "stream is enabled")
)
