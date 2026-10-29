package user

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func restoreOIDCSettings(t *testing.T) {
	t.Helper()

	oldOIDCSettings := *settings.OIDCSettings

	t.Cleanup(func() {
		*settings.OIDCSettings = oldOIDCSettings
	})
}

// issueOIDCCallbackJSON posts an auth-code callback payload as JSON with a
// matching oidc_state cookie and returns the recorder, mirroring the browser
// path the issue reproduces.
func issueOIDCCallbackJSON(t *testing.T, code, state string) *httptest.ResponseRecorder {
	t.Helper()

	router := gin.New()
	router.POST("/api/oidc_callback", OIDCCallback)

	body := bytes.NewBufferString(fmt.Sprintf(`{"code":%q,"state":%q}`, code, state))
	req := httptest.NewRequest(http.MethodPost, "/api/oidc_callback", body)
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "oidc_state", Value: state})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder
}

// TestOIDCCallbackAcceptsLongAuthorizationCode is the regression for the bug
// where the authorization code was capped at 255 characters. OAuth 2.0 sets no
// limit, and real IdPs (Microsoft Entra ID issues ~1300-character codes) exceed
// it. The old `binding:"max=255"` made ShouldBind fail on a long code, which then
// fell back to the (empty) query params and surfaced as a misleading
// "Missing code or state".
//
// The OIDC config is left unset so the callback stops at "OIDC is not
// configured" — strictly after the code/state presence check — proving the long
// code was received and passed validation rather than dropped.
func TestOIDCCallbackAcceptsLongAuthorizationCode(t *testing.T) {
	restoreOIDCSettings(t)
	*settings.OIDCSettings = settings.OIDC{}
	gin.SetMode(gin.TestMode)

	const state = "nginx-ui-oidc_test123"
	longCode := strings.Repeat("a", 1300)

	recorder := issueOIDCCallbackJSON(t, longCode, state)

	require.Equal(t, http.StatusInternalServerError, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "OIDC is not configured")
	assert.NotContains(t, recorder.Body.String(), "Missing code or state")
}

// TestOIDCCallbackStillReportsMissingCode guards the presence check itself: a
// genuinely empty code must still be reported as missing, so the long-code test
// above is not passing vacuously.
func TestOIDCCallbackStillReportsMissingCode(t *testing.T) {
	restoreOIDCSettings(t)
	*settings.OIDCSettings = settings.OIDC{}
	gin.SetMode(gin.TestMode)

	recorder := issueOIDCCallbackJSON(t, "", "nginx-ui-oidc_test123")

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "Missing code or state")
}
