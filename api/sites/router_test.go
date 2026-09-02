package sites

import (
	"net/http"
	"testing"

	"github.com/0xJacky/Nginx-UI/internal/sitecheck"
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
