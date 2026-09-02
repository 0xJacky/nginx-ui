package nginx

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
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

func TestReloadTestsConfigurationAndHoldsApplyLock(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "/api/nginx/reload", nil)

	events := make([]string, 0, 3)
	now := time.Unix(100, 0)
	dependencies := reloadDependencies{
		now: func() time.Time { return now },
		tryLockApply: func() (func(), bool) {
			events = append(events, "lock")
			return func() { events = append(events, "unlock") }, true
		},
		tryTestAndReload: func() (*internalnginx.ControlResult, *internalnginx.ControlResult, bool) {
			events = append(events, "test-and-reload")
			return controlResult("configuration ok", nil), controlResult("reload ok", nil), true
		},
	}

	reload(context, dependencies, &reloadGate{cooldown: reloadCooldown})

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, []string{"lock", "test-and-reload", "unlock"}, events)
}

func TestReloadRejectsBusyConfigurationWithoutQueueing(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "/api/nginx/reload", nil)

	controlCalled := false
	dependencies := reloadDependencies{
		now:          time.Now,
		tryLockApply: func() (func(), bool) { return nil, false },
		tryTestAndReload: func() (*internalnginx.ControlResult, *internalnginx.ControlResult, bool) {
			controlCalled = true
			return nil, nil, false
		},
	}

	reload(context, dependencies, &reloadGate{cooldown: reloadCooldown})

	assert.Equal(t, http.StatusConflict, recorder.Code)
	assert.False(t, controlCalled)
}

func TestReloadCooldownReturnsRetryAfter(t *testing.T) {
	gin.SetMode(gin.TestMode)
	now := time.Unix(200, 0)
	var calls atomic.Int64
	dependencies := reloadDependencies{
		now:          func() time.Time { return now },
		tryLockApply: func() (func(), bool) { return func() {}, true },
		tryTestAndReload: func() (*internalnginx.ControlResult, *internalnginx.ControlResult, bool) {
			calls.Add(1)
			return controlResult("configuration ok", nil), controlResult("reload ok", nil), true
		},
	}
	gate := &reloadGate{cooldown: reloadCooldown}

	first := performReloadRequest(dependencies, gate)
	second := performReloadRequest(dependencies, gate)

	assert.Equal(t, http.StatusOK, first.Code)
	assert.Equal(t, http.StatusTooManyRequests, second.Code)
	assert.Equal(t, "2", second.Header().Get("Retry-After"))
	assert.EqualValues(t, 1, calls.Load())

	now = now.Add(reloadCooldown)
	third := performReloadRequest(dependencies, gate)
	assert.Equal(t, http.StatusOK, third.Code)
	assert.EqualValues(t, 2, calls.Load())
}

func TestReloadSingleFlightRejectsConcurrentRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	started := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	dependencies := reloadDependencies{
		now:          time.Now,
		tryLockApply: func() (func(), bool) { return func() {}, true },
		tryTestAndReload: func() (*internalnginx.ControlResult, *internalnginx.ControlResult, bool) {
			once.Do(func() { close(started) })
			<-release
			return controlResult("configuration ok", nil), controlResult("reload ok", nil), true
		},
	}
	gate := &reloadGate{cooldown: reloadCooldown}
	firstDone := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		firstDone <- performReloadRequest(dependencies, gate)
	}()

	<-started
	second := performReloadRequest(dependencies, gate)
	close(release)
	first := <-firstDone

	assert.Equal(t, http.StatusOK, first.Code)
	assert.Equal(t, http.StatusConflict, second.Code)
}

func TestReloadStormAB(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const requests = 64
	const simulatedCommandDuration = time.Millisecond

	var baselineCalls atomic.Int64
	var baselineMutex sync.Mutex
	baselineStarted := time.Now()
	var baselineGroup sync.WaitGroup
	for range requests {
		baselineGroup.Add(1)
		go func() {
			defer baselineGroup.Done()
			baselineMutex.Lock()
			baselineCalls.Add(1)
			time.Sleep(simulatedCommandDuration)
			baselineMutex.Unlock()
		}()
	}
	baselineGroup.Wait()
	baselineDuration := time.Since(baselineStarted)

	fixedNow := time.Unix(300, 0)
	var optimizedCalls atomic.Int64
	dependencies := reloadDependencies{
		now:          func() time.Time { return fixedNow },
		tryLockApply: func() (func(), bool) { return func() {}, true },
		tryTestAndReload: func() (*internalnginx.ControlResult, *internalnginx.ControlResult, bool) {
			optimizedCalls.Add(1)
			time.Sleep(simulatedCommandDuration)
			return controlResult("configuration ok", nil), controlResult("reload ok", nil), true
		},
	}
	gate := &reloadGate{cooldown: time.Minute}
	optimizedStarted := time.Now()
	var optimizedGroup sync.WaitGroup
	var accepted atomic.Int64
	for range requests {
		optimizedGroup.Add(1)
		go func() {
			defer optimizedGroup.Done()
			if recorder := performReloadRequest(dependencies, gate); recorder.Code == http.StatusOK {
				accepted.Add(1)
			}
		}()
	}
	optimizedGroup.Wait()
	optimizedDuration := time.Since(optimizedStarted)

	assert.EqualValues(t, requests, baselineCalls.Load())
	assert.EqualValues(t, 1, optimizedCalls.Load())
	assert.EqualValues(t, 1, accepted.Load())
	t.Logf("A/B reload storm: requests=%d baseline_executions=%d baseline_duration=%s optimized_executions=%d optimized_duration=%s reduction=%.2f%%",
		requests, baselineCalls.Load(), baselineDuration, optimizedCalls.Load(), optimizedDuration,
		100*(1-float64(optimizedCalls.Load())/float64(baselineCalls.Load())))
}

func controlResult(output string, err error) *internalnginx.ControlResult {
	return internalnginx.Control(func() (string, error) {
		return output, err
	})
}

func performReloadRequest(dependencies reloadDependencies, gate *reloadGate) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "/api/nginx/reload", nil)
	reload(context, dependencies, gate)
	return recorder
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
