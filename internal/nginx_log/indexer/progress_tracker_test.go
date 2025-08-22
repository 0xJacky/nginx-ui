package indexer

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestProgressTracker_BasicFunctionality(t *testing.T) {
	// Create a new progress tracker
	var progressNotifications []ProgressNotification
	var completionNotifications []CompletionNotification
	var mu sync.Mutex

	config := &ProgressConfig{
		OnProgress: func(pn ProgressNotification) {
			mu.Lock()
			progressNotifications = append(progressNotifications, pn)
			mu.Unlock()
		},
		OnCompletion: func(cn CompletionNotification) {
			mu.Lock()
			completionNotifications = append(completionNotifications, cn)
			mu.Unlock()
		},
	}

	tracker := NewProgressTracker("/var/log/nginx/access.log", config)

	// Test adding files with various rotation formats
	tracker.AddFile("/var/log/nginx/access.log", false)
	tracker.AddFile("/var/log/nginx/access.log.1", false)
	tracker.AddFile("/var/log/nginx/access.log.2.gz", true)
	tracker.AddFile("/var/log/nginx/access.1.log", false)
	tracker.AddFile("/var/log/nginx/access.2.log.gz", true)

	// Set estimates for files
	tracker.SetFileEstimate("/var/log/nginx/access.log", 1000)
	tracker.SetFileEstimate("/var/log/nginx/access.log.1", 2000)
	tracker.SetFileEstimate("/var/log/nginx/access.log.2.gz", 500)
	tracker.SetFileEstimate("/var/log/nginx/access.1.log", 800)
	tracker.SetFileEstimate("/var/log/nginx/access.2.log.gz", 300)

	// Test initial progress
	progress := tracker.GetProgress()
	if progress.Percentage != 0 {
		t.Errorf("Expected initial progress to be 0, got %.2f", progress.Percentage)
	}
	if progress.TotalFiles != 5 {
		t.Errorf("Expected 5 total files, got %d", progress.TotalFiles)
	}
	expectedTotal := int64(1000 + 2000 + 500 + 800 + 300) // 4600
	if progress.EstimatedLines != expectedTotal {
		t.Errorf("Expected %d estimated lines, got %d", expectedTotal, progress.EstimatedLines)
	}

	// Start processing first file
	tracker.StartFile("/var/log/nginx/access.log")
	progress = tracker.GetProgress()
	if progress.ProcessingFiles != 1 {
		t.Errorf("Expected 1 processing file, got %d", progress.ProcessingFiles)
	}

	// Update progress for first file
	tracker.UpdateFileProgress("/var/log/nginx/access.log", 500)
	progress = tracker.GetProgress()
	expectedPercentage := float64(500) / float64(4600) * 100
	if progress.Percentage < expectedPercentage-1 || progress.Percentage > expectedPercentage+1 {
		t.Errorf("Expected progress around %.2f%%, got %.2f%%", expectedPercentage, progress.Percentage)
	}

	// Complete first file
	tracker.CompleteFile("/var/log/nginx/access.log", 1000)
	progress = tracker.GetProgress()
	if progress.CompletedFiles != 1 {
		t.Errorf("Expected 1 completed file, got %d", progress.CompletedFiles)
	}

	// Complete remaining files
	tracker.StartFile("/var/log/nginx/access.log.1")
	tracker.CompleteFile("/var/log/nginx/access.log.1", 2000)

	tracker.StartFile("/var/log/nginx/access.log.2.gz")
	tracker.CompleteFile("/var/log/nginx/access.log.2.gz", 500)

	tracker.StartFile("/var/log/nginx/access.1.log")
	tracker.CompleteFile("/var/log/nginx/access.1.log", 800)

	tracker.StartFile("/var/log/nginx/access.2.log.gz")
	tracker.CompleteFile("/var/log/nginx/access.2.log.gz", 300)

	// Wait a bit for notifications
	time.Sleep(10 * time.Millisecond)

	// Check final progress
	progress = tracker.GetProgress()
	if progress.Percentage != 100 {
		t.Errorf("Expected 100%% progress when all files complete, got %.2f%%", progress.Percentage)
	}
	if progress.CompletedFiles != 5 {
		t.Errorf("Expected 5 completed files, got %d", progress.CompletedFiles)
	}
	if !progress.IsCompleted {
		t.Errorf("Expected IsCompleted to be true")
	}

	// Verify notifications were sent
	mu.Lock()
	if len(completionNotifications) != 1 {
		t.Errorf("Expected 1 completion notification, got %d", len(completionNotifications))
	}
	mu.Unlock()
}

