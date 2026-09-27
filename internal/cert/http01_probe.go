package cert

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/settings"
)

// The HTTP-01 route probe answers one question before lego talks to the CA:
// does a request that arrives at the local Nginx on port 80 with
// "Host: <domain>" end up at the challenge server nginx-ui runs on
// settings.CertSettings.HTTPChallengePort?
//
// It binds the challenge port exactly like lego's http01.NewProviderServer,
// serves a random token and fetches it through Nginx. Unlike a static reading
// of the site file, it follows whatever Nginx really does: include files,
// "proxy_pass http://localhost:<port>", regex server names, upstream blocks.
//
// Redirects are followed the way the Let's Encrypt validator follows them:
// at most 10, only to http/https URLs on ports 80 or 443, never to IP-address
// hosts, and without certificate verification on HTTPS. Every hop is sent to
// the local Nginx again (port 80 -> the endpoint's HTTP address, port 443 ->
// its HTTPS address) with Host and SNI set to the redirect host.
//
// Limits:
//   - It only runs when Nginx is controlled locally. In external container or
//     SSH host mode, the loopback address of nginx-ui is not the one of Nginx,
//     so the probe reports HTTP01ProbeStatusSkipped.
//   - It targets the loopback addresses and the explicit port-80 listen
//     addresses of the site (when a config name is given). It cannot see a
//     firewall, NAT, CDN or DNS record between the CA and this host.
//   - A domain passes when any target serves the token, so a server bound to
//     a public address that is not listed in the site file (for example in an
//     include) is only covered through the loopback targets.
//   - A redirect to another host that the local Nginx does not answer for is
//     reported as HTTP01ProbeStatusWarning: that host may live elsewhere.
//
// Settling: "nginx -s reload" returns as soon as the master process has been
// signalled, before the new workers accept connections. For a short window
// the old workers keep answering with the previous configuration, so a probe
// sent right after a site was saved or enabled can see the old routing (for
// example the default server's 404). The probe therefore polls every domain
// until it passes or the settle deadline expires, keeping the challenge
// server up for the whole window, and only reports the last observed result
// after the deadline. A domain passes only after http01ProbeRequiredSuccesses
// consecutive successes on fresh connections, so an old and a new worker
// that briefly accept side by side cannot produce a false pass: once the
// probe succeeds, the CA's validation requests hit the new configuration.

const (
	http01ProbePathPrefix   = "/.well-known/acme-challenge/"
	http01ProbeTokenPrefix  = "nginxui-probe-"
	http01ProbeMaxBodyBytes = 4096
	http01ProbeMaxRedirects = 10
	http01ProbeLockPoll     = 50 * time.Millisecond
	// http01ProbeRequiredSuccesses is the number of consecutive successful
	// rounds a domain needs before it passes (see "Settling" above).
	http01ProbeRequiredSuccesses = 2
)

// HTTP01ProbeStatus is the outcome of the probe for one domain.
type HTTP01ProbeStatus string

const (
	// HTTP01ProbeStatusSuccess means the token was served through Nginx.
	HTTP01ProbeStatusSuccess HTTP01ProbeStatus = "success"
	// HTTP01ProbeStatusFailure means the CA would fail the same way.
	HTTP01ProbeStatusFailure HTTP01ProbeStatus = "failure"
	// HTTP01ProbeStatusWarning means the route leaves this host (a redirect
	// to another host the local Nginx does not serve the token for), so it
	// cannot be verified locally. Issuance is not blocked.
	HTTP01ProbeStatusWarning HTTP01ProbeStatus = "warning"
	// HTTP01ProbeStatusSkipped means the probe did not run (Nginx not local).
	HTTP01ProbeStatusSkipped HTTP01ProbeStatus = "skipped"
)

