package mcp

import (
	"testing"

	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/stretchr/testify/require"
)

func TestRedactURLCredentials(t *testing.T) {
	placeholder := settings.RedactedSensitiveValue
	for _, tt := range []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", ""},
		{"plain URL", "https://example.com/health", "https://example.com/health"},
		{"password", "https://probe:secret@example.com/health", "https://probe:" + placeholder + "@example.com/health"},
		{"bare userinfo token", "https://secret@example.com/", "https://" + placeholder + "@example.com/"},
		{"query values", "https://example.com/health?token=secret&mode=full", "https://example.com/health?mode=" + placeholder + "&token=" + placeholder},
		{"unparseable", "https://exa mple.com/%zz?token=secret", placeholder},
	} {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, RedactURLCredentials(tt.in))
		})
	}
}
