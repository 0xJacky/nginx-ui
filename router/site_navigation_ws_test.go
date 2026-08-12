//go:build unembed

package router

import (
	"bufio"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/0xJacky/Nginx-UI/internal/cache"
	internaluser "github.com/0xJacky/Nginx-UI/internal/user"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	cosyRouter "github.com/uozi-tech/cosy/router"
	cSettings "github.com/uozi-tech/cosy/settings"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// errHijackUnsupported keeps the handshake from completing. gorilla/websocket
// only reaches Hijack after the Origin check and every handshake header check
// have passed, so a recorded Hijack call proves the request travelled through
// the whole middleware chain into the WebSocket handler.
var errHijackUnsupported = errors.New("hijack unsupported in test")

// hijackableRecorder satisfies http.Hijacker so gin's ResponseWriter type
// assertion succeeds instead of panicking on a plain httptest recorder.
type hijackableRecorder struct {
	*httptest.ResponseRecorder
	hijacked bool
}

func (r *hijackableRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	r.hijacked = true
	return nil, nil, errHijackUnsupported
}

func setupRouterTestEnvironment(t *testing.T) *model.User {
	t.Helper()

	gin.SetMode(gin.TestMode)
	cache.InitInMemoryCache()

	originalJWTSecret := cSettings.AppSettings.JwtSecret
	t.Cleanup(func() {
		cache.Shutdown()
		cSettings.AppSettings.JwtSecret = originalJWTSecret
		model.Use(nil)
	})
	cSettings.AppSettings.JwtSecret = "router-websocket-test-secret"

	database, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, database.AutoMigrate(&model.User{}, &model.AuthToken{}, &model.Passkey{}, &model.Node{}))
	model.Use(database)
	query.SetDefault(database)

	currentUser := &model.User{
		Model:  model.Model{ID: 1},
		Name:   "admin",
		Status: true,
	}
	require.NoError(t, database.Create(currentUser).Error)

	cosyRouter.Init()
	InitRouter()

	return currentUser
}

// queryToken mirrors app/src/lib/websocket/index.ts, which sends the long JWT
// URL-safe base64 encoded because a browser cannot set an Authorization header
// on a WebSocket handshake.
func queryToken(t *testing.T, currentUser *model.User) string {
	t.Helper()

	payload, err := internaluser.GenerateJWT(currentUser)
	require.NoError(t, err)

	return base64.RawURLEncoding.EncodeToString([]byte(payload.Token))
}

// newWebSocketUpgradeRequest builds a handshake identical to what a browser
// sends: no Authorization header, credentials in the query string.
func newWebSocketUpgradeRequest(target, origin string) *http.Request {
	request := httptest.NewRequest(http.MethodGet, target, nil)
	request.Header.Set("Connection", "Upgrade")
	request.Header.Set("Upgrade", "websocket")
	request.Header.Set("Sec-WebSocket-Version", "13")
	request.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")
	if origin != "" {
		request.Header.Set("Origin", origin)
	}
	return request
}

func serveRequest(request *http.Request) *hijackableRecorder {
	recorder := &hijackableRecorder{ResponseRecorder: httptest.NewRecorder()}
	cosyRouter.GetEngine().ServeHTTP(recorder, request)
	return recorder
}

// TestSiteNavigationWebSocketAcceptsQueryToken is the regression test for issue
// #1793. The endpoint used to be mounted on the plain HTTP router group, whose
// auth middleware only reads the Authorization header. A browser cannot set that
// header on a WebSocket handshake, so every upgrade was rejected with
// 403 {"message":"Authorization failed"} before any token was inspected.
func TestSiteNavigationWebSocketAcceptsQueryToken(t *testing.T) {
	currentUser := setupRouterTestEnvironment(t)
	token := queryToken(t, currentUser)

	recorder := serveRequest(newWebSocketUpgradeRequest(
		"/api/site_navigation_ws?token="+token,
		"http://example.com",
	))

	require.NotEqual(t, http.StatusForbidden, recorder.Code)
	require.NotContains(t, recorder.Body.String(), "Authorization failed")
	require.True(t, recorder.hijacked, "handshake never reached the WebSocket handler")
}

// TestSiteNavigationWebSocketReachesHandlerWithoutUpgradeHeaders asserts the same
// authorization outcome with a deterministic status code: without the upgrade
// headers gorilla/websocket answers 400 before touching the connection, so a 400
// proves the auth middleware let the request through.
func TestSiteNavigationWebSocketReachesHandlerWithoutUpgradeHeaders(t *testing.T) {
	currentUser := setupRouterTestEnvironment(t)
	token := queryToken(t, currentUser)

	request := httptest.NewRequest(http.MethodGet, "/api/site_navigation_ws?token="+token, nil)
	recorder := serveRequest(request)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.NotContains(t, recorder.Body.String(), "Authorization failed")
}

// TestSiteNavigationWebSocketRejectsMissingToken keeps the endpoint closed to
// anonymous handshakes.
func TestSiteNavigationWebSocketRejectsMissingToken(t *testing.T) {
	setupRouterTestEnvironment(t)

	recorder := serveRequest(newWebSocketUpgradeRequest(
		"/api/site_navigation_ws",
		"http://example.com",
	))

	require.Equal(t, http.StatusForbidden, recorder.Code)
	require.Contains(t, recorder.Body.String(), "Authorization failed")
	require.False(t, recorder.hijacked)
}

// TestSiteNavigationWebSocketRejectsCrossSiteOrigin proves the CSWSH guard still
// applies after the route moved to the WebSocket router group.
func TestSiteNavigationWebSocketRejectsCrossSiteOrigin(t *testing.T) {
	currentUser := setupRouterTestEnvironment(t)
	token := queryToken(t, currentUser)

	recorder := serveRequest(newWebSocketUpgradeRequest(
		"/api/site_navigation_ws?token="+token,
		"http://attacker.example.net",
	))

	require.Equal(t, http.StatusForbidden, recorder.Code)
	require.NotContains(t, recorder.Body.String(), "Authorization failed")
	require.False(t, recorder.hijacked)
}

// TestCodeCompletionWebSocketAcceptsQueryToken covers the second WebSocket route
// that was mounted behind the header-only auth middleware.
func TestCodeCompletionWebSocketAcceptsQueryToken(t *testing.T) {
	currentUser := setupRouterTestEnvironment(t)
	token := queryToken(t, currentUser)

	recorder := serveRequest(newWebSocketUpgradeRequest(
		"/api/code_completion?token="+token,
		"http://example.com",
	))

	require.NotEqual(t, http.StatusForbidden, recorder.Code)
	require.NotContains(t, recorder.Body.String(), "Authorization failed")
}
