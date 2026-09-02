package transport

import (
	"crypto/tls"
	"net/http"
	"net/url"
	"time"

	"github.com/0xJacky/Nginx-UI/settings"
)

// Connection pool bounds for transports handed out by this package.
//
// Callers build a fresh transport per client - per cluster node, per reconnect
// attempt, per proxied request - so an unbounded idle pool is a real leak: a
// parked keep-alive connection holds a socket plus a read/write goroutine pair
// and keeps its throwaway transport reachable, forever, because the zero value
// of IdleConnTimeout means "never expire".
const (
	transportIdleConnTimeout     = 90 * time.Second
	transportMaxIdleConns        = 32
	transportMaxIdleConnsPerHost = 4
)

// NewTransport creates a new http.Transport with the provided options.
func NewTransport(options ...func(transport *http.Transport) error) (t *http.Transport, err error) {
	t = &http.Transport{
		Proxy:               http.ProxyFromEnvironment,
		TLSClientConfig:     &tls.Config{InsecureSkipVerify: settings.HTTPSettings.InsecureSkipVerify},
		IdleConnTimeout:     transportIdleConnTimeout,
		MaxIdleConns:        transportMaxIdleConns,
		MaxIdleConnsPerHost: transportMaxIdleConnsPerHost,
	}

	for _, option := range options {
		if err := option(t); err != nil {
			return nil, err
		}
	}

	return
}

// WithProxy returns a function that sets the proxy of the http.Transport to the provided proxy URL.
func WithProxy(proxyUrl string) func(transport *http.Transport) error {
	if proxyUrl == "" {
		return func(transport *http.Transport) error {
			return nil
		}
	}
	proxy, err := url.Parse(proxyUrl)
	if err != nil {
		return func(transport *http.Transport) error {
			return err
		}
	}

	return func(transport *http.Transport) error {
		transport.Proxy = http.ProxyURL(proxy)
		return nil
	}
}
