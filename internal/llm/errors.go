package llm

import (
	"github.com/uozi-tech/cosy"
)

var (
	e                           = cosy.NewErrorScope("llm")
	ErrCodeCompletionNotEnabled = e.New(400, "code completion is not enabled")
	ErrModelListUnauthorized    = e.New(40101, "The provider rejected the API token")
	ErrModelListUnsupported     = e.New(40102, "The endpoint does not provide a model list")
	ErrModelListTimeout         = e.New(40103, "Timed out connecting to the endpoint")
	ErrModelListUpstream        = e.New(40104, "The endpoint returned an error: {0}")
	ErrModelListUnreachable     = e.New(40105, "Unable to reach the endpoint: {0}")
	ErrModelListTokenRequired   = e.New(40106, "Enter the API token again to list models from a different endpoint")
)
