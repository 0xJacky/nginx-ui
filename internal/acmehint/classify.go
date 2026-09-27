package acmehint

import (
	"net/netip"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/go-acme/lego/v5/acme"
)

// Hint is an actionable explanation of an ACME failure.
type Hint struct {
	// Code is a stable machine code such as "port80_unreachable".
	Code string `json:"code"`
	// Message is one plain English sentence telling the user what to check.
	Message string `json:"message"`
	// Params carries the facts behind the hint (domain, resolved addresses, ...).
	Params map[string]string `json:"params,omitempty"`
}

// Evidence is optional context that sharpens Classify. The zero value is valid
// and simply means that nothing is known about DNS or local addresses.
type Evidence struct {
	// Resolved maps a domain to the addresses it resolved to.
	Resolved map[string][]string
	// Local lists the addresses assigned to this host's interfaces.
	Local []string
}

// problem is the normalized view of one failed ACME identifier.
type problem struct {
	domain     string
	typ        string // short ACME error type, e.g. "unauthorized"
	detail     string // text used for keyword matching
	retryAfter time.Duration
}

const acmeErrorNamespace = "urn:ietf:params:acme:error:"

var (
	domainsErrorMarker = "one or more domains had a problem:"
	// domainSegmentRe matches the "[domain: " prefix lego's DomainsError uses
	// for every failed identifier. IPv6 identifiers contain colons, hence the
	// lazy quantifier and the mandatory ": " separator.
	domainSegmentRe = regexp.MustCompile(`\[([A-Za-z0-9*._:\-]+?): `)
	acmeTypeRe      = regexp.MustCompile(`urn:ietf:params:acme:error:([A-Za-z]+)`)
	dialTCPRe       = regexp.MustCompile(`dial tcp (\[[0-9A-Fa-f:.]+\]|[0-9.]+):(\d+)`)
	// Let's Encrypt prefixes the validated address: ":: 203.0.113.7: Invalid response from ...".
	leRemoteRe       = regexp.MustCompile(`:: ([0-9A-Fa-f:.]+): (?:Invalid response|Fetching|Timeout|Connection|Error getting validation data)`)
	retryAfterTextRe = regexp.MustCompile(`(?i)retry after ([0-9]{4}-[0-9]{2}-[0-9]{2}[ T][0-9:.]+(?:Z| ?UTC)?)`)
	httpsChallengeRe = regexp.MustCompile(`https://[^\s"]*/\.well-known/acme-challenge/[^\s":]*`)
)

// codeRank orders hint codes when several identifiers failed for different
// reasons; the lowest rank is reported because it blocks everything else.
var codeRank = map[string]int{
	CodeRateLimited:         0,
	CodeCAAForbidden:        1,
	CodeIdentifierRejected:  2,
	CodeDNSNotResolved:      3,
	CodePort80Unreachable:   4,
	CodeChallengeRedirected: 5,
	CodeDNSPointsElsewhere:  6,
	CodeChallengeNotServed:  7,
	CodeUnknown:             8,
}

// Classify maps an issuance error to an actionable Hint. It returns nil for a
// nil error and a CodeUnknown hint when the failure is not recognized.
//
// Structured lego errors are preferred: the error tree is walked (the same
// traversal errors.As performs, including multi-errors such as lego's
// per-domain error map) for *acme.ProblemDetails and *acme.RateLimitedError.
// Errors whose structure was flattened into a string (nginx-ui wraps lego
// errors with cosy.WrapErrorWithParams) fall back to parsing the
// "urn:ietf:params:acme:error:<type>" markers and lego's "[domain: ...]"
// segments from the message.
func Classify(err error, ev Evidence) *Hint {
	if err == nil {
		return nil
	}
	msg := err.Error()

	problems := structuredProblems(err, msg)
	if len(problems) == 0 {
		problems = parseProblems(msg)
	}

	var best *Hint
	var failed []string
	for _, p := range problems {
		if p.domain != "" && !slices.Contains(failed, p.domain) {
			failed = append(failed, p.domain)
		}
		h := classifyProblem(p, ev)
		if best == nil || codeRank[h.Code] < codeRank[best.Code] {
			best = h
		}
	}
	if best == nil {
		best = newHint(CodeUnknown, nil)
	}
	if len(failed) > 1 {
		if best.Params == nil {
			best.Params = map[string]string{}
		}
		best.Params["failed_domains"] = strings.Join(failed, ", ")
	}
	return best
}

