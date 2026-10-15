package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	internalmcp "github.com/0xJacky/Nginx-UI/internal/mcp"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMCPStreamableHTTP(t *testing.T) {
	router, _, userID := setupMCPSecurityRouter(t)
	internalmcp.Init(context.Background())
	_, token, err := internalmcp.CreateServiceToken("streamable-reader", []string{model.MCPTokenScopeRead}, nil, userID)
	require.NoError(t, err)

	request := func(method, body, sessionID string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, "/mcp", strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json, text/event-stream")
		if sessionID != "" {
			req.Header.Set("Mcp-Session-Id", sessionID)
			req.Header.Set("MCP-Protocol-Version", "2025-03-26")
		}
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, req)
		return recorder
	}

	initialized := request(http.MethodPost, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"transport-test","version":"1.0.0"}}}`, "")
	require.Equal(t, http.StatusOK, initialized.Code, initialized.Body.String())
	require.Contains(t, initialized.Header().Get("Content-Type"), "application/json")
	sessionID := initialized.Header().Get("Mcp-Session-Id")
	require.NotEmpty(t, sessionID)
	var response struct {
		ID     int `json:"id"`
		Result struct {
			ProtocolVersion string `json:"protocolVersion"`
		} `json:"result"`
	}
	require.NoError(t, json.Unmarshal(initialized.Body.Bytes(), &response))
	assert.Equal(t, 1, response.ID)
	assert.Equal(t, "2025-03-26", response.Result.ProtocolVersion)

	notification := request(http.MethodPost, `{"jsonrpc":"2.0","method":"notifications/initialized"}`, sessionID)
	assert.Equal(t, http.StatusAccepted, notification.Code, notification.Body.String())

	ping := request(http.MethodPost, `{"jsonrpc":"2.0","id":2,"method":"ping"}`, sessionID)
	assert.Equal(t, http.StatusOK, ping.Code, ping.Body.String())
	assert.JSONEq(t, `{"jsonrpc":"2.0","id":2,"result":{}}`, ping.Body.String())

	tools := request(http.MethodPost, `{"jsonrpc":"2.0","id":3,"method":"tools/list"}`, sessionID)
	assert.Equal(t, http.StatusOK, tools.Code, tools.Body.String())
	assert.Contains(t, tools.Body.String(), `"nginx_config_get"`)

	write := request(http.MethodPost, `{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"nginx_config_modify"}}`, sessionID)
	assert.Equal(t, http.StatusForbidden, write.Code)
	assert.JSONEq(t, `{"message":"MCP token scope is insufficient"}`, write.Body.String())

	unauthenticated := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":5,"method":"ping"}`))
	unauthenticated.Header.Set("Mcp-Session-Id", sessionID)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, unauthenticated)
	assert.Equal(t, http.StatusForbidden, recorder.Code)

	terminated := request(http.MethodDelete, "", sessionID)
	assert.Equal(t, http.StatusOK, terminated.Code, terminated.Body.String())
}

func TestMCPStreamableHTTPGetProtocolHeader(t *testing.T) {
	router, token, _ := setupMCPSecurityRouter(t)
	req := httptest.NewRequest(http.MethodGet, "/mcp", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("MCP-Protocol-Version", "2026-07-28")
	recorder := httptest.NewRecorder()

	// This protocol version removed standalone GET streams; the SDK must handle
	// the request instead of opening a legacy SSE connection.
	router.ServeHTTP(recorder, req)
	assert.Equal(t, http.StatusMethodNotAllowed, recorder.Code)
}

func TestMCPStreamableHTTPGetSessionHeader(t *testing.T) {
	router, token, _ := setupMCPSecurityRouter(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	req := httptest.NewRequest(http.MethodGet, "/mcp", nil).WithContext(ctx)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Mcp-Session-Id", "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)
	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Header().Get("Content-Type"), "text/event-stream")
	assert.NotContains(t, recorder.Body.String(), "event: endpoint")
}

func TestMCPLegacySSETransport(t *testing.T) {
	router, token, _ := setupMCPSecurityRouter(t)
	server := httptest.NewServer(router)
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL+"/mcp", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "text/event-stream")
	response, err := server.Client().Do(req)
	require.NoError(t, err)
	defer response.Body.Close()
	require.Equal(t, http.StatusOK, response.StatusCode)
	require.Contains(t, response.Header.Get("Content-Type"), "text/event-stream")

	scanner := bufio.NewScanner(response.Body)
	require.True(t, scanner.Scan())
	assert.Equal(t, "event: endpoint", scanner.Text())
	require.True(t, scanner.Scan())
	endpoint := strings.TrimPrefix(scanner.Text(), "data: ")
	require.True(t, strings.HasPrefix(endpoint, "/mcp_message?sessionId="), endpoint)

	message, err := http.NewRequestWithContext(ctx, http.MethodPost, server.URL+endpoint,
		bytes.NewBufferString(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"legacy-test","version":"1.0.0"}}}`))
	require.NoError(t, err)
	message.Header.Set("Authorization", "Bearer "+token)
	message.Header.Set("Content-Type", "application/json")
	accepted, err := server.Client().Do(message)
	require.NoError(t, err)
	defer accepted.Body.Close()
	assert.Equal(t, http.StatusAccepted, accepted.StatusCode)

	for scanner.Scan() {
		if strings.HasPrefix(scanner.Text(), "data: ") {
			var result map[string]any
			require.NoError(t, json.Unmarshal([]byte(strings.TrimPrefix(scanner.Text(), "data: ")), &result))
			assert.EqualValues(t, 1, result["id"])
			assert.Contains(t, result, "result")
			return
		}
	}
	t.Fatalf("legacy SSE response missing: %v", scanner.Err())
}
