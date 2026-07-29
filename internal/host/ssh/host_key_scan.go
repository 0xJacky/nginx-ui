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
	HostKeyStatusRevoked      = "revoked"
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
	keys, _, err := ScanHostKeysWithCoverage(ctx, hostPort, timeout)
	return keys, err
}

// scanHostKeysWithCoverage also reports which algorithms were actually probed.
// Without that, an algorithm whose probe failed is indistinguishable from one
// the server no longer offers, and a trusted entry gets labelled stale.
func ScanHostKeysWithCoverage(ctx context.Context, hostPort string, timeout time.Duration) ([]gossh.PublicKey, map[string]bool, error) {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}

	seenFingerprints := make(map[string]bool)
	probed := make(map[string]bool, len(preferredHostKeyAlgorithms))
	keys := make([]gossh.PublicKey, 0, len(preferredHostKeyAlgorithms))
	var lastErr error
	for _, algorithm := range preferredHostKeyAlgorithms {
		key, err := scanHostKeyWithAlgorithm(ctx, hostPort, timeout, algorithm)
		if err != nil {
			lastErr = err
			continue
		}
		// The handshake answered for this algorithm, so its absence from the
		// reply is real evidence rather than a failed probe.
		probed[algorithm] = true
		fingerprint := gossh.FingerprintSHA256(key)
		if seenFingerprints[fingerprint] {
			continue
		}
		seenFingerprints[fingerprint] = true
		keys = append(keys, key)
	}
	if len(keys) == 0 {
		if lastErr != nil {
			return nil, nil, cosy.WrapErrorWithParams(ErrHostKeyScanFailed, lastErr.Error())
		}
		return nil, nil, cosy.WrapErrorWithParams(ErrHostKeyScanFailed, "server did not present a host key")
	}
	return keys, probed, nil
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

// ClassifyHostKeys compares scanned keys against known_hosts. probedAlgorithms
// may be nil when the caller could not establish which algorithms answered; a
// trusted entry is then never reported stale, because an unanswered probe is
// not evidence that the server dropped the key.
func ClassifyHostKeys(hostPort string, scanned []gossh.PublicKey, kh *KnownHosts) (HostKeyScanResult, error) {
	return ClassifyScannedHostKeys(hostPort, scanned, nil, kh)
}

func ClassifyScannedHostKeys(hostPort string, scanned []gossh.PublicKey, probedAlgorithms map[string]bool, kh *KnownHosts) (HostKeyScanResult, error) {
	known, err := kh.List(hostPort)
	if err != nil {
		return HostKeyScanResult{}, err
	}
	result := HostKeyScanResult{HostAddress: hostPort, Keys: make([]HostKeyScanItem, 0, len(scanned))}
	knownByAlgorithm := make(map[string]HostKeyEntry, len(known))
	revokedFingerprints := make(map[string]bool, len(known))
	trustedCount := 0
	for _, entry := range known {
		if entry.Marker == markerRevoked {
			revokedFingerprints[entry.Fingerprint] = true
			continue
		}
		knownByAlgorithm[entry.Algorithm] = entry
		trustedCount++
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
		case revokedFingerprints[fingerprint]:
			// The dial callback refuses a revoked key, so it must never read
			// as trusted here.
			item.Status = HostKeyStatusRevoked
		case trustedCount == 0:
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
		if entry.Marker == markerRevoked || seenAlgorithms[entry.Algorithm] {
			continue
		}
		// Only an algorithm the server actually answered for can prove the
		// entry is stale. Deleting on a failed probe would drop a good key.
		if probedAlgorithms != nil && !probedAlgorithms[entry.Algorithm] {
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
