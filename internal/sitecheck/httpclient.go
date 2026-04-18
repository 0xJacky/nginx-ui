package sitecheck

import (
	"crypto/tls"
	"net"
	"net/http"
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
)

// SharedTransport returns the package-level http.Transport used by every
// Site Checker request. Centralising it ensures connection reuse across
// goroutines and across sweep cycles.
func SharedTransport() *http.Transport {
	sharedTransportOnce.Do(func() {
		sharedTransport = newPooledTransport(&tls.Config{
			InsecureSkipVerify: settings.HTTPSettings.InsecureSkipVerify,
		})
	})
	return sharedTransport
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
// It reuses the shared transport whenever possible. A dedicated transport is
// only built when the per-site TLS configuration genuinely diverges from the
// global default (custom validation, hostname check, or client certificate),
// and it still uses the shared dialer + pool sizing.
func ClientForHealthCheck(cfg *model.HealthCheckConfig, timeout time.Duration) *http.Client {
	if cfg == nil || !needsCustomTLS(cfg) {
		return SharedClient(timeout)
	}

	tlsConfig := &tls.Config{
		InsecureSkipVerify: !cfg.ValidateSSL,
	}
	if cfg.ClientCert != "" && cfg.ClientKey != "" {
		if cert, err := tls.LoadX509KeyPair(cfg.ClientCert, cfg.ClientKey); err == nil {
			tlsConfig.Certificates = []tls.Certificate{cert}
		}
	}

	return &http.Client{
		Transport: newPooledTransport(tlsConfig),
		Timeout:   timeout,
	}
}

func needsCustomTLS(cfg *model.HealthCheckConfig) bool {
	if cfg == nil {
		return false
	}
	if cfg.ValidateSSL || cfg.VerifyHostname {
		return true
	}
	if cfg.ClientCert != "" && cfg.ClientKey != "" {
		return true
	}
	return false
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