// HTTP01ProbeAttempt records the requests sent through one local endpoint.
type HTTP01ProbeAttempt struct {
	// Target is the local address that received the first, port-80 request.
	Target string            `json:"target"`
	Status HTTP01ProbeStatus `json:"status"`
	// StatusCode and Location come from the last response received.
	StatusCode int    `json:"status_code,omitempty"`
	Location   string `json:"location,omitempty"`
	// Redirects lists the absolute URLs that were followed, in order.
	Redirects []string `json:"redirects,omitempty"`
	Error     string   `json:"error,omitempty"`
}

// HTTP01ProbeResult is the probe outcome for one domain. Target, StatusCode,
// Location, Redirects and Error describe the deciding attempt: the successful
// one, otherwise the most informative one. Attempts lists every endpoint tried.
type HTTP01ProbeResult struct {
	Domain     string            `json:"domain"`
	Status     HTTP01ProbeStatus `json:"status"`
	Target     string            `json:"target,omitempty"`
	StatusCode int               `json:"status_code,omitempty"`
	Location   string            `json:"location,omitempty"`
	Redirects  []string          `json:"redirects,omitempty"`
	Error      string            `json:"error,omitempty"`
	SkipReason string            `json:"skip_reason,omitempty"`
	// Attempts lists every endpoint tried in the last round.
	Attempts []HTTP01ProbeAttempt `json:"attempts,omitempty"`
	// Rounds is the number of probe rounds run for this domain while
	// waiting for Nginx to settle.
	Rounds int `json:"rounds,omitempty"`

	// final marks a result that no amount of waiting can change (e.g. a
	// wildcard identifier), so the domain is not probed again.
	final bool
}

// HTTP01ProbeOption customizes a probe run.
type HTTP01ProbeOption func(*http01ProbeConfig)

type http01ProbeConfig struct {
	configName    string
	settleTimeout time.Duration
}

// WithHTTP01ProbeConfigName adds the explicit port-80 listen addresses of the
// enabled site configName (e.g. "listen 192.0.2.10:80;") to the probe
// targets, so servers that do not listen on the wildcard address are covered.
func WithHTTP01ProbeConfigName(configName string) HTTP01ProbeOption {
	return func(c *http01ProbeConfig) {
		c.configName = configName
	}
}

// WithHTTP01ProbeSettleTimeout overrides how long a failing domain is probed
// again while waiting for a reload of Nginx to take effect. Zero or less
// runs a single round (plus the confirmation round of a success); use it
// when no reload happened just before the probe.
func WithHTTP01ProbeSettleTimeout(timeout time.Duration) HTTP01ProbeOption {
	return func(c *http01ProbeConfig) {
		c.settleTimeout = timeout
	}
}

// http01ProbeEndpoint is one local address pair of Nginx: HTTP receives the
// requests the CA sends to port 80, HTTPS the ones redirected to port 443.
type http01ProbeEndpoint struct {
	HTTP  string
	HTTPS string
}

// Test seams. Production code never reassigns them.
var (
	// http01ProbeListenHost mirrors the empty interface lego passes to
	// http01.NewProviderServer, i.e. all addresses.
	http01ProbeListenHost = ""
	// http01ProbeChallengePort returns the port lego binds for HTTP-01.
	http01ProbeChallengePort = func() string {
		return settings.CertSettings.HTTPChallengePort
	}
	// http01ProbeDefaultEndpoints are the addresses Nginx is expected to
	// accept traffic on. IPv6 loopback covers IPv6-only listeners.
	http01ProbeDefaultEndpoints = func() []http01ProbeEndpoint {
		return []http01ProbeEndpoint{
			{HTTP: "127.0.0.1:80", HTTPS: "127.0.0.1:443"},
			{HTTP: "[::1]:80", HTTPS: "[::1]:443"},
		}
	}
	// http01ProbeSiteEndpoints derives extra endpoints from the enabled site.
	http01ProbeSiteEndpoints = enabledSiteHTTPListenEndpoints
	// http01ProbeSkipReason returns a non-empty reason when the loopback of
	// nginx-ui is not the loopback of Nginx.
	http01ProbeSkipReason = func() string {
		mode := settings.NginxSettings.ControlMode()
		if mode == settings.ControlModeLocal {
			return ""
		}
		return fmt.Sprintf("Nginx is not local to nginx-ui (control mode: %s)", mode)
	}
	http01ProbeRequestTimeout = 5 * time.Second
	// http01ProbeSettleTimeout is how long a failing domain is probed again
	// after the first round, waiting for a reload of Nginx to take effect.
	http01ProbeSettleTimeout = 5 * time.Second
	// http01ProbeSettleInterval is the pause between two probe rounds.
	http01ProbeSettleInterval = 250 * time.Millisecond
)

