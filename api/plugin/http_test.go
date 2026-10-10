package plugin

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/0xJacky/Nginx-UI/internal/cache"
	"github.com/0xJacky/Nginx-UI/internal/testdb"
	internaluser "github.com/0xJacky/Nginx-UI/internal/user"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	cSettings "github.com/uozi-tech/cosy/settings"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// setupHTTPRouteAuth serves the plugin http route with a real authentication
// chain. No plugin is installed, so an authenticated request ends in the
// handler's 404 while an unauthenticated one is stopped with 403.
func setupHTTPRouteAuth(t *testing.T) (router *gin.Engine, jwt, shortToken string) {
	t.Helper()
	cache.InitInMemoryCache()

	previousSecret := cSettings.AppSettings.JwtSecret
	cSettings.AppSettings.JwtSecret = "plugin-http-route-test-secret"

	db, err := gorm.Open(sqlite.Open(testdb.DSN(t)), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.AuthToken{}, &model.Passkey{}, &model.Plugin{}, &model.PluginKV{}))
	model.Use(db)
	query.Use(db)
	query.SetDefault(db)

	u := &model.User{Model: model.Model{ID: 1}, Name: "plugin-http-user", Status: true}
	require.NoError(t, db.Create(u).Error)
	payload, err := internaluser.GenerateJWT(u)
	require.NoError(t, err)
	short, err := internaluser.GenerateShortTokenForSession(u.ID, payload.Token)
	require.NoError(t, err)

	t.Cleanup(func() {
		cache.Shutdown()
		cSettings.AppSettings.JwtSecret = previousSecret
		model.Use(nil)
	})

	router = gin.New()
	InitHTTPRouter(router.Group("/api"))
	return router, payload.Token, short
}

func pluginHTTPRequest(router http.Handler, target, authorization string, upgrade bool) int {
	return pluginHTTPRequestFrom(router, target, authorization, upgrade, "http://example.com")
}

// pluginHTTPRequestFrom sends the request with the given browser origin. The
// host of an httptest request is example.com.
func pluginHTTPRequestFrom(router http.Handler, target, authorization string, upgrade bool, origin string) int {
	request := httptest.NewRequest(http.MethodGet, target, nil)
	if origin != "" {
		request.Header.Set("Origin", origin)
	}
	if authorization != "" {
		request.Header.Set("Authorization", authorization)
	}
	if upgrade {
		request.Header.Set("Connection", "Upgrade")
		request.Header.Set("Upgrade", "websocket")
	}
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	return response.Code
}

func TestPluginHTTPRouteAcceptsQueryCredentialsOnlyForUpgrades(t *testing.T) {
	router, jwt, shortToken := setupHTTPRouteAuth(t)
	const route = "/api/plugins/com.example.demo/http/events"
	queryJWT := base64.RawURLEncoding.EncodeToString([]byte(jwt))

	assert.Equal(t, http.StatusNotFound, pluginHTTPRequest(router, route, "Bearer "+jwt, false))
	assert.Equal(t, http.StatusNotFound, pluginHTTPRequest(router, route, "Bearer "+jwt, true))
	assert.Equal(t, http.StatusNotFound, pluginHTTPRequest(router, route+"?token="+queryJWT, "", true))
	assert.Equal(t, http.StatusNotFound, pluginHTTPRequest(router, route+"?token="+shortToken+"&x_node_id=0", "", true))

	assert.Equal(t, http.StatusForbidden, pluginHTTPRequest(router, route+"?token="+queryJWT, "", false))
	assert.Equal(t, http.StatusForbidden, pluginHTTPRequest(router, route+"?token="+shortToken, "", false))
	assert.Equal(t, http.StatusForbidden, pluginHTTPRequest(router, route, "Bearer "+shortToken, false))
	assert.Equal(t, http.StatusForbidden, pluginHTTPRequest(router, route, "", true))
	assert.Equal(t, http.StatusForbidden, pluginHTTPRequest(router, route+"?token=garbage", "", true))
}

func TestPluginHTTPRouteRejectsUpgradesFromForeignOrigins(t *testing.T) {
	router, jwt, _ := setupHTTPRouteAuth(t)
	const route = "/api/plugins/com.example.demo/http/events"
	queryJWT := base64.RawURLEncoding.EncodeToString([]byte(jwt))

	assert.Equal(t, http.StatusForbidden, pluginHTTPRequestFrom(router, route+"?token="+queryJWT, "", true, "https://evil.example"))
	assert.Equal(t, http.StatusForbidden, pluginHTTPRequestFrom(router, route+"?token="+queryJWT, "", true, ""))
	// Plain requests are not upgrades and keep working without an Origin.
	assert.Equal(t, http.StatusNotFound, pluginHTTPRequestFrom(router, route, "Bearer "+jwt, false, ""))
}