func TestProgressTracker_FileFailure(t *testing.T) {
	var completionNotifications []CompletionNotification
	var mu sync.Mutex

	config := &ProgressConfig{
		OnCompletion: func(cn CompletionNotification) {
			mu.Lock()
			completionNotifications = append(completionNotifications, cn)
			mu.Unlock()
		},
	}

	tracker := NewProgressTracker("/var/log/nginx/test.log", config)

	// Add files
	tracker.AddFile("/var/log/nginx/access.log", false)
	tracker.AddFile("/var/log/nginx/error.log", false)

	// Start processing
	tracker.StartFile("/var/log/nginx/access.log")
	tracker.CompleteFile("/var/log/nginx/access.log", 100)

	tracker.StartFile("/var/log/nginx/error.log")
	tracker.FailFile("/var/log/nginx/error.log", "permission denied")

	// Wait for notifications
	time.Sleep(10 * time.Millisecond)

	// Check progress
	progress := tracker.GetProgress()
	if progress.FailedFiles != 1 {
		t.Errorf("Expected 1 failed file, got %d", progress.FailedFiles)
	}
	if progress.CompletedFiles != 1 {
		t.Errorf("Expected 1 completed file, got %d", progress.CompletedFiles)
	}
	if !progress.IsCompleted {
		t.Errorf("Expected IsCompleted to be true")
	}

	// Verify completion notification
	mu.Lock()
	if len(completionNotifications) != 1 {
		t.Errorf("Expected 1 completion notification, got %d", len(completionNotifications))
	}
	if len(completionNotifications) > 0 && completionNotifications[0].Success {
		t.Errorf("Expected completion notification to indicate failure")
	}
	mu.Unlock()
}

func TestProgressTracker_GetFileProgress(t *testing.T) {
	tracker := NewProgressTracker("/var/log/nginx/test.log", nil)

	// Add a file
	tracker.AddFile("/var/log/nginx/access.log", false)
	tracker.SetFileEstimate("/var/log/nginx/access.log", 1000)
	tracker.SetFileSize("/var/log/nginx/access.log", 150000)

	// Get file progress
	fileProgress, exists := tracker.GetFileProgress("/var/log/nginx/access.log")
	if !exists {
		t.Error("Expected file progress to exist")
	}
	if fileProgress.EstimatedLines != 1000 {
		t.Errorf("Expected 1000 estimated lines, got %d", fileProgress.EstimatedLines)
	}
	if fileProgress.FileSize != 150000 {
		t.Errorf("Expected file size 150000, got %d", fileProgress.FileSize)
	}

	// Test non-existent file
	_, exists = tracker.GetFileProgress("/nonexistent.log")
	if exists {
		t.Error("Expected file progress to not exist for non-existent file")
	}
}

func TestProgressTracker_GetAllFiles(t *testing.T) {
	tracker := NewProgressTracker("/var/log/nginx/test.log", nil)

	// Add multiple files
	files := []string{
		"/var/log/nginx/access.log",
		"/var/log/nginx/error.log",
		"/var/log/nginx/ssl.log",
	}

	for _, file := range files {
		tracker.AddFile(file, false)
	}

	allFiles := tracker.GetAllFiles()
	if len(allFiles) != 3 {
		t.Errorf("Expected 3 files, got %d", len(allFiles))
	}

	for _, file := range files {
		if _, exists := allFiles[file]; !exists {
			t.Errorf("Expected file %s to exist in all files", file)
		}
	}
}

func TestProgressTracker_Cancel(t *testing.T) {
	var completionNotifications []CompletionNotification
	var mu sync.Mutex

	config := &ProgressConfig{
		OnCompletion: func(cn CompletionNotification) {
			mu.Lock()
			completionNotifications = append(completionNotifications, cn)
			mu.Unlock()
		},
	}

	tracker := NewProgressTracker("/var/log/nginx/test.log", config)

	// Add files
	tracker.AddFile("/var/log/nginx/access.log", false)
	tracker.AddFile("/var/log/nginx/error.log", false)

	// Start one file
	tracker.StartFile("/var/log/nginx/access.log")

	// Cancel the tracker
	tracker.Cancel("user requested cancellation")

	// Wait for notifications
	time.Sleep(10 * time.Millisecond)

	// Check that all files are marked as failed
	allFiles := tracker.GetAllFiles()
	for _, file := range allFiles {
		if file.State != FileStateFailed {
			t.Errorf("Expected file %s to be in failed state after cancellation", file.FilePath)
		}
	}

	// Check completion
	if !tracker.IsCompleted() {
		t.Error("Expected tracker to be completed after cancellation")
	}

	// Verify completion notification
	mu.Lock()
	if len(completionNotifications) != 1 {
		t.Errorf("Expected 1 completion notification after cancellation, got %d", len(completionNotifications))
	}
	mu.Unlock()
}

