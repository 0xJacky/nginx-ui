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
// Endpoints: for every domain, the probe reads the effective configuration
// (nginx -T) and asks which listening sockets on port 80 serve the domain,
// by name or as the default server. Each socket becomes one local target: a
// specific listen address is dialed as is, a wildcard IPv4 socket through
// 127.0.0.1 and a wildcard IPv6 socket through [::1]. The name of a site or
// certificate plays no role, so a certificate whose SANs live in several
// server blocks is covered too.
//
// Redirects are followed the way the Let's Encrypt validator follows them:
// at most 10, only to http/https URLs on ports 80 or 443, never to IP-address
// hosts, and without certificate verification on HTTPS. Every hop is sent to
// the local Nginx again, to the sockets that serve the redirect host on the
// redirect port, with Host and SNI set to the redirect host.
//
// Outcomes: a domain fails only on HTTP evidence, i.e. the local Nginx
// answered, but not with the token (unexpected status, wrong body, a redirect
// the CA refuses). Anything that yields no HTTP answer at all (connection
// refused, timeout, TLS handshake error, no server block listening on port 80
// for the domain, an unreadable configuration) is a warning: the CA may reach
// the domain in a way this host cannot reproduce, for example through a load
// balancer or a port mapping.
//
// Limits:
//   - It only runs when Nginx is controlled locally. In external container or
//     SSH host mode, the loopback address of nginx-ui is not the one of Nginx,
//     so the probe reports HTTP01ProbeStatusSkipped.
//   - It cannot see a firewall, NAT, CDN or DNS record between the CA and
//     this host.
//   - When nginx -T cannot be read, it falls back to the explicit listen
//     addresses of the enabled site (when a config name is given) and to the
//     loopback addresses. Without the configuration it cannot tell which
//     server block answered, so a wrong answer is only a warning then.
//   - A redirect to another host that the local Nginx does not answer for is
//     reported as HTTP01ProbeStatusWarning: that host may live elsewhere.
//
// Settling: "nginx -s reload" returns as soon as the master process has been
// signalled, before the new workers accept connections. For a short window
// the old workers keep answering with the previous configuration, so a probe
// sent right after a site was saved or enabled can see the old routing (for
// example the default server's 404). The probe therefore polls every failing
// domain until it passes or the settle deadline expires, keeping the challenge
// server up for the whole window, and only reports the last observed result
// after the deadline. Warnings are not polled again: a reload keeps the
// listening sockets open, so a missing answer is not caused by it. A domain
// passes only after http01ProbeRequiredSuccesses consecutive successes on
// fresh connections, so an old and a new worker that briefly accept side by
// side cannot produce a false pass: once the probe succeeds, the CA's
// validation requests hit the new configuration.

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
	// HTTP01ProbeStatusFailure means the local Nginx answered, but not with
	// the token (unexpected status, wrong body, a redirect the CA refuses),
	// so the CA would fail the same way.
	HTTP01ProbeStatusFailure HTTP01ProbeStatus = "failure"
	// HTTP01ProbeStatusWarning means the route cannot be verified locally: no
	// HTTP answer came back (connection refused, timeout, TLS handshake
	// error), no server block listens on port 80 for the domain, the local
	// configuration could not be read, or the route leaves this host through
	// a redirect. Issuance is not blocked.
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

	// socket is the port-80 socket of the first request, hop the socket
	// that answered the last request of a redirect chain; both are nil when
	// the configuration is unknown. They explain a failure.
	socket *nginx.ServerSocket
	hop    *http01HopTarget
}

// HTTP01ProbeResult is the probe outcome for one domain. Target, StatusCode,
// Location, Redirects and Error describe the deciding attempt (see
// aggregateHTTP01Attempts): the first failure, else the first success, else
// the most informative warning. Attempts lists every endpoint tried.
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
	// cause is the static explanation of a failure, derived from the server
	// block Nginx selects for the domain on the socket that failed.
	cause string
	// deciding is the attempt the result was taken from.
	deciding HTTP01ProbeAttempt
}

