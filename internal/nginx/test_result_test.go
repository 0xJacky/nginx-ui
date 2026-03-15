package nginx

import (
	"errors"
	"testing"
)

func TestDetectErrorCategory(t *testing.T) {
	tests := []struct {
		name    string
		message string
		want    ErrorCategory
	}{
		{
			name:    "missing include",
			message: `open() "/tmp/nginx-ui-sandbox/sites-available/fastcgi.conf" failed (2: No such file or directory)`,
			want:    ErrorCategoryMissingInclude,
		},
		{
			name:    "syntax error",
			message: `nginx: [emerg] unknown directive "servername" in /etc/nginx/nginx.conf:5`,
			want:    ErrorCategorySyntaxError,
		},
		{
			name:    "runtime error",
			message: `nginx: [emerg] bind() to 0.0.0.0:80 failed (98: Address already in use)`,
			want:    ErrorCategoryNginxRuntimeError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectErrorCategory(tt.message, errors.New("exit status 1"))
			if got != tt.want {
				t.Fatalf("DetectErrorCategory() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNewSandboxBuildFailureResultPreservesCategory(t *testing.T) {
	result := NewSandboxBuildFailureResult(&SandboxBuildError{
		Category: ErrorCategoryMissingInclude,
		Message:  "sandbox include not found: fastcgi.conf",
	})

	if result.SandboxStatus != SandboxStatusFailed {
		t.Fatalf("SandboxStatus = %q, want %q", result.SandboxStatus, SandboxStatusFailed)
	}
	if result.ErrorCategory != ErrorCategoryMissingInclude {
		t.Fatalf("ErrorCategory = %q, want %q", result.ErrorCategory, ErrorCategoryMissingInclude)
	}
}
