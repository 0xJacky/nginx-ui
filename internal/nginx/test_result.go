package nginx

import (
	"errors"
	"fmt"
	"strings"
)

type TestScope string

const (
	TestScopeGlobal           TestScope = "global"
	TestScopeNamespaceSandbox TestScope = "namespace_sandbox"
)

type SandboxStatus string

const (
	SandboxStatusOK      SandboxStatus = "ok"
	SandboxStatusSkipped SandboxStatus = "skipped"
	SandboxStatusFailed  SandboxStatus = "failed"
)

type ErrorCategory string

const (
	ErrorCategoryNone              ErrorCategory = ""
	ErrorCategoryMissingInclude    ErrorCategory = "missing_include"
	ErrorCategorySandboxBuildError ErrorCategory = "sandbox_build_error"
	ErrorCategorySyntaxError       ErrorCategory = "syntax_error"
	ErrorCategoryNginxRuntimeError ErrorCategory = "nginx_runtime_error"
)

type TestConfigResult struct {
	Message       string        `json:"message"`
	Level         int           `json:"level"`
	TestScope     TestScope     `json:"test_scope"`
	SandboxStatus SandboxStatus `json:"sandbox_status,omitempty"`
	ErrorCategory ErrorCategory `json:"error_category,omitempty"`
}

type SandboxBuildError struct {
	Category ErrorCategory
	Message  string
}

func (e *SandboxBuildError) Error() string {
	return e.Message
}

func newSandboxIncludeError(baseDir string, includePath string) error {
	return &SandboxBuildError{
		Category: ErrorCategoryMissingInclude,
		Message:  fmt.Sprintf("sandbox include not found: %s (resolved from %s)", includePath, baseDir),
	}
}

func NewSandboxBuildFailureResult(err error) TestConfigResult {
	category := ErrorCategorySandboxBuildError
	var sandboxErr *SandboxBuildError
	if errors.As(err, &sandboxErr) && sandboxErr.Category != ErrorCategoryNone {
		category = sandboxErr.Category
	}

	return TestConfigResult{
		Message:       fmt.Sprintf("Sandbox test setup failed: %v", err),
		Level:         Error,
		TestScope:     TestScopeNamespaceSandbox,
		SandboxStatus: SandboxStatusFailed,
		ErrorCategory: category,
	}
}

func NewTestConfigResult(stdOut string, stdErr error, scope TestScope, sandboxStatus SandboxStatus) TestConfigResult {
	message := stdOut
	if stdErr != nil {
		message = strings.TrimSpace(strings.Join([]string{stdOut, stdErr.Error()}, " "))
	}

	level := GetLogLevel(message)
	if level == Unknown && stdErr != nil {
		level = Error
	}

	result := TestConfigResult{
		Message:       message,
		Level:         level,
		TestScope:     scope,
		SandboxStatus: sandboxStatus,
		ErrorCategory: DetectErrorCategory(message, stdErr),
	}

	if sandboxStatus == "" && scope == TestScopeNamespaceSandbox {
		result.SandboxStatus = SandboxStatusOK
	}

	return result
}

func DetectErrorCategory(message string, stdErr error) ErrorCategory {
	if stdErr == nil {
		return ErrorCategoryNone
	}

	lowerMessage := strings.ToLower(message)

	switch {
	case strings.Contains(lowerMessage, "failed (2: no such file or directory)") ||
		strings.Contains(lowerMessage, "open()") && strings.Contains(lowerMessage, "no such file or directory"):
		return ErrorCategoryMissingInclude
	case strings.Contains(lowerMessage, "unknown directive") ||
		strings.Contains(lowerMessage, "invalid number of arguments") ||
		strings.Contains(lowerMessage, "directive is not terminated by") ||
		strings.Contains(lowerMessage, "unexpected end of file"):
		return ErrorCategorySyntaxError
	default:
		return ErrorCategoryNginxRuntimeError
	}
}