// structuredProblems collects ACME problem documents from the error tree.
func structuredProblems(err error, msg string) []problem {
	var problems []problem
	retry := map[*acme.ProblemDetails]time.Duration{}
	var details []*acme.ProblemDetails

	walkErrors(err, func(e error) {
		switch v := e.(type) {
		case *acme.RateLimitedError:
			if v != nil && v.ProblemDetails != nil {
				retry[v.ProblemDetails] = v.RetryAfter
			}
		case acme.RateLimitedError:
			if v.ProblemDetails != nil {
				retry[v.ProblemDetails] = v.RetryAfter
				// The value form does not implement Unwrap on its own.
				if !slices.Contains(details, v.ProblemDetails) {
					details = append(details, v.ProblemDetails)
				}
			}
		case *acme.ProblemDetails:
			if v != nil && !slices.Contains(details, v) {
				details = append(details, v)
			}
		}
	})

	segments := splitSegments(msg)
	for _, pd := range details {
		if len(pd.SubProblems) > 0 {
			for _, sub := range pd.SubProblems {
				problems = append(problems, problem{
					domain:     sub.Identifier.Value,
					typ:        shortType(sub.Type),
					detail:     sub.Detail,
					retryAfter: retry[pd],
				})
			}
			continue
		}
		problems = append(problems, problem{
			domain:     domainForText(segments, pd.Error()),
			typ:        shortType(pd.Type),
			detail:     pd.Error(),
			retryAfter: retry[pd],
		})
	}
	return problems
}

// walkErrors visits err and every error it wraps, depth first.
func walkErrors(err error, visit func(error)) {
	if err == nil {
		return
	}
	visit(err)
	switch x := err.(type) {
	case interface{ Unwrap() []error }:
		for _, e := range x.Unwrap() {
			walkErrors(e, visit)
		}
	case interface{ Unwrap() error }:
		walkErrors(x.Unwrap(), visit)
	}
}

type segment struct {
	domain string
	text   string
}

// splitSegments splits lego's "prefix: one or more domains had a problem:
// [a: err] [b: err]" message into per-domain segments. Messages without the
// marker yield a single segment with an empty domain.
func splitSegments(msg string) []segment {
	idx := strings.Index(msg, domainsErrorMarker)
	if idx < 0 {
		return []segment{{text: msg}}
	}
	body := msg[idx+len(domainsErrorMarker):]
	matches := domainSegmentRe.FindAllStringSubmatchIndex(body, -1)
	if len(matches) == 0 {
		return []segment{{text: msg}}
	}
	segments := make([]segment, 0, len(matches))
	for i, m := range matches {
		end := len(body)
		if i+1 < len(matches) {
			end = matches[i+1][0]
		}
		text := strings.TrimSpace(body[m[1]:end])
		text = strings.TrimSuffix(text, "]")
		segments = append(segments, segment{domain: body[m[2]:m[3]], text: text})
	}
	return segments
}

func domainForText(segments []segment, text string) string {
	for _, s := range segments {
		if s.domain != "" && strings.Contains(s.text, text) {
			return s.domain
		}
	}
	return ""
}

func parseProblems(msg string) []problem {
	segments := splitSegments(msg)
	problems := make([]problem, 0, len(segments))
	for _, s := range segments {
		p := problem{domain: s.domain, detail: s.text}
		if m := acmeTypeRe.FindStringSubmatch(s.text); m != nil {
			p.typ = m[1]
		}
		problems = append(problems, p)
	}
	return problems
}

func shortType(t string) string {
	return strings.TrimPrefix(t, acmeErrorNamespace)
}

