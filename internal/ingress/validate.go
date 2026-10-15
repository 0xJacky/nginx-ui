package ingress

import (
	"fmt"
	"net/netip"
	"regexp"
	"strings"

	internalTemplate "github.com/0xJacky/Nginx-UI/internal/template"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/uozi-tech/cosy"
	"github.com/uozi-tech/cosy/logger"
	"golang.org/x/net/idna"
)

// acmeChallengePrefix is answered by Nginx UI on every ingress that issues
// its certificate over HTTP-01, so routes may not claim it.
const acmeChallengePrefix = "/.well-known/acme-challenge"

// Limits of the spec values that have no natural bound in nginx.
const (
	maxHosts      = 100
	maxBackends   = 32
	maxServers    = 64
	maxRoutes     = 100
	maxCIDRs      = 256
	maxHeaders    = 32
	maxKeepalive  = 10000
	maxRPS        = 10000
	maxBurst      = 100000
	maxPathLength = 1024
)

var (
	// reLDHLabel is an ASCII DNS label: letters, digits and inner hyphens.
	reLDHLabel = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$`)
	// reTime is an nginx time value such as 500ms, 60s or 5m.
	reTime = regexp.MustCompile(`^\d{1,9}(ms|s|m|h|d|w|M|y)?$`)
	// reHeaderName is a request header name an ingress may set.
	reHeaderName = regexp.MustCompile(`^[A-Za-z0-9-]{1,64}$`)
	// reUnsafeHeaderValue matches what could end the directive, break the
	// quoted parameter or start a comment.
	reUnsafeHeaderValue = regexp.MustCompile(`[;{}"'\\\x00-\x1f\x7f]`)
	// reUnsafePath matches what may not appear in a location prefix.
	reUnsafePath = regexp.MustCompile(`[\s;{}"'#\\\x00-\x1f\x7f]`)
)

// reservedHeaders are set on every route by the renderer.
var reservedHeaders = map[string]struct{}{
	"host":              {},
	"x-real-ip":         {},
	"x-forwarded-for":   {},
	"x-forwarded-proto": {},
	"upgrade":           {},
	"connection":        {},
}

var validKeyTypes = map[string]struct{}{
	"2048": {}, "3072": {}, "4096": {}, "P256": {}, "P384": {},
}

// Validate checks a spec without rendering it: names, hosts (IDNA), backend
// addresses against settings.IngressSettings.AllowedBackends, routes, TLS and
// policy values.
func Validate(spec *model.IngressSpec) error {
	if spec == nil {
		return cosy.WrapErrorWithParams(ErrInvalidHost, "at least one host is required")
	}
	if err := validateTLS(&spec.TLS); err != nil {
		return err
	}
	if _, err := normalizeHosts(spec); err != nil {
		return err
	}
	if err := validateBackends(spec.Backends); err != nil {
		return err
	}
	if err := validateRoutes(spec); err != nil {
		return err
	}
	return validatePolicy(&spec.Policy)
}

// normalizeHosts validates the hosts and returns them in their ASCII form,
// in the order given.
func normalizeHosts(spec *model.IngressSpec) ([]string, error) {
	if len(spec.Hosts) == 0 {
		return nil, cosy.WrapErrorWithParams(ErrInvalidHost, "at least one host is required")
	}
	if len(spec.Hosts) > maxHosts {
		return nil, cosy.WrapErrorWithParams(ErrInvalidHost, fmt.Sprintf("at most %d hosts are allowed", maxHosts))
	}
	wildcardAllowed := spec.TLS.Mode != model.IngressTLSModeACME || challengeOf(&spec.TLS) == model.IngressChallengeDNS01

	result := make([]string, 0, len(spec.Hosts))
	seen := make(map[string]struct{}, len(spec.Hosts))
	for _, host := range spec.Hosts {
		ascii, err := HostToASCII(host)
		if err != nil {
			return nil, err
		}
		if strings.HasPrefix(ascii, "*.") && !wildcardAllowed {
			return nil, cosy.WrapErrorWithParams(ErrInvalidHost,
				host+": wildcard hosts need tls.mode cert or none, or the dns01 challenge")
		}
		if _, ok := seen[ascii]; ok {
			return nil, cosy.WrapErrorWithParams(ErrInvalidHost, host+": listed twice")
		}
		seen[ascii] = struct{}{}
		result = append(result, ascii)
	}
	return result, nil
}

// HostToASCII validates a host name and returns its lowercase ASCII
// (punycode) form. A leading "*." wildcard label is kept. The whole name is
// converted first, because IDNA mapping folds separators such as U+3002 into
// dots; when that fails (labels like "ab--cd" break IDNA2008 hyphen rules but
// are valid DNS) the labels are converted one by one.
func HostToASCII(host string) (string, error) {
	invalid := func(reason string) error {
		return cosy.WrapErrorWithParams(ErrInvalidHost, host+": "+reason)
	}
	name := strings.TrimSpace(host)
	if name == "" || name != host {
		return "", invalid("must not be empty or have surrounding spaces")
	}

	wildcard := false
	if strings.HasPrefix(name, "*.") {
		wildcard = true
		name = name[2:]
	}

	var labels []string
	if converted, err := idna.Lookup.ToASCII(name); err == nil {
		labels = strings.Split(strings.TrimSuffix(strings.ToLower(converted), "."), ".")
	} else {
		labels = strings.Split(strings.TrimSuffix(name, "."), ".")
		for i, label := range labels {
			if converted, err := idna.Lookup.ToASCII(label); err == nil {
				labels[i] = converted
			}
			labels[i] = strings.ToLower(labels[i])
		}
	}

	if len(labels) < 2 {
		return "", invalid("needs at least two labels")
	}
	for _, label := range labels {
		if !reLDHLabel.MatchString(label) {
			return "", invalid("invalid label " + label)
		}
	}
	if isAllDigits(labels[len(labels)-1]) {
		return "", invalid("IP addresses are not allowed")
	}
	ascii := strings.Join(labels, ".")
	if wildcard {
		ascii = "*." + ascii
	}
	if len(ascii) > 253 {
		return "", invalid("longer than 253 characters")
	}
	return ascii, nil
}

func isAllDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return s != ""
}

