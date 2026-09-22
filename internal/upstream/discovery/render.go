package discovery

import (
	"errors"
	"net/netip"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/0xJacky/Nginx-UI/internal/config"
)

const (
	// maxWeight bounds the weight of one server.
	maxWeight = 1000
	// maxHostNameLength is the longest host name DNS allows.
	maxHostNameLength = 253
)

var (
	// upstreamNamePattern keeps an upstream name a single nginx token and a
	// safe file name.
	upstreamNamePattern = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9_.-]{0,63}$`)
	// hostLabelPattern is one label of a host name. Underscores are allowed,
	// container runtimes use them in service names.
	hostLabelPattern = regexp.MustCompile(`^[A-Za-z0-9_]([A-Za-z0-9_-]{0,61}[A-Za-z0-9_])?$`)

	// ErrNoTargets reports an answer without any valid server, which cannot
	// be written as an upstream block.
	ErrNoTargets = errors.New("no valid server to write, the previous upstream is kept")
)

// IsValidUpstreamName reports whether name may name an upstream and its
// file.
func IsValidUpstreamName(name string) bool {
	return upstreamNamePattern.MatchString(name)
}

// IsValidExtraDirectives reports whether text may be appended inside an
// upstream block: it must not open or close a block.
func IsValidExtraDirectives(text string) bool {
	return !strings.ContainsAny(text, "{}\x00")
}

// Rendered is an answer turned into the content of its file.
type Rendered struct {
	Content []byte
	// Written counts the servers of the file.
	Written int
	// Dropped counts the targets with an invalid address, port or weight.
	Dropped int
	// Duplicates counts the targets another target already listed.
	Duplicates int
}

// server is one validated target.
type server struct {
	host   string
	port   int
	weight int
}

// Render validates every target and writes the ones that pass as the servers
// of an upstream block, followed by the extra directives. The result depends
// on the set of targets only, so the same answer always renders to the same
// file. No other text of the answer reaches the file. It fails with
// ErrNoTargets when no target is valid.
func Render(upstreamName string, targets []Target, extraDirectives string) (Rendered, error) {
	var rendered Rendered
	if !IsValidUpstreamName(upstreamName) {
		return rendered, errors.New("invalid upstream name")
	}
	if !IsValidExtraDirectives(extraDirectives) {
		return rendered, errors.New("extra directives must not contain braces")
	}

	seen := map[string]struct{}{}
	var servers []server
	for _, target := range targets {
		host, ok := serverHost(target.Address)
		if !ok || target.Port < 1 || target.Port > 65535 || target.Weight < 0 || target.Weight > maxWeight {
			rendered.Dropped++
			continue
		}
		key := host + ":" + strconv.Itoa(target.Port)
		if _, dup := seen[key]; dup {
			rendered.Duplicates++
			continue
		}
		seen[key] = struct{}{}
		weight := target.Weight
		if weight == 0 {
			weight = 1
		}
		servers = append(servers, server{host: host, port: target.Port, weight: weight})
	}
	if len(servers) == 0 {
		return rendered, ErrNoTargets
	}
	sort.Slice(servers, func(i, j int) bool {
		if servers[i].host != servers[j].host {
			return servers[i].host < servers[j].host
		}
		return servers[i].port < servers[j].port
	})

	var b strings.Builder
	b.WriteString(config.GeneratedHeader)
	b.WriteString("\n# Resolved by upstream discovery; changes to this file are overwritten.\n")
	b.WriteString("upstream ")
	b.WriteString(upstreamName)
	b.WriteString(" {\n")
	for _, s := range servers {
		b.WriteString("    server ")
		b.WriteString(s.host)
		b.WriteString(":")
		b.WriteString(strconv.Itoa(s.port))
		b.WriteString(" weight=")
		b.WriteString(strconv.Itoa(s.weight))
		b.WriteString(";\n")
	}
	for _, line := range strings.Split(strings.ReplaceAll(extraDirectives, "\r\n", "\n"), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			b.WriteString("    ")
			b.WriteString(line)
			b.WriteString("\n")
		}
	}
	b.WriteString("}\n")

	rendered.Content = []byte(b.String())
	rendered.Written = len(servers)
	return rendered, nil
}

// serverHost validates an address and formats it for a server directive: an
// IPv6 address in brackets, an IPv4 address or a host name as it is.
func serverHost(address string) (string, bool) {
	address = strings.TrimSpace(address)
	if address == "" {
		return "", false
	}
	if addr, err := netip.ParseAddr(address); err == nil {
		if addr.Zone() != "" {
			return "", false
		}
		addr = addr.Unmap()
		if addr.Is6() {
			return "[" + addr.String() + "]", true
		}
		return addr.String(), true
	}

	host := strings.TrimSuffix(address, ".")
	if len(host) == 0 || len(host) > maxHostNameLength {
		return "", false
	}
	for _, label := range strings.Split(host, ".") {
		if !hostLabelPattern.MatchString(label) {
			return "", false
		}
	}
	return strings.ToLower(host), true
}