// HTTP01ProbeOption customizes a probe run.
type HTTP01ProbeOption func(*http01ProbeConfig)

type http01ProbeConfig struct {
	configName    string
	settleTimeout time.Duration
}

// WithHTTP01ProbeConfigName names the enabled site whose explicit port-80
// listen addresses (e.g. "listen 192.0.2.10:80;") are probed when the
// effective configuration (nginx -T) cannot be read. Endpoint discovery
// otherwise works per domain and does not need it.
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
// requests the CA sends to port 80, HTTPS the ones redirected to port 443
// when the configuration is unknown (otherwise port-443 hops go to the
// sockets that serve the redirect host).
type http01ProbeEndpoint struct {
	HTTP  string
	HTTPS string
	// socket is the port-80 socket HTTP reaches, nil when the configuration
	// is unknown.
	socket *nginx.ServerSocket
	// guessed marks a loopback default probed without knowing the
	// configuration: its answer may come from another server block than the
	// one the CA reaches, so it can only prove success.
	guessed bool
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
	// http01ProbeListen binds the challenge address for the probe.
	http01ProbeListen = listenHTTP01Challenge
	// http01ProbeServerBlocks reads the server blocks of the effective
	// configuration (nginx -T).
	http01ProbeServerBlocks = nginx.GetServerBlocks
	// http01ProbeSocketAddress maps a listening socket to the address the
	// probe dials to reach it.
	http01ProbeSocketAddress = http01SocketDialAddress
	// http01ProbeDefaultEndpoints are the last resort when the configuration
	// cannot be read. IPv6 loopback covers IPv6-only listeners.
	http01ProbeDefaultEndpoints = func() []http01ProbeEndpoint {
		return []http01ProbeEndpoint{
			{HTTP: "127.0.0.1:80", HTTPS: "127.0.0.1:443"},
			{HTTP: "[::1]:80", HTTPS: "[::1]:443"},
		}
	}
	// http01ProbeSiteEndpoints derives endpoints from the enabled site when
	// the configuration cannot be read.
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

// http01ProbeConfigUnreadable is the warning reported for a domain when the
// effective configuration cannot be read and no address is left to probe.
const http01ProbeConfigUnreadable = "the local Nginx configuration could not be read"

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

	listener, err := http01ProbeListen(net.JoinHostPort(http01ProbeListenHost, port))
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

	routing := newHTTP01ProbeRouting(cfg)
	results := settleHTTP01Probe(ctx, domains, cfg.settleTimeout, func(domain string) HTTP01ProbeResult {
		return probeHTTP01Domain(ctx, routing, domain, path, body)
	})
	if routing.configKnown() {
		for i := range results {
			if results[i].Status == HTTP01ProbeStatusFailure {
				results[i].cause = explainHTTP01AttemptFailure(routing.blocks, port, results[i].Domain, results[i].deciding)
			}
		}
	}
	return results, nil
}

