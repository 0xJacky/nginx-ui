package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setDemoMode flips the package-level NodeSettings pointer for one test and
// restores it afterwards. NodeSettings is global state, so these tests must not
// run in parallel.
func setDemoMode(t *testing.T, enabled bool) {
	t.Helper()

	previous := settings.NodeSettings.Demo
	settings.NodeSettings.Demo = enabled
	t.Cleanup(func() {
		settings.NodeSettings.Demo = previous
	})
}

func newGuardedRouter(guard gin.HandlerFunc) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	handler := func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "reached"})
	}
	r.GET("/probe", guard, handler)
	r.POST("/probe", guard, handler)
	return r
}

func doRequest(t *testing.T, r *gin.Engine, method string) *httptest.ResponseRecorder {
	t.Helper()

	req, err := http.NewRequest(method, "/probe", nil)
	require.NoError(t, err)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestRejectInDemoBlocksWhenDemoEnabled(t *testing.T) {
	setDemoMode(t, true)

	w := doRequest(t, newGuardedRouter(RejectInDemo()), http.MethodPost)

	assert.NotContains(t, w.Body.String(), "reached")
	assert.Contains(t, w.Body.String(), "disabled in demo mode")
}

func TestRejectInDemoAllowsWhenDemoDisabled(t *testing.T) {
	setDemoMode(t, false)

	w := doRequest(t, newGuardedRouter(RejectInDemo()), http.MethodPost)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "reached")
}

func TestRejectInDemoBlocksReadsToo(t *testing.T) {
	setDemoMode(t, true)

	w := doRequest(t, newGuardedRouter(RejectInDemo()), http.MethodGet)

	assert.NotContains(t, w.Body.String(), "reached")
}

func TestDemoReadOnlyAllowsReadsBlocksWrites(t *testing.T) {
	setDemoMode(t, true)
	r := newGuardedRouter(DemoReadOnly())

	read := doRequest(t, r, http.MethodGet)
	assert.Equal(t, http.StatusOK, read.Code)
	assert.Contains(t, read.Body.String(), "reached")

	write := doRequest(t, r, http.MethodPost)
	assert.NotContains(t, write.Body.String(), "reached")
	assert.Contains(t, write.Body.String(), "disabled in demo mode")
}

func TestDemoReadOnlyIsInertWhenDemoDisabled(t *testing.T) {
	setDemoMode(t, false)
	r := newGuardedRouter(DemoReadOnly())

	for _, method := range []string{http.MethodGet, http.MethodPost} {
		w := doRequest(t, r, method)
		assert.Equal(t, http.StatusOK, w.Code, method)
		assert.Contains(t, w.Body.String(), "reached", method)
	}
}

func TestIsDemoSafeMethod(t *testing.T) {
	safe := []string{http.MethodGet, http.MethodHead, http.MethodOptions}
	unsafe := []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete}

	for _, method := range safe {
		assert.True(t, isDemoSafeMethod(method), method)
	}
	for _, method := range unsafe {
		assert.False(t, isDemoSafeMethod(method), method)
	}
}
