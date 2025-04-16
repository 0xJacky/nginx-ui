package llm

import (
	"github.com/uozi-tech/cosy"
)

var (
	e                           = cosy.NewErrorScope("llm")
	ErrCodeCompletionNotEnabled = e.New(400, "code completion is not enabled")
)
