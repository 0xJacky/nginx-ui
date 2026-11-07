//go:build unembed

package router

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	cosyRouter "github.com/uozi-tech/cosy/router"
)

// TestLogAnalyticsSupportRoutesRegistered guards the routes the built-in log
// analytics UI depends on. The advanced indexing dialog only enables its
// confirm button after GET /api/geolite/status reports the database, and the
// China city map loads its boundaries from /api/nginx_log/geo/boundary.
func TestLogAnalyticsSupportRoutesRegistered(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cosyRouter.Init()
	InitRouter()

	registered := map[string]bool{}
	for _, route := range cosyRouter.GetEngine().Routes() {
		registered[route.Method+" "+route.Path] = true
	}

	for _, route := range []string{
		http.MethodGet + " /api/geolite/status",
		http.MethodGet + " /api/geolite/download",
		http.MethodGet + " /api/nginx_log/geo/boundary/:filename",
	} {
		assert.True(t, registered[route], "%s is not registered", route)
	}
}

// TestGeoLiteRoutesRequireAuthentication checks that the GeoLite2 routes sit
// behind the authenticated groups: an anonymous request must be rejected by
// the auth middleware rather than reach the handler or fall through to the
// "not found" handler.
func TestGeoLiteRoutesRequireAuthentication(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cosyRouter.Init()
	InitRouter()

	for _, path := range []string{"/api/geolite/status", "/api/geolite/download"} {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		recorder := httptest.NewRecorder()
		cosyRouter.GetEngine().ServeHTTP(recorder, request)

		require.Equal(t, http.StatusForbidden, recorder.Code, "%s: %s", path, recorder.Body.String())
		assert.JSONEq(t, `{"message":"Authorization failed"}`, recorder.Body.String(), path)
	}
}
