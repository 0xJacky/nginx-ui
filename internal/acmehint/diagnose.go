package acmehint

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/0xJacky/Nginx-UI/settings"
)

// DefaultLookupTimeout bounds every single DNS lookup made by Diagnose.
const DefaultLookupTimeout = 3 * time.Second

// Diagnostic is one finding about an identifier, computed before or after an
// issuance attempt.
type Diagnostic struct {
	// Level is LevelInfo or LevelWarning.
	Level string `json:"level"`
	// Code is a stable machine code such as "dns_points_elsewhere".
	Code string `json:"code"`
	// Message is one plain English sentence.
	Message string `json:"message"`
	// Params carries the facts behind the diagnostic. Address lists are joined
	// with ", ".
	Params map[string]string `json:"params,omitempty"`
}

// Resolver resolves A and AAAA records. *net.Resolver satisfies it.
type Resolver interface {
	LookupIPAddr(ctx context.Context, host string) ([]net.IPAddr, error)
}

// CAARecord is a single CAA resource record.
type CAARecord struct {
	Flag  uint8
	Tag   string
	Value string
}

// CAALookuper returns the CAA records published exactly at name (no tree
// climbing; Diagnose climbs itself). An empty result with a nil error means
// that name has no CAA records.
type CAALookuper interface {
	LookupCAA(ctx context.Context, name string) ([]CAARecord, error)
}

// Options configures Diagnose. Every field is optional.
type Options struct {
	// Resolver overrides address resolution. When nil, a resolver using
	// settings.CertSettings.RecursiveNameservers (or the system resolver when
	// none are configured) is used.
	Resolver Resolver
	// LocalAddrs returns the addresses assigned to this host. When nil, the
	// addresses of all network interfaces are used.
	LocalAddrs func() ([]net.IP, error)
	// HasIPv6Listen reports whether the site listens on IPv6 port 80. When
	// false and a domain has AAAA records, an aaaa_without_ipv6_listen warning
	// is emitted.
	HasIPv6Listen bool
	// CAAIdentities are the CAA issuer domains of the selected CA (see
	// CAAIdentitiesForDirectory). The CAA check is skipped when empty.
	CAAIdentities []string
	// CAALookuper overrides CAA lookups. When nil, CAA records are queried
	// with miekg/dns against the recursive nameservers from settings, or the
	// nameservers from /etc/resolv.conf.
	CAALookuper CAALookuper
	// LookupTimeout bounds every single lookup. Defaults to DefaultLookupTimeout.
	LookupTimeout time.Duration
}

// Diagnose resolves the A/AAAA (and optionally CAA) records of domains and
// compares them with the local addresses of this host. Wildcard identifiers
// only get the CAA check and IP identifiers are skipped. Results keep the
// order of domains. Diagnose never fails: lookup errors become diagnostics.
func Diagnose(ctx context.Context, domains []string, opts Options) []Diagnostic {
	resolver := opts.Resolver
	if resolver == nil {
		resolver = newDefaultResolver(settings.CertSettings.RecursiveNameservers)
	}
	timeout := opts.LookupTimeout
	if timeout <= 0 {
		timeout = DefaultLookupTimeout
	}
	localFn := opts.LocalAddrs
	if localFn == nil {
		localFn = interfaceAddrs
	}
	var local []string
	if ips, err := localFn(); err == nil {
		for _, ip := range ips {
			if a, ok := netip.AddrFromSlice(ip); ok {
				local = append(local, a.Unmap().String())
			}
		}
	}
	localSet := addrSet(local)
	localList := sortedKeys(localSet)

	caa := opts.CAALookuper
	if caa == nil && len(opts.CAAIdentities) > 0 {
		caa = newDefaultCAALookuper(settings.CertSettings.RecursiveNameservers, timeout)
	}

	names := normalizeDomains(domains)
	results := make([][]Diagnostic, len(names))
	var wg sync.WaitGroup
	for i, name := range names {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results[i] = diagnoseDomain(ctx, name, resolver, caa, timeout, localSet, localList, opts)
		}()
	}
	wg.Wait()

	var out []Diagnostic
	for _, r := range results {
		out = append(out, r...)
	}
	return out
}

