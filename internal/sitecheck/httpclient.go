package sitecheck

import (
	"crypto/tls"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/settings"
)

// Connection-pool sizing. Kept small on purpose: the Site Checker probes
// hosts that may resolve to ingress services with multiple A records, and we
// must not exhaust conntrack tables on consumer routers. See issue #1608.
const (
	siteCheckMaxIdleConns        = 50
	siteCheckMaxIdleConnsPerHost = 2
	siteCheckMaxConnsPerHost     = 2
	siteCheckIdleConnTimeout     = 90 * time.Second
	siteCheckTLSHandshakeTimeout = 10 * time.Second
	siteCheckResponseHdrTimeout  = 15 * time.Second
	siteCheckDialTimeout         = 5 * time.Second
	siteCheckDialKeepAlive       = 30 * time.Second
)

var (
	sharedDialer = &net.Dialer{
		Timeout:       siteCheckDialTimeout,
		KeepAlive:     siteCheckDialKeepAlive,
		FallbackDelay: -1, // disable Happy Eyeballs IPv6 race that storms TIME_WAIT
	}

	sharedTransport     *http.Transport
	sharedTransportOnce sync.Once

	verifyingTransport     *http.Transport
	verifyingTransportOnce sync.Once
)

// probeVerifiesCertificate decides whether a health probe must validate the
// peer's certificate chain.
//
// The four cases are written out separately because the decision trades a
// false "down" report on the dashboard against handing operator-supplied data
// to a peer whose identity was never checked. Both failure modes are real, so
// neither a blanket "always verify" nor a blanket "never verify" is correct.
func probeVerifiesCertificate(cfg *model.HealthCheckConfig) bool {
	// 1. Explicit per-site opt-in. An operator who asked this probe to assert
	//    certificate trust gets it, and nothing below may override that.
	if cfg != nil && cfg.ValidateSSL {
		return true
	}

	// 2. The operator has globally declared that this instance does not verify
	//    TLS peers anywhere. Respect that rather than making the Site Checker
	//    the one client that ignores the setting. This is also the escape
	//    hatch for case 3: a self-signed site that needs custom headers can be
	//    covered by turning this on (or by installing the private CA into the
	//    host trust store and using case 1).
	if settings.HTTPSettings.InsecureSkipVerify {
		return false
	}

	// 3. The probe carries data the operator supplied — custom headers, which
	//    routinely hold an Authorization token or an API key, or a request
	//    body. That data must not be sent to an unauthenticated peer, so
	//    verification stays on even though it can make a self-signed site read
	//    as down.
	if probeCarriesOperatorData(cfg) {
		return true
	}

	// 4. Plain liveness probe. Nothing but the request line and a User-Agent
	//    goes out, so the only question is whether the vhost is serving, not
	//    whether its certificate chains to a publicly trusted CA. Internal
	//    vhosts commonly present a self-signed certificate or one issued by a
	//    private CA the Nginx UI host does not trust; verifying the chain here
	//    turned those healthy sites into a permanent red dot (#1790).
	return false
}

// probeCarriesOperatorData reports whether the probe sends anything the
// operator configured beyond the request line and the User-Agent header.
//
// The body is counted regardless of method: checkHTTP only transmits it for
// POST/PUT today, but treating any non-empty body as sensitive keeps this
// decision from silently weakening if that ever changes.
func probeCarriesOperatorData(cfg *model.HealthCheckConfig) bool {
	if cfg == nil {
		return false
	}
	if len(cfg.Headers) > 0 {
		return true
	}
	return strings.TrimSpace(cfg.Body) != ""
}

