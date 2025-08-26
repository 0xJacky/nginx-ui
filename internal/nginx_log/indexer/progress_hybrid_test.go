package indexer

import (
	"testing"
)

func TestProgressTracker_HybridCalculation(t *testing.T) {
	// Test the hybrid progress calculation logic
	tracker := NewProgressTracker("/var/log/nginx/access.log", nil)

	// Add 4 files with different estimates
	files := []string{
		"/var/log/nginx/access.log.1",
		"/var/log/nginx/access.log.2",
		"/var/log/nginx/access.log.3",
		"/var/log/nginx/access.log.4",
	}

	estimates := []int64{1000, 2000, 1500, 2500}

	for i, file := range files {
		tracker.AddFile(file, false)
		tracker.SetFileEstimate(file, estimates[i])
	}

	// Test scenario 1: 2 files completed, 1 processing
	tracker.CompleteFile(files[0], 1000) // File 1: completed with 1000 lines
	tracker.CompleteFile(files[1], 2000) // File 2: completed with 2000 lines
	tracker.StartFile(files[2])
	tracker.UpdateFileProgress(files[2], 750) // File 3: processing, 750/1500 lines

	progress := tracker.GetProgress()

	// Calculate expected hybrid progress with dynamic line estimation
	// After 2 completed files, dynamic estimation kicks in
	// Completed files: 1000 + 2000 = 3000 lines, average = 1500 per file
	// Dynamic total estimate: 1500 * 4 = 6000 lines
	// Line progress: (1000 + 2000 + 750) / 6000 = 62.5%
	// File progress: 2 / 4 = 50% (only completed files count fully)
	// Hybrid: (62.5 * 0.4) + (50 * 0.6) = 25 + 30 = 55%
	
	avgLinesPerCompletedFile := float64(3000) / 2.0 // 1500
	dynamicTotalEstimate := avgLinesPerCompletedFile * 4 // 6000
	expectedLineProgress := float64(3750) / dynamicTotalEstimate * 100 // 62.5%
	expectedFileProgress := 2.0 / 4.0 * 100                           // 50%
	expectedHybrid := (expectedLineProgress * 0.4) + (expectedFileProgress * 0.6)

	if progress.Percentage < expectedHybrid-1 || progress.Percentage > expectedHybrid+1 {
		t.Errorf("Expected hybrid progress around %.2f%%, got %.2f%%", expectedHybrid, progress.Percentage)
	}

	// Verify the safety cap: percentage shouldn't exceed file progress by more than 15%
	maxAllowed := expectedFileProgress + 15.0
	if progress.Percentage > maxAllowed {
		t.Errorf("Progress %.2f%% exceeded safety cap of %.2f%%", progress.Percentage, maxAllowed)
	}

	t.Logf("Line progress: %.2f%%, File progress: %.2f%%, Hybrid: %.2f%%, Actual: %.2f%%",
		expectedLineProgress, expectedFileProgress, expectedHybrid, progress.Percentage)
}

func TestProgressTracker_DynamicEstimation(t *testing.T) {
	// Test dynamic estimation adjustment
	tracker := NewProgressTracker("/var/log/nginx/access.log", nil)

	// Add 5 files with initial low estimates
	files := []string{
		"/var/log/nginx/access.log.1",
		"/var/log/nginx/access.log.2",
		"/var/log/nginx/access.log.3",
		"/var/log/nginx/access.log.4",
		"/var/log/nginx/access.log.5",
	}

	initialEstimate := int64(1000) // Low estimate
	for _, file := range files {
		tracker.AddFile(file, false)
		tracker.SetFileEstimate(file, initialEstimate)
	}

	// Complete 3 files with much higher actual counts
	actualCounts := []int64{3000, 3500, 2800} // Much higher than estimated
	for i := 0; i < 3; i++ {
		tracker.CompleteFile(files[i], actualCounts[i])
	}

	progress := tracker.GetProgress()

	// After completing 3 files (60% of files), the estimate should be fully adjusted
	// Average per completed file: (3000 + 3500 + 2800) / 3 = 3100
	// Projected total: 3100 * 5 = 15500
	// Since 3/5 = 60% > 20%, full dynamic adjustment is applied

	avgLinesPerCompletedFile := float64(3000+3500+2800) / 3 // 3100
	expectedAdjustedEstimate := int64(avgLinesPerCompletedFile * 5) // 15500

	if progress.EstimatedLines != expectedAdjustedEstimate {
		t.Errorf("Expected adjusted estimate to be %d (dynamic), got %d", expectedAdjustedEstimate, progress.EstimatedLines)
	}

	t.Logf("Original total estimate: %d, Adjusted estimate: %d, Actual average per file: %.0f",
		initialEstimate*5, progress.EstimatedLines, float64(3000+3500+2800)/3)
}