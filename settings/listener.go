package settings

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	cosysettings "github.com/uozi-tech/cosy/settings"
)

// DefaultUnixSocketMode is applied to the Unix socket when SocketMode is
// empty. The reverse proxy usually runs as a different user than Nginx UI
// (for example the nginx worker user), and connect() on a Unix socket needs
// write permission, so the default is world-writable. Operators restrict
// access through the parent directory or by setting SocketMode.
const DefaultUnixSocketMode = "0666"

type Listener struct {
	UnixSocket string `json:"unix_socket"`
	SocketMode string `json:"socket_mode"`
}

var ListenerSettings = &Listener{}

// unixSocketPathAllowed reports whether r may appear in UnixSocket. The path
// is written unquoted into generated Nginx upstreams, so it is limited to a
// conservative set that needs no escaping in an nginx configuration token.
func unixSocketPathAllowed(r rune) bool {
	switch {
	case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		return true
	}
	return strings.ContainsRune("/._-+@~", r)
}

// Address selects a Unix socket only when explicitly configured.
func (l Listener) Address(server cosysettings.Server) (network, address string, err error) {
	if l.UnixSocket == "" {
		return "tcp", fmt.Sprintf("%s:%d", server.Host, server.Port), nil
	}
	if runtime.GOOS == "windows" {
		return "", "", fmt.Errorf("listener UnixSocket is not supported on Windows")
	}
	if !filepath.IsAbs(l.UnixSocket) {
		return "", "", fmt.Errorf("listener UnixSocket must be an absolute path")
	}
	for _, r := range l.UnixSocket {
		if !unixSocketPathAllowed(r) {
			return "", "", fmt.Errorf("listener UnixSocket may only contain letters, digits and / . _ - + @ ~")
		}
	}
	if _, err := l.Mode(); err != nil {
		return "", "", err
	}
	if server.EnableH3 {
		return "", "", fmt.Errorf("listener UnixSocket cannot be used with server EnableH3")
	}
	return "unix", l.UnixSocket, nil
}

// Mode parses SocketMode as an octal permission set for the Unix socket.
func (l Listener) Mode() (os.FileMode, error) {
	raw := strings.TrimSpace(l.SocketMode)
	if raw == "" {
		raw = DefaultUnixSocketMode
	}
	mode, err := strconv.ParseUint(strings.TrimPrefix(raw, "0o"), 8, 32)
	if err != nil || mode > 0o777 {
		return 0, fmt.Errorf("listener SocketMode must be an octal permission such as 0660")
	}
	return os.FileMode(mode), nil
}

// Upstream returns the proxy_pass target that reaches this Nginx UI instance
// from a local Nginx, for example "http://127.0.0.1:9000" or
// "https://unix:/run/nginx-ui/nginx-ui.sock:". The result is safe to embed
// unquoted because Address restricts the socket path characters.
func (l Listener) Upstream(scheme string, port uint) string {
	if l.UnixSocket != "" {
		return scheme + "://unix:" + l.UnixSocket + ":"
	}
	return fmt.Sprintf("%s://127.0.0.1:%d", scheme, port)
}

// LocalUpstream is Upstream with the scheme derived from the server settings.
func (l Listener) LocalUpstream(server cosysettings.Server) string {
	scheme := "http"
	if server.EnableHTTPS {
		scheme = "https"
	}
	return l.Upstream(scheme, server.Port)
}
