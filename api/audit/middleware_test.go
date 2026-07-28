package audit

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestSanitizeAuditLogRedactsAuthenticationMaterial(t *testing.T) {
	gin.SetMode(gin.TestMode)
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Request = httptest.NewRequest("POST", "/mcp?node_secret=leaked&safe=value", nil)
	context.Request.Header.Set("Authorization", "Bearer nui_pat_identifier_secret")
	context.Request.Header.Set("X-Node-Secret", "legacy-secret")
	MarkSensitiveRequest(context)
	MarkSensitiveResponse(context)
	logMap := map[string]string{
		"req_header": `{"Authorization":["Bearer nui_pat_identifier_secret"]}`,
		"req_url":    "/mcp?node_secret=leaked&safe=value",
		"req_body":   `{"code":"single-use-secret"}`,
		"resp_body":  `{"token":"nui_pat_identifier_secret"}`,
	}

	sanitizeAuditLog(context, logMap)

	assert.NotContains(t, logMap["req_header"], "nui_pat_identifier_secret")
	assert.NotContains(t, logMap["req_header"], "legacy-secret")
	assert.NotContains(t, logMap["req_url"], "leaked")
	assert.Contains(t, logMap["req_url"], "safe=value")
	assert.Equal(t, "[sensitive request redacted]", logMap["req_body"])
	assert.Equal(t, "[sensitive response redacted]", logMap["resp_body"])
}
