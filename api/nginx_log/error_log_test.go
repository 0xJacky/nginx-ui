package nginx_log

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/0xJacky/Nginx-UI/internal/nginx_log"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestEnrichErrorLogEntryRereadsTheRawLine(t *testing.T) {
	entry := enrichErrorLogEntry(map[string]interface{}{
		"level": "error",
		"raw":   `2026/10/01 12:00:01 [error] 12#13: *34 upstream timed out (110: Connection timed out), client: 192.0.2.10, server: example.com, request: "GET /api HTTP/1.1", upstream: "http://127.0.0.1:8080/api", host: "example.com"`,
	})
	assert.Equal(t, "upstream timed out (110: Connection timed out)", entry["message"])
	assert.EqualValues(t, 12, entry["pid"])
	assert.EqualValues(t, 34, entry["connection"])
	assert.Equal(t, "example.com", entry["server"])
	assert.Equal(t, "http://127.0.0.1:8080/api", entry["upstream"])
	assert.Equal(t, "GET /api HTTP/1.1", entry["request"])

	access := enrichErrorLogEntry(map[string]interface{}{"raw": "192.0.2.1 - - [01/Oct/2026:12:00:00 +0000] \"GET / HTTP/1.1\" 200 1"})
	assert.NotContains(t, access, "message", "access log entries are left alone")
}

func TestRejectErrorLogAnswersBadRequestForErrorLogs(t *testing.T) {
	gin.SetMode(gin.TestMode)
	nginx_log.AddLogPath("/var/log/nginx/reject-test.error.log", "error", "reject-test.error.log", "/etc/nginx/reject-test.conf")
	nginx_log.AddLogPath("/var/log/nginx/reject-test.access.log", "access", "reject-test.access.log", "/etc/nginx/reject-test.conf")
	t.Cleanup(func() { nginx_log.RemoveLogPathsFromConfig("/etc/nginx/reject-test.conf") })

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	assert.True(t, rejectErrorLog(c, "/var/log/nginx/reject-test.error.log"))
	assert.Equal(t, http.StatusBadRequest, recorder.Code)

	recorder = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(recorder)
	assert.False(t, rejectErrorLog(c, "/var/log/nginx/reject-test.access.log"))
	assert.False(t, rejectErrorLog(c, ""))
}
