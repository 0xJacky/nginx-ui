package template

import (
	"testing"

	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/stretchr/testify/require"
	cosysettings "github.com/uozi-tech/cosy/settings"
)

func TestNginxUIListenerTemplate(t *testing.T) {
	previous := *settings.ListenerSettings
	previousServer := *cosysettings.ServerSettings
	t.Cleanup(func() { *settings.ListenerSettings = previous; *cosysettings.ServerSettings = previousServer })
	cosysettings.ServerSettings.Port = 9000

	cases := []struct {
		socket string
		https  bool
		want   string
	}{
		{"", false, "proxy_pass http://127.0.0.1:9000/;"},
		{"", true, "proxy_pass https://127.0.0.1:9000/;"},
		{"/run/nginx-ui/nginx-ui.sock", false, "proxy_pass http://unix:/run/nginx-ui/nginx-ui.sock:/;"},
		{"/run/nginx-ui/nginx-ui.sock", true, "proxy_pass https://unix:/run/nginx-ui/nginx-ui.sock:/;"},
	}
	for _, tc := range cases {
		settings.ListenerSettings.UnixSocket = tc.socket
		cosysettings.ServerSettings.EnableHTTPS = tc.https
		result, err := ParseTemplate("block", "nginx-ui.conf", nil)
		require.NoError(t, err)
		require.Len(t, result.Locations, 1)
		content := result.Locations[0].Content
		require.Contains(t, content, tc.want)
		if tc.socket != "" {
			require.NotContains(t, content, "127.0.0.1:9000")
		}
		require.NotContains(t, content, `"`)
	}
}
