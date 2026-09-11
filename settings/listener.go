package settings

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"

	cosysettings "github.com/uozi-tech/cosy/settings"
)

type Listener struct {
	UnixSocket string `json:"unix_socket"`
}

var ListenerSettings = &Listener{}

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
	// These characters cannot be represented as a literal Unix upstream in
	// the built-in Nginx proxy configurations.
	if strings.ContainsAny(l.UnixSocket, ":$\x00\r\n") {
		return "", "", fmt.Errorf("listener UnixSocket cannot contain a colon, dollar sign, NUL or newline")
	}
	if server.EnableH3 {
		return "", "", fmt.Errorf("listener UnixSocket cannot be used with server EnableH3")
	}
	return "unix", l.UnixSocket, nil
}
