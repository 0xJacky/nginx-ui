package nginx_log

import (
	"context"
	"testing"
	"time"
)

// TestPerformanceMonitor tests the performance monitoring system
func TestPerformanceMonitor(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance monitor test in short mode")
	}

	t.Run("BasicFunctionality", func(t *testing.T) {
		// Create monitor with custom thresholds for testing
		thresholds := &PerformanceThresholds{
			ParseStreamOpsPerSec: 100.0,    // Low threshold for testing
			SIMDOpsPerSec:        1000.0,   // Low threshold for testing
			MemoryPoolOpsPerSec:  10000.0,  // Low threshold for testing
			RegexCacheOpsPerSec:  100000.0, // Low threshold for testing
			MaxMemoryUsageMB:     1000.0,   // High threshold for testing
			MaxResponseTimeMS:    5000.0,   // High threshold for testing
		}

		monitor := NewPerformanceMonitor(thresholds)

		// Test metrics collection
		monitor.collectMetrics()
		metrics := monitor.GetCurrentMetrics()

		// Verify metrics are populated
		if metrics.ParseStreamRate == 0 {
			t.Error("ParseStream rate not measured")
		}

		if metrics.SIMDRate == 0 {
			t.Error("SIMD rate not measured")
		}

		if metrics.MemoryPoolRate == 0 {
			t.Error("Memory pool rate not measured")
		}

		if metrics.RegexCacheRate == 0 {
			t.Error("Regex cache rate not measured")
		}

		t.Logf("Performance Metrics:")
		t.Logf("  ParseStream: %.2f ops/sec", metrics.ParseStreamRate)
		t.Logf("  SIMD: %.2f ops/sec", metrics.SIMDRate)
		t.Logf("  Memory Pool: %.2f ops/sec", metrics.MemoryPoolRate)
		t.Logf("  Regex Cache: %.2f ops/sec", metrics.RegexCacheRate)
		t.Logf("  Cache Hit Rate: %.2f%%", metrics.CacheHitRate*100)
	})

	t.Run("AlertGeneration", func(t *testing.T) {
		// Create monitor with very high thresholds to trigger alerts
		thresholds := &PerformanceThresholds{
			ParseStreamOpsPerSec: 1000000.0,  // Unrealistically high
			SIMDOpsPerSec:        10000000.0, // Unrealistically high
			MemoryPoolOpsPerSec:  100000000.0, // Unrealistically high
			RegexCacheOpsPerSec:  1000000000.0, // Unrealistically high
			MaxMemoryUsageMB:     1.0,         // Very low to trigger alert
			MaxResponseTimeMS:    0.1,         // Very low to trigger alert
		}

		monitor := NewPerformanceMonitor(thresholds)
		
		// Set up alert collection
		alertsReceived := make([]PerformanceAlert, 0)
		monitor.SetAlertCallback(func(alert PerformanceAlert) {
			alertsReceived = append(alertsReceived, alert)
		})

		// Collect metrics and check thresholds
		monitor.collectMetrics()
		monitor.checkThresholds()

		// Allow some time for alerts to be processed
		time.Sleep(100 * time.Millisecond)

		// Verify alerts were generated
		recentAlerts := monitor.GetRecentAlerts(time.Minute)
		if len(recentAlerts) == 0 {
			t.Error("Expected alerts to be generated with high thresholds")
		}

		t.Logf("Generated %d alerts:", len(recentAlerts))
		for _, alert := range recentAlerts {
			t.Logf("  [%s] %s: %.2f vs threshold %.2f",
				alert.Level, alert.Component, alert.CurrentValue, alert.ThresholdValue)
		}

		// Test health status
		health := monitor.GetHealthStatus()
		if health == "healthy" {
			t.Error("Expected unhealthy status with high thresholds")
		}
		t.Logf("Health status: %s", health)
	})

	t.Run("MetricsExport", func(t *testing.T) {
		monitor := NewPerformanceMonitor(DefaultPerformanceThresholds())
		monitor.collectMetrics()

		// Test JSON export
		jsonData, err := monitor.ExportMetrics()
		if err != nil {
			t.Fatalf("Failed to export metrics: %v", err)
		}

		if len(jsonData) == 0 {
			t.Error("Exported JSON is empty")
		}

		t.Logf("Exported JSON length: %d bytes", len(jsonData))

		// Test performance report generation
		report := GetPerformanceReport(monitor)
		if len(report) == 0 {
			t.Error("Performance report is empty")
		}

		t.Logf("Performance Report:\n%s", report)
	})

	t.Run("ContinuousMonitoring", func(t *testing.T) {
		monitor := NewPerformanceMonitor(DefaultPerformanceThresholds())
		
		// Set up alert tracking
		alertCount := 0
		monitor.SetAlertCallback(func(alert PerformanceAlert) {
			alertCount++
		})

		// Start monitoring for a short period
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		// Start monitoring with short interval
		go monitor.StartMonitoring(ctx, 500*time.Millisecond)

		// Wait for monitoring to run
		<-ctx.Done()

		// Stop monitoring
		monitor.StopMonitoring()

		// Verify some metrics were collected
		metrics := monitor.GetCurrentMetrics()
		if metrics.Timestamp.IsZero() {
			t.Error("No metrics were collected during monitoring")
		}

		t.Logf("Monitoring completed. Final metrics timestamp: %s", 
			metrics.Timestamp.Format(time.RFC3339))
	})
}

