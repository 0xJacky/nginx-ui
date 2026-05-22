package nginx_log

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestSplitCommaSeparated verifies that comma-joined filter values produced by
// the frontend multi-select inputs (browser/os/device) are split back into
// individual values so each can be matched independently.
func TestSplitCommaSeparated(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{"single value", "Chrome", []string{"Chrome"}},
		{"multiple values", "Chrome,Firefox", []string{"Chrome", "Firefox"}},
		{"values containing spaces", "Internet Explorer,Samsung Browser", []string{"Internet Explorer", "Samsung Browser"}},
		{"trims surrounding whitespace", " Chrome , Firefox ", []string{"Chrome", "Firefox"}},
		{"drops empty segments", "Chrome,,Firefox,", []string{"Chrome", "Firefox"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, splitCommaSeparated(tt.input))
		})
	}
}
