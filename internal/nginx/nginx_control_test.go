package nginx

import (
	"errors"
	"sync"
	"testing"
	"time"
)

func TestStartRestartPublishesRunningStateBeforeExecutionCompletes(t *testing.T) {
	resetControlStateForTest(t)

	started := make(chan struct{})
	release := make(chan struct{})
	var releaseOnce sync.Once
	releaseRestart := func() { releaseOnce.Do(func() { close(release) }) }
	t.Cleanup(releaseRestart)
	operation, err := startRestart("8d78c88f-e8af-4fa7-b607-b0a78f274260", func() (string, error) {
		close(started)
		<-release
		return "restart complete", nil
	})
	if err != nil {
		t.Fatalf("startRestart() error = %v", err)
	}
	if operation.State != ControlOperationRunning {
		t.Fatalf("operation state = %q, want %q", operation.State, ControlOperationRunning)
	}

	<-started
	resultRead := make(chan *ControlResult, 1)
	go func() {
		resultRead <- GetLastResult()
	}()
	select {
	case <-resultRead:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("GetLastResult() blocked while restart was running")
	}

	snapshot := GetControlOperation()
	if snapshot == nil || snapshot.ID != operation.ID || snapshot.State != ControlOperationRunning {
		t.Fatalf("GetControlOperation() = %#v, want running operation %q", snapshot, operation.ID)
	}

	releaseRestart()
	waitForControlOperationState(t, operation.ID, ControlOperationSucceeded)

	result := GetLastResult()
	if result.GetOutput() != "restart complete" {
		t.Fatalf("GetLastResult().GetOutput() = %q, want %q", result.GetOutput(), "restart complete")
	}
}

func TestStartRestartRejectsConcurrentOperationAndReusesOperationID(t *testing.T) {
	resetControlStateForTest(t)

	release := make(chan struct{})
	var releaseOnce sync.Once
	releaseRestart := func() { releaseOnce.Do(func() { close(release) }) }
	t.Cleanup(releaseRestart)
	operationID := "806811b0-17ba-4ff0-84cf-c414d9242052"
	operation, err := startRestart(operationID, func() (string, error) {
		<-release
		return "ok", nil
	})
	if err != nil {
		t.Fatalf("startRestart() error = %v", err)
	}

	retry, err := startRestart(operationID, func() (string, error) {
		t.Fatal("idempotent retry executed the restart command again")
		return "", nil
	})
	if err != nil {
		t.Fatalf("idempotent startRestart() error = %v", err)
	}
	if retry.ID != operation.ID {
		t.Fatalf("idempotent operation ID = %q, want %q", retry.ID, operation.ID)
	}

	_, err = startRestart("dc0e3aa2-3c28-412e-a02c-1b7da4899d4b", func() (string, error) {
		return "", nil
	})
	if !errors.Is(err, ErrControlOperationRunning) {
		t.Fatalf("concurrent startRestart() error = %v, want %v", err, ErrControlOperationRunning)
	}

	releaseRestart()
	waitForControlOperationState(t, operation.ID, ControlOperationSucceeded)
}

func TestStartRestartRecordsFailure(t *testing.T) {
	resetControlStateForTest(t)

	operation, err := startRestart("0be955a5-dcc0-4311-863f-c91356fc00ea", func() (string, error) {
		return "restart output", errors.New("restart failed")
	})
	if err != nil {
		t.Fatalf("startRestart() error = %v", err)
	}

	snapshot := waitForControlOperationState(t, operation.ID, ControlOperationFailed)
	if snapshot.Level != Error {
		t.Fatalf("operation level = %d, want %d", snapshot.Level, Error)
	}
	if snapshot.Message != "restart output restart failed" {
		t.Fatalf("operation message = %q, want combined command output", snapshot.Message)
	}
	if snapshot.FinishedAt == nil {
		t.Fatal("failed operation did not record finished_at")
	}
}

func resetControlStateForTest(t *testing.T) {
	t.Helper()
	commandMutex.Lock()
	resultMutex.Lock()
	lastStdOut = ""
	lastStdErr = nil
	lastControlOperation = nil
	resultMutex.Unlock()
	commandMutex.Unlock()
	t.Cleanup(func() {
		commandMutex.Lock()
		resultMutex.Lock()
		lastStdOut = ""
		lastStdErr = nil
		lastControlOperation = nil
		resultMutex.Unlock()
		commandMutex.Unlock()
	})
}

func waitForControlOperationState(t *testing.T, operationID string, state ControlOperationState) *ControlOperation {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		operation := GetControlOperation()
		if operation != nil && operation.ID == operationID && operation.State == state {
			return operation
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("operation %q did not reach state %q", operationID, state)
	return nil
}
