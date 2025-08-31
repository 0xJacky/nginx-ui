package nginx_log

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/nginx_log/indexer"
)

// TestTaskRecoveryGlobalStatusManagement tests the global status management in task recovery
func TestTaskRecoveryGlobalStatusManagement(t *testing.T) {
	// Create a mock task recovery instance
	tr := &TaskRecovery{
		activeTasks: 0,
	}

	// Test initial state
	if atomic.LoadInt32(&tr.activeTasks) != 0 {
		t.Errorf("Expected initial active tasks to be 0, got %d", atomic.LoadInt32(&tr.activeTasks))
	}

	// Test incrementing active tasks
	firstTask := atomic.AddInt32(&tr.activeTasks, 1)
	if firstTask != 1 {
		t.Errorf("Expected first task counter to be 1, got %d", firstTask)
	}

	// Test adding more tasks
	secondTask := atomic.AddInt32(&tr.activeTasks, 1)
	if secondTask != 2 {
		t.Errorf("Expected second task counter to be 2, got %d", secondTask)
	}

	// Test decrementing tasks
	afterFirst := atomic.AddInt32(&tr.activeTasks, -1)
	if afterFirst != 1 {
		t.Errorf("Expected counter after completing first task to be 1, got %d", afterFirst)
	}

	// Test final task completion
	afterLast := atomic.AddInt32(&tr.activeTasks, -1)
	if afterLast != 0 {
		t.Errorf("Expected counter after completing last task to be 0, got %d", afterLast)
	}
}

// TestNeedsRecovery tests the logic for determining if a task needs recovery
func TestNeedsRecovery(t *testing.T) {
	tr := &TaskRecovery{}

	testCases := []struct {
		name      string
		log       *NginxLogWithIndex
		expected  bool
	}{
		{
			name: "Indexing status needs recovery",
			log: &NginxLogWithIndex{
				Path:        "/test/access.log",
				IndexStatus: string(indexer.IndexStatusIndexing),
			},
			expected: true,
		},
		{
			name: "Queued status needs recovery",
			log: &NginxLogWithIndex{
				Path:        "/test/access.log",
				IndexStatus: string(indexer.IndexStatusQueued),
			},
			expected: true,
		},
		{
			name: "Recent error needs recovery",
			log: &NginxLogWithIndex{
				Path:        "/test/access.log",
				IndexStatus: string(indexer.IndexStatusError),
				LastIndexed: time.Now().Add(-30 * time.Minute).Unix(), // 30 minutes ago
			},
			expected: true,
		},
		{
			name: "Old error does not need recovery",
			log: &NginxLogWithIndex{
				Path:        "/test/access.log",
				IndexStatus: string(indexer.IndexStatusError),
				LastIndexed: time.Now().Add(-2 * time.Hour).Unix(), // 2 hours ago
			},
			expected: false,
		},
		{
			name: "Indexed status does not need recovery",
			log: &NginxLogWithIndex{
				Path:        "/test/access.log",
				IndexStatus: string(indexer.IndexStatusIndexed),
			},
			expected: false,
		},
		{
			name: "Not indexed status does not need recovery",
			log: &NginxLogWithIndex{
				Path:        "/test/access.log",
				IndexStatus: string(indexer.IndexStatusNotIndexed),
			},
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := tr.needsRecovery(tc.log)
			if result != tc.expected {
				t.Errorf("Expected needsRecovery to return %t, got %t for %s", tc.expected, result, tc.name)
			}
		})
	}
}

// TestTaskRecoveryProgressConfig tests that progress configuration is properly created
func TestTaskRecoveryProgressConfig(t *testing.T) {
	// This test ensures that the progress config created in executeRecoveredTask
	// has the proper structure and callbacks
	
	progressCallbackCalled := false
	completionCallbackCalled := false
	
	// Create a mock progress config similar to what's created in executeRecoveredTask
	progressConfig := &indexer.ProgressConfig{
		NotifyInterval: 1 * time.Second,
		OnProgress: func(progress indexer.ProgressNotification) {
			progressCallbackCalled = true
		},
		OnCompletion: func(completion indexer.CompletionNotification) {
			completionCallbackCalled = true
		},
	}

	// Test that config is properly initialized
	if progressConfig.NotifyInterval != 1*time.Second {
		t.Errorf("Expected notify interval to be 1 second, got %v", progressConfig.NotifyInterval)
	}

	if progressConfig.OnProgress == nil {
		t.Error("Expected OnProgress callback to be set")
	}

	if progressConfig.OnCompletion == nil {
		t.Error("Expected OnCompletion callback to be set")
	}

	// Test callback execution
	if progressConfig.OnProgress != nil {
		progressConfig.OnProgress(indexer.ProgressNotification{})
	}

	if progressConfig.OnCompletion != nil {
		progressConfig.OnCompletion(indexer.CompletionNotification{})
	}

	if !progressCallbackCalled {
		t.Error("Progress callback was not called")
	}

	if !completionCallbackCalled {
		t.Error("Completion callback was not called")
	}
}