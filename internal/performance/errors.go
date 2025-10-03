package performance

import "github.com/uozi-tech/cosy"

var (
	e                     = cosy.NewErrorScope("performance")
	ErrStubStatusDisabled = e.New(51000, "stub_status is not enabled")
	ErrStubStatusRequest  = e.New(51001, "failed to get stub status: {0}")
	ErrResponseRead       = e.New(51002, "failed to read response body: {0}")
	ErrTemplateParseError = e.New(51003, "failed to parse template: {0}")
	ErrTemplateExecError  = e.New(51004, "failed to execute template: {0}")
	ErrConfigParseError   = e.New(51005, "failed to parse nginx config: {0}")
	ErrConfigBuildError   = e.New(51006, "failed to build nginx config: {0}")
	ErrNginxConfPathEmpty = e.New(51007, "failed to get nginx.conf path")
)