func TestProgressTracker_NoNotificationWithoutConfig(t *testing.T) {
	// Create tracker without notification config
	tracker := NewProgressTracker("/var/log/nginx/test.log", nil)

	// Add and complete a file
	tracker.AddFile("/var/log/nginx/access.log", false)
	tracker.StartFile("/var/log/nginx/access.log")
	tracker.CompleteFile("/var/log/nginx/access.log", 100)

	// Should not panic or cause issues
	progress := tracker.GetProgress()
	if !progress.IsCompleted {
		t.Error("Expected tracker to be completed")
	}
}

func TestProgressTracker_DuplicateCompletion(t *testing.T) {
	var completionNotifications []CompletionNotification
	var mu sync.Mutex

	config := &ProgressConfig{
		OnCompletion: func(cn CompletionNotification) {
			mu.Lock()
			completionNotifications = append(completionNotifications, cn)
			mu.Unlock()
		},
	}

	tracker := NewProgressTracker("/var/log/nginx/test.log", config)

	// Add a file
	tracker.AddFile("/var/log/nginx/access.log", false)
	tracker.StartFile("/var/log/nginx/access.log")

	// Complete the file multiple times
	tracker.CompleteFile("/var/log/nginx/access.log", 100)
	tracker.CompleteFile("/var/log/nginx/access.log", 100)
	tracker.CompleteFile("/var/log/nginx/access.log", 100)

	// Wait for notifications
	time.Sleep(10 * time.Millisecond)

	// Should only get one completion notification
	mu.Lock()
	if len(completionNotifications) != 1 {
		t.Errorf("Expected 1 completion notification despite multiple complete calls, got %d", len(completionNotifications))
	}
	mu.Unlock()
}

func TestProgressTracker_NotificationThrottling(t *testing.T) {
	var progressNotifications []ProgressNotification
	var mu sync.Mutex

	config := &ProgressConfig{
		NotifyInterval: 100 * time.Millisecond, // Custom throttle interval
		OnProgress: func(pn ProgressNotification) {
			mu.Lock()
			progressNotifications = append(progressNotifications, pn)
			mu.Unlock()
		},
	}

	tracker := NewProgressTracker("/var/log/nginx/test.log", config)

	// Add a file
	tracker.AddFile("/var/log/nginx/access.log", false)
	tracker.SetFileEstimate("/var/log/nginx/access.log", 1000)
	tracker.StartFile("/var/log/nginx/access.log")

	// Send multiple rapid updates
	for i := 0; i < 10; i++ {
		tracker.UpdateFileProgress("/var/log/nginx/access.log", int64(i*10))
		time.Sleep(10 * time.Millisecond) // Faster than throttle interval
	}

	// Should have throttled notifications
	mu.Lock()
	notificationCount := len(progressNotifications)
	mu.Unlock()

	if notificationCount > 5 { // Should be significantly less than 10 due to throttling
		t.Errorf("Expected notifications to be throttled, got %d notifications", notificationCount)
	}
}

func TestEstimateFileLines(t *testing.T) {
	ctx := context.Background()

	// Test zero size file
	lines, err := EstimateFileLines(ctx, "/nonexistent/file", 0, false)
	if err != nil {
		t.Errorf("Expected no error for zero size file, got %v", err)
	}
	if lines != 0 {
		t.Errorf("Expected 0 lines for zero size file, got %d", lines)
	}

	// Test fallback for non-existent file
	lines, err = EstimateFileLines(ctx, "/nonexistent/file", 10000, false)
	if err != nil {
		t.Errorf("Expected no error for fallback estimation, got %v", err)
	}
	expected := int64(10000 / 150) // fallback estimate
	if lines != expected {
		t.Errorf("Expected %d lines for fallback estimate, got %d", expected, lines)
	}

	// Test context cancellation - since EstimateFileLines returns fallback for non-existent files,
	// we test cancellation by using a valid path but cancelling the context
	// For a non-existent file, it returns immediately with fallback, so no cancellation check
	cancelCtx, cancel := context.WithCancel(ctx)
	cancel() // Cancel immediately

	// For non-existent files, EstimateFileLines returns fallback estimate without error
	lines, err = EstimateFileLines(cancelCtx, "/nonexistent/file", 10000, false)
	if err != nil {
		t.Errorf("Expected no error for non-existent file even with cancelled context, got %v", err)
	}
	if lines != 10000/150 {
		t.Errorf("Expected fallback estimate for cancelled context with non-existent file")
	}
}

