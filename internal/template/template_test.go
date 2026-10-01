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

func TestBuiltinBlockSource(t *testing.T) {
	info, body, err := BuiltinBlockSource("hsts.conf")
	require.NoError(t, err)
	require.NotEmpty(t, info.Name)
	require.NotEmpty(t, info.Variables)
	require.NotContains(t, body, HeaderStart)
	require.NotContains(t, body, HeaderEnd)
	require.Contains(t, body, "Strict-Transport-Security")

	for _, name := range []string{"../config/nginx.conf", "missing.conf", "hsts"} {
		_, _, err := BuiltinBlockSource(name)
		require.ErrorIs(t, err, ErrBuiltinNotFound, name)
	}
}

func TestTrimActionLines(t *testing.T) {
	content := "location / {\n    {{ if .keep }}\n    return 301 x$request_uri;\n    {{- else }}\n    return 301 x;\n    {{ end }}{{/* done */}}\n    add_header A {{ .a }};\n}\n"
	rendered, err := RenderText("t", content, map[string]Variable{"keep": {Value: false}, "a": {Value: "b"}})
	require.NoError(t, err)
	require.Equal(t, "location / {\n    return 301 x;\n    add_header A b;\n}\n", rendered)

	rendered, err = RenderText("t", "{{ if .keep }}\nkeep;\n{{ end }}\nlast;\n", map[string]Variable{"keep": {Value: true}})
	require.NoError(t, err)
	require.Equal(t, "keep;\nlast;\n", rendered)

	// An action inside a line, or one that prints a value, keeps the line.
	require.Equal(t, "gzip {{ if .g }}on{{ else }}off{{ end }};", TrimActionLines("gzip {{ if .g }}on{{ else }}off{{ end }};"))
	require.Equal(t, "server {\n    {{ .extra }}\n}", TrimActionLines("server {\n    {{ .extra }}\n}"))
}