// TestPerformanceThresholds tests the default thresholds configuration
func TestPerformanceThresholds(t *testing.T) {
	thresholds := DefaultPerformanceThresholds()

	// Verify default thresholds are reasonable
	if thresholds.ParseStreamOpsPerSec < 100.0 {
		t.Error("ParseStream threshold too low")
	}

	if thresholds.SIMDOpsPerSec < 1000.0 {
		t.Error("SIMD threshold too low")
	}

	if thresholds.MemoryPoolOpsPerSec < 10000.0 {
		t.Error("Memory pool threshold too low")
	}

	if thresholds.RegexCacheOpsPerSec < 100000.0 {
		t.Error("Regex cache threshold too low")
	}

	t.Logf("Default thresholds: ParseStream=%.0f, SIMD=%.0f, MemPool=%.0f, Cache=%.0f",
		thresholds.ParseStreamOpsPerSec, thresholds.SIMDOpsPerSec,
		thresholds.MemoryPoolOpsPerSec, thresholds.RegexCacheOpsPerSec)
}

// TestOptimizationMonitoringIntegration tests the complete monitoring integration
func TestOptimizationMonitoringIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping optimization monitoring integration test in short mode")
	}

	// Test the high-level monitoring function
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	monitor := StartOptimizationMonitoring(ctx)

	// Wait for some monitoring cycles
	time.Sleep(1 * time.Second)

	// Check that monitoring is working
	metrics := monitor.GetCurrentMetrics()
	if metrics.Timestamp.IsZero() {
		t.Error("Monitoring did not collect metrics")
	}

	// Get performance report
	report := GetPerformanceReport(monitor)
	if len(report) == 0 {
		t.Error("Performance report generation failed")
	}

	// Check health status
	health := monitor.GetHealthStatus()
	if health == "" {
		t.Error("Health status not determined")
	}

	t.Logf("Integration test completed successfully")
	t.Logf("Health: %s", health)
	t.Logf("Latest metrics: ParseStream=%.2f, SIMD=%.2f ops/sec",
		metrics.ParseStreamRate, metrics.SIMDRate)

	// Stop monitoring
	monitor.StopMonitoring()
}

// BenchmarkPerformanceMonitoring benchmarks the monitoring overhead
func BenchmarkPerformanceMonitoring(b *testing.B) {
	monitor := NewPerformanceMonitor(DefaultPerformanceThresholds())
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		monitor.collectMetrics()
		monitor.checkThresholds()
	}
}