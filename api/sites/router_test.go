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
	InitRouter(router.Group("/api"))

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
