package parser

import "github.com/uozi-tech/cosy"

var (
	e                       = cosy.NewErrorScope("nginx_log.parser")
	ErrEmptyLogLine         = e.New(50101, "empty log line")
	ErrLineTooLong          = e.New(50102, "log line exceeds maximum length")
	ErrUnsupportedLogFormat = e.New(50103, "unsupported log format")
	ErrInvalidTimestamp     = e.New(50104, "invalid timestamp format")
)
