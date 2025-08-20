package nginx_log

import (
	"testing"
)

func TestProgressTracker(t *testing.T) {
	// Create a new progress tracker
	tracker := NewProgressTracker("/var/log/nginx/access.log")

	// Test adding files
	tracker.AddFile("/var/log/nginx/access.log", false)
	tracker.AddFile("/var/log/nginx/access.log.1", false)
	tracker.AddFile("/var/log/nginx/access.log.2.gz", true)

	// Set estimates for files
	tracker.SetFileEstimate("/var/log/nginx/access.log", 1000)
	tracker.SetFileEstimate("/var/log/nginx/access.log.1", 2000)
	tracker.SetFileEstimate("/var/log/nginx/access.log.2.gz", 500)

	// Test initial progress
	percentage, stats := tracker.GetProgress()
	if percentage != 0 {
		t.Errorf("Expected initial progress to be 0, got %.2f", percentage)
	}
	if stats.TotalFiles != 3 {
		t.Errorf("Expected 3 total files, got %d", stats.TotalFiles)
	}
	if stats.EstimatedLines != 3500 {
		t.Errorf("Expected 3500 estimated lines, got %d", stats.EstimatedLines)
	}

	// Start processing first file
	tracker.StartFile("/var/log/nginx/access.log")
	percentage, stats = tracker.GetProgress()
	if stats.ProcessingFiles != 1 {
		t.Errorf("Expected 1 processing file, got %d", stats.ProcessingFiles)
	}

	// Update progress for first file
	tracker.UpdateFileProgress("/var/log/nginx/access.log", 500)
	percentage, stats = tracker.GetProgress()
	expectedPercentage := float64(500) / float64(3500) * 100
	if percentage < expectedPercentage-1 || percentage > expectedPercentage+1 {
		t.Errorf("Expected progress around %.2f%%, got %.2f%%", expectedPercentage, percentage)
	}

	// Complete first file
	tracker.CompleteFile("/var/log/nginx/access.log", 1000)
	percentage, stats = tracker.GetProgress()
	if stats.CompletedFiles != 1 {
		t.Errorf("Expected 1 completed file, got %d", stats.CompletedFiles)
	}
	expectedPercentage = float64(1000) / float64(3500) * 100
	if percentage < expectedPercentage-1 || percentage > expectedPercentage+1 {
		t.Errorf("Expected progress around %.2f%% after first file completion, got %.2f%%", expectedPercentage, percentage)
	}

	// Complete all files - should only trigger completion notification once
	tracker.StartFile("/var/log/nginx/access.log.1")
	tracker.CompleteFile("/var/log/nginx/access.log.1", 2000)
	
	// Check that completion hasn't been triggered yet
	percentage, stats = tracker.GetProgress()
	if stats.IsCompleted {
		t.Errorf("Expected IsCompleted to be false when not all files are done")
	}
	
	tracker.StartFile("/var/log/nginx/access.log.2.gz")
	tracker.CompleteFile("/var/log/nginx/access.log.2.gz", 500)

	// Now all files should be complete
	percentage, stats = tracker.GetProgress()
	if percentage != 100 {
		t.Errorf("Expected 100%% progress when all files complete, got %.2f%%", percentage)
	}
	if stats.CompletedFiles != 3 {
		t.Errorf("Expected 3 completed files, got %d", stats.CompletedFiles)
	}
	if !stats.IsCompleted {
		t.Errorf("Expected IsCompleted to be true")
	}
	
	// Try to complete a file again - should not trigger another notification
	// (this simulates the bug scenario where multiple complete notifications could be sent)
	tracker.CompleteFile("/var/log/nginx/access.log", 1000)
	// The completion should have already been notified and won't notify again
}

func TestEstimateFileLines(t *testing.T) {
	// Test uncompressed file estimation
	lines := EstimateFileLines("/var/log/nginx/access.log", 10000, false)
	expected := int64(10000 / 100) // 100 bytes per line estimate
	if lines != expected {
		t.Errorf("Expected %d lines for uncompressed file, got %d", expected, lines)
	}

	// Test compressed file estimation
	lines = EstimateFileLines("/var/log/nginx/access.log.gz", 1000, true)
	expected = int64(1000 * 3 / 100) // 3:1 compression ratio, 100 bytes per line
	if lines != expected {
		t.Errorf("Expected %d lines for compressed file, got %d", expected, lines)
	}

	// Test zero size file
	lines = EstimateFileLines("/var/log/nginx/access.log", 0, false)
	if lines != 0 {
		t.Errorf("Expected 0 lines for zero size file, got %d", lines)
	}
}

func TestProgressTrackerCleanup(t *testing.T) {
	logGroupPath := "/test/log/group"
	
	// Get a progress tracker
	tracker1 := GetProgressTracker(logGroupPath)
	if tracker1 == nil {
		t.Errorf("Expected non-nil progress tracker")
	}
	
	// Get the same tracker again - should be the same instance
	tracker2 := GetProgressTracker(logGroupPath)
	if tracker1 != tracker2 {
		t.Errorf("Expected same tracker instance for same log group path")
	}
	
	// Remove the tracker
	RemoveProgressTracker(logGroupPath)
	
	// Get tracker again - should be a new instance
	tracker3 := GetProgressTracker(logGroupPath)
	if tracker3 == tracker1 {
		t.Errorf("Expected new tracker instance after removal")
	}
}