func diagnoseDomain(ctx context.Context, name string, resolver Resolver, caa CAALookuper,
	timeout time.Duration, localSet map[netip.Addr]struct{}, localList []string, opts Options) []Diagnostic {
	var out []Diagnostic
	wildcard := strings.HasPrefix(name, "*.")
	base := strings.TrimPrefix(name, "*.")

	if !wildcard {
		out = append(out, diagnoseAddresses(ctx, name, resolver, timeout, localSet, localList, opts.HasIPv6Listen)...)
	}
	if caa != nil && len(opts.CAAIdentities) > 0 {
		if d := diagnoseCAA(ctx, name, base, wildcard, caa, timeout, opts.CAAIdentities); d != nil {
			out = append(out, *d)
		}
	}
	return out
}

func diagnoseAddresses(ctx context.Context, name string, resolver Resolver, timeout time.Duration,
	localSet map[netip.Addr]struct{}, localList []string, hasIPv6Listen bool) []Diagnostic {
	lctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	addrs, err := resolver.LookupIPAddr(lctx, name)

	params := map[string]string{"domain": name}
	if err != nil {
		var dnsErr *net.DNSError
		if errors.As(err, &dnsErr) && dnsErr.IsNotFound {
			return []Diagnostic{newDiagnostic(LevelWarning, CodeDNSNoRecords, params)}
		}
		params["error"] = err.Error()
		return []Diagnostic{newDiagnostic(LevelWarning, CodeDNSLookupFailed, params)}
	}

	var resolved, ipv6, elsewhere []string
	for _, ia := range addrs {
		a, ok := netip.AddrFromSlice(ia.IP)
		if !ok {
			continue
		}
		a = a.Unmap()
		s := a.String()
		if slices.Contains(resolved, s) {
			continue
		}
		resolved = append(resolved, s)
		if a.Is6() {
			ipv6 = append(ipv6, s)
		}
		if _, isLocal := localSet[a]; len(localSet) > 0 && !isLocal {
			elsewhere = append(elsewhere, s)
		}
	}
	if len(resolved) == 0 {
		return []Diagnostic{newDiagnostic(LevelWarning, CodeDNSNoRecords, params)}
	}
	params["resolved"] = strings.Join(resolved, ", ")

	var out []Diagnostic
	switch {
	case len(localSet) == 0:
		out = append(out, newDiagnostic(LevelInfo, CodeDNSResolved, params))
	case len(elsewhere) > 0:
		p := cloneParams(params)
		p["local"] = strings.Join(localList, ", ")
		p["elsewhere"] = strings.Join(elsewhere, ", ")
		out = append(out, Diagnostic{
			Level:   LevelWarning,
			Code:    CodeDNSPointsElsewhere,
			Message: dnsPointsElsewhereDiagnostic.Message,
			Params:  p,
		})
	default:
		p := cloneParams(params)
		p["local"] = strings.Join(localList, ", ")
		out = append(out, newDiagnostic(LevelInfo, CodeDNSOK, p))
	}

	if len(ipv6) > 0 && !hasIPv6Listen {
		out = append(out, newDiagnostic(LevelWarning, CodeAAAAWithoutIPv6Listen, map[string]string{
			"domain": name,
			"ip":     strings.Join(ipv6, ", "),
		}))
	}
	return out
}

// diagnoseCAA applies the RFC 8659 relevant-record-set search: the first name
// on the way from the identifier to its parent domains that has CAA records
// decides. Lookup errors are ignored because the CA reports CAA failures itself.
func diagnoseCAA(ctx context.Context, name, base string, wildcard bool, caa CAALookuper,
	timeout time.Duration, identities []string) *Diagnostic {
	labels := strings.Split(base, ".")
	// Stop before the TLD; TLD-level CAA records are practically non-existent.
	for i := 0; i < len(labels)-1; i++ {
		candidate := strings.Join(labels[i:], ".")
		lctx, cancel := context.WithTimeout(ctx, timeout)
		records, err := caa.LookupCAA(lctx, candidate)
		cancel()
		if err != nil {
			return nil
		}
		if len(records) == 0 {
			continue
		}
		if caaAllows(records, wildcard, identities) {
			return nil
		}
		var values []string
		for _, r := range records {
			values = append(values, r.Tag+" "+r.Value)
		}
		d := newDiagnostic(LevelWarning, CodeCAABlocksCA, map[string]string{
			"domain":   name,
			"caa_name": candidate,
			"caa":      strings.Join(values, "; "),
			"ca":       strings.Join(identities, ", "),
		})
		return &d
	}
	return nil
}