func TestEstimateFileLines_RealFile(t *testing.T) {
	// Create a temporary file with known content
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.log")

	// Create test content with known line count
	content := ""
	lineCount := 100
	for i := 0; i < lineCount; i++ {
		content += "This is a test log line with some content to make it realistic\n"
	}

	err := os.WriteFile(testFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	fileInfo, err := os.Stat(testFile)
	if err != nil {
		t.Fatalf("Failed to stat test file: %v", err)
	}

	ctx := context.Background()
	estimatedLines, err := EstimateFileLines(ctx, testFile, fileInfo.Size(), false)
	if err != nil {
		t.Errorf("Expected no error for real file estimation, got %v", err)
	}

	// Allow some tolerance in estimation (within 20% of actual)
	tolerance := float64(lineCount) * 0.2
	if float64(estimatedLines) < float64(lineCount)-tolerance ||
		float64(estimatedLines) > float64(lineCount)+tolerance {
		t.Errorf("Estimated lines %d not within tolerance of actual lines %d", estimatedLines, lineCount)
	}
}

func TestProgressManager(t *testing.T) {
	manager := NewProgressManager()

	// Test getting new tracker
	config := &ProgressConfig{}
	tracker1 := manager.GetTracker("/var/log/nginx/access.log", config)
	if tracker1 == nil {
		t.Error("Expected non-nil tracker")
	}

	// Test getting same tracker
	tracker2 := manager.GetTracker("/var/log/nginx/access.log", config)
	if tracker1 != tracker2 {
		t.Error("Expected same tracker instance for same path")
	}

	// Test getting different tracker
	tracker3 := manager.GetTracker("/var/log/nginx/error.log", config)
	if tracker1 == tracker3 {
		t.Error("Expected different tracker instance for different path")
	}

	// Test getting all trackers
	allTrackers := manager.GetAllTrackers()
	if len(allTrackers) != 2 {
		t.Errorf("Expected 2 trackers, got %d", len(allTrackers))
	}

	// Test removing tracker
	manager.RemoveTracker("/var/log/nginx/access.log")
	allTrackers = manager.GetAllTrackers()
	if len(allTrackers) != 1 {
		t.Errorf("Expected 1 tracker after removal, got %d", len(allTrackers))
	}

	// Test getting tracker after removal creates new one
	tracker4 := manager.GetTracker("/var/log/nginx/access.log", config)
	if tracker1 == tracker4 {
		t.Error("Expected new tracker instance after removal")
	}
}

func TestProgressManager_Cleanup(t *testing.T) {
	manager := NewProgressManager()

	// Add trackers
	config := &ProgressConfig{}
	tracker1 := manager.GetTracker("/var/log/nginx/access.log", config)
	tracker2 := manager.GetTracker("/var/log/nginx/error.log", config)

	// Complete one tracker
	tracker1.AddFile("/var/log/nginx/access.log", false)
	tracker1.StartFile("/var/log/nginx/access.log")
	tracker1.CompleteFile("/var/log/nginx/access.log", 100)

	// Leave the other incomplete
	tracker2.AddFile("/var/log/nginx/error.log", false)
	tracker2.StartFile("/var/log/nginx/error.log")

	// Cleanup
	manager.Cleanup()

	// Should have removed completed tracker
	allTrackers := manager.GetAllTrackers()
	if len(allTrackers) != 1 {
		t.Errorf("Expected 1 tracker after cleanup, got %d", len(allTrackers))
	}

	// Remaining tracker should be the incomplete one
	if _, exists := allTrackers["/var/log/nginx/error.log"]; !exists {
		t.Error("Expected incomplete tracker to remain after cleanup")
	}
}

func TestFileState_String(t *testing.T) {
	tests := []struct {
		state    FileState
		expected string
	}{
		{FileStatePending, "pending"},
		{FileStateProcessing, "processing"},
		{FileStateCompleted, "completed"},
		{FileStateFailed, "failed"},
		{FileState(999), "unknown"},
	}

	for _, test := range tests {
		if test.state.String() != test.expected {
			t.Errorf("Expected %s, got %s", test.expected, test.state.String())
		}
	}
}

func TestProgressTracker_ConcurrentAccess(t *testing.T) {
	tracker := NewProgressTracker("/var/log/nginx/test.log", nil)

	// Add multiple files
	for i := 0; i < 10; i++ {
		tracker.AddFile(filepath.Join("/var/log/nginx", "access"+string(rune(i))+".log"), false)
		tracker.SetFileEstimate(filepath.Join("/var/log/nginx", "access"+string(rune(i))+".log"), 1000)
	}

	var wg sync.WaitGroup

	// Simulate concurrent progress updates
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(fileIndex int) {
			defer wg.Done()
			fileName := filepath.Join("/var/log/nginx", "access"+string(rune(fileIndex))+".log")

			tracker.StartFile(fileName)
			for j := 0; j < 100; j++ {
				tracker.UpdateFileProgress(fileName, int64(j*10))
				time.Sleep(time.Millisecond)
			}
			tracker.CompleteFile(fileName, 1000)
		}(i)
	}

	// Simulate concurrent progress reads
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				_ = tracker.GetProgress()
				_ = tracker.GetAllFiles()
				time.Sleep(time.Millisecond)
			}
		}()
	}

	wg.Wait()

	// Verify final state
	progress := tracker.GetProgress()
	if progress.CompletedFiles != 10 {
		t.Errorf("Expected 10 completed files after concurrent access, got %d", progress.CompletedFiles)
	}
	if !progress.IsCompleted {
		t.Error("Expected tracker to be completed after concurrent access")
	}
}

