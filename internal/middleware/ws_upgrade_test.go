package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsWebSocketUpgrade(t *testing.T) {
	cases := []struct {
		name       string
		upgrade    string
		connection string
		want       bool
	}{
		{name: "browser handshake", upgrade: "websocket", connection: "Upgrade", want: true},
		{name: "mixed case", upgrade: "WebSocket", connection: "upgrade", want: true},
		{name: "connection token list", upgrade: "websocket", connection: "keep-alive, Upgrade", want: true},
		{name: "plain request", want: false},
		{name: "upgrade header only", upgrade: "websocket", connection: "keep-alive", want: false},
		{name: "connection header only", connection: "Upgrade", want: false},
		{name: "other protocol", upgrade: "h2c", connection: "Upgrade", want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/ws", nil)
			if tc.upgrade != "" {
				request.Header.Set("Upgrade", tc.upgrade)
			}
			if tc.connection != "" {
				request.Header.Set("Connection", tc.connection)
			}
			assert.Equal(t, tc.want, IsWebSocketUpgrade(request))
		})
	}
}

func TestForWebSocketUpgradeSelectsPerRequest(t *testing.T) {
	var picked string
	mark := func(name string) gin.HandlerFunc {
		return func(c *gin.Context) {
			picked = name
			c.Next()
		}
	}

	router := gin.New()
	router.GET("/x", ForWebSocketUpgrade(mark("upgrade"), mark("regular")), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	plain := httptest.NewRecorder()
	router.ServeHTTP(plain, httptest.NewRequest(http.MethodGet, "/x", nil))
	require.Equal(t, http.StatusNoContent, plain.Code)
	assert.Equal(t, "regular", picked)

	request := httptest.NewRequest(http.MethodGet, "/x", nil)
	request.Header.Set("Connection", "Upgrade")
	request.Header.Set("Upgrade", "websocket")
	upgrade := httptest.NewRecorder()
	router.ServeHTTP(upgrade, request)
	require.Equal(t, http.StatusNoContent, upgrade.Code)
	assert.Equal(t, "upgrade", picked)
}
