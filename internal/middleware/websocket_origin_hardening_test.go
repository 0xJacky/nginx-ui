package middleware

import (
	"crypto/tls"
	"net/http/httptest"
	"testing"

	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/stretchr/testify/assert"
)

// TestCheckWebSocketOrigin_Hardening pins the CheckWebSocketOrigin bypass
// classes documented in GHSA-78mf-482w-62qj / CVE-2026-34403 (patched in
// v2.3.5) so future refactors of origin parsing cannot silently re-open the
// CSWSH vector.
//
// Each subtest is a named regression for a specific bypass pattern the
// advisory enumerated. Kept in a separate file from websocket_origin_test.go
// to make the hardening surface easy to audit in one place.
func TestCheckWebSocketOrigin_Hardening(t *testing.T) {
	originalOrigins := settings.HTTPSettings.WebSocketTrustedOrigins
	originalSecret := settings.NodeSettings.Secret

	t.Cleanup(func() {
		settings.HTTPSettings.WebSocketTrustedOrigins = originalOrigins
		settings.NodeSettings.Secret = originalSecret
	})

	reset := func() {
		settings.HTTPSettings.WebSocketTrustedOrigins = nil
		settings.NodeSettings.Secret = ""
	}

	t.Run("rejects_subdomain_confusion", func(t *testing.T) {
		reset()
		req := httptest.NewRequest("GET", "http://127.0.0.1/ws", nil)
		req.Host = "admin.example.com"
		req.TLS = &tls.ConnectionState{}
		req.Header.Set("Origin", "https://evil.admin.example.com")
		assert.False(t, CheckWebSocketOrigin(req))
	})

	t.Run("rejects_suffix_confusion", func(t *testing.T) {
		reset()
		req := httptest.NewRequest("GET", "http://127.0.0.1/ws", nil)
		req.Host = "admin.example.com"
		req.TLS = &tls.ConnectionState{}
		req.Header.Set("Origin", "https://admin.example.com.evil.io")
		assert.False(t, CheckWebSocketOrigin(req))
	})

	t.Run("rejects_scheme_downgrade", func(t *testing.T) {
		reset()
		req := httptest.NewRequest("GET", "http://127.0.0.1/ws", nil)
		req.Host = "admin.example.com"
		req.TLS = &tls.ConnectionState{}
		req.Header.Set("Origin", "http://admin.example.com")
		assert.False(t, CheckWebSocketOrigin(req))
	})

	t.Run("rejects_port_mismatch", func(t *testing.T) {
		reset()
		req := httptest.NewRequest("GET", "http://127.0.0.1/ws", nil)
		req.Host = "admin.example.com:8443"
		req.Header.Set("Origin", "http://admin.example.com:9443")
		assert.False(t, CheckWebSocketOrigin(req))
	})

	t.Run("allows_default_http_port_normalization", func(t *testing.T) {
		reset()
		req := httptest.NewRequest("GET", "http://127.0.0.1/ws", nil)
		req.Host = "admin.example.com"
		req.Header.Set("Origin", "http://admin.example.com:80")
		assert.True(t, CheckWebSocketOrigin(req))
	})

	t.Run("allows_default_https_port_normalization", func(t *testing.T) {
		reset()
		req := httptest.NewRequest("GET", "http://127.0.0.1/ws", nil)
		req.Host = "admin.example.com"
		req.TLS = &tls.ConnectionState{}
		req.Header.Set("Origin", "https://admin.example.com:443")
		assert.True(t, CheckWebSocketOrigin(req))
	})

	t.Run("allows_ws_http_scheme_equivalence", func(t *testing.T) {
		reset()
		req := httptest.NewRequest("GET", "http://127.0.0.1/ws", nil)
		req.Host = "admin.example.com"
		req.Header.Set("Origin", "ws://admin.example.com")
		assert.True(t, CheckWebSocketOrigin(req))
	})

	t.Run("allows_wss_https_scheme_equivalence", func(t *testing.T) {
		reset()
		req := httptest.NewRequest("GET", "http://127.0.0.1/ws", nil)
		req.Host = "admin.example.com"
		req.TLS = &tls.ConnectionState{}
		req.Header.Set("Origin", "wss://admin.example.com")
		assert.True(t, CheckWebSocketOrigin(req))
	})

	t.Run("allows_case_insensitive_host", func(t *testing.T) {
		reset()
		req := httptest.NewRequest("GET", "http://127.0.0.1/ws", nil)
		req.Host = "Admin.Example.COM"
		req.TLS = &tls.ConnectionState{}
		req.Header.Set("Origin", "https://admin.example.com")
		assert.True(t, CheckWebSocketOrigin(req))
	})

	t.Run("allows_ipv6_literal_origin", func(t *testing.T) {
		reset()
		req := httptest.NewRequest("GET", "http://127.0.0.1/ws", nil)
		req.Host = "[::1]:8080"
		req.Header.Set("Origin", "http://[::1]:8080")
		assert.True(t, CheckWebSocketOrigin(req))
	})

	t.Run("allows_rfc7239_forwarded_header", func(t *testing.T) {
		reset()
		req := httptest.NewRequest("GET", "http://127.0.0.1/ws", nil)
		req.Host = "internal:9000"
		req.Header.Set("Forwarded", "proto=https;host=panel.example.com")
		req.Header.Set("Origin", "https://panel.example.com")
		assert.True(t, CheckWebSocketOrigin(req))
	})

	t.Run("picks_first_of_multi_valued_x_forwarded_host", func(t *testing.T) {
		reset()
		req := httptest.NewRequest("GET", "http://127.0.0.1/ws", nil)
		req.Host = "internal:9000"
		req.Header.Set("X-Forwarded-Proto", "https")
		req.Header.Set("X-Forwarded-Host", "panel.example.com, evil.example.com")
		req.Header.Set("Origin", "https://panel.example.com")
		assert.True(t, CheckWebSocketOrigin(req))
	})

	t.Run("rejects_scheme_only_origin", func(t *testing.T) {
		reset()
		req := httptest.NewRequest("GET", "http://127.0.0.1/ws", nil)
		req.Host = "admin.example.com"
		req.Header.Set("Origin", "https://")
		assert.False(t, CheckWebSocketOrigin(req))
	})

	t.Run("rejects_malformed_origin", func(t *testing.T) {
		reset()
		req := httptest.NewRequest("GET", "http://127.0.0.1/ws", nil)
		req.Host = "admin.example.com"
		req.Header.Set("Origin", "not-a-url")
		assert.False(t, CheckWebSocketOrigin(req))
	})

	t.Run("allows_query_string_node_secret_fallback", func(t *testing.T) {
		reset()
		settings.NodeSettings.Secret = "node-secret"
		req := httptest.NewRequest("GET", "http://127.0.0.1/ws?node_secret=node-secret", nil)
		req.Host = "child:9000"
		assert.True(t, CheckWebSocketOrigin(req))
	})

	t.Run("empty_configured_secret_never_matches_empty_request_secret", func(t *testing.T) {
		reset()
		settings.NodeSettings.Secret = ""
		req := httptest.NewRequest("GET", "http://127.0.0.1/ws", nil)
		req.Header.Set("X-Node-Secret", "")
		assert.False(t, CheckWebSocketOrigin(req))
	})

	t.Run("trailing_slash_in_configured_trusted_origin_still_matches", func(t *testing.T) {
		reset()
		settings.HTTPSettings.WebSocketTrustedOrigins = []string{"https://panel.example.com/"}
		req := httptest.NewRequest("GET", "http://127.0.0.1/ws", nil)
		req.Host = "internal:9000"
		req.Header.Set("Origin", "https://panel.example.com")
		assert.True(t, CheckWebSocketOrigin(req))
	})
}