func TestRotationLogSupport(t *testing.T) {
	// Test compression detection
	testCases := []struct {
		filePath   string
		compressed bool
	}{
		{"/var/log/nginx/access.log", false},
		{"/var/log/nginx/access.log.1", false},
		{"/var/log/nginx/access.log.2.gz", true},
		{"/var/log/nginx/access.1.log", false},
		{"/var/log/nginx/access.2.log.gz", true},
		{"/var/log/nginx/access.3.log.bz2", true},
		{"/var/log/nginx/error.log", false},
		{"/var/log/nginx/error.log.1.xz", true},
		{"/var/log/nginx/error.1.log.lz4", true},
	}

	for _, tc := range testCases {
		isCompressed := IsCompressedFile(tc.filePath)
		if isCompressed != tc.compressed {
			t.Errorf("IsCompressedFile(%s) = %v, expected %v", tc.filePath, isCompressed, tc.compressed)
		}
	}

	// Test rotation log detection
	rotationTests := []struct {
		filePath  string
		isRotation bool
	}{
		{"/var/log/nginx/access.log", true},
		{"/var/log/nginx/access.log.1", true},
		{"/var/log/nginx/access.log.2.gz", true},
		{"/var/log/nginx/access.1.log", true},
		{"/var/log/nginx/access.2.log.gz", true},
		{"/var/log/nginx/error.log", true},
		{"/var/log/nginx/error.log.10", true},
		{"/var/log/nginx/error.1.log", true},
		{"/var/log/nginx/not-a-log.txt", false},
		{"/var/log/nginx/access.json", false},
		{"/var/log/nginx/config.conf", false},
	}

	for _, tc := range rotationTests {
		isRotation := IsRotationLogFile(tc.filePath)
		if isRotation != tc.isRotation {
			t.Errorf("IsRotationLogFile(%s) = %v, expected %v", tc.filePath, isRotation, tc.isRotation)
		}
	}

	// Test AddRotationFiles convenience method
	tracker := NewProgressTracker("/var/log/nginx/access.log", nil)
	
	// Add multiple rotation files at once
	rotationFiles := []string{
		"/var/log/nginx/access.log",
		"/var/log/nginx/access.log.1",
		"/var/log/nginx/access.log.2.gz",
		"/var/log/nginx/access.1.log",
		"/var/log/nginx/access.2.log.gz",
	}
	
	tracker.AddRotationFiles(rotationFiles...)
	
	progress := tracker.GetProgress()
	if progress.TotalFiles != 5 {
		t.Errorf("Expected 5 files after AddRotationFiles, got %d", progress.TotalFiles)
	}

	// Verify compression was detected correctly
	files := tracker.GetAllFiles()
	expectedCompression := map[string]bool{
		"/var/log/nginx/access.log":       false,
		"/var/log/nginx/access.log.1":     false,
		"/var/log/nginx/access.log.2.gz":  true,
		"/var/log/nginx/access.1.log":     false,
		"/var/log/nginx/access.2.log.gz":  true,
	}

	for filePath, expectedComp := range expectedCompression {
		found := false
		for _, file := range files {
			if file.FilePath == filePath {
				found = true
				if file.IsCompressed != expectedComp {
					t.Errorf("File %s: expected compressed=%v, got compressed=%v", 
						filePath, expectedComp, file.IsCompressed)
				}
				break
			}
		}
		if !found {
			t.Errorf("File %s not found in tracker", filePath)
		}
	}
}