// ProbeHTTP01Routes verifies through the local Nginx that the HTTP-01 route of
// every domain reaches the nginx-ui challenge port. It takes the certificate
// issuance lock, so it never races lego for the challenge port.
//
// The returned error is non-nil only when the probe could not run: the lock
// was not acquired before ctx ended, or the challenge port could not be bound
// (a cosy error with code 50059, see NewHTTP01ChallengePortUnavailableError).
// Route problems are reported per domain (HTTP01ProbeStatusFailure or
// HTTP01ProbeStatusWarning); use HTTP01RouteCheckError to turn failures into
// an error. When Nginx is not local, every result has
// HTTP01ProbeStatusSkipped and the error is nil.
func ProbeHTTP01Routes(ctx context.Context, domains []string, opts ...HTTP01ProbeOption) ([]HTTP01ProbeResult, error) {
	if reason := http01ProbeSkipReason(); reason != "" {
		return skippedHTTP01ProbeResults(domains, reason), nil
	}
	// Only the mutex is taken here: the probe is not an issuance, so the
	// "certificate processing" status shown in the UI is left untouched.
	if err := lockMutexWithContext(ctx); err != nil {
		return nil, err
	}
	defer mutex.Unlock()
	return probeHTTP01Routes(ctx, domains, opts...)
}

// probeHTTP01Routes is ProbeHTTP01Routes for callers already holding the
// certificate lock (IssueCert).
func probeHTTP01Routes(ctx context.Context, domains []string, opts ...HTTP01ProbeOption) ([]HTTP01ProbeResult, error) {
	if reason := http01ProbeSkipReason(); reason != "" {
		return skippedHTTP01ProbeResults(domains, reason), nil
	}

	cfg := &http01ProbeConfig{settleTimeout: http01ProbeSettleTimeout}
	for _, opt := range opts {
		opt(cfg)
	}

	port := strings.TrimSpace(http01ProbeChallengePort())
	if port == "" {
		return nil, NewHTTP01ChallengePortUnavailableError(port, "the HTTP-01 challenge port is not configured")
	}

	token, err := randomHTTP01ProbeString()
	if err != nil {
		return nil, err
	}
	body, err := randomHTTP01ProbeString()
	if err != nil {
		return nil, err
	}
	path := http01ProbePathPrefix + http01ProbeTokenPrefix + token

	listener, err := net.Listen("tcp", net.JoinHostPort(http01ProbeListenHost, port))
	if err != nil {
		return nil, NewHTTP01ChallengePortUnavailableError(port, err.Error())
	}
	server := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet || r.URL.Path != path {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", "text/plain")
			_, _ = w.Write([]byte(body))
		}),
		ReadHeaderTimeout: http01ProbeRequestTimeout,
	}
	go func() {
		_ = server.Serve(listener)
	}()
	// Close releases the listener synchronously, so lego can bind the port
	// right after this function returns.
	defer server.Close()

	endpoints := http01ProbeEndpoints(cfg)
	results := settleHTTP01Probe(ctx, domains, cfg.settleTimeout, func(domain string) HTTP01ProbeResult {
		return probeHTTP01Domain(ctx, endpoints, domain, path, body)
	})
	return results, nil
}

