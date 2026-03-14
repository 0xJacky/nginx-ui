package mcp

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestMCPEndpointsRequireAuthentication(t *testing.T) {
	gin.SetMode(gin.TestMode)

	originalIPWhiteList := settings.AuthSettings.IPWhiteList
	t.Cleanup(func() {
		settings.AuthSettings.IPWhiteList = originalIPWhiteList
	})

	settings.AuthSettings.IPWhiteList = nil

	router := gin.New()
	InitRouter(router)

	for _, endpoint := range []string{"/mcp", "/mcp_message"} {
		req := httptest.NewRequest(http.MethodPost, endpoint, nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.JSONEq(t, `{"message":"Authorization failed"}`, w.Body.String())
	}
}
