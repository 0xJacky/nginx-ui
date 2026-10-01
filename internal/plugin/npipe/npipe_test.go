package npipe

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValid(t *testing.T) {
	for _, name := range []string{
		`\\.\pipe\nginx-ui-plugin-0123abcd`,
		`\\.\pipe\a.b_c-D`,
	} {
		assert.True(t, Valid(name), name)
	}
	for _, name := range []string{
		``,
		`\\.\pipe\`,
		`\\server\pipe\x`,
		`\\.\pipe\..\x`,
		`\\.\pipe\..`,
		`\\.\pipe\.x`,
		`\\.\pipe\-x`,
		`\\.\pipe\a\b`,
		`\\.\pipe\a b`,
		`\\.\pipe\a/b`,
		`//./pipe/x`,
		`\\?\pipe\x`,
		`\\.\PIPE\x`,
		`\\.\pipe\` + strings.Repeat("a", 250),
	} {
		assert.False(t, Valid(name), name)
	}
}
