package middleware

import (
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/0xJacky/Nginx-UI/settings"
)

// CheckWebSocketOrigin validates browser origins for WebSocket upgrade requests.
// Trusted node-to-node traffic (via X-Node-Secret) is always allowed,
// regardless of the Origin header, because proxied requests carry the
// browser's original Origin which won't match the downstream node's host.
func CheckWebSocketOrigin(r *http.Request) bool {
	if isTrustedNodeRequest(r) {
		return true
	}

	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		return false
	}

	if requestOrigin, ok := getRequestOrigin(r); ok && sameOrigin(origin, requestOrigin) {
		return true
	}

	for _, allowedOrigin := range settings.HTTPSettings.WebSocketTrustedOrigins {
		if sameOrigin(origin, allowedOrigin) {
			return true
		}
	}

	return false
}

func isTrustedNodeRequest(r *http.Request) bool {
	secret := strings.TrimSpace(r.Header.Get("X-Node-Secret"))
	if secret == "" {
		secret = strings.TrimSpace(r.URL.Query().Get("node_secret"))
	}

	return secret != "" && secret == settings.NodeSettings.Secret
}

func getRequestOrigin(r *http.Request) (string, bool) {
	scheme := getForwardedParam(r.Header.Get("Forwarded"), "proto")
	host := getForwardedParam(r.Header.Get("Forwarded"), "host")

	if host == "" {
		host = firstHeaderValue(r.Header.Get("X-Forwarded-Host"))
	}
	if scheme == "" {
		scheme = firstHeaderValue(r.Header.Get("X-Forwarded-Proto"))
	}
	if host == "" {
		host = strings.TrimSpace(r.Host)
	}
	if scheme == "" {
		if r.TLS != nil {
			scheme = "https"
		} else {
			scheme = "http"
		}
	}

	return buildNormalizedOrigin(scheme, host)
}

func sameOrigin(left, right string) bool {
	normalizedLeft, ok := normalizeOrigin(left)
	if !ok {
		return false
	}

	normalizedRight, ok := normalizeOrigin(right)
	if !ok {
		return false
	}

	return normalizedLeft == normalizedRight
}

func normalizeOrigin(raw string) (string, bool) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" {
		return "", false
	}

	scheme, ok := normalizeScheme(u.Scheme)
	if !ok {
		return "", false
	}

	host := normalizeHost(u.Host, scheme)
	if host == "" {
		return "", false
	}

	return scheme + "://" + host, true
}

func buildNormalizedOrigin(rawScheme, rawHost string) (string, bool) {
	scheme, ok := normalizeScheme(rawScheme)
	if !ok {
		return "", false
	}

	host := normalizeHost(rawHost, scheme)
	if host == "" {
		return "", false
	}

	return scheme + "://" + host, true
}

func normalizeScheme(scheme string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(scheme)) {
	case "http", "ws":
		return "http", true
	case "https", "wss":
		return "https", true
	default:
		return "", false
	}
}

func normalizeHost(host, scheme string) string {
	host = firstHeaderValue(host)
	if host == "" {
		return ""
	}

	u, err := url.Parse("//" + host)
	if err != nil || u.Hostname() == "" {
		return ""
	}

	hostname := strings.ToLower(u.Hostname())
	port := u.Port()

	if port == defaultPortForScheme(scheme) {
		port = ""
	}

	if port != "" {
		return net.JoinHostPort(hostname, port)
	}

	if strings.Contains(hostname, ":") {
		return "[" + hostname + "]"
	}

	return hostname
}

func defaultPortForScheme(scheme string) string {
	switch scheme {
	case "https":
		return "443"
	default:
		return "80"
	}
}

func firstHeaderValue(value string) string {
	if value == "" {
		return ""
	}

	parts := strings.Split(value, ",")
	return strings.TrimSpace(parts[0])
}

func getForwardedParam(forwardedValue, key string) string {
	if forwardedValue == "" {
		return ""
	}

	firstEntry := firstHeaderValue(forwardedValue)
	for _, part := range strings.Split(firstEntry, ";") {
		name, value, ok := strings.Cut(strings.TrimSpace(part), "=")
		if !ok || !strings.EqualFold(name, key) {
			continue
		}

		return strings.Trim(strings.TrimSpace(value), "\"")
	}

	return ""
}
