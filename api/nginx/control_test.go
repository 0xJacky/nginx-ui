package nginx

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	internalnginx "github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestBuildNamespaceTestConfigResponseIncludesSandboxFields(t *testing.T) {
	response := buildNamespaceTestConfigResponse(9, internalnginx.TestConfigResult{
		Message:       "sandbox failed",
		Level:         internalnginx.Error,
		TestScope:     internalnginx.TestScopeNamespaceSandbox,
		SandboxStatus: internalnginx.SandboxStatusFailed,
		ErrorCategory: internalnginx.ErrorCategoryMissingInclude,
	})

	assert.Equal(t, uint64(9), response["namespace_id"])
	assert.Equal(t, "sandbox failed", response["message"])
	assert.Equal(t, internalnginx.TestScopeNamespaceSandbox, response["test_scope"])
	assert.Equal(t, internalnginx.SandboxStatusFailed, response["sandbox_status"])
	assert.Equal(t, internalnginx.ErrorCategoryMissingInclude, response["error_category"])
}

func TestRestartAcceptsLegacyEmptyRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "/api/nginx/restart", nil)

	var receivedOperationID string
	restart(context, func(operationID string) (*internalnginx.ControlOperation, error) {
		receivedOperationID = operationID
		return &internalnginx.ControlOperation{
			ID:        "server-generated-id",
			Action:    "restart",
			State:     internalnginx.ControlOperationRunning,
			StartedAt: time.Now(),
			Level:     internalnginx.Unknown,
		}, nil
	})

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Empty(t, receivedOperationID)
	var response struct {
		Message string                         `json:"message"`
		Control internalnginx.ControlOperation `json:"control"`
	}
	assert.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	assert.Equal(t, "ok", response.Message)
	assert.Equal(t, "server-generated-id", response.Control.ID)
	assert.Equal(t, internalnginx.ControlOperationRunning, response.Control.State)
}

func TestRestartPassesValidOperationID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	operationID := "86bb559e-1e89-43d8-a568-6f1e0078116e"
	context.Request = httptest.NewRequest(http.MethodPost, "/api/nginx/restart", bytes.NewBufferString(`{"operation_id":"`+operationID+`"}`))
	context.Request.Header.Set("Content-Type", "application/json")

	var receivedOperationID string
	restart(context, func(received string) (*internalnginx.ControlOperation, error) {
		receivedOperationID = received
		return &internalnginx.ControlOperation{
			ID:        received,
			Action:    "restart",
			State:     internalnginx.ControlOperationRunning,
			StartedAt: time.Now(),
			Level:     internalnginx.Unknown,
		}, nil
	})

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, operationID, receivedOperationID)
}
