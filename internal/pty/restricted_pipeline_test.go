package pty

import (
	"strings"
	"testing"
)

func TestNormalizeRestrictedCommand(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr error
	}{
		{
			name:  "normalize spaces",
			input: "  ls   -la  ",
			want:  "ls -la",
		},
		{
			name:    "reject shell separator",
			input:   "ls; id",
			wantErr: errRestrictedCommandInvalid,
		},
		{
			name:    "reject path traversal",
			input:   "cat ../etc/passwd",
			wantErr: errRestrictedCommandInvalid,
		},
		{
			name:    "reject long command",
			input:   strings.Repeat("a", 129),
			wantErr: errRestrictedCommandTooLong,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeRestrictedCommand(tt.input)
			if tt.wantErr != nil {
				if err != tt.wantErr {
					t.Fatalf("expected error %v, got %v", tt.wantErr, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, got)
			}
		})
	}
}

func TestExecuteRestrictedCommand(t *testing.T) {
	output, err := executeRestrictedCommand("pwd")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if output != "/\r\n" {
		t.Fatalf("expected pwd output %q, got %q", "/\r\n", output)
	}

	output, err = executeRestrictedCommand("help")
	if err != nil {
		t.Fatalf("unexpected help error: %v", err)
	}
	if !strings.Contains(output, "Allowed commands:") {
		t.Fatalf("expected help output to contain allowed commands list")
	}

	_, err = executeRestrictedCommand("rm -rf /")
	if err != errRestrictedCommandNotAllowed {
		t.Fatalf("expected not allowed error, got %v", err)
	}
}
