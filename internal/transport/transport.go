package transport

import (
	"crypto/tls"
	"github.com/0xJacky/Nginx-UI/settings"
	"net/http"
	"net/url"
)

// NewTransport creates a new http.Transport with the provided options.
func NewTransport(options ...func(transport *http.Transport) error) (t *http.Transport, err error) {
	t = &http.Transport{
		Proxy:           http.ProxyFromEnvironment,
		TLSClientConfig: &tls.Config{InsecureSkipVerify: settings.HTTPSettings.InsecureSkipVerify},
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
