package nginx_log

import (
	"sync"
	"testing"
	"time"
)

// TestProgressTrackerIntegration tests the complete integration flow to ensure single completion notification
func TestProgressTrackerIntegration(t *testing.T) {
	// Test scenario: simulating what happens in ForceReindexFileGroup
	mainLogPath := "/var/log/nginx/access.log"
	
	// Step 1: Get progress tracker (simulates what happens in IndexLogFileFull)
	progressTracker := GetProgressTracker(mainLogPath)
	
	// Step 2: Add files and set estimates (simulates discovery phase)
	files := []struct {
		path       string
		compressed bool
		lines      int64
	}{
		{"/var/log/nginx/access.log", false, 1000},
		{"/var/log/nginx/access.log.1", false, 800},
		{"/var/log/nginx/access.log.2.gz", true, 600},
	}
	
	for _, file := range files {
		progressTracker.AddFile(file.path, file.compressed)
		progressTracker.SetFileEstimate(file.path, file.lines)
	}
	
	// Step 3: Simulate concurrent file processing (like what happens in real indexing)
	var wg sync.WaitGroup
	
	// Simulate concurrent file processing
	for i, file := range files {
		wg.Add(1)
		go func(f struct {
			path       string
			compressed bool
			lines      int64
		}, index int) {
			defer wg.Done()
			
			// Simulate processing time variation
			time.Sleep(time.Duration(index*10) * time.Millisecond)
			
			// Start processing
			progressTracker.StartFile(f.path)
			
			// Simulate incremental progress
			for progress := int64(100); progress <= f.lines; progress += 100 {
				progressTracker.UpdateFileProgress(f.path, progress)
				time.Sleep(time.Millisecond) // Small delay to simulate work
			}
			
			// Complete the file
			progressTracker.CompleteFile(f.path, f.lines)
			t.Logf("File %s completed with %d lines", f.path, f.lines)
		}(file, i)
	}
	
	// Wait for all files to complete
	wg.Wait()
	
	// Give a small delay to ensure all notifications are processed
	time.Sleep(10 * time.Millisecond)
	
	// Verify final state
	percentage, stats := progressTracker.GetProgress()
	if percentage != 100 {
		t.Errorf("Expected 100%% progress, got %.2f%%", percentage)
	}
	if stats.CompletedFiles != 3 {
		t.Errorf("Expected 3 completed files, got %d", stats.CompletedFiles)
	}
	if !stats.IsCompleted {
		t.Errorf("Expected IsCompleted to be true")
	}
	if stats.ProcessedLines != 2400 {
		t.Errorf("Expected 2400 processed lines, got %d", stats.ProcessedLines)
	}
	
	// Clean up
	RemoveProgressTracker(mainLogPath)
	
	t.Logf("Integration test passed: Progress tracking completed successfully")
}

// TestProgressTrackerRaceCondition tests concurrent CompleteFile calls to ensure no race conditions
func TestProgressTrackerRaceCondition(t *testing.T) {
	mainLogPath := "/var/log/nginx/test.log"
	progressTracker := GetProgressTracker(mainLogPath)
	
	// Add files
	files := []string{
		"/var/log/nginx/test.log",
		"/var/log/nginx/test.log.1", 
		"/var/log/nginx/test.log.2",
		"/var/log/nginx/test.log.3",
		"/var/log/nginx/test.log.4",
	}
	
	for _, file := range files {
		progressTracker.AddFile(file, false)
		progressTracker.SetFileEstimate(file, 100)
	}
	
	var wg sync.WaitGroup
	
	// Simulate simultaneous completion of all files (stress test)
	for i, file := range files {
		wg.Add(1)
		go func(f string, index int) {
			defer wg.Done()
			progressTracker.StartFile(f)
			
			// Try to complete multiple times to test race conditions
			for j := 0; j < 3; j++ {
				progressTracker.CompleteFile(f, 100)
				time.Sleep(time.Microsecond)
			}
		}(file, i)
	}
	
	wg.Wait()
	
	// Should still only have completed state once
	percentage, stats := progressTracker.GetProgress()
	if percentage != 100 {
		t.Errorf("Expected 100%% progress, got %.2f%%", percentage)
	}
	if !stats.IsCompleted {
		t.Errorf("Expected IsCompleted to be true")
	}
	
	RemoveProgressTracker(mainLogPath)
	t.Logf("Race condition test passed")
}