// settleHTTP01Probe probes every domain in rounds until each one has passed
// http01ProbeRequiredSuccesses consecutive rounds, ended with a result that
// waiting cannot change (a warning or a final failure), or the settle
// deadline expired. A failure resets the success streak. A domain whose last
// round succeeded when the deadline expires still gets its confirmation
// round, so a success reported after the deadline has been confirmed too;
// ctx bounds everything. Each domain reports its last observed result.
func settleHTTP01Probe(ctx context.Context, domains []string, settle time.Duration, probeOnce func(domain string) HTTP01ProbeResult) []HTTP01ProbeResult {
	results := make([]HTTP01ProbeResult, len(domains))
	streaks := make([]int, len(domains))
	done := make([]bool, len(domains))
	deadline := time.Now().Add(settle)

	for round := 1; ; round++ {
		pending, confirming := false, false
		for i, domain := range domains {
			if done[i] {
				continue
			}
			result := probeOnce(domain)
			if round > 1 && ctx.Err() != nil {
				// The round was cut short: keep the previous, complete result.
				return results
			}
			result.Rounds = round
			results[i] = result

			switch {
			case result.Status == HTTP01ProbeStatusSuccess:
				streaks[i]++
				done[i] = streaks[i] >= http01ProbeRequiredSuccesses
				confirming = confirming || !done[i]
			case result.Status == HTTP01ProbeStatusWarning || result.final:
				done[i] = true
			default:
				streaks[i] = 0
			}
			pending = pending || !done[i]
		}

		if !pending || ctx.Err() != nil {
			return results
		}
		if !confirming && !time.Now().Before(deadline) {
			return results
		}

		timer := time.NewTimer(http01ProbeSettleInterval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return results
		case <-timer.C:
		}
	}
}

// HTTP01RouteCheckError returns nil when no result failed. Otherwise it
// returns a cosy error (code 50058) for the first failing domain. When
// configName is set, the static analyzer's view of the enabled site is
// appended as a possible cause. Warnings and skipped results never fail.
func HTTP01RouteCheckError(results []HTTP01ProbeResult, configName string) error {
	for _, result := range results {
		if result.Status != HTTP01ProbeStatusFailure {
			continue
		}
		reason := describeHTTP01ProbeFailure(result)
		if cause := explainHTTP01RouteFailure(configName, result.Domain); cause != "" {
			reason += "; possible cause: " + cause
		}
		return NewHTTP01ChallengeRouteCheckError(result.Domain, reason)
	}
	return nil
}

// HTTP01ProbeSkipped reports whether the probe did not run.
func HTTP01ProbeSkipped(results []HTTP01ProbeResult) bool {
	return len(results) > 0 && results[0].Status == HTTP01ProbeStatusSkipped
}

// SummarizeHTTP01ProbeResults renders the results as one short line for logs
// and notifications.
func SummarizeHTTP01ProbeResults(results []HTTP01ProbeResult) string {
	if len(results) == 0 {
		return "no domains checked"
	}
	if HTTP01ProbeSkipped(results) {
		return "skipped: " + results[0].SkipReason
	}
	parts := make([]string, 0, len(results))
	for _, result := range results {
		switch result.Status {
		case HTTP01ProbeStatusSuccess:
			parts = append(parts, fmt.Sprintf("%s: ok via %s", result.Domain, result.Target))
		case HTTP01ProbeStatusWarning:
			parts = append(parts, fmt.Sprintf("%s: cannot verify locally: %s", result.Domain, describeHTTP01ProbeFailure(result)))
		default:
			parts = append(parts, fmt.Sprintf("%s: %s", result.Domain, describeHTTP01ProbeFailure(result)))
		}
	}
	return strings.Join(parts, "; ")
}

func describeHTTP01ProbeFailure(result HTTP01ProbeResult) string {
	if result.Target == "" {
		return result.Error
	}
	return fmt.Sprintf("%s (via %s)", result.Error, result.Target)
}

func skippedHTTP01ProbeResults(domains []string, reason string) []HTTP01ProbeResult {
	results := make([]HTTP01ProbeResult, 0, len(domains))
	for _, domain := range domains {
		results = append(results, HTTP01ProbeResult{
			Domain:     domain,
			Status:     HTTP01ProbeStatusSkipped,
			SkipReason: reason,
		})
	}
	return results
}

