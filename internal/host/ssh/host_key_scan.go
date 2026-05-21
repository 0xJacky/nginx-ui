package ssh

import (
	"bufio"
	"context"
	"net"
	"strings"
	"time"

	"github.com/uozi-tech/cosy"
	gossh "golang.org/x/crypto/ssh"
)

const (
	HostKeyStatusTrusted      = "trusted"
	HostKeyStatusUnknownHost  = "unknown_host"
	HostKeyStatusNewAlgorithm = "new_algorithm"
	HostKeyStatusChanged      = "changed"
	HostKeyStatusStale        = "stale"
)

type HostKeyScanItem struct {
	Algorithm           string `json:"algorithm"`
	PublicKey           string `json:"public_key"`
	Fingerprint         string `json:"fingerprint"`
	ExistingFingerprint string `json:"existing_fingerprint,omitempty"`
	Status              string `json:"status"`
}

type KnownHostsPersistence struct {
	Path        string `json:"path"`
	Recommended bool   `json:"recommended"`
	Warning     string `json:"warning,omitempty"`
}

type HostKeyScanResult struct {
	HostAddress    string                `json:"host_address"`
	KnownHostsPath string                `json:"known_hosts_path"`
	Keys           []HostKeyScanItem     `json:"keys"`
	StaleKeys      []HostKeyScanItem     `json:"stale_keys"`
	Persistence    KnownHostsPersistence `json:"persistence"`
}

var preferredHostKeyAlgorithms = []string{
	gossh.KeyAlgoED25519,
	gossh.KeyAlgoECDSA256,
	gossh.KeyAlgoECDSA384,
	gossh.KeyAlgoECDSA521,
	gossh.KeyAlgoRSASHA512,
	gossh.KeyAlgoRSASHA256,
	gossh.KeyAlgoRSA,
}

// ScanHostKeys reads host keys presented during SSH handshakes without trusting them.
func ScanHostKeys(ctx context.Context, hostPort string, timeout time.Duration) ([]gossh.PublicKey, error) {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}

	seenFingerprints := make(map[string]bool)
	keys := make([]gossh.PublicKey, 0, len(preferredHostKeyAlgorithms))
	var lastErr error
	for _, algorithm := range preferredHostKeyAlgorithms {
		key, err := scanHostKeyWithAlgorithm(ctx, hostPort, timeout, algorithm)
		if err != nil {
			lastErr = err
			continue
		}
		fingerprint := gossh.FingerprintSHA256(key)
		if seenFingerprints[fingerprint] {
			continue
		}
		seenFingerprints[fingerprint] = true
		keys = append(keys, key)
	}
	if len(keys) == 0 {
		if lastErr != nil {
			return nil, cosy.WrapErrorWithParams(ErrHostKeyScanFailed, lastErr.Error())
		}
		return nil, cosy.WrapErrorWithParams(ErrHostKeyScanFailed, "server did not present a host key")
	}
	return keys, nil
}

func scanHostKeyWithAlgorithm(ctx context.Context, hostPort string, timeout time.Duration, algorithm string) (gossh.PublicKey, error) {
	var scanned gossh.PublicKey
	config := &gossh.ClientConfig{
		User: "nginx-ui-host-key-scan",
		Auth: []gossh.AuthMethod{},
		HostKeyCallback: func(hostname string, remote net.Addr, key gossh.PublicKey) error {
			scanned = key
			return nil
		},
		HostKeyAlgorithms: []string{algorithm},
		Timeout:           timeout,
	}

	dialer := net.Dialer{Timeout: timeout}
	conn, err := dialer.DialContext(ctx, "tcp", hostPort)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	sshConn, _, _, err := gossh.NewClientConn(conn, hostPort, config)
	if sshConn != nil {
		_ = sshConn.Close()
	}
	if scanned != nil {
		return scanned, nil
	}
	if err != nil {
		return nil, err
	}
	return nil, cosy.WrapErrorWithParams(ErrHostKeyScanFailed, "server did not present a host key")
}

func ParseSSHKeyscanOutput(output string) ([]gossh.PublicKey, error) {
	var keys []gossh.PublicKey
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 3 {
			return nil, cosy.WrapErrorWithParams(ErrPublicKeyParse, line)
		}
		keyLine := strings.Join(parts[1:], " ")
		key, _, _, _, err := gossh.ParseAuthorizedKey([]byte(keyLine))
		if err != nil {
			return nil, cosy.WrapErrorWithParams(ErrPublicKeyParse, err.Error())
		}
		keys = append(keys, key)
	}
	if err := scanner.Err(); err != nil {
		return nil, cosy.WrapErrorWithParams(ErrPublicKeyParse, err.Error())
	}
	return keys, nil
}

func ClassifyHostKeys(hostPort string, scanned []gossh.PublicKey, kh *KnownHosts) (HostKeyScanResult, error) {
	known, err := kh.List(hostPort)
	if err != nil {
		return HostKeyScanResult{}, err
	}
	result := HostKeyScanResult{HostAddress: hostPort, Keys: make([]HostKeyScanItem, 0, len(scanned))}
	knownByAlgorithm := make(map[string]HostKeyEntry, len(known))
	for _, entry := range known {
		knownByAlgorithm[entry.Algorithm] = entry
	}
	seenAlgorithms := make(map[string]bool, len(scanned))

	for _, key := range scanned {
		algorithm := key.Type()
		fingerprint := gossh.FingerprintSHA256(key)
		seenAlgorithms[algorithm] = true
		item := HostKeyScanItem{
			Algorithm:   algorithm,
			PublicKey:   strings.TrimSpace(string(gossh.MarshalAuthorizedKey(key))),
			Fingerprint: fingerprint,
		}
		knownEntry, exists := knownByAlgorithm[algorithm]
		switch {
		case len(known) == 0:
			item.Status = HostKeyStatusUnknownHost
		case !exists:
			item.Status = HostKeyStatusNewAlgorithm
		case knownEntry.Fingerprint == fingerprint:
			item.Status = HostKeyStatusTrusted
		default:
			item.Status = HostKeyStatusChanged
			item.ExistingFingerprint = knownEntry.Fingerprint
		}
		result.Keys = append(result.Keys, item)
	}

	for _, entry := range known {
		if seenAlgorithms[entry.Algorithm] {
			continue
		}
		result.StaleKeys = append(result.StaleKeys, HostKeyScanItem{
			Algorithm:   entry.Algorithm,
			PublicKey:   entry.PublicKey,
			Fingerprint: entry.Fingerprint,
			Status:      HostKeyStatusStale,
		})
	}
	return result, nil
}