func validateBackends(backends []model.IngressBackend) error {
	if len(backends) == 0 {
		return cosy.WrapErrorWithParams(ErrInvalidBackend, "-", "at least one backend is required")
	}
	if len(backends) > maxBackends {
		return cosy.WrapErrorWithParams(ErrInvalidBackend, "-", fmt.Sprintf("at most %d backends are allowed", maxBackends))
	}
	allowed := allowedBackendPrefixes()
	seen := make(map[string]struct{}, len(backends))
	for _, b := range backends {
		invalid := func(reason string) error {
			return cosy.WrapErrorWithParams(ErrInvalidBackend, b.Name, reason)
		}
		if !ValidateBackendName(b.Name) {
			return invalid("name must be 1-16 lowercase letters, digits and hyphens, starting and ending with a letter or digit")
		}
		if _, ok := seen[b.Name]; ok {
			return invalid("name is used twice")
		}
		seen[b.Name] = struct{}{}

		switch b.LB {
		case "", model.IngressLBRoundRobin, model.IngressLBLeastConn, model.IngressLBIPHash:
		default:
			return invalid("lb must be round_robin, least_conn or ip_hash")
		}
		if b.Keepalive < 0 || b.Keepalive > maxKeepalive {
			return invalid(fmt.Sprintf("keepalive must be between 0 and %d", maxKeepalive))
		}
		if len(b.Servers) == 0 {
			return invalid("at least one server is required")
		}
		if len(b.Servers) > maxServers {
			return invalid(fmt.Sprintf("at most %d servers are allowed", maxServers))
		}
		addresses := make(map[string]struct{}, len(b.Servers))
		for _, s := range b.Servers {
			addr, err := parseServerAddress(s.Address)
			if err != nil {
				return invalid(err.Error())
			}
			if !addressAllowed(addr.Addr(), allowed) {
				return cosy.WrapErrorWithParams(ErrBackendNotAllowed, s.Address)
			}
			if _, ok := addresses[addr.String()]; ok {
				return invalid("server " + s.Address + " is listed twice")
			}
			addresses[addr.String()] = struct{}{}
			if s.Weight < 0 {
				return invalid("server " + s.Address + ": weight must not be negative")
			}
			if s.MaxFails < 0 {
				return invalid("server " + s.Address + ": max_fails must not be negative")
			}
			if s.FailTimeout != "" && !reTime.MatchString(s.FailTimeout) {
				return invalid("server " + s.Address + ": fail_timeout must be an nginx time such as 10s")
			}
			if s.Backup && b.LB == model.IngressLBIPHash {
				return invalid("backup servers cannot be used with ip_hash")
			}
		}
	}
	return nil
}