func lockMutexWithContext(ctx context.Context) error {
	for !mutex.TryLock() {
		timer := time.NewTimer(http01ProbeLockPoll)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
	return nil
}

func randomHTTP01ProbeString() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

// newHTTP01ProbeClient returns a client that sends every request, whatever
// its URL host, to the local endpoint: port 443 to endpoint.HTTPS and every
// other port to endpoint.HTTP. Redirects are handled by the caller.
func newHTTP01ProbeClient(endpoint http01ProbeEndpoint) *http.Client {
	dialer := &net.Dialer{Timeout: http01ProbeRequestTimeout}
	dial := func(ctx context.Context, _ string, address string) (net.Conn, error) {
		local := endpoint.HTTP
		if _, port, err := net.SplitHostPort(address); err == nil && port == "443" {
			local = endpoint.HTTPS
		}
		if local == "" {
			return nil, fmt.Errorf("no local address for %s", address)
		}
		return dialer.DialContext(ctx, "tcp", local)
	}
	return &http.Client{
		Timeout: http01ProbeRequestTimeout,
		Transport: &http.Transport{
			// Never go through an HTTP proxy: the request must hit local Nginx.
			Proxy:       nil,
			DialContext: dial,
			// Like the CA, accept any certificate after a redirect to HTTPS.
			// SNI still carries the redirect host.
			TLSClientConfig:     &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // mirrors the ACME validator
			TLSHandshakeTimeout: http01ProbeRequestTimeout,
			DisableKeepAlives:   true,
		},
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

// http01ProbeEndpoints returns the site's explicit listen addresses first,
// then the default loopback endpoints, without duplicates.
func http01ProbeEndpoints(cfg *http01ProbeConfig) []http01ProbeEndpoint {
	var endpoints []http01ProbeEndpoint
	seen := map[string]bool{}
	add := func(endpoint http01ProbeEndpoint) {
		if endpoint.HTTP == "" || seen[endpoint.HTTP] {
			return
		}
		seen[endpoint.HTTP] = true
		endpoints = append(endpoints, endpoint)
	}
	if cfg != nil && cfg.configName != "" {
		for _, endpoint := range http01ProbeSiteEndpoints(cfg.configName) {
			add(endpoint)
		}
	}
	for _, endpoint := range http01ProbeDefaultEndpoints() {
		add(endpoint)
	}
	return endpoints
}

func probeHTTP01Domain(ctx context.Context, endpoints []http01ProbeEndpoint, domain, path, body string) HTTP01ProbeResult {
	result := HTTP01ProbeResult{Domain: domain, Status: HTTP01ProbeStatusFailure}
	if strings.HasPrefix(domain, "*.") {
		result.Error = "wildcard identifiers cannot be validated with HTTP-01"
		result.final = true
		return result
	}
	if len(endpoints) == 0 {
		result.Error = "no local Nginx address to probe"
		result.final = true
		return result
	}

	for _, endpoint := range endpoints {
		attempt := probeHTTP01Endpoint(ctx, endpoint, domain, path, body)
		result.Attempts = append(result.Attempts, attempt)
		if attempt.Status == HTTP01ProbeStatusSuccess || ctx.Err() != nil {
			break
		}
	}

	deciding := decidingHTTP01Attempt(result.Attempts)
	result.Status = deciding.Status
	result.Target = deciding.Target
	result.StatusCode = deciding.StatusCode
	result.Location = deciding.Location
	result.Redirects = deciding.Redirects
	result.Error = deciding.Error
	return result
}

// decidingHTTP01Attempt picks a success, else a warning, else the first
// failure that got an HTTP answer: "404 from 127.0.0.1:80" says more than
// "connection refused on [::1]:80".
func decidingHTTP01Attempt(attempts []HTTP01ProbeAttempt) HTTP01ProbeAttempt {
	rank := func(attempt HTTP01ProbeAttempt) int {
		switch {
		case attempt.Status == HTTP01ProbeStatusSuccess:
			return 3
		case attempt.Status == HTTP01ProbeStatusWarning:
			return 2
		case attempt.StatusCode != 0:
			return 1
		default:
			return 0
		}
	}
	deciding := attempts[0]
	for _, attempt := range attempts[1:] {
		if rank(attempt) > rank(deciding) {
			deciding = attempt
		}
	}
	return deciding
}

// probeHTTP01Endpoint fetches the token for domain through one local
// endpoint, following redirects under the CA's rules.
func probeHTTP01Endpoint(ctx context.Context, endpoint http01ProbeEndpoint, domain, path, body string) HTTP01ProbeAttempt {
	attempt := HTTP01ProbeAttempt{Target: endpoint.HTTP, Status: HTTP01ProbeStatusFailure}
	client := newHTTP01ProbeClient(endpoint)
	defer client.CloseIdleConnections()

	current := &url.URL{Scheme: "http", Host: http01ProbeHostHeader(domain), Path: path}
	initial := current.String()
	visited := map[string]bool{initial: true}

	withChain := func(message string) string {
		if len(attempt.Redirects) == 0 {
			return message
		}
		chain := append([]string{initial}, attempt.Redirects...)
		return fmt.Sprintf("%s (redirect chain: %s)", message, strings.Join(chain, " -> "))
	}

	for {
		statusCode, location, gotBody, err := fetchHTTP01Probe(ctx, client, current)
		if err != nil {
			attempt.Error = err.Error()
			break
		}
		attempt.StatusCode = statusCode

		if statusCode >= 300 && statusCode < 400 {
			attempt.Location = location
			next, reason := nextHTTP01ProbeRedirect(current, location, len(attempt.Redirects), visited)
			if reason != "" {
				// The CA refuses this redirect as well: a real failure.
				attempt.Error = withChain(reason)
				return attempt
			}
			visited[next.String()] = true
			attempt.Redirects = append(attempt.Redirects, next.String())
			current = next
			continue
		}

		switch {
		case statusCode != http.StatusOK:
			attempt.Error = fmt.Sprintf("unexpected status %d", statusCode)
		// ACME servers ignore trailing whitespace in the key authorization.
		case strings.TrimRight(gotBody, " \t\r\n") != body:
			attempt.Error = "status 200 but the response body is not the challenge token, so the request did not reach the nginx-ui challenge server"
		default:
			attempt.Status = HTTP01ProbeStatusSuccess
			return attempt
		}
		break
	}

	if host := current.Hostname(); len(attempt.Redirects) > 0 && !strings.EqualFold(host, domain) {
		attempt.Status = HTTP01ProbeStatusWarning
		attempt.Error = withChain(fmt.Sprintf(
			"redirected to %s, which the local Nginx did not answer for (%s); the CA will contact that host directly, so the route cannot be verified locally",
			host, attempt.Error))
		return attempt
	}
	attempt.Error = withChain(attempt.Error)
	return attempt
}

func fetchHTTP01Probe(ctx context.Context, client *http.Client, target *url.URL) (statusCode int, location, body string, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
	if err != nil {
		return 0, "", "", err
	}
	req.Header.Set("User-Agent", "nginx-ui-http01-probe")

	resp, err := client.Do(req)
	if err != nil {
		var urlErr *url.Error
		if errors.As(err, &urlErr) && urlErr.Err != nil {
			err = urlErr.Err
		}
		return 0, "", "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		return resp.StatusCode, resp.Header.Get("Location"), "", nil
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, http01ProbeMaxBodyBytes))
	if err != nil {
		return resp.StatusCode, "", "", fmt.Errorf("read response body: %w", err)
	}
	return resp.StatusCode, "", string(data), nil
}

// nextHTTP01ProbeRedirect resolves a Location header and applies the CA's
// redirect rules. It returns a non-empty reason when the CA would refuse it.
func nextHTTP01ProbeRedirect(current *url.URL, location string, followed int, visited map[string]bool) (*url.URL, string) {
	if followed >= http01ProbeMaxRedirects {
		return nil, fmt.Sprintf("too many redirects: the CA follows at most %d", http01ProbeMaxRedirects)
	}
	if strings.TrimSpace(location) == "" {
		return nil, "redirect without a Location header"
	}
	next, err := current.Parse(location)
	if err != nil {
		return nil, fmt.Sprintf("invalid redirect Location %q", location)
	}
	next.Scheme = strings.ToLower(next.Scheme)
	next.Fragment = ""
	if next.Scheme != "http" && next.Scheme != "https" {
		return nil, fmt.Sprintf("redirect to %q: the CA only follows http and https redirects", location)
	}
	host := next.Hostname()
	if host == "" {
		return nil, fmt.Sprintf("invalid redirect Location %q", location)
	}
	if net.ParseIP(host) != nil {
		return nil, fmt.Sprintf("redirect to IP address %s: the CA only follows redirects to domain names", host)
	}
	if port := next.Port(); port != "" && port != "80" && port != "443" {
		return nil, fmt.Sprintf("redirect to port %s: the CA only follows redirects to ports 80 and 443", port)
	}
	if visited[next.String()] {
		return nil, fmt.Sprintf("redirect loop at %s", next.String())
	}
	return next, ""
}

func http01ProbeHostHeader(domain string) string {
	if ip := net.ParseIP(domain); ip != nil && ip.To4() == nil {
		return "[" + domain + "]"
	}
	return domain
}

// enabledSiteHTTPListenEndpoints reads the enabled configuration of
// configName and returns an endpoint for every explicit, non-wildcard
// port-80 listen address; its HTTPS side is the same host on port 443.
// Wildcard listeners are covered by the default loopback endpoints.
// Unreadable files yield no extra endpoints.
func enabledSiteHTTPListenEndpoints(configName string) []http01ProbeEndpoint {
	candidates, err := enabledSiteConfigCandidates(configName)
	if err != nil {
		return nil
	}
	var endpoints []http01ProbeEndpoint
	for _, candidate := range candidates {
		exists, err := nginx.Exists(candidate)
		if err != nil || !exists {
			continue
		}
		parsed, err := nginx.ParseNgxConfig(candidate)
		if err != nil || parsed == nil {
			continue
		}
		for _, target := range httpListenTargets(parsed) {
			host, _, _ := net.SplitHostPort(target)
			endpoints = append(endpoints, http01ProbeEndpoint{
				HTTP:  target,
				HTTPS: net.JoinHostPort(host, "443"),
			})
		}
	}
	return endpoints
}

func httpListenTargets(config *nginx.NgxConfig) []string {
	var targets []string
	for _, server := range config.Servers {
		if server == nil {
			continue
		}
		for _, directive := range server.Directives {
			if directive.Directive != "listen" {
				continue
			}
			if target := httpListenTarget(directive.Params); target != "" {
				targets = append(targets, target)
			}
		}
	}
	return targets
}

// httpListenTarget maps the address part of a listen directive to a probe
// target, or "" when it is not an explicit port-80 address.
func httpListenTarget(params string) string {
	fields := strings.Fields(strings.TrimSuffix(strings.TrimSpace(params), ";"))
	if len(fields) == 0 {
		return ""
	}
	address := fields[0]
	if strings.HasPrefix(address, "unix:") {
		return ""
	}
	if isAllDigits(address) {
		// "listen 80;" binds the wildcard address.
		return ""
	}

	host, port, err := net.SplitHostPort(address)
	if err != nil {
		// "listen 192.0.2.10;" and "listen [2001:db8::1];" default to port 80.
		host = strings.TrimSuffix(strings.TrimPrefix(address, "["), "]")
		port = "80"
	}
	if port != "80" {
		return ""
	}
	switch host {
	case "", "*", "0.0.0.0", "::":
		return ""
	}
	return net.JoinHostPort(host, port)
}

func isAllDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
