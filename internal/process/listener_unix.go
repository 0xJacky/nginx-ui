package process

import (
	"errors"
	"fmt"
	"io/fs"
	"net"
	"os"
	"syscall"
	"time"
)

// PrepareUnixSocket makes room for a Unix listener at path. A socket file left
// behind by an unclean exit (SIGKILL, power loss) would otherwise make every
// later start fail with "address already in use". The file is removed only
// when nothing accepts connections on it; a live socket or a non-socket file
// is reported as an error so a running instance or an unrelated file is never
// clobbered.
func PrepareUnixSocket(path string) error {
	info, err := os.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.Mode().Type() != fs.ModeSocket {
		return fmt.Errorf("%s exists and is not a socket", path)
	}

	conn, err := net.DialTimeout("unix", path, time.Second)
	if err == nil {
		_ = conn.Close()
		return fmt.Errorf("%s is in use by another process", path)
	}
	if !errors.Is(err, syscall.ECONNREFUSED) && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("probe stale socket %s: %w", path, err)
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

// ApplyUnixSocketMode sets the permission bits of a listening Unix socket.
// The socket inherits the process umask when it is created, which usually
// hides it from the reverse proxy user, so the configured mode is applied
// right after the listener is bound.
func ApplyUnixSocketMode(path string, mode os.FileMode) error {
	return os.Chmod(path, mode.Perm())
}