// SharedTransport returns the package-level http.Transport used by probes that
// do not verify the peer certificate — cases 2 and 4 of
// probeVerifiesCertificate. Centralising it ensures connection reuse across
// goroutines and across sweep cycles.
//
// Skipping verification is safe for exactly these probes because no
// operator-supplied credential travels over their connections: anything
// carrying custom headers or a request body is routed to VerifyingTransport
// instead.
//
// The relaxation is scoped to Site Checker probes only. Every other outbound
// client (cluster nodes, ACME, upgrade downloads, external notifications, ...)
// keeps going through internal/transport, which still honours
// settings.HTTPSettings.InsecureSkipVerify.
func SharedTransport() *http.Transport {
	sharedTransportOnce.Do(func() {
		sharedTransport = newPooledTransport(&tls.Config{
			InsecureSkipVerify: true, // Credential-free liveness probe; see doc comment above.
		})
	})
	return sharedTransport
}

// VerifyingTransport returns the package-level http.Transport used by probes
// that must validate the peer certificate — cases 1 and 3 of
// probeVerifiesCertificate.
//
// It is a second pooled transport rather than a per-probe one on purpose.
// Allocating a fresh http.Transport for every check would forfeit connection
// reuse and reopen the conntrack exhaustion that #1608 fixed, and case 3 can
// cover a large share of a deployment's sites. Only a per-site client
// certificate, which genuinely cannot be shared, still gets a dedicated
// transport in ClientForHealthCheck.
func VerifyingTransport() *http.Transport {
	verifyingTransportOnce.Do(func() {
		verifyingTransport = newPooledTransport(&tls.Config{
			InsecureSkipVerify: false,
		})
	})
	return verifyingTransport
}

// SharedClient returns an http.Client backed by the shared transport with the
// given per-request timeout. The client is cheap to construct; only the
// transport must be reused.
func SharedClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Transport: SharedTransport(),
		Timeout:   timeout,
	}
}

// ClientForHealthCheck returns the right client for a per-site health check.
// probeVerifiesCertificate makes the TLS decision; this function only selects
// the transport that implements it, preferring a pooled one.
func ClientForHealthCheck(cfg *model.HealthCheckConfig, timeout time.Duration) *http.Client {
	verify := probeVerifiesCertificate(cfg)

	// A client certificate is per-site state that cannot be shared across a
	// pooled transport, so it is the only case that still allocates one. It
	// still uses the shared dialer and pool sizing.
	if cfg != nil && cfg.ClientCert != "" && cfg.ClientKey != "" {
		tlsConfig := &tls.Config{
			InsecureSkipVerify: !verify,
		}
		if cert, err := tls.LoadX509KeyPair(cfg.ClientCert, cfg.ClientKey); err == nil {
			tlsConfig.Certificates = []tls.Certificate{cert}
		}

		return &http.Client{
			Transport: newPooledTransport(tlsConfig),
			Timeout:   timeout,
		}
	}

	if verify {
		return &http.Client{
			Transport: VerifyingTransport(),
			Timeout:   timeout,
		}
	}

	return SharedClient(timeout)
}

// needsCustomTLS reports whether this probe needs a client other than the
// package default, whose transport skips verification.
//
// VerifyHostname is deliberately absent. It has never been implemented as a
// hostname-only check, and with the default transport now always skipping
// verification a hostname-only config produces exactly the shared TLS
// configuration — routing it to a separate client would only cost a pool.
func needsCustomTLS(cfg *model.HealthCheckConfig) bool {
	if cfg == nil {
		return false
	}
	if probeVerifiesCertificate(cfg) {
		return true
	}
	return cfg.ClientCert != "" && cfg.ClientKey != ""
}

func newPooledTransport(tlsConfig *tls.Config) *http.Transport {
	return &http.Transport{
		DialContext:           sharedDialer.DialContext,
		TLSHandshakeTimeout:   siteCheckTLSHandshakeTimeout,
		ResponseHeaderTimeout: siteCheckResponseHdrTimeout,
		IdleConnTimeout:       siteCheckIdleConnTimeout,
		MaxIdleConns:          siteCheckMaxIdleConns,
		MaxIdleConnsPerHost:   siteCheckMaxIdleConnsPerHost,
		MaxConnsPerHost:       siteCheckMaxConnsPerHost,
		ForceAttemptHTTP2:     true,
		TLSClientConfig:       tlsConfig,
	}
}
