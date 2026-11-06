package blocklist

import (
	"net/netip"
	"sort"
	"strings"

	"github.com/0xJacky/Nginx-UI/internal/config"
)

// MaxEntries bounds the deny rules of one file.
const MaxEntries = 100000

// Rendered is a list turned into the content of its file.
type Rendered struct {
	Content []byte
	// Written counts the deny rules of the file.
	Written int
	// Dropped counts the entries that were not addresses or networks,
	// covered every address or went past MaxEntries.
	Dropped int
	// Duplicates counts the entries another entry already listed.
	Duplicates int
}

// Render validates every entry and writes the ones that parse as deny rules.
// The result depends on the set of entries only, so the same answer always
// renders to the same file. No other text of the answer reaches the file.
func Render(entries []Entry) Rendered {
	var rendered Rendered
	seen := make(map[netip.Prefix]struct{}, len(entries))
	prefixes := make([]netip.Prefix, 0, len(entries))
	for _, entry := range entries {
		prefix, ok := ParseEntry(entry.CIDR)
		if !ok {
			rendered.Dropped++
			continue
		}
		if _, dup := seen[prefix]; dup {
			rendered.Duplicates++
			continue
		}
		seen[prefix] = struct{}{}
		prefixes = append(prefixes, prefix)
	}

	sort.Slice(prefixes, func(i, j int) bool {
		if c := prefixes[i].Addr().Compare(prefixes[j].Addr()); c != 0 {
			return c < 0
		}
		return prefixes[i].Bits() < prefixes[j].Bits()
	})
	if len(prefixes) > MaxEntries {
		rendered.Dropped += len(prefixes) - MaxEntries
		prefixes = prefixes[:MaxEntries]
	}

	var b strings.Builder
	b.WriteString(config.GeneratedHeader)
	b.WriteString("\n# Refreshed from a blocklist source; changes to this file are overwritten.\n")
	for _, prefix := range prefixes {
		b.WriteString("deny ")
		b.WriteString(FormatPrefix(prefix))
		b.WriteString(";\n")
	}
	rendered.Content = []byte(b.String())
	rendered.Written = len(prefixes)
	return rendered
}

// ParseEntry parses an IPv4 or IPv6 address or CIDR network into the
// network it covers. It refuses zones, host names and networks that cover
// every address. An IPv4-mapped IPv6 address becomes IPv4.
func ParseEntry(value string) (netip.Prefix, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return netip.Prefix{}, false
	}

	var prefix netip.Prefix
	if strings.Contains(value, "/") {
		parsed, err := netip.ParsePrefix(value)
		if err != nil {
			return netip.Prefix{}, false
		}
		prefix = parsed
	} else {
		addr, err := netip.ParseAddr(value)
		// PrefixFrom would drop a zone silently, nginx has no use for one.
		if err != nil || addr.Zone() != "" {
			return netip.Prefix{}, false
		}
		prefix = netip.PrefixFrom(addr, addr.BitLen())
	}

	if addr := prefix.Addr(); addr.Is4In6() && prefix.Bits() >= 96 {
		prefix = netip.PrefixFrom(addr.Unmap(), prefix.Bits()-96)
	}
	prefix = prefix.Masked()
	if !prefix.IsValid() || prefix.Bits() == 0 {
		return netip.Prefix{}, false
	}
	return prefix, true
}

// FormatPrefix writes a single address without its length and a network in
// CIDR notation.
func FormatPrefix(prefix netip.Prefix) string {
	if prefix.IsSingleIP() {
		return prefix.Addr().String()
	}
	return prefix.String()
}
