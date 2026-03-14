package middleware

import (
	"crypto/tls"
	"net/http/httptest"
	"testing"

	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/stretchr/testify/assert"
)

func TestCheckWebSocketOrigin(t *testing.T) {
	originalOrigins := settings.HTTPSettings.WebSocketTrustedOrigins
	originalSecret := settings.NodeSettings.Secret

	t.Cleanup(func() {
		settings.HTTPSettings.WebSocketTrustedOrigins = originalOrigins
		settings.NodeSettings.Secret = originalSecret
	})

	t.Run("allows same origin requests", func(t *testing.T) {
		settings.HTTPSettings.WebSocketTrustedOrigins = nil
		settings.NodeSettings.Secret = ""

		req := httptest.NewRequest("GET", "http://127.0.0.1/ws", nil)
		req.Host = "admin.example.com"
		req.TLS = &tls.ConnectionState{}
		req.Header.Set("Origin", "https://admin.example.com:443")

		assert.True(t, CheckWebSocketOrigin(req))
	})

	t.Run("allows reverse proxy forwarded origin", func(t *testing.T) {
		settings.HTTPSettings.WebSocketTrustedOrigins = nil
		settings.NodeSettings.Secret = ""

		req := httptest.NewRequest("GET", "http://127.0.0.1/ws", nil)
		req.Host = "127.0.0.1:9000"
		req.Header.Set("Origin", "https://panel.example.com")
		req.Header.Set("X-Forwarded-Proto", "https")
		req.Header.Set("X-Forwarded-Host", "panel.example.com")

		assert.True(t, CheckWebSocketOrigin(req))
	})

	t.Run("allows configured trusted origins", func(t *testing.T) {
		settings.HTTPSettings.WebSocketTrustedOrigins = []string{"http://localhost:5173/"}
		settings.NodeSettings.Secret = ""

		req := httptest.NewRequest("GET", "http://127.0.0.1/ws", nil)
		req.Host = "127.0.0.1:9000"
		req.Header.Set("Origin", "http://localhost:5173")

		assert.True(t, CheckWebSocketOrigin(req))
	})

	t.Run("allows node secret requests without origin", func(t *testing.T) {
		settings.HTTPSettings.WebSocketTrustedOrigins = nil
		settings.NodeSettings.Secret = "node-secret"

		req := httptest.NewRequest("GET", "http://127.0.0.1/ws", nil)
		req.Header.Set("X-Node-Secret", "node-secret")

		assert.True(t, CheckWebSocketOrigin(req))
	})

	t.Run("rejects cross site requests", func(t *testing.T) {
		settings.HTTPSettings.WebSocketTrustedOrigins = nil
		settings.NodeSettings.Secret = ""

		req := httptest.NewRequest("GET", "http://127.0.0.1/ws", nil)
		req.Host = "admin.example.com"
		req.TLS = &tls.ConnectionState{}
		req.Header.Set("Origin", "https://evil.example.com")

		assert.False(t, CheckWebSocketOrigin(req))
	})

	t.Run("rejects missing origin without trusted node secret", func(t *testing.T) {
		settings.HTTPSettings.WebSocketTrustedOrigins = nil
		settings.NodeSettings.Secret = "node-secret"

		req := httptest.NewRequest("GET", "http://127.0.0.1/ws?token=abc123", nil)

		assert.False(t, CheckWebSocketOrigin(req))
	})
}
