package sites

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/0xJacky/Nginx-UI/internal/sitecheck"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
)

func TestInitRouterWithoutInitializedSiteCheckService(t *testing.T) {
	if service := sitecheck.GetService(); service != nil {
		t.Fatal("expected site check service to be uninitialized before route setup")
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	group := router.Group("/api")
	InitRouter(group)
	InitWebSocketRouter(group)

	routeFound := false
	for _, route := range router.Routes() {
		if route.Method == http.MethodGet && route.Path == "/api/site_navigation_ws" {
			routeFound = true
			break
		}
	}
	if !routeFound {
		t.Fatal("expected site routes to be registered before the service starts")
	}
}

// TestInitRouterDoesNotRegisterWebSocketRoute guards the fix for issue #1793.
// The site navigation WebSocket must only be reachable through the WebSocket
// router group (AuthRequiredWS), because a browser handshake authenticates with
// the `token` query parameter instead of an Authorization header.
func TestInitRouterDoesNotRegisterWebSocketRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	InitRouter(router.Group("/api"))

	for _, route := range router.Routes() {
		if route.Method == http.MethodGet && route.Path == "/api/site_navigation_ws" {
			t.Fatal("site_navigation_ws must not be registered on the plain HTTP router group")
		}
	}
}

func TestHealthCheckRoutesRejectDemoRequests(t *testing.T) {
	setSitesDemoMode(t, true)

	router := newSitesTestRouter()
	tests := []struct {
		name   string
		method string
		path   string
	}{
		{name: "stored update", method: http.MethodPost, path: "/api/site_navigation/health_check/1"},
		{name: "stored sync", method: http.MethodPut, path: "/api/site_navigation/health_check/sync"},
		{name: "direct test", method: http.MethodPost, path: "/api/site_navigation/test_health_check/1"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := requestSitesRoute(t, router, test.method, test.path)

			if !strings.Contains(response.Body.String(), "disabled in demo mode") {
				t.Fatalf("expected Demo request to be rejected, got %d: %s", response.Code, response.Body.String())
			}
		})
	}
}

func TestHealthCheckRoutesRemainAvailableOutsideDemo(t *testing.T) {
	setSitesDemoMode(t, false)

	router := newSitesTestRouter()
	tests := []struct {
		name   string
		method string
		path   string
	}{
		{name: "stored update", method: http.MethodPost, path: "/api/site_navigation/health_check/1"},
		{name: "stored sync", method: http.MethodPut, path: "/api/site_navigation/health_check/sync"},
		{name: "direct test", method: http.MethodPost, path: "/api/site_navigation/test_health_check/1"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := requestSitesRoute(t, router, test.method, test.path)

			if response.Code == http.StatusNotFound {
				t.Fatalf("expected non-Demo route to remain registered")
			}
			if strings.Contains(response.Body.String(), "disabled in demo mode") {
				t.Fatalf("expected non-Demo request to reach its handler, got %d: %s", response.Code, response.Body.String())
			}
		})
	}
}

func setSitesDemoMode(t *testing.T, enabled bool) {
	t.Helper()

	previous := settings.NodeSettings.Demo
	settings.NodeSettings.Demo = enabled
	t.Cleanup(func() {
		settings.NodeSettings.Demo = previous
	})
}

func newSitesTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	InitRouter(router.Group("/api"))
	return router
}

func requestSitesRoute(t *testing.T, router *gin.Engine, method, path string) *httptest.ResponseRecorder {
	t.Helper()

	request := httptest.NewRequest(method, path, bytes.NewBufferString("{}"))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	return response
}
