package ssh

import (
	"bufio"
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
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

type HostKeyEntry struct {
	Host        string `json:"host"`
	Algorithm   string `json:"algorithm"`
	PublicKey   string `json:"public_key"`
	Fingerprint string `json:"fingerprint"`
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

func (k *KnownHosts) List(hostPort string) ([]HostKeyEntry, error) {
	k.mu.Lock()
	defer k.mu.Unlock()

	lines, err := k.readLinesLocked()
	if err != nil {
		return nil, err
	}
	entries := make([]HostKeyEntry, 0)
	for _, line := range lines {
		entry, ok, err := parseKnownHostsLine(line, hostPort)
		if err != nil {
			return nil, cosy.WrapErrorWithParams(ErrKnownHostsRead, err.Error())
		}
		if ok {
			entries = append(entries, entry)
		}
	}
	return entries, nil
}

func (k *KnownHosts) Replace(hostPort string, oldFingerprint string, key gossh.PublicKey) error {
	k.mu.Lock()
	defer k.mu.Unlock()

	newLine := knownhosts.Line([]string{hostPort}, key)
	newAlgorithm := key.Type()
	lines, err := k.readLinesLocked()
	if err != nil {
		return err
	}

	replaced := false
	for i, line := range lines {
		entry, ok, err := parseKnownHostsLine(line, hostPort)
		if err != nil {
			return cosy.WrapErrorWithParams(ErrKnownHostsRead, err.Error())
		}
		if ok && entry.Algorithm == newAlgorithm && entry.Fingerprint == oldFingerprint {
			lines[i] = newLine
			replaced = true
		}
	}
	if !replaced {
		return cosy.WrapErrorWithParams(ErrKnownHostsEntryNotFound, oldFingerprint)
	}
	return k.writeLinesAndReloadLocked(lines)
}

func (k *KnownHosts) Delete(hostPort string, algorithm string, fingerprint string) error {
	k.mu.Lock()
	defer k.mu.Unlock()

	lines, err := k.readLinesLocked()
	if err != nil {
		return err
	}
	kept := make([]string, 0, len(lines))
	deleted := false
	for _, line := range lines {
		entry, ok, err := parseKnownHostsLine(line, hostPort)
		if err != nil {
			return cosy.WrapErrorWithParams(ErrKnownHostsRead, err.Error())
		}
		if ok && entry.Algorithm == algorithm && entry.Fingerprint == fingerprint {
			deleted = true
			continue
		}
		kept = append(kept, line)
	}
	if !deleted {
		return cosy.WrapErrorWithParams(ErrKnownHostsEntryNotFound, fingerprint)
	}
	return k.writeLinesAndReloadLocked(kept)
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

func (k *KnownHosts) readLinesLocked() ([]string, error) {
	f, err := os.Open(k.path)
	if err != nil {
		return nil, cosy.WrapErrorWithParams(ErrKnownHostsRead, err.Error())
	}
	defer f.Close()

	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		lines = append(lines, line)
	}
	if err := scanner.Err(); err != nil {
		return nil, cosy.WrapErrorWithParams(ErrKnownHostsRead, err.Error())
	}
	return lines, nil
}

func (k *KnownHosts) writeLinesAndReloadLocked(lines []string) error {
	var buf bytes.Buffer
	for _, line := range lines {
		buf.WriteString(line)
		buf.WriteByte('\n')
	}
	tmp := k.path + ".tmp"
	if err := os.WriteFile(tmp, buf.Bytes(), 0o600); err != nil {
		return cosy.WrapErrorWithParams(ErrKnownHostsWrite, err.Error())
	}
	if err := os.Remove(k.path); err != nil && !errors.Is(err, os.ErrNotExist) {
		_ = os.Remove(tmp)
		return cosy.WrapErrorWithParams(ErrKnownHostsWrite, err.Error())
	}
	if err := os.Rename(tmp, k.path); err != nil {
		_ = os.Remove(tmp)
		return cosy.WrapErrorWithParams(ErrKnownHostsWrite, err.Error())
	}
	return k.reloadLocked()
}

func parseKnownHostsLine(line string, hostPort string) (HostKeyEntry, bool, error) {
	parts := strings.Fields(line)
	if len(parts) == 0 || strings.HasPrefix(parts[0], "#") {
		return HostKeyEntry{}, false, nil
	}
	if strings.HasPrefix(parts[0], "@") {
		parts = parts[1:]
	}
	if len(parts) < 2 {
		return HostKeyEntry{}, false, errors.New("invalid known_hosts line")
	}
	hosts := strings.Split(parts[0], ",")
	matched := false
	for _, host := range hosts {
		if knownHostsHostMatches(host, hostPort) {
			matched = true
			break
		}
	}
	if !matched {
		return HostKeyEntry{}, false, nil
	}
	key, _, _, _, err := gossh.ParseAuthorizedKey([]byte(strings.Join(parts[1:], " ")))
	if err != nil {
		return HostKeyEntry{}, false, err
	}
	return HostKeyEntry{
		Host:        hostPort,
		Algorithm:   key.Type(),
		PublicKey:   strings.TrimSpace(string(gossh.MarshalAuthorizedKey(key))),
		Fingerprint: gossh.FingerprintSHA256(key),
	}, true, nil
}

func knownHostsHostMatches(entryHost string, hostPort string) bool {
	if entryHost == hostPort {
		return true
	}
	host, port, ok := strings.Cut(hostPort, ":")
	if !ok || host == "" || port == "" {
		return false
	}
	if port == "22" && entryHost == host {
		return true
	}
	return entryHost == "["+host+"]:"+port
}

// fakeAddr satisfies net.Addr for the HostKeyCallback signature.
type fakeAddr struct{ s string }

func (a *fakeAddr) Network() string { return "tcp" }
func (a *fakeAddr) String() string  { return a.s }
