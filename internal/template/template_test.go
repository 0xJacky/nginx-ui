package template

import (
	"strings"
	"testing"

	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/stretchr/testify/require"
	cosysettings "github.com/uozi-tech/cosy/settings"
)

func TestNginxUIListenerTemplate(t *testing.T) {
	previous := *settings.ListenerSettings
	previousPort := cosysettings.ServerSettings.Port
	t.Cleanup(func() { *settings.ListenerSettings = previous; cosysettings.ServerSettings.Port = previousPort })
	cosysettings.ServerSettings.Port = 9000
	for _, socket := range []string{"", "/tmp/nginx ui.sock"} {
		settings.ListenerSettings.UnixSocket = socket
		result, err := ParseTemplate("block", "nginx-ui.conf", nil)
		require.NoError(t, err)
		require.Len(t, result.Locations, 1)
		content := result.Locations[0].Content
		if socket == "" {
			require.Contains(t, content, "proxy_pass http://127.0.0.1:9000/;")
		} else {
			require.Contains(t, content, `proxy_pass "http://unix:/tmp/nginx ui.sock:/";`)
			require.False(t, strings.Contains(content, "127.0.0.1:9000"))
		}
	}
}
