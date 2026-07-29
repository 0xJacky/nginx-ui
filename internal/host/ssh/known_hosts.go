package ssh

import (
	"bufio"
	"bytes"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"errors"
	"os"
	"path"
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
	Host string `json:"host"`
	// Marker carries @revoked so a refused key is never shown as trusted.
	Marker      string `json:"marker,omitempty"`
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

// HostKeyAlgorithms returns the SSH negotiation algorithms backed by keys
// trusted for hostPort. Restricting negotiation prevents a multi-key server
// from selecting an untrusted key type before HostKeyCallback runs.
func (k *KnownHosts) HostKeyAlgorithms(hostPort string) ([]string, error) {
	k.mu.Lock()
	defer k.mu.Unlock()

	lines, err := k.readLinesLocked()
	if err != nil {
		return nil, err
	}
	seen := make(map[string]bool)
	algorithms := make([]string, 0)
	for _, line := range lines {
		key, ok := parseKnownHostsPublicKey(line)
		if !ok || k.callback(hostPort, &fakeAddr{hostPort}, key) != nil {
			continue
		}
		for _, algorithm := range negotiationAlgorithmsForKey(key.Type()) {
			if !seen[algorithm] {
				seen[algorithm] = true
				algorithms = append(algorithms, algorithm)
			}
		}
	}
	return algorithms, nil
}

func negotiationAlgorithmsForKey(keyType string) []string {
	switch keyType {
	case gossh.KeyAlgoRSA:
		return []string{gossh.KeyAlgoRSASHA512, gossh.KeyAlgoRSASHA256, gossh.KeyAlgoRSA}
	case gossh.CertAlgoRSAv01:
		return []string{gossh.CertAlgoRSASHA512v01, gossh.CertAlgoRSASHA256v01, gossh.CertAlgoRSAv01}
	default:
		return []string{keyType}
	}
}

func parseKnownHostsPublicKey(line string) (gossh.PublicKey, bool) {
	parts := strings.Fields(line)
	if len(parts) == 0 || strings.HasPrefix(parts[0], "#") {
		return nil, false
	}
	if strings.HasPrefix(parts[0], "@") {
		// A CA key does not reveal the host certificate's key algorithm.
		return nil, false
	}
	if len(parts) < 3 {
		return nil, false
	}
	key, _, _, _, err := gossh.ParseAuthorizedKey([]byte(strings.Join(parts[1:], " ")))
	return key, err == nil
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
		if ok && entry.Marker == "" && entry.Algorithm == newAlgorithm && entry.Fingerprint == oldFingerprint {
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
		if ok && entry.Marker == "" && entry.Algorithm == algorithm && entry.Fingerprint == fingerprint {
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
	// A unique temp file plus a plain rename keeps a concurrent writer from
	// sharing the scratch path, and never leaves the file missing.
	tmp, err := os.CreateTemp(filepath.Dir(k.path), ".known_hosts-*")
	if err != nil {
		return cosy.WrapErrorWithParams(ErrKnownHostsWrite, err.Error())
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()

	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return cosy.WrapErrorWithParams(ErrKnownHostsWrite, err.Error())
	}
	if _, err := tmp.Write(buf.Bytes()); err != nil {
		_ = tmp.Close()
		return cosy.WrapErrorWithParams(ErrKnownHostsWrite, err.Error())
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return cosy.WrapErrorWithParams(ErrKnownHostsWrite, err.Error())
	}
	if err := tmp.Close(); err != nil {
		return cosy.WrapErrorWithParams(ErrKnownHostsWrite, err.Error())
	}
	if err := os.Rename(tmpName, k.path); err != nil {
		return cosy.WrapErrorWithParams(ErrKnownHostsWrite, err.Error())
	}
	return k.reloadLocked()
}

const (
	markerRevoked       = "@revoked"
	markerCertAuthority = "@cert-authority"
)

func parseKnownHostsLine(line string, hostPort string) (HostKeyEntry, bool, error) {
	parts := strings.Fields(line)
	if len(parts) == 0 || strings.HasPrefix(parts[0], "#") {
		return HostKeyEntry{}, false, nil
	}
	marker := ""
	if strings.HasPrefix(parts[0], "@") {
		marker = parts[0]
		parts = parts[1:]
	}
	if len(parts) < 2 {
		return HostKeyEntry{}, false, errors.New("invalid known_hosts line")
	}
	// A cert-authority line is a trust anchor rather than a host key, so it
	// never takes part in listing, replacing or deleting one.
	if marker == markerCertAuthority {
		return HostKeyEntry{}, false, nil
	}
	if !knownHostsLineMatches(parts[0], hostPort) {
		return HostKeyEntry{}, false, nil
	}
	key, _, _, _, err := gossh.ParseAuthorizedKey([]byte(strings.Join(parts[1:], " ")))
	if err != nil {
		return HostKeyEntry{}, false, err
	}
	return HostKeyEntry{
		Host:        hostPort,
		Marker:      marker,
		Algorithm:   key.Type(),
		PublicKey:   strings.TrimSpace(string(gossh.MarshalAuthorizedKey(key))),
		Fingerprint: gossh.FingerprintSHA256(key),
	}, true, nil
}

// knownHostsLineMatches applies OpenSSH host matching to the host field of a
// known_hosts line. A literal comparison alone misses hashed entries, which
// OpenSSH writes by default on Debian and Ubuntu, and would report a changed
// host key as an unknown host.
func knownHostsLineMatches(hostField string, hostPort string) bool {
	candidate := knownhosts.Normalize(hostPort)
	if strings.HasPrefix(hostField, "|") {
		return hashedHostMatches(hostField, candidate)
	}

	matched := false
	for _, pattern := range strings.Split(hostField, ",") {
		pattern = strings.TrimSpace(pattern)
		if pattern == "" {
			continue
		}
		// A negated pattern suppresses the whole line.
		if negated := strings.HasPrefix(pattern, "!"); negated {
			if hostPatternMatches(pattern[1:], candidate) {
				return false
			}
			continue
		}
		if hostPatternMatches(pattern, candidate) {
			matched = true
		}
	}
	return matched
}

func hostPatternMatches(pattern string, candidate string) bool {
	if !strings.ContainsAny(pattern, "*?") {
		return knownhosts.Normalize(pattern) == candidate
	}
	// A bracketed host:port form is literal here, so keep the glob matcher from
	// reading the brackets as a character class.
	escaped := strings.NewReplacer("[", "\\[", "]", "\\]").Replace(pattern)
	ok, err := path.Match(escaped, candidate)
	return err == nil && ok
}

// hashedHostMatches implements the |1|salt|hash form, where the hash is
// HMAC-SHA1 of the normalized host keyed by the salt.
func hashedHostMatches(hostField string, candidate string) bool {
	parts := strings.Split(hostField, "|")
	if len(parts) != 4 || parts[0] != "" || parts[1] != "1" {
		return false
	}
	salt, err := base64.StdEncoding.DecodeString(parts[2])
	if err != nil {
		return false
	}
	want, err := base64.StdEncoding.DecodeString(parts[3])
	if err != nil {
		return false
	}
	mac := hmac.New(sha1.New, salt)
	mac.Write([]byte(candidate))
	return hmac.Equal(mac.Sum(nil), want)
}

// fakeAddr satisfies net.Addr for the HostKeyCallback signature.
type fakeAddr struct{ s string }

func (a *fakeAddr) Network() string { return "tcp" }
func (a *fakeAddr) String() string  { return a.s }
