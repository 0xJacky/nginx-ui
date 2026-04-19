package upstream

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestRouteRegistrationSeparatesHTTPAndWebSocketEndpoints(t *testing.T) {
	router := gin.New()

	httpGroup := router.Group("/", func(c *gin.Context) {
		c.Header("X-Upstream-Group", "http")
		c.AbortWithStatus(http.StatusNoContent)
	})
	InitHTTPRouter(httpGroup)

	wsGroup := router.Group("/", func(c *gin.Context) {
		c.Header("X-Upstream-Group", "ws")
		c.AbortWithStatus(http.StatusNoContent)
	})
	InitWebSocketRouter(wsGroup)

	testCases := []struct {
		name         string
		method       string
		target       string
		expectedMark string
	}{
		{
			name:         "availability uses http proxy group",
			method:       http.MethodGet,
			target:       "/upstream/availability",
			expectedMark: "http",
		},
		{
			name:         "socket list uses http proxy group",
			method:       http.MethodGet,
			target:       "/upstream/sockets",
			expectedMark: "http",
		},
		{
			name:         "socket update uses http proxy group",
			method:       http.MethodPut,
			target:       "/upstream/socket/127.0.0.1%3A8080",
			expectedMark: "http",
		},
		{
			name:         "availability websocket uses websocket proxy group",
			method:       http.MethodGet,
			target:       "/upstream/availability_ws",
			expectedMark: "ws",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.target, nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			require.Equal(t, http.StatusNoContent, w.Code)
			require.Equal(t, tc.expectedMark, w.Header().Get("X-Upstream-Group"))
		})
	}
}
