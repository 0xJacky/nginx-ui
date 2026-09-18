package settings

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
	cosysettings "github.com/uozi-tech/cosy/settings"
	"gopkg.in/ini.v1"
)

func TestListenerAddress(t *testing.T) {
	server := cosysettings.Server{Host: "0.0.0.0", Port: 9000}
	network, address, err := (Listener{}).Address(server)
	require.NoError(t, err)
	require.Equal(t, "tcp", network)
	require.Equal(t, "0.0.0.0:9000", address)

	server.Host, server.Port = "127.0.0.1", 8080
	network, address, err = (Listener{}).Address(server)
	require.NoError(t, err)
	require.Equal(t, "tcp", network)
	require.Equal(t, "127.0.0.1:8080", address)

	listener := Listener{UnixSocket: filepath.Join(t.TempDir(), "http.sock")}
	network, address, err = listener.Address(server)
	if runtime.GOOS == "windows" {
		require.ErrorContains(t, err, "not supported on Windows")
		return
	}
	require.NoError(t, err)
	require.Equal(t, "unix", network)
	require.Equal(t, listener.UnixSocket, address)

	_, _, err = (Listener{UnixSocket: "relative.sock"}).Address(server)
	require.ErrorContains(t, err, "absolute path")
	for _, path := range []string{"/tmp/a:b.sock", "/tmp/$host.sock", "/tmp/a\nb.sock", "/tmp/a\x00b.sock"} {
		_, _, err = (Listener{UnixSocket: path}).Address(server)
		require.Error(t, err)
	}
	server.EnableH3 = true
	_, _, err = listener.Address(server)
	require.ErrorContains(t, err, "EnableH3")
}

func TestListenerSettingsINIAndEnv(t *testing.T) {
	// Exercise the registered section and its environment override.
	previous := *ListenerSettings
	previousConf := cosysettings.Conf
	t.Cleanup(func() { *ListenerSettings = previous; cosysettings.Conf = previousConf })
	conf, err := ini.Load([]byte("[listener]\nUnixSocket = /tmp/from-ini.sock\n"))
	require.NoError(t, err)
	cosysettings.Conf = conf
	require.NoError(t, cosysettings.MapTo("listener", ListenerSettings))
	require.Equal(t, "/tmp/from-ini.sock", ListenerSettings.UnixSocket)
	ptr, ok := sections.Get("listener")
	require.True(t, ok)
	require.Same(t, ListenerSettings, ptr)
	require.Same(t, ListenerSettings, envPrefixMap["LISTENER"])

	t.Setenv("NGINX_UI_LISTENER_UNIX_SOCKET", "/tmp/from-env.sock")
	parseEnv(ListenerSettings, "LISTENER_")
	require.Equal(t, "/tmp/from-env.sock", ListenerSettings.UnixSocket)
	// A fresh process with an empty setting uses TCP again.
	*ListenerSettings = Listener{}
	conf.Section("listener").Key("UnixSocket").SetValue("")
	require.NoError(t, cosysettings.MapTo("listener", ListenerSettings))
	require.Empty(t, ListenerSettings.UnixSocket)
}
