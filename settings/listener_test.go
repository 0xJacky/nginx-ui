package settings

import (
	"os"
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
	// Characters that would need quoting or escaping in an nginx token, or
	// that Go's %q would turn into escapes nginx does not understand.
	for _, path := range []string{
		"/tmp/a:b.sock", "/tmp/$host.sock", "/tmp/a\nb.sock", "/tmp/a\x00b.sock",
		"/tmp/nginx ui.sock", "/tmp/a;b.sock", "/tmp/a\"b.sock", "/tmp/a\x7fb.sock", "/tmp/a{b}.sock", "/tmp/a\\b.sock",
	} {
		_, _, err = (Listener{UnixSocket: path}).Address(server)
		require.Error(t, err, path)
	}
	_, _, err = (Listener{UnixSocket: "/run/nginx-ui/n.ui_1+@~.sock"}).Address(server)
	require.NoError(t, err)

	_, _, err = (Listener{UnixSocket: listener.UnixSocket, SocketMode: "rw-rw----"}).Address(server)
	require.ErrorContains(t, err, "SocketMode")

	server.EnableH3 = true
	_, _, err = listener.Address(server)
	require.ErrorContains(t, err, "EnableH3")
}

func TestListenerMode(t *testing.T) {
	mode, err := (Listener{}).Mode()
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o666), mode)
	for raw, want := range map[string]os.FileMode{"0660": 0o660, "660": 0o660, "0o600": 0o600, " 0777 ": 0o777, "0": 0} {
		mode, err = (Listener{SocketMode: raw}).Mode()
		require.NoError(t, err, raw)
		require.Equal(t, want, mode, raw)
	}
	for _, raw := range []string{"1777", "0888", "abc", "0x1ff"} {
		_, err = (Listener{SocketMode: raw}).Mode()
		require.Error(t, err, raw)
	}
}

func TestListenerUpstream(t *testing.T) {
	require.Equal(t, "http://127.0.0.1:9000", (Listener{}).Upstream("http", 9000))
	require.Equal(t, "https://unix:/run/nginx-ui.sock:", (Listener{UnixSocket: "/run/nginx-ui.sock"}).Upstream("https", 9000))
	server := cosysettings.Server{Port: 9001}
	require.Equal(t, "http://127.0.0.1:9001", (Listener{}).LocalUpstream(server))
	server.EnableHTTPS = true
	require.Equal(t, "https://127.0.0.1:9001", (Listener{}).LocalUpstream(server))
	require.Equal(t, "https://unix:/run/nginx-ui.sock:", (Listener{UnixSocket: "/run/nginx-ui.sock"}).LocalUpstream(server))
}

func TestListenerSettingsINIAndEnv(t *testing.T) {
	// Exercise the registered section and its environment override.
	previous := *ListenerSettings
	previousConf := cosysettings.Conf
	t.Cleanup(func() { *ListenerSettings = previous; cosysettings.Conf = previousConf })
	conf, err := ini.Load([]byte("[listener]\nUnixSocket = /tmp/from-ini.sock\nSocketMode = 0660\n"))
	require.NoError(t, err)
	cosysettings.Conf = conf
	require.NoError(t, cosysettings.MapTo("listener", ListenerSettings))
	require.Equal(t, "/tmp/from-ini.sock", ListenerSettings.UnixSocket)
	require.Equal(t, "0660", ListenerSettings.SocketMode)
	ptr, ok := sections.Get("listener")
	require.True(t, ok)
	require.Same(t, ListenerSettings, ptr)
	require.Same(t, ListenerSettings, envPrefixMap["LISTENER"])

	t.Setenv("NGINX_UI_LISTENER_UNIX_SOCKET", "/tmp/from-env.sock")
	t.Setenv("NGINX_UI_LISTENER_SOCKET_MODE", "0600")
	parseEnv(ListenerSettings, "LISTENER_")
	require.Equal(t, "/tmp/from-env.sock", ListenerSettings.UnixSocket)
	require.Equal(t, "0600", ListenerSettings.SocketMode)
	// A fresh process with an empty setting uses TCP again.
	*ListenerSettings = Listener{}
	conf.Section("listener").Key("UnixSocket").SetValue("")
	require.NoError(t, cosysettings.MapTo("listener", ListenerSettings))
	require.Empty(t, ListenerSettings.UnixSocket)
}