// settleHTTP01Probe probes every domain in rounds until each one has passed
// http01ProbeRequiredSuccesses consecutive rounds, ended with a result that
// waiting cannot change (a warning or a final failure), or the settle
// deadline expired. Only failures, which carry HTTP evidence, are probed
// again: a reload keeps the listening sockets open, so a warning is not
// caused by it. A failure resets the success streak. A domain whose last
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
			case result.Status != HTTP01ProbeStatusFailure || result.final:
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
// returns a cosy error (code 50058) for the first failing domain, with the
// static explanation of the server block Nginx selects for the domain
// appended as a possible cause when the probe could read the configuration.
// Warnings and skipped results never fail. configName is kept for
// compatibility and no longer used.
func HTTP01RouteCheckError(results []HTTP01ProbeResult, configName string) error {
	_ = configName
	for _, result := range results {
		if result.Status != HTTP01ProbeStatusFailure {
			continue
		}
		return NewHTTP01ChallengeRouteCheckError(result.Domain, describeHTTP01ProbeFailureWithCause(result))
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
			parts = append(parts, fmt.Sprintf("%s: %s", result.Domain, describeHTTP01ProbeFailureWithCause(result)))
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

func describeHTTP01ProbeFailureWithCause(result HTTP01ProbeResult) string {
	reason := describeHTTP01ProbeFailure(result)
	if result.cause != "" {
		reason += "; possible cause: " + result.cause
	}
	return reason
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

// http01ProbeRouting knows where the local Nginx accepts the requests of one
// probe run. blocks is nil when the effective configuration could not be
// read; fallback then lists the addresses to try instead.
type http01ProbeRouting struct {
	blocks   []nginx.ServerBlock
	fallback []http01ProbeEndpoint
}

// newHTTP01ProbeRouting reads the effective configuration once per run.
func newHTTP01ProbeRouting(cfg *http01ProbeConfig) *http01ProbeRouting {
	blocks, err := http01ProbeServerBlocks()
	if err == nil && len(blocks) > 0 {
		return &http01ProbeRouting{blocks: blocks}
	}
	return &http01ProbeRouting{fallback: http01ProbeEndpoints(cfg)}
}

func (r *http01ProbeRouting) configKnown() bool {
	return r != nil && r.blocks != nil
}

// http01HopTarget is a local address a request is dialed to, with the
// socket (and the host it was resolved for) behind it when known.
type http01HopTarget struct {
	address string
	host    string
	socket  *nginx.ServerSocket
}

// targets returns the dial targets of the deciding sockets on port for host
// (see decidingServerSockets), without duplicate addresses.
func (r *http01ProbeRouting) targets(host, port string) []http01HopTarget {
	var targets []http01HopTarget
	seen := map[string]bool{}
	for _, socket := range decidingServerSockets(r.blocks, host, port) {
		address := http01ProbeSocketAddress(socket.Listen)
		if address == "" || seen[address] {
			continue
		}
		seen[address] = true
		selected := socket
		targets = append(targets, http01HopTarget{address: address, host: host, socket: &selected})
	}
	return targets
}

// endpoints returns the local targets for domain, or a warning reason when
// there is nothing the CA's request could be reproduced against.
func (r *http01ProbeRouting) endpoints(domain string) ([]http01ProbeEndpoint, string) {
	if !r.configKnown() {
		if r == nil || len(r.fallback) == 0 {
			return nil, http01ProbeConfigUnreadable
		}
		return r.fallback, ""
	}
	targets := r.targets(domain, "80")
	if len(targets) == 0 {
		return nil, fmt.Sprintf("no server block listens on port 80 for %s; the certificate authority may reach it through a load balancer or port mapping", domain)
	}
	endpoints := make([]http01ProbeEndpoint, 0, len(targets))
	for _, target := range targets {
		endpoint := http01ProbeEndpoint{HTTP: target.address, socket: target.socket}
		if host, _, err := net.SplitHostPort(target.address); err == nil {
			endpoint.HTTPS = net.JoinHostPort(host, "443")
		}
		endpoints = append(endpoints, endpoint)
	}
	return endpoints, ""
}

// http01NoSocketError reports a redirect hop that no local socket serves.
type http01NoSocketError struct {
	Host string
	Port string
}

func (e *http01NoSocketError) Error() string {
	return fmt.Sprintf("no server block listens on port %s for %s", e.Port, e.Host)
}

// hopTargets returns the local targets a redirect hop to host:port is sent
// to. A hop back to the probed domain on port 80 stays on the endpoint's own
// socket; any other hop goes to the deciding sockets of its host and port,
// or to the endpoint's fixed addresses when the configuration is unknown.
func (r *http01ProbeRouting) hopTargets(endpoint http01ProbeEndpoint, domain, host, port string) ([]http01HopTarget, error) {
	if port == "80" && strings.EqualFold(host, domain) {
		return []http01HopTarget{{address: endpoint.HTTP, host: host, socket: endpoint.socket}}, nil
	}
	if !r.configKnown() {
		local := endpoint.HTTP
		if port == "443" {
			local = endpoint.HTTPS
		}
		if local == "" {
			return nil, &http01NoSocketError{Host: host, Port: port}
		}
		return []http01HopTarget{{address: local, host: host}}, nil
	}
	targets := r.targets(host, port)
	if len(targets) == 0 {
		return nil, &http01NoSocketError{Host: host, Port: port}
	}
	return targets, nil
}

// newHTTP01ProbeClient returns a client that sends every request, whatever
// its URL host, to the local address. Redirects are handled by the caller.
func newHTTP01ProbeClient(address string) *http.Client {
	dialer := &net.Dialer{Timeout: http01ProbeRequestTimeout}
	dial := func(ctx context.Context, _ string, _ string) (net.Conn, error) {
		return dialer.DialContext(ctx, "tcp", address)
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

// http01ProbeEndpoints returns the fallback targets used when the effective
// configuration cannot be read: the site's explicit listen addresses first,
// then the default loopback endpoints (marked as guessed), without
// duplicates.
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
		endpoint.guessed = true
		add(endpoint)
	}
	return endpoints
}

// http01SocketDialAddress maps a listening socket to the address the probe
// dials: a specific address as is (IPv6 bracketed), the wildcard IPv4 socket
// through 127.0.0.1 and the wildcard IPv6 socket through [::1]. UNIX sockets
// yield "".
func http01SocketDialAddress(listen nginx.ServerListen) string {
	host := strings.TrimSpace(listen.Addr)
	if strings.HasPrefix(host, "unix:") {
		return ""
	}
	host = strings.TrimSuffix(strings.TrimPrefix(host, "["), "]")
	port := strings.TrimSpace(listen.Port)
	if port == "" {
		port = "80"
	}
	switch host {
	case "", "*", "0.0.0.0", "::":
		if listen.IPv6 || host == "::" {
			host = "::1"
		} else {
			host = "127.0.0.1"
		}
	}
	return net.JoinHostPort(host, port)
}

// probeHTTP01Domain probes every deciding port-80 socket of domain. The CA
// may reach any of them (the probe cannot know which address family or IP
// it uses), so they are all probed and aggregated.
func probeHTTP01Domain(ctx context.Context, routing *http01ProbeRouting, domain, path, body string) HTTP01ProbeResult {
	result := HTTP01ProbeResult{Domain: domain, Status: HTTP01ProbeStatusFailure}
	if strings.HasPrefix(domain, "*.") {
		result.Error = "wildcard identifiers cannot be validated with HTTP-01"
		result.final = true
		return result
	}
	endpoints, warning := routing.endpoints(domain)
	if warning != "" {
		result.Status = HTTP01ProbeStatusWarning
		result.Error = warning
		result.final = true
		return result
	}

	for _, endpoint := range endpoints {
		result.Attempts = append(result.Attempts, probeHTTP01Endpoint(ctx, routing, endpoint, domain, path, body))
		if ctx.Err() != nil {
			break
		}
	}

	deciding := aggregateHTTP01Attempts(result.Attempts)
	result.Status = deciding.Status
	result.Target = deciding.Target
	result.StatusCode = deciding.StatusCode
	result.Location = deciding.Location
	result.Redirects = deciding.Redirects
	result.Error = deciding.Error
	result.deciding = deciding
	return result
}

// aggregateHTTP01Attempts combines the attempts of the sockets the CA may
// reach: the first failure wins, because HTTP evidence of a wrong route on
// any of them can fail the validation; else the first success, because a
// connection-level warning on one socket does not hide a working route on
// another; else the first warning that got an HTTP answer, else the first
// warning.
func aggregateHTTP01Attempts(attempts []HTTP01ProbeAttempt) HTTP01ProbeAttempt {
	rank := func(attempt HTTP01ProbeAttempt) int {
		switch {
		case attempt.Status == HTTP01ProbeStatusFailure:
			return 4
		case attempt.Status == HTTP01ProbeStatusSuccess:
			return 3
		case attempt.Status == HTTP01ProbeStatusWarning && attempt.StatusCode != 0:
			return 2
		case attempt.Status == HTTP01ProbeStatusWarning:
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

// http01ProbeWalk follows the redirect chain of one endpoint under the CA's
// rules. A hop served by several local sockets fans out to each of them and
// the branches are aggregated like the endpoints of a domain.
type http01ProbeWalk struct {
	ctx      context.Context
	routing  *http01ProbeRouting
	endpoint http01ProbeEndpoint
	domain   string
	body     string
	initial  string
}

// probeHTTP01Endpoint fetches the token for domain through one local
// endpoint, following redirects under the CA's rules.
func probeHTTP01Endpoint(ctx context.Context, routing *http01ProbeRouting, endpoint http01ProbeEndpoint, domain, path, body string) HTTP01ProbeAttempt {
	start := &url.URL{Scheme: "http", Host: http01ProbeHostHeader(domain), Path: path}
	walk := &http01ProbeWalk{
		ctx:      ctx,
		routing:  routing,
		endpoint: endpoint,
		domain:   domain,
		body:     body,
		initial:  start.String(),
	}
	first := http01HopTarget{address: endpoint.HTTP, host: domain, socket: endpoint.socket}
	return walk.run(start, first, nil, map[string]bool{walk.initial: true})
}

func (w *http01ProbeWalk) run(current *url.URL, target http01HopTarget, redirects []string, visited map[string]bool) HTTP01ProbeAttempt {
	attempt := HTTP01ProbeAttempt{
		Target:    w.endpoint.HTTP,
		Status:    HTTP01ProbeStatusFailure,
		Redirects: redirects,
		socket:    w.endpoint.socket,
	}
	if len(redirects) > 0 {
		attempt.hop = &target
	}

	client := newHTTP01ProbeClient(target.address)
	defer client.CloseIdleConnections()
	statusCode, location, gotBody, err := fetchHTTP01Probe(w.ctx, client, current)
	if err != nil {
		attempt.StatusCode = statusCode
		attempt.Error = err.Error()
		return w.finish(attempt, current, err, statusCode == 0, false)
	}
	attempt.StatusCode = statusCode

	if statusCode >= 300 && statusCode < 400 {
		attempt.Location = location
		next, reason := nextHTTP01ProbeRedirect(current, location, len(redirects), visited)
		if reason != "" {
			// The CA refuses this redirect as well.
			attempt.Error = reason
			return w.finish(attempt, current, nil, false, true)
		}
		nextRedirects := append(append([]string(nil), redirects...), next.String())
		nextVisited := make(map[string]bool, len(visited)+1)
		for key := range visited {
			nextVisited[key] = true
		}
		nextVisited[next.String()] = true

		hops, hopErr := w.routing.hopTargets(w.endpoint, w.domain, next.Hostname(), http01ProbeURLPort(next))
		if hopErr != nil {
			attempt.Redirects = nextRedirects
			attempt.Error = hopErr.Error()
			attempt.hop = nil
			return w.finish(attempt, next, hopErr, true, false)
		}
		branches := make([]HTTP01ProbeAttempt, 0, len(hops))
		for _, hop := range hops {
			branches = append(branches, w.run(next, hop, nextRedirects, nextVisited))
			if w.ctx.Err() != nil {
				break
			}
		}
		return aggregateHTTP01Attempts(branches)
	}

	switch {
	case statusCode != http.StatusOK:
		attempt.Error = fmt.Sprintf("unexpected status %d", statusCode)
	// ACME servers ignore trailing whitespace in the key authorization.
	case strings.TrimRight(gotBody, " \t\r\n") != w.body:
		attempt.Error = "status 200 but the response body is not the challenge token, so the request did not reach the nginx-ui challenge server"
	default:
		attempt.Status = HTTP01ProbeStatusSuccess
		return attempt
	}
	return w.finish(attempt, current, nil, false, false)
}

// finish classifies an attempt that did not get the token. noAnswer means
// the last request got no HTTP response at all; refused means the CA would
// refuse the last redirect.
func (w *http01ProbeWalk) finish(attempt HTTP01ProbeAttempt, current *url.URL, fetchErr error, noAnswer, refused bool) HTTP01ProbeAttempt {
	withChain := func(message string) string {
		if len(attempt.Redirects) == 0 {
			return message
		}
		chain := append([]string{w.initial}, attempt.Redirects...)
		return fmt.Sprintf("%s (redirect chain: %s)", message, strings.Join(chain, " -> "))
	}

	if host := current.Hostname(); !refused && len(attempt.Redirects) > 0 && !strings.EqualFold(host, w.domain) {
		attempt.Status = HTTP01ProbeStatusWarning
		attempt.Error = withChain(fmt.Sprintf(
			"redirected to %s, which the local Nginx did not answer for (%s); the CA will contact that host directly, so the route cannot be verified locally",
			host, attempt.Error))
		return attempt
	}

	var noSocket *http01NoSocketError
	switch {
	case noAnswer && errors.As(fetchErr, &noSocket):
		attempt.Status = HTTP01ProbeStatusWarning
		attempt.Error = withChain(fmt.Sprintf(
			"%s; the certificate authority may reach it through a load balancer or port mapping, so the route cannot be verified locally",
			noSocket.Error()))
	case noAnswer && w.endpoint.guessed:
		attempt.Status = HTTP01ProbeStatusWarning
		attempt.Error = withChain(fmt.Sprintf(
			"%s and the local Nginx did not answer (%s), so the route cannot be verified locally",
			http01ProbeConfigUnreadable, attempt.Error))
	case noAnswer:
		attempt.Status = HTTP01ProbeStatusWarning
		attempt.Error = withChain(fmt.Sprintf(
			"the local Nginx did not answer (%s), so the route cannot be verified locally",
			attempt.Error))
	case w.endpoint.guessed:
		// Without the configuration the answer may come from another server
		// block than the one the CA reaches.
		attempt.Status = HTTP01ProbeStatusWarning
		attempt.Error = withChain(fmt.Sprintf(
			"%s; %s, so this answer may come from another server block than the one the certificate authority reaches and the route cannot be verified locally",
			attempt.Error, http01ProbeConfigUnreadable))
	default:
		attempt.Error = withChain(attempt.Error)
	}
	return attempt
}

// http01ProbeURLPort returns the explicit or scheme-default port of u.
func http01ProbeURLPort(u *url.URL) string {
	if port := u.Port(); port != "" {
		return port
	}
	if u.Scheme == "https" {
		return "443"
	}
	return "80"
}

// explainHTTP01AttemptFailure explains a failed attempt from the socket that
// produced it: the port-80 socket of its first request, then, when that
// block looks right, the socket that answered the last redirect hop.
func explainHTTP01AttemptFailure(blocks []nginx.ServerBlock, challengePort, domain string, attempt HTTP01ProbeAttempt) string {
	if attempt.socket == nil {
		return ""
	}
	if cause := explainHTTP01Socket(blocks, challengePort, domain, *attempt.socket); cause != "" {
		return cause
	}
	if attempt.hop != nil && attempt.hop.socket != nil {
		return explainHTTP01Socket(blocks, challengePort, attempt.hop.host, *attempt.hop.socket)
	}
	return ""
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

// enabledSiteHTTPListenEndpoints is the fallback source of endpoints when the
// effective configuration (nginx -T) cannot be read. It reads the enabled
// configuration of configName and returns an endpoint for every explicit, non-wildcard
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

// listenHTTP01Challenge binds the HTTP-01 challenge address.
func listenHTTP01Challenge(address string) (net.Listener, error) {
	return net.Listen("tcp", address)
}
