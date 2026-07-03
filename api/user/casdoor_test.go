package user

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	cSettings "github.com/uozi-tech/cosy/settings"
)

func restoreCasdoorTestSettings(t *testing.T) {
	t.Helper()

	oldCasdoorSettings := *settings.CasdoorSettings
	oldEnableHTTPS := cSettings.ServerSettings.EnableHTTPS

	t.Cleanup(func() {
		*settings.CasdoorSettings = oldCasdoorSettings
		cSettings.ServerSettings.EnableHTTPS = oldEnableHTTPS
	})
}

func configureCasdoorTestSettings(t *testing.T) {
	t.Helper()

	restoreCasdoorTestSettings(t)

	settings.CasdoorSettings.Endpoint = "https://casdoor.example.com"
	settings.CasdoorSettings.ClientId = "client-id"
	settings.CasdoorSettings.RedirectUri = "https://nginx-ui.example.com/callback"
	settings.CasdoorSettings.Application = "nginx-ui-app"
	cSettings.ServerSettings.EnableHTTPS = true
}

func findResponseCookie(t *testing.T, recorder *httptest.ResponseRecorder, name string) *http.Cookie {
	t.Helper()

	for _, cookie := range recorder.Result().Cookies() {
		if cookie.Name == name {
			return cookie
		}
	}

	t.Fatalf("response cookie %q not found", name)
	return nil
}

func TestGetCasdoorUriSetsRandomStateCookie(t *testing.T) {
	configureCasdoorTestSettings(t)
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/user/casdoor_uri", nil)

	GetCasdoorUri(c)

	require.Equal(t, http.StatusOK, recorder.Code)

	var response struct {
		URI string `json:"uri"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.NotEmpty(t, response.URI)

	parsedURI, err := url.Parse(response.URI)
	require.NoError(t, err)

	state := parsedURI.Query().Get("state")
	require.NotEmpty(t, state)
	assert.NotEqual(t, settings.CasdoorSettings.Application, state)
	assert.True(t, strings.HasPrefix(state, "nginx-ui-casdoor_"))

	cookie := findResponseCookie(t, recorder, casdoorStateCookie)
	assert.Equal(t, state, cookie.Value)
	assert.Equal(t, casdoorStateMaxAge, cookie.MaxAge)
	assert.True(t, cookie.HttpOnly)
	assert.True(t, cookie.Secure)
	assert.Equal(t, http.SameSiteLaxMode, cookie.SameSite)
}

func TestCasdoorCallbackRejectsStateMismatchBeforeExchange(t *testing.T) {
	restoreCasdoorTestSettings(t)
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.POST("/api/user/casdoor_callback", CasdoorCallback)

	body := bytes.NewBufferString(`{"code":"oauth-code","state":"request-state"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/user/casdoor_callback", body)
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: casdoorStateCookie, Value: "cookie-state"})
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	assert.Equal(t, http.StatusForbidden, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "State mismatch")
}

func TestValidateCasdoorStateClearsCookieOnMatch(t *testing.T) {
	restoreCasdoorTestSettings(t)
	cSettings.ServerSettings.EnableHTTPS = true
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/user/casdoor_callback", nil)
	c.Request.AddCookie(&http.Cookie{Name: casdoorStateCookie, Value: "matching-state"})

	require.True(t, validateCasdoorState(c, "matching-state"))

	cookie := findResponseCookie(t, recorder, casdoorStateCookie)
	assert.Empty(t, cookie.Value)
	assert.True(t, cookie.HttpOnly)
	assert.True(t, cookie.Secure)
	assert.Equal(t, http.SameSiteLaxMode, cookie.SameSite)
	assert.Contains(t, strings.Join(recorder.Header().Values("Set-Cookie"), "\n"), "Max-Age=0")
}