// caaAllows reports whether the relevant record set authorizes one of the CA
// identities for the identifier.
func caaAllows(records []CAARecord, wildcard bool, identities []string) bool {
	tag := "issue"
	if wildcard {
		for _, r := range records {
			if strings.EqualFold(r.Tag, "issuewild") {
				tag = "issuewild"
				break
			}
		}
	}
	var relevant []CAARecord
	for _, r := range records {
		if strings.EqualFold(r.Tag, tag) {
			relevant = append(relevant, r)
		}
	}
	if len(relevant) == 0 {
		// Only iodef or other tags: issuance is not restricted.
		return true
	}
	for _, r := range relevant {
		issuer := strings.ToLower(strings.TrimSpace(strings.SplitN(r.Value, ";", 2)[0]))
		if issuer == "" {
			continue
		}
		for _, id := range identities {
			if issuer == strings.ToLower(strings.TrimSpace(id)) {
				return true
			}
		}
	}
	return false
}

// CAAIdentitiesForDirectory maps a well-known ACME directory URL to the CAA
// issuer domains of its CA. It returns nil for unknown directories, which
// disables the CAA check in Diagnose.
func CAAIdentitiesForDirectory(dirURL string) []string {
	u := strings.ToLower(dirURL)
	switch {
	case u == "" || strings.Contains(u, "letsencrypt.org"):
		// An empty directory means the Let's Encrypt default.
		return []string{"letsencrypt.org"}
	case strings.Contains(u, "zerossl.com"):
		return []string{"sectigo.com"}
	case strings.Contains(u, "pki.goog"):
		return []string{"pki.goog"}
	case strings.Contains(u, "buypass."):
		return []string{"buypass.com"}
	case strings.Contains(u, "ssl.com"):
		return []string{"ssl.com"}
	}
	return nil
}

// EvidenceFromDiagnostics collects the resolved and local addresses reported
// by Diagnose into Evidence for Classify.
func EvidenceFromDiagnostics(diags []Diagnostic) Evidence {
	ev := Evidence{Resolved: map[string][]string{}}
	var local []string
	for _, d := range diags {
		domain := d.Params["domain"]
		if r := splitList(d.Params["resolved"]); domain != "" && len(r) > 0 {
			ev.Resolved[domain] = r
		}
		for _, l := range splitList(d.Params["local"]) {
			if !slices.Contains(local, l) {
				local = append(local, l)
			}
		}
	}
	ev.Local = local
	return ev
}

func splitList(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func normalizeDomains(domains []string) []string {
	var out []string
	for _, d := range domains {
		d = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(d)), ".")
		if d == "" || slices.Contains(out, d) {
			continue
		}
		if _, err := netip.ParseAddr(strings.Trim(d, "[]")); err == nil {
			continue
		}
		out = append(out, d)
	}
	return out
}

func newDiagnostic(level, code string, params map[string]string) Diagnostic {
	return Diagnostic{Level: level, Code: code, Message: Message(code), Params: params}
}

func cloneParams(p map[string]string) map[string]string {
	out := make(map[string]string, len(p)+2)
	for k, v := range p {
		out[k] = v
	}
	return out
}

func interfaceAddrs() ([]net.IP, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return nil, err
	}
	ips := make([]net.IP, 0, len(addrs))
	for _, a := range addrs {
		switch v := a.(type) {
		case *net.IPNet:
			ips = append(ips, v.IP)
		case *net.IPAddr:
			ips = append(ips, v.IP)
		}
	}
	return ips, nil
}

// newDefaultResolver returns the system resolver, or a pure-Go resolver that
// sends every query to the configured recursive nameservers in rotation.
func newDefaultResolver(nameservers []string) Resolver {
	servers := withDNSPort(nameservers)
	if len(servers) == 0 {
		return net.DefaultResolver
	}
	var next atomic.Uint32
	var dialer net.Dialer
	return &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, _ string) (net.Conn, error) {
			server := servers[int(next.Add(1)-1)%len(servers)]
			return dialer.DialContext(ctx, network, server)
		},
	}
}

func withDNSPort(nameservers []string) []string {
	var out []string
	for _, ns := range nameservers {
		ns = strings.TrimSpace(ns)
		if ns == "" {
			continue
		}
		if _, _, err := net.SplitHostPort(ns); err != nil {
			ns = net.JoinHostPort(strings.Trim(ns, "[]"), "53")
		}
		out = append(out, ns)
	}
	return out
}
