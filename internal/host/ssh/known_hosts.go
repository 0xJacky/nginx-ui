package ssh

import (
	"errors"
	"os"
	"path/filepath"
	"sync"

	"github.com/uozi-tech/cosy"
	gossh "golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

// KnownHosts wraps a known_hosts file with thread-safe append-on-trust semantics.
type KnownHosts struct {
	path string
	mu   sync.Mutex

	// callback is rebuilt every time the file changes.
	callback gossh.HostKeyCallback
}

// NewKnownHosts opens (or creates) a known_hosts file at path. Missing parents
// are created. Returns an initialized KnownHosts with a callback that mirrors
// the current file contents.
func NewKnownHosts(path string) (*KnownHosts, error) {
	if path == "" {
		return nil, errors.New("known_hosts path is empty")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, cosy.WrapErrorWithParams(ErrKnownHostsWrite, err.Error())
	}
	// Touch the file if it doesn't exist so knownhosts.New can parse it.
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		if f, ferr := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0o600); ferr != nil {
			return nil, cosy.WrapErrorWithParams(ErrKnownHostsWrite, ferr.Error())
		} else {
			f.Close()
		}
	}
	kh := &KnownHosts{path: path}
	if err := kh.reload(); err != nil {
		return nil, err
	}
	return kh, nil
}

// HostKeyCallback returns a callback usable in gossh.ClientConfig. The callback
// uses the current snapshot at construction time, so reload after Trust calls.
func (k *KnownHosts) HostKeyCallback() gossh.HostKeyCallback {
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.callback
}

// IsTrusted reports whether the given key is recognized for hostPort
// (e.g. "example.com:22").
func (k *KnownHosts) IsTrusted(hostPort string, key gossh.PublicKey) bool {
	k.mu.Lock()
	cb := k.callback
	k.mu.Unlock()
	if cb == nil {
		return false
	}
	err := cb(hostPort, &fakeAddr{hostPort}, key)
	return err == nil
}

// Trust appends an entry for hostPort -> key to the known_hosts file
// and reloads the callback.
func (k *KnownHosts) Trust(hostPort string, key gossh.PublicKey) error {
	k.mu.Lock()
	defer k.mu.Unlock()

	line := knownhosts.Line([]string{hostPort}, key) + "\n"
	f, err := os.OpenFile(k.path, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o600)
	if err != nil {
		return cosy.WrapErrorWithParams(ErrKnownHostsWrite, err.Error())
	}
	if _, err := f.WriteString(line); err != nil {
		f.Close()
		return cosy.WrapErrorWithParams(ErrKnownHostsWrite, err.Error())
	}
	if err := f.Close(); err != nil {
		return cosy.WrapErrorWithParams(ErrKnownHostsWrite, err.Error())
	}
	return k.reloadLocked()
}

func (k *KnownHosts) reload() error {
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.reloadLocked()
}

func (k *KnownHosts) reloadLocked() error {
	cb, err := knownhosts.New(k.path)
	if err != nil {
		return cosy.WrapErrorWithParams(ErrKnownHostsRead, err.Error())
	}
	k.callback = cb
	return nil
}

// fakeAddr satisfies net.Addr for the HostKeyCallback signature.
type fakeAddr struct{ s string }

func (a *fakeAddr) Network() string { return "tcp" }
func (a *fakeAddr) String() string  { return a.s }
