package nginx

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	internalnginx "github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestBuildNamespaceTestConfigResponseIncludesSandboxFields(t *testing.T) {
	response := buildNamespaceTestConfigResponse(9, 2, 1, internalnginx.TestConfigResult{
		Message:       "sandbox failed",
		Level:         internalnginx.Error,
		TestScope:     internalnginx.TestScopeNamespaceSandbox,
		SandboxStatus: internalnginx.SandboxStatusFailed,
		SandboxReason: internalnginx.SandboxReasonCustomTestCommand,
		ErrorCategory: internalnginx.ErrorCategoryMissingInclude,
	})

	assert.Equal(t, uint64(9), response["namespace_id"])
	assert.Equal(t, 2, response["site_count"])
	assert.Equal(t, 1, response["stream_count"])
	assert.Equal(t, "sandbox failed", response["message"])
	assert.Equal(t, internalnginx.TestScopeNamespaceSandbox, response["test_scope"])
	assert.Equal(t, internalnginx.SandboxStatusFailed, response["sandbox_status"])
	assert.Equal(t, internalnginx.SandboxReasonCustomTestCommand, response["sandbox_reason"])
	assert.Equal(t, internalnginx.ErrorCategoryMissingInclude, response["error_category"])
}

func TestTestConfigWithNamespaceFailsOnSiteQueryError(t *testing.T) {
	testConfigWithNamespaceQueryError(t, true)
}

func TestTestConfigWithNamespaceFailsOnStreamQueryError(t *testing.T) {
	testConfigWithNamespaceQueryError(t, false)
}

func testConfigWithNamespaceQueryError(t *testing.T, failSites bool) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	testCalled := false
	queryErr := errors.New("database is closed")
	dependencies := namespaceTestConfigDependencies{
		findNamespace: func(uint64) (*model.Namespace, error) {
			return &model.Namespace{Name: "demo", DeployMode: model.DeployModeLocal}, nil
		},
		findSitePaths: func(uint64) ([]string, error) {
			if failSites {
				return nil, queryErr
			}
			return []string{"site.conf"}, nil
		},
		findStreamPaths: func(uint64) ([]string, error) {
			if !failSites {
				return nil, queryErr
			}
			return []string{"stream.conf"}, nil
		},
		testConfig: func(*internalnginx.NamespaceInfo, []string, []string) internalnginx.TestConfigResult {
			testCalled = true
			return internalnginx.TestConfigResult{SandboxStatus: internalnginx.SandboxStatusOK}
		},
	}

	testConfigWithNamespace(context, 9, dependencies)
	assert.False(t, testCalled)
	assert.NotEqual(t, http.StatusOK, recorder.Code)
}

func TestTestConfigWithNamespaceReportsValidatedCoverageCounts(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	dependencies := namespaceTestConfigDependencies{
		findNamespace: func(uint64) (*model.Namespace, error) {
			return &model.Namespace{Name: "empty", DeployMode: model.DeployModeLocal}, nil
		},
		findSitePaths:   func(uint64) ([]string, error) { return nil, nil },
		findStreamPaths: func(uint64) ([]string, error) { return nil, nil },
		testConfig: func(*internalnginx.NamespaceInfo, []string, []string) internalnginx.TestConfigResult {
			return internalnginx.TestConfigResult{
				Level:         internalnginx.Info,
				SandboxStatus: internalnginx.SandboxStatusOK,
			}
		},
	}

	testConfigWithNamespace(context, 9, dependencies)
	assert.Equal(t, http.StatusOK, recorder.Code)
	var response map[string]any
	assert.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	assert.Equal(t, float64(0), response["site_count"])
	assert.Equal(t, float64(0), response["stream_count"])
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