func classifyProblem(p problem, ev Evidence) *Hint {
	d := strings.ToLower(p.detail)
	params := map[string]string{}
	if p.domain != "" {
		params["domain"] = p.domain
	}
	if ip := remoteIP(p.detail); ip != "" {
		params["remote_ip"] = ip
	}

	switch {
	case p.typ == "rateLimited" || strings.Contains(d, "too many certificates") ||
		strings.Contains(d, "too many failed authorizations") || strings.Contains(d, "rate limit"):
		if p.retryAfter > 0 {
			params["retry_after"] = p.retryAfter.Round(time.Second).String()
		} else if m := retryAfterTextRe.FindStringSubmatch(p.detail); m != nil {
			params["retry_after"] = strings.TrimSpace(m[1])
		}
		return newHint(CodeRateLimited, params)

	case p.typ == "caa" || (strings.Contains(d, "caa record") && strings.Contains(d, "prevents issuance")):
		return newHint(CodeCAAForbidden, params)

	case p.typ == "rejectedIdentifier" || p.typ == "unsupportedIdentifier":
		return newHint(CodeIdentifierRejected, params)

	case p.typ == "dns" || strings.Contains(d, "nxdomain") || strings.Contains(d, "servfail") ||
		strings.Contains(d, "no valid a records") || strings.Contains(d, "no such host") ||
		strings.Contains(d, "dns problem"):
		return newHint(CodeDNSNotResolved, params)

	case strings.Contains(d, "redirect") || httpsChallengeRe.MatchString(p.detail):
		// The CA follows redirects (Let's Encrypt: up to 10, ports 80/443), so
		// a redirect is only reported when the redirected request failed.
		if u := httpsChallengeRe.FindString(p.detail); u != "" {
			params["url"] = u
		}
		return newHint(CodeChallengeRedirected, params)

	case p.typ == "connection" || isConnectionFailure(d):
		if m := dialTCPRe.FindStringSubmatch(p.detail); m != nil {
			params["port"] = m[2]
		}
		return newHint(CodePort80Unreachable, params)

	case p.typ == "unauthorized" || p.typ == "incorrectResponse" ||
		strings.Contains(d, "invalid response") || strings.Contains(d, "non-200 status"):
		if resolved, local, elsewhere := pointsElsewhere(p.domain, params["remote_ip"], ev); len(elsewhere) > 0 {
			if len(resolved) > 0 {
				params["resolved"] = strings.Join(resolved, ", ")
			}
			params["local"] = strings.Join(local, ", ")
			params["elsewhere"] = strings.Join(elsewhere, ", ")
			return newHint(CodeDNSPointsElsewhere, params)
		}
		return newHint(CodeChallengeNotServed, params)
	}

	return newHint(CodeUnknown, params)
}

func isConnectionFailure(d string) bool {
	for _, k := range []string{
		"connection refused", "no route to host", "i/o timeout", "timeout during connect",
		"network is unreachable", "connection reset", "host is down",
	} {
		if strings.Contains(d, k) {
			return true
		}
	}
	return false
}

// remoteIP extracts the address the CA connected to, when the error names it.
func remoteIP(detail string) string {
	if m := dialTCPRe.FindStringSubmatch(detail); m != nil {
		if a, err := netip.ParseAddr(strings.Trim(m[1], "[]")); err == nil {
			return a.Unmap().String()
		}
	}
	if m := leRemoteRe.FindStringSubmatch(detail); m != nil {
		if a, err := netip.ParseAddr(m[1]); err == nil {
			return a.Unmap().String()
		}
	}
	return ""
}

// pointsElsewhere reports the addresses of domain (or the CA-reported remote
// address) that are not local. It returns nothing when the local addresses are
// unknown, because then no comparison is possible.
func pointsElsewhere(domain, remote string, ev Evidence) (resolved, local, elsewhere []string) {
	localSet := addrSet(ev.Local)
	if len(localSet) == 0 {
		return nil, nil, nil
	}
	local = sortedKeys(localSet)

	if domain != "" {
		resolved = ev.Resolved[domain]
	} else if len(ev.Resolved) == 1 {
		// Single identifier without a domain prefix in the message.
		for _, v := range ev.Resolved {
			resolved = v
		}
	}

	candidates := append([]string{}, resolved...)
	if remote != "" {
		candidates = append(candidates, remote)
	}
	for _, c := range candidates {
		a, err := netip.ParseAddr(c)
		if err != nil {
			continue
		}
		a = a.Unmap()
		if _, ok := localSet[a]; !ok && !slices.Contains(elsewhere, a.String()) {
			elsewhere = append(elsewhere, a.String())
		}
	}
	return resolved, local, elsewhere
}

// addrSet builds the set of usable local addresses. Loopback, link-local and
// unspecified addresses are dropped: a public CA can never reach them, so a
// domain resolving to one of them must not count as "points to this server".
func addrSet(addrs []string) map[netip.Addr]struct{} {
	set := make(map[netip.Addr]struct{}, len(addrs))
	for _, s := range addrs {
		a, err := netip.ParseAddr(strings.TrimSpace(s))
		if err != nil {
			continue
		}
		a = a.Unmap()
		if a.IsLoopback() || a.IsLinkLocalUnicast() || a.IsUnspecified() {
			continue
		}
		set[a] = struct{}{}
	}
	return set
}

func sortedKeys(set map[netip.Addr]struct{}) []string {
	addrs := make([]netip.Addr, 0, len(set))
	for a := range set {
		addrs = append(addrs, a)
	}
	slices.SortFunc(addrs, func(a, b netip.Addr) int { return a.Compare(b) })
	out := make([]string, len(addrs))
	for i, a := range addrs {
		out[i] = a.String()
	}
	return out
}

func newHint(code string, params map[string]string) *Hint {
	if len(params) == 0 {
		params = nil
	}
	return &Hint{Code: code, Message: Message(code), Params: params}
}
