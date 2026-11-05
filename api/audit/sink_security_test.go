package audit

import (
	"net/http"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uozi-tech/cosy/logger"
	cosySettings "github.com/uozi-tech/cosy/settings"
)

// requireNoAuditSink checks that the test binary runs without an audit sink.
// It does not change the sink settings: cosy goroutines left over from earlier
// tests still read them, and the race detector flags any write.
func requireNoAuditSink(t *testing.T) {
	t.Helper()
	require.Nil(t, logger.GetMonitorReporter())
	require.False(t, cosySettings.SLSSettings.Enable())
}

// TestAuditLogSkipsSanitizingWithoutSink makes sure installs without an audit
// sink do not pay for a redaction pass whose result nobody reads.
func TestAuditLogSkipsSanitizingWithoutSink(t *testing.T) {
	requireNoAuditSink(t)

	requestID := "no-sink-request"
	// LoggingMiddleware hands over no snapshot when there is no sink.
	pending := pendingAuditFor(requestID)
	close(pending.ready)

	body := `{"password":"` + syntheticSecret + `"}`
	logMap := map[string]string{
		logger.FieldRequestID: requestID,
		"req_url":             "/api/user/password",
		"req_body":            body,
	}
	handleAuditLog(logMap)

	// The entry is left as cosy built it, and the request is released.
	assert.Equal(t, body, logMap["req_body"])
	assert.Equal(t, "/api/user/password", logMap["req_url"])
	assert.NotContains(t, logMap, "user_id")
	select {
	case <-pending.released:
	default:
		t.Fatal("the request was not released")
	}
	_, stillPending := pendingAudits.Load(requestID)
	assert.False(t, stillPending)
}

func TestAuditSinkConfigured(t *testing.T) {
	assert.False(t, auditSinkConfigured(nil, &cosySettings.SLS{}))
	assert.True(t, auditSinkConfigured(func(string, map[string]string) {}, &cosySettings.SLS{}))
	// SLS alone keeps the sanitizer, and with it the fail-closed redaction.
	assert.True(t, auditSinkConfigured(nil, &cosySettings.SLS{
		AccessKeyId:         "id",
		AccessKeySecret:     "secret",
		EndPoint:            "endpoint",
		ProjectName:         "project",
		APILogStoreName:     "api",
		DefaultLogStoreName: "default",
	}))
}

// TestLoggingMiddlewareInstalledTwiceAuditsOnce makes a second installation
// harmless: without the guard every request waited out the full context hold
// and was logged twice.
func TestLoggingMiddlewareInstalledTwiceAuditsOnce(t *testing.T) {
	server := newAuditedServer(t, func(engine *gin.Engine) {
		engine.Use(LoggingMiddleware())
		engine.POST("/api/sites/:name", bindAndEcho)
	})

	started := time.Now()
	server.send(t, http.MethodPost, "/api/sites/example", `{"content":"server_name example;"}`)
	assert.Less(t, time.Since(started), auditContextHoldTimeout/2)

	entries := server.waitForLogs(t, 1)
	assert.Contains(t, entries[0]["req_body"], "server_name example;")
	// Give a duplicate entry time to show up.
	time.Sleep(100 * time.Millisecond)
	server.mu.Lock()
	defer server.mu.Unlock()
	require.Len(t, server.logs, 1)
}