// parseServerAddress parses an "ip:port" or "[ipv6]:port" server address.
func parseServerAddress(address string) (netip.AddrPort, error) {
	addr, err := netip.ParseAddrPort(address)
	if err != nil {
		return netip.AddrPort{}, fmt.Errorf("server %s: address must be ip:port", address)
	}
	if addr.Port() == 0 {
		return netip.AddrPort{}, fmt.Errorf("server %s: port must be between 1 and 65535", address)
	}
	if addr.Addr().Zone() != "" {
		return netip.AddrPort{}, fmt.Errorf("server %s: zoned addresses are not allowed", address)
	}
	return addr, nil
}

// allowedBackendPrefixes parses settings.IngressSettings.AllowedBackends.
// Entries may be CIDRs or single addresses; invalid entries are skipped.
func allowedBackendPrefixes() []netip.Prefix {
	prefixes := make([]netip.Prefix, 0, len(settings.IngressSettings.AllowedBackends))
	for _, entry := range settings.IngressSettings.AllowedBackends {
		entry = strings.TrimSpace(entry)
		if prefix, err := netip.ParsePrefix(entry); err == nil {
			prefixes = append(prefixes, prefix.Masked())
			continue
		}
		if addr, err := netip.ParseAddr(entry); err == nil {
			prefixes = append(prefixes, netip.PrefixFrom(addr.Unmap(), addr.Unmap().BitLen()))
			continue
		}
		logger.Warnf("Ignoring invalid ingress AllowedBackends entry %q", entry)
	}
	return prefixes
}

func addressAllowed(addr netip.Addr, allowed []netip.Prefix) bool {
	addr = addr.Unmap()
	for _, prefix := range allowed {
		if prefix.Contains(addr) {
			return true
		}
	}
	return false
}

func validateRoutes(spec *model.IngressSpec) error {
	if len(spec.Routes) == 0 {
		return cosy.WrapErrorWithParams(ErrInvalidRoute, "-", "at least one route is required")
	}
	if len(spec.Routes) > maxRoutes {
		return cosy.WrapErrorWithParams(ErrInvalidRoute, "-", fmt.Sprintf("at most %d routes are allowed", maxRoutes))
	}
	backends := make(map[string]struct{}, len(spec.Backends))
	for _, b := range spec.Backends {
		backends[b.Name] = struct{}{}
	}
	seen := make(map[string]struct{}, len(spec.Routes))
	for _, r := range spec.Routes {
		invalid := func(reason string) error {
			return cosy.WrapErrorWithParams(ErrInvalidRoute, r.PathPrefix, reason)
		}
		if !strings.HasPrefix(r.PathPrefix, "/") {
			return invalid("path_prefix must start with /")
		}
		if len(r.PathPrefix) > maxPathLength || reUnsafePath.MatchString(r.PathPrefix) {
			return invalid("path_prefix must not contain spaces, ;, braces, quotes, backslashes or #")
		}
		if strings.HasPrefix(r.PathPrefix, acmeChallengePrefix) {
			return invalid("path_prefix must not cover " + acmeChallengePrefix)
		}
		if _, ok := seen[r.PathPrefix]; ok {
			return invalid("path_prefix is used twice")
		}
		seen[r.PathPrefix] = struct{}{}
		if _, ok := backends[r.Backend]; !ok {
			return invalid("backend " + r.Backend + " does not exist")
		}
		if r.ReadTimeout != "" && !reTime.MatchString(r.ReadTimeout) {
			return invalid("read_timeout must be an nginx time such as 60s")
		}
		if r.SendTimeout != "" && !reTime.MatchString(r.SendTimeout) {
			return invalid("send_timeout must be an nginx time such as 60s")
		}
		if r.ClientMaxBodySize != "" && !internalTemplate.IsNginxSize(r.ClientMaxBodySize) {
			return invalid("client_max_body_size must be an nginx size such as 64m")
		}
		if err := validateCIDRs(r.AllowCIDRs); err != nil {
			return invalid(err.Error())
		}
	}
	return nil
}

