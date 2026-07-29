package setup

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"net"
	"os"
	"strings"
	"sync"
	"time"
)

// SSHTarget is a candidate address for the nginx host, with the result of a
// plain TCP probe. It is produced before any SSH session exists, so the wizard
// can offer a working address instead of asking the operator to guess one.
type SSHTarget struct {
	Address   string `json:"address"`
	Source    string `json:"source"`
	Reachable bool   `json:"reachable"`
}

const (
	sshTargetProbeTimeout = 1500 * time.Millisecond
	// defaultSSHPort is the only port guessed for a discovered host. A target
	// on another port is reached by entering the address, which is probed
	// exactly as typed.
	defaultSSHPort = "22"
)

// dockerHostAliases are the names Docker publishes for the host gateway.
var dockerHostAliases = []string{"host.docker.internal", "gateway.docker.internal"}

// DiscoverSSHTargets probes the addresses a container can normally use to reach
// its own host, plus any address the caller supplies, and reports which of them
// accept TCP.
func DiscoverSSHTargets(ctx context.Context, requested ...string) []SSHTarget {
	candidates := collectSSHTargetCandidates(requested)

	var wg sync.WaitGroup
	for i := range candidates {
		wg.Add(1)
		go func(target *SSHTarget) {
			defer wg.Done()
			target.Reachable = probeTCP(ctx, target.Address)
		}(&candidates[i])
	}
	wg.Wait()

	return candidates
}

func collectSSHTargetCandidates(requested []string) []SSHTarget {
	// The API marshals this directly, and the TypeScript client declares an
	// array, so an empty result must serialise as [] rather than null.
	candidates := []SSHTarget{}
	seen := make(map[string]struct{})

	addAddress := func(address, source string) {
		address = strings.TrimSpace(address)
		if address == "" {
			return
		}
		if _, ok := seen[address]; ok {
			return
		}
		seen[address] = struct{}{}
		candidates = append(candidates, SSHTarget{Address: address, Source: source})
	}
	addHost := func(host, source string) {
		host = strings.TrimSpace(host)
		if host == "" {
			return
		}
		addAddress(net.JoinHostPort(host, defaultSSHPort), source)
	}

	// An address the operator already typed is probed exactly as given, so a
	// non standard port is never silently replaced.
	for _, address := range requested {
		address = strings.TrimSpace(address)
		if address == "" {
			continue
		}
		if _, _, err := net.SplitHostPort(address); err != nil {
			addHost(address, "requested")
			continue
		}
		addAddress(address, "requested")
	}

	for _, alias := range dockerHostAliases {
		if _, err := net.LookupHost(alias); err == nil {
			addHost(alias, "docker-host-alias")
		}
	}
	addHost(defaultGatewayIP(), "default-gateway")
	return candidates
}

// defaultGatewayIP reads the container's default route. The gateway is the host
// on a bridge network, which is where nginx usually runs.
func defaultGatewayIP() string {
	raw, err := os.ReadFile("/proc/net/route")
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(raw), "\n")[1:] {
		fields := strings.Fields(line)
		if len(fields) < 3 || fields[1] != "00000000" {
			continue
		}
		return parseHexIPv4(fields[2])
	}
	return ""
}

// parseHexIPv4 decodes the little endian hex address used by /proc/net/route.
func parseHexIPv4(value string) string {
	decoded, err := hex.DecodeString(value)
	if err != nil || len(decoded) != net.IPv4len {
		return ""
	}
	ip := make(net.IP, net.IPv4len)
	binary.LittleEndian.PutUint32(ip, binary.BigEndian.Uint32(decoded))
	if ip.IsUnspecified() {
		return ""
	}
	return ip.String()
}

func probeTCP(ctx context.Context, address string) bool {
	dialCtx, cancel := context.WithTimeout(ctx, sshTargetProbeTimeout)
	defer cancel()
	dialer := net.Dialer{}
	conn, err := dialer.DialContext(dialCtx, "tcp", address)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}
