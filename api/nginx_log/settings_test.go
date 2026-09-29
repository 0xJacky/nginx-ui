package nginx_log

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetDefaultLogDirReportsTheDefaultLogFiles(t *testing.T) {
	gin.SetMode(gin.TestMode)

	logsDir := t.TempDir()
	accessLog := filepath.Join(logsDir, "access.log")
	errorLog := filepath.Join(logsDir, "error.log")

	previous := settings.NginxSettings
	t.Cleanup(func() { settings.NginxSettings = previous })
	settings.NginxSettings.AccessLogPath = accessLog
	settings.NginxSettings.ErrorLogPath = errorLog
	settings.NginxSettings.LogDirWhiteList = []string{logsDir}

	get := func() map[string]any {
		t.Helper()
		router := gin.New()
		router.GET("/nginx_log/default_log_dir", GetDefaultLogDir)
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/nginx_log/default_log_dir", nil))
		require.Equal(t, http.StatusOK, recorder.Code)

		var body map[string]any
		require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
		return body
	}

	body := get()
	assert.Equal(t, logsDir, body["access_log_dir"])
	assert.Equal(t, accessLog, body["access_log_path"])
	assert.Equal(t, errorLog, body["error_log_path"])
}