func validateCIDRs(cidrs []string) error {
	if len(cidrs) > maxCIDRs {
		return fmt.Errorf("at most %d allow_cidrs are allowed", maxCIDRs)
	}
	for _, cidr := range cidrs {
		if _, err := parseCIDR(cidr); err != nil {
			return err
		}
	}
	return nil
}

// parseCIDR accepts a CIDR or a single address and returns its canonical
// nginx form.
func parseCIDR(cidr string) (string, error) {
	if prefix, err := netip.ParsePrefix(cidr); err == nil && prefix.Addr().Zone() == "" {
		return prefix.Masked().String(), nil
	}
	if addr, err := netip.ParseAddr(cidr); err == nil && addr.Zone() == "" {
		return addr.String(), nil
	}
	return "", fmt.Errorf("allow_cidrs entry %q is not an IP address or CIDR", cidr)
}

func challengeOf(tls *model.IngressTLS) string {
	if tls.Challenge == "" {
		return model.IngressChallengeHTTP01
	}
	return tls.Challenge
}

func validateTLS(tls *model.IngressTLS) error {
	invalid := func(reason string) error {
		return cosy.WrapErrorWithParams(ErrInvalidTLS, reason)
	}
	switch tls.Mode {
	case model.IngressTLSModeNone:
	case model.IngressTLSModeCert:
		if tls.CertID == 0 {
			return invalid("mode cert needs cert_id")
		}
	case model.IngressTLSModeACME:
		switch challengeOf(tls) {
		case model.IngressChallengeHTTP01:
		case model.IngressChallengeDNS01:
			if tls.DNSCredentialID == 0 {
				return invalid("the dns01 challenge needs dns_credential_id")
			}
		default:
			return invalid("challenge must be http01 or dns01")
		}
		if tls.KeyType != "" {
			if _, ok := validKeyTypes[tls.KeyType]; !ok {
				return invalid("key_type must be 2048, 3072, 4096, P256 or P384")
			}
		}
	default:
		return invalid("mode must be none, acme or cert")
	}
	return nil
}

func validatePolicy(policy *model.IngressPolicy) error {
	invalid := func(reason string) error {
		return cosy.WrapErrorWithParams(ErrInvalidPolicy, reason)
	}
	if err := validateCIDRs(policy.AllowCIDRs); err != nil {
		return invalid(err.Error())
	}
	if rl := policy.RateLimit; rl != nil {
		if rl.RPS < 1 || rl.RPS > maxRPS {
			return invalid(fmt.Sprintf("rate_limit.rps must be between 1 and %d", maxRPS))
		}
		if rl.Burst < 0 || rl.Burst > maxBurst {
			return invalid(fmt.Sprintf("rate_limit.burst must be between 0 and %d", maxBurst))
		}
	}
	if policy.ClientMaxBodySize != "" && !internalTemplate.IsNginxSize(policy.ClientMaxBodySize) {
		return invalid("client_max_body_size must be an nginx size such as 64m")
	}
	if len(policy.ExtraHeaders) > maxHeaders {
		return invalid(fmt.Sprintf("at most %d extra_headers are allowed", maxHeaders))
	}
	names := make(map[string]struct{}, len(policy.ExtraHeaders))
	for _, h := range policy.ExtraHeaders {
		if !reHeaderName.MatchString(h.Name) {
			return invalid("header name " + h.Name + " must use letters, digits and hyphens")
		}
		lower := strings.ToLower(h.Name)
		if _, ok := reservedHeaders[lower]; ok {
			return invalid("header " + h.Name + " is set by the ingress itself")
		}
		if _, ok := names[lower]; ok {
			return invalid("header " + h.Name + " is listed twice")
		}
		names[lower] = struct{}{}
		if len(h.Value) > 1024 || reUnsafeHeaderValue.MatchString(h.Value) {
			return invalid("header " + h.Name + " value must not contain ;, braces, quotes, backslashes or line breaks")
		}
	}
	return nil
}
