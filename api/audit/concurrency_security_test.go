package audit

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uozi-tech/cosy/logger"
)

// auditedServer serves an engine behind the real LoggingMiddleware and
// collects every finished audit entry through cosy's monitor reporter, which
// runs on cosy's audit goroutine right after the nginx-ui log handler.
type auditedServer struct {
	*httptest.Server
	mu   sync.Mutex
	logs []map[string]string
}

func newAuditedServer(t *testing.T, register func(engine *gin.Engine)) *auditedServer {
	t.Helper()
	gin.SetMode(gin.TestMode)

	server := &auditedServer{}
	previous := logger.GetMonitorReporter()
	logger.SetMonitorReporter(func(_ string, logMap map[string]string) {
		entry := make(map[string]string, len(logMap))
		for key, value := range logMap {
			entry[key] = value
		}
		server.mu.Lock()
		server.logs = append(server.logs, entry)
		server.mu.Unlock()
	})

	engine := gin.New()
	engine.Use(LoggingMiddleware())
	register(engine)
	server.Server = httptest.NewServer(engine)
	t.Cleanup(func() {
		server.Close()
		logger.SetMonitorReporter(previous)
	})
	return server
}

func (s *auditedServer) send(t *testing.T, method, target, body string) {
	t.Helper()
	// assert rather than require: send also runs on worker goroutines.
	request, err := http.NewRequest(method, s.URL+target, bytes.NewBufferString(body))
	if !assert.NoError(t, err) {
		return
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := s.Client().Do(request)
	if !assert.NoError(t, err) {
		return
	}
	_, _ = io.Copy(io.Discard, response.Body)
	_ = response.Body.Close()
}

// waitForLogs returns the collected entries once count of them have arrived.
func (s *auditedServer) waitForLogs(t *testing.T, count int) []map[string]string {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for {
		s.mu.Lock()
		got := len(s.logs)
		s.mu.Unlock()
		if got >= count || time.Now().After(deadline) {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	require.Len(t, s.logs, count)
	return append([]map[string]string(nil), s.logs...)
}

func bindAndEcho(c *gin.Context) {
	var payload map[string]any
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "ok"})
}

// TestAuditLogRedactionSurvivesConcurrentRequests drives sensitive and
// ordinary requests concurrently through cosy's audit goroutine. gin recycles
// a request's context as soon as the handlers return, so redaction decided
// from the live context can be applied to the wrong request. The DNS
// credential body has no credential-like field names, so only the route rule
// keeps its secret out of the log.
func TestAuditLogRedactionSurvivesConcurrentRequests(t *testing.T) {
	server := newAuditedServer(t, func(engine *gin.Engine) {
		engine.POST("/api/user/password", bindAndEcho)
		engine.POST("/api/dns_credentials", bindAndEcho)
		engine.POST("/api/sites/:name", bindAndEcho)
	})

	const workers = 32
	const iterations = 30
	var wg sync.WaitGroup
	for worker := range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range iterations {
				secret := fmt.Sprintf("concurrent-secret-%d-%d", worker, i)
				server.send(t, http.MethodPost, "/api/user/password", `{"old_password":"`+secret+`","new_password":"`+secret+`"}`)
				server.send(t, http.MethodPost, "/api/dns_credentials", `{"name":"n","configuration":{"credentials":{"PROVIDER_X":"`+secret+`"}}}`)
				server.send(t, http.MethodPost, fmt.Sprintf("/api/sites/site-%d-%d", worker, i), `{"content":"server_name site-`+fmt.Sprint(worker)+`;"}`)
			}
		}()
	}
	wg.Wait()

	logs := server.waitForLogs(t, workers*iterations*3)
	leaked, keptSites := 0, 0
	for _, entry := range logs {
		for _, field := range []string{"req_body", "resp_body", "req_url", "session_logs"} {
			if strings.Contains(entry[field], "concurrent-secret-") {
				leaked++
			}
		}
		if strings.Contains(entry["req_body"], "server_name site-") {
			keptSites++
			// The URL must belong to the same request as the body.
			assert.Contains(t, entry["req_url"], "/api/sites/site-")
		}
		if strings.HasSuffix(entry["req_url"], "/api/user/password") || strings.HasSuffix(entry["req_url"], "/api/dns_credentials") {
			assert.Equal(t, "[sensitive request redacted]", entry["req_body"])
		}
	}
	assert.Zero(t, leaked, "credentials reached the audit log unredacted")
	assert.Equal(t, workers*iterations, keptSites, "ordinary bodies must stay in the audit log")
}
