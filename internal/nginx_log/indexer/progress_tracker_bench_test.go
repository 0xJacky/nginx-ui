package indexer

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

func BenchmarkProgressTracker_UpdateFileProgress(b *testing.B) {
	tracker := NewProgressTracker("/var/log/nginx/benchmark.log", nil)
	
	// Add a file
	tracker.AddFile("/var/log/nginx/access.log", false)
	tracker.SetFileEstimate("/var/log/nginx/access.log", 100000)
	tracker.StartFile("/var/log/nginx/access.log")
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		tracker.UpdateFileProgress("/var/log/nginx/access.log", int64(i))
	}
}

func BenchmarkProgressTracker_GetProgress(b *testing.B) {
	tracker := NewProgressTracker("/var/log/nginx/benchmark.log", nil)
	
	// Add multiple files to make it realistic
	for i := 0; i < 10; i++ {
		fileName := fmt.Sprintf("/var/log/nginx/access-%d.log", i)
		tracker.AddFile(fileName, false)
		tracker.SetFileEstimate(fileName, 10000)
		tracker.StartFile(fileName)
		tracker.UpdateFileProgress(fileName, int64(i*1000))
	}
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		_ = tracker.GetProgress()
	}
}

func BenchmarkProgressTracker_GetAllFiles(b *testing.B) {
	tracker := NewProgressTracker("/var/log/nginx/benchmark.log", nil)
	
	// Add many files
	for i := 0; i < 100; i++ {
		fileName := fmt.Sprintf("/var/log/nginx/access-%d.log", i)
		tracker.AddFile(fileName, false)
		tracker.SetFileEstimate(fileName, 10000)
	}
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		_ = tracker.GetAllFiles()
	}
}

func BenchmarkProgressTracker_ConcurrentAccess(b *testing.B) {
	tracker := NewProgressTracker("/var/log/nginx/benchmark.log", nil)
	
	// Add files
	fileCount := 10
	for i := 0; i < fileCount; i++ {
		fileName := fmt.Sprintf("/var/log/nginx/access-%d.log", i)
		tracker.AddFile(fileName, false)
		tracker.SetFileEstimate(fileName, 10000)
		tracker.StartFile(fileName)
	}
	
	b.ResetTimer()
	b.ReportAllocs()
	
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			// Mix of operations
			switch i % 4 {
			case 0:
				// Update progress
				fileName := fmt.Sprintf("/var/log/nginx/access-%d.log", i%fileCount)
				tracker.UpdateFileProgress(fileName, int64(i))
			case 1:
				// Get progress
				_ = tracker.GetProgress()
			case 2:
				// Get file progress
				fileName := fmt.Sprintf("/var/log/nginx/access-%d.log", i%fileCount)
				_, _ = tracker.GetFileProgress(fileName)
			case 3:
				// Get all files
				_ = tracker.GetAllFiles()
			}
			i++
		}
	})
}

func BenchmarkProgressManager_GetTracker(b *testing.B) {
	manager := NewProgressManager()
	config := &ProgressConfig{}
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		logPath := fmt.Sprintf("/var/log/nginx/access-%d.log", i%100) // Reuse some paths
		_ = manager.GetTracker(logPath, config)
	}
}

func BenchmarkProgressManager_ConcurrentAccess(b *testing.B) {
	manager := NewProgressManager()
	config := &ProgressConfig{}
	
	b.ResetTimer()
	b.ReportAllocs()
	
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			switch i % 3 {
			case 0:
				// Get tracker
				logPath := fmt.Sprintf("/var/log/nginx/access-%d.log", i%50)
				_ = manager.GetTracker(logPath, config)
			case 1:
				// Get all trackers
				_ = manager.GetAllTrackers()
			case 2:
				// Remove tracker
				logPath := fmt.Sprintf("/var/log/nginx/access-%d.log", i%50)
				manager.RemoveTracker(logPath)
			}
			i++
		}
	})
}

func BenchmarkEstimateFileLines_NonExistent(b *testing.B) {
	ctx := context.Background()
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		_, _ = EstimateFileLines(ctx, "/nonexistent/file.log", 100000, false)
	}
}

func BenchmarkProgressTracker_CompleteWorkflow(b *testing.B) {
	// Benchmark a complete workflow: add files, process them, complete them
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		logPath := fmt.Sprintf("/var/log/nginx/benchmark-%d.log", i)
		tracker := NewProgressTracker(logPath, nil)
		b.StartTimer()
		
		// Add files
		for j := 0; j < 5; j++ {
			fileName := fmt.Sprintf("%s.%d", logPath, j)
			tracker.AddFile(fileName, j%2 == 0) // Mix compressed and uncompressed
			tracker.SetFileEstimate(fileName, 1000)
			tracker.SetFileSize(fileName, 150000)
		}
		
		// Process files
		for j := 0; j < 5; j++ {
			fileName := fmt.Sprintf("%s.%d", logPath, j)
			tracker.StartFile(fileName)
			
			// Simulate progress updates
			for k := 0; k < 10; k++ {
				tracker.UpdateFileProgress(fileName, int64(k*100))
			}
			
			tracker.CompleteFile(fileName, 1000)
		}
		
		// Get final progress
		_ = tracker.GetProgress()
	}
}

func BenchmarkProgressTracker_NotificationOverhead(b *testing.B) {
	var notificationCount int64
	var mu sync.Mutex
	
	config := &ProgressConfig{
		OnProgress: func(pn ProgressNotification) {
			mu.Lock()
			notificationCount++
			mu.Unlock()
		},
		OnCompletion: func(cn CompletionNotification) {
			mu.Lock()
			notificationCount++
			mu.Unlock()
		},
	}
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		tracker := NewProgressTracker(fmt.Sprintf("/var/log/nginx/bench-%d.log", i), config)
		tracker.AddFile(fmt.Sprintf("/var/log/nginx/bench-%d.log", i), false)
		tracker.SetFileEstimate(fmt.Sprintf("/var/log/nginx/bench-%d.log", i), 1000)
		b.StartTimer()
		
		tracker.StartFile(fmt.Sprintf("/var/log/nginx/bench-%d.log", i))
		
		// Multiple rapid updates to test throttling
		for j := 0; j < 100; j++ {
			tracker.UpdateFileProgress(fmt.Sprintf("/var/log/nginx/bench-%d.log", i), int64(j*10))
		}
		
		tracker.CompleteFile(fmt.Sprintf("/var/log/nginx/bench-%d.log", i), 1000)
	}
	
	// Brief wait for any pending notifications
	time.Sleep(time.Millisecond)
}

func BenchmarkProgressTracker_MemoryUsage(b *testing.B) {
	// Benchmark memory usage with many files
	tracker := NewProgressTracker("/var/log/nginx/memory-test.log", nil)
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		fileName := fmt.Sprintf("/var/log/nginx/access-%d.log", i)
		tracker.AddFile(fileName, false)
		tracker.SetFileEstimate(fileName, 10000)
		tracker.SetFileSize(fileName, 1500000)
		
		if i%100 == 0 {
			// Periodically check progress to ensure data structures are used
			_ = tracker.GetProgress()
		}
	}
}