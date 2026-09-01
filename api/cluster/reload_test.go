package cluster

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestReloadNginxDispatchSingleFlight(t *testing.T) {
	gin.SetMode(gin.TestMode)
	started := make(chan struct{})
	release := make(chan struct{})
	finished := make(chan struct{})
	received := make(chan []uint64, 1)
	var once sync.Once
	dependencies := reloadDispatchDependencies{
		now: time.Now,
		dispatch: func(nodeIDs []uint64) {
			received <- append([]uint64(nil), nodeIDs...)
			once.Do(func() { close(started) })
			<-release
			close(finished)
		},
	}
	gate := &reloadDispatchGate{cooldown: reloadDispatchCooldown}

	first := performClusterReloadRequest(dependencies, gate)
	require.Equal(t, http.StatusOK, first.Code)
	<-started
	require.Equal(t, []uint64{1, 2}, <-received)

	second := performClusterReloadRequest(dependencies, gate)
	require.Equal(t, http.StatusConflict, second.Code)

	close(release)
	<-finished
	require.Eventually(t, func() bool {
		gate.mutex.Lock()
		defer gate.mutex.Unlock()
		return !gate.inFlight
	}, time.Second, time.Millisecond)
}

func TestReloadNginxDispatchCooldown(t *testing.T) {
	gin.SetMode(gin.TestMode)
	now := time.Unix(400, 0)
	var calls atomic.Int64
	dependencies := reloadDispatchDependencies{
		now: func() time.Time { return now },
		dispatch: func([]uint64) {
			calls.Add(1)
		},
	}
	gate := &reloadDispatchGate{cooldown: reloadDispatchCooldown}

	first := performClusterReloadRequest(dependencies, gate)
	require.Equal(t, http.StatusOK, first.Code)
	waitForReloadDispatch(t, gate)

	second := performClusterReloadRequest(dependencies, gate)
	require.Equal(t, http.StatusTooManyRequests, second.Code)
	require.Equal(t, "2", second.Header().Get("Retry-After"))
	require.EqualValues(t, 1, calls.Load())

	now = now.Add(reloadDispatchCooldown)
	third := performClusterReloadRequest(dependencies, gate)
	require.Equal(t, http.StatusOK, third.Code)
	waitForReloadDispatch(t, gate)
	require.EqualValues(t, 2, calls.Load())
}

func performClusterReloadRequest(dependencies reloadDispatchDependencies, gate *reloadDispatchGate) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "/api/nodes/reload_nginx",
		bytes.NewBufferString(`{"node_ids":[1,2]}`))
	context.Request.Header.Set("Content-Type", "application/json")
	reloadNginx(context, dependencies, gate)
	return recorder
}

func waitForReloadDispatch(t *testing.T, gate *reloadDispatchGate) {
	t.Helper()
	require.Eventually(t, func() bool {
		gate.mutex.Lock()
		defer gate.mutex.Unlock()
		return !gate.inFlight && !gate.lastFinished.IsZero()
	}, time.Second, time.Millisecond)
}
