package nginx_log

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/nginx_log/parser"
	"github.com/0xJacky/Nginx-UI/internal/nginx_log/utils"
)

// PerformanceThresholds defines minimum acceptable performance metrics
type PerformanceThresholds struct {
	ParseStreamOpsPerSec  float64 `json:"parse_stream_ops_per_sec"`
	SIMDOpsPerSec         float64 `json:"simd_ops_per_sec"`
	MemoryPoolOpsPerSec   float64 `json:"memory_pool_ops_per_sec"`
	RegexCacheOpsPerSec   float64 `json:"regex_cache_ops_per_sec"`
	MaxMemoryUsageMB      float64 `json:"max_memory_usage_mb"`
	MaxResponseTimeMS     float64 `json:"max_response_time_ms"`
}

// DefaultPerformanceThresholds returns the expected minimum performance levels
func DefaultPerformanceThresholds() *PerformanceThresholds {
	return &PerformanceThresholds{
		ParseStreamOpsPerSec: 500.0,     // 7-8x improvement target
		SIMDOpsPerSec:        10000.0,   // 235x improvement target
		MemoryPoolOpsPerSec:  100000.0,  // 48-81% improvement target
		RegexCacheOpsPerSec:  1000000.0, // High-performance caching target
		MaxMemoryUsageMB:     500.0,     // Maximum memory usage
		MaxResponseTimeMS:    1000.0,    // Maximum response time
	}
}

// PerformanceMetrics represents current system performance
type PerformanceMetrics struct {
	Timestamp             time.Time `json:"timestamp"`
	ParseStreamRate       float64   `json:"parse_stream_rate"`
	SIMDRate             float64   `json:"simd_rate"`
	MemoryPoolRate       float64   `json:"memory_pool_rate"`
	RegexCacheRate       float64   `json:"regex_cache_rate"`
	MemoryUsageMB        float64   `json:"memory_usage_mb"`
	ResponseTimeMS       float64   `json:"response_time_ms"`
	CacheHitRate         float64   `json:"cache_hit_rate"`
	TotalOperations      int64     `json:"total_operations"`
	ErrorRate            float64   `json:"error_rate"`
}

// PerformanceAlert represents a performance issue alert
type PerformanceAlert struct {
	Level       string    `json:"level"`       // "warning", "critical"
	Component   string    `json:"component"`   // "parser", "simd", "memory", "cache"
	Message     string    `json:"message"`
	CurrentValue float64   `json:"current_value"`
	ThresholdValue float64 `json:"threshold_value"`
	Timestamp   time.Time `json:"timestamp"`
	Suggestions []string  `json:"suggestions"`
}

// PerformanceMonitor provides real-time performance monitoring and alerting
type PerformanceMonitor struct {
	thresholds    *PerformanceThresholds
	metrics       *PerformanceMetrics
	alerts        []PerformanceAlert
	alertCallback func(PerformanceAlert)
	mu           sync.RWMutex
	running      bool
	stopChan     chan struct{}
}

// NewPerformanceMonitor creates a new performance monitor
func NewPerformanceMonitor(thresholds *PerformanceThresholds) *PerformanceMonitor {
	if thresholds == nil {
		thresholds = DefaultPerformanceThresholds()
	}
	
	return &PerformanceMonitor{
		thresholds: thresholds,
		metrics:    &PerformanceMetrics{},
		alerts:     make([]PerformanceAlert, 0),
		stopChan:   make(chan struct{}),
	}
}

// SetAlertCallback sets a callback function for performance alerts
func (pm *PerformanceMonitor) SetAlertCallback(callback func(PerformanceAlert)) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.alertCallback = callback
}

// StartMonitoring begins continuous performance monitoring
func (pm *PerformanceMonitor) StartMonitoring(ctx context.Context, interval time.Duration) {
	pm.mu.Lock()
	if pm.running {
		pm.mu.Unlock()
		return
	}
	pm.running = true
	pm.mu.Unlock()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			pm.stopMonitoring()
			return
		case <-pm.stopChan:
			return
		case <-ticker.C:
			pm.collectMetrics()
			pm.checkThresholds()
		}
	}
}

// StopMonitoring stops the performance monitoring
func (pm *PerformanceMonitor) StopMonitoring() {
	pm.stopMonitoring()
}

func (pm *PerformanceMonitor) stopMonitoring() {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	
	if pm.running {
		pm.running = false
		close(pm.stopChan)
		pm.stopChan = make(chan struct{})
	}
}

// collectMetrics gathers current performance metrics
func (pm *PerformanceMonitor) collectMetrics() {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	startTime := time.Now()
	
	// Test ParseStream performance
	parseRate := pm.benchmarkParseStream()
	
	// Test SIMD performance
	simdRate := pm.benchmarkSIMD()
	
	// Test memory pool performance
	poolRate := pm.benchmarkMemoryPools()
	
	// Test regex cache performance
	cacheRate := pm.benchmarkRegexCache()
	
	// Get cache hit rate
	hitRate := pm.getCacheHitRate()
	
	// Calculate response time
	responseTime := float64(time.Since(startTime).Nanoseconds()) / 1e6

	pm.metrics = &PerformanceMetrics{
		Timestamp:       time.Now(),
		ParseStreamRate: parseRate,
		SIMDRate:       simdRate,
		MemoryPoolRate: poolRate,
		RegexCacheRate: cacheRate,
		MemoryUsageMB:  pm.getMemoryUsage(),
		ResponseTimeMS: responseTime,
		CacheHitRate:   hitRate,
		TotalOperations: pm.getTotalOperations(),
		ErrorRate:      0.0, // Can be enhanced with actual error tracking
	}
}

// benchmarkParseStream tests ParseStream performance
func (pm *PerformanceMonitor) benchmarkParseStream() float64 {
	ctx := context.Background()
	testData := generateMonitoringTestData(100)
	
	config := parser.DefaultParserConfig()
	config.BatchSize = 100
	
	optimizedParser := parser.NewOptimizedParser(
		config,
		parser.NewSimpleUserAgentParser(),
		&mockMonitorGeoIPService{},
	)
	
	start := time.Now()
	result, err := optimizedParser.OptimizedParseStream(ctx, testData)
	if err != nil {
		return 0.0
	}
	
	duration := time.Since(start)
	return float64(result.Processed) / duration.Seconds()
}

// benchmarkSIMD tests SIMD parser performance
func (pm *PerformanceMonitor) benchmarkSIMD() float64 {
	testLine := `192.168.1.100 - - [06/Sep/2025:10:00:00 +0000] "GET /monitor HTTP/1.1" 200 1024 "https://test.com" "Monitor/1.0"`
	simdParser := parser.NewOptimizedLogLineParser()
	
	operations := 1000
	start := time.Now()
	
	for i := 0; i < operations; i++ {
		_ = simdParser.ParseLine([]byte(testLine))
	}
	
	duration := time.Since(start)
	return float64(operations) / duration.Seconds()
}

// benchmarkMemoryPools tests memory pool performance
func (pm *PerformanceMonitor) benchmarkMemoryPools() float64 {
	operations := 1000
	start := time.Now()
	
	for i := 0; i < operations; i++ {
		// String builder pool
		sb := utils.LogStringBuilderPool.Get()
		sb.WriteString("performance monitor test")
		utils.LogStringBuilderPool.Put(sb)
		
		// Byte slice pool
		slice := utils.GlobalByteSlicePool.Get(1024)
		utils.GlobalByteSlicePool.Put(slice)
	}
	
	duration := time.Since(start)
	return float64(operations*2) / duration.Seconds()
}

// benchmarkRegexCache tests regex cache performance
func (pm *PerformanceMonitor) benchmarkRegexCache() float64 {
	cache := parser.GetGlobalRegexCache()
	operations := 1000
	
	start := time.Now()
	
	for i := 0; i < operations; i++ {
		_, _ = cache.GetCommonRegex("ipv4")
		_, _ = cache.GetCommonRegex("timestamp")
		_, _ = cache.GetCommonRegex("status")
	}
	
	duration := time.Since(start)
	return float64(operations*3) / duration.Seconds()
}

// getCacheHitRate gets current cache hit rate
func (pm *PerformanceMonitor) getCacheHitRate() float64 {
	cache := parser.GetGlobalRegexCache()
	stats := cache.GetStats()
	return stats.HitRate
}

// getMemoryUsage returns current memory usage in MB (simplified)
func (pm *PerformanceMonitor) getMemoryUsage() float64 {
	// In a real implementation, this would use runtime.MemStats
	return 50.0 // Placeholder
}

// getTotalOperations returns total operations processed
func (pm *PerformanceMonitor) getTotalOperations() int64 {
	// In a real implementation, this would track actual operations
	return int64(time.Since(time.Now()).Seconds()) // Placeholder
}

// checkThresholds compares current metrics against thresholds and generates alerts
func (pm *PerformanceMonitor) checkThresholds() {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	// Check ParseStream performance
	if pm.metrics.ParseStreamRate < pm.thresholds.ParseStreamOpsPerSec {
		alert := PerformanceAlert{
			Level:       "critical",
			Component:   "parser",
			Message:     "ParseStream performance below threshold",
			CurrentValue: pm.metrics.ParseStreamRate,
			ThresholdValue: pm.thresholds.ParseStreamOpsPerSec,
			Timestamp:   time.Now(),
			Suggestions: []string{
				"Check if OptimizedParseStream is being used",
				"Verify batch size configuration (recommended: 500-1500)",
				"Monitor memory usage and GC pressure",
				"Check for context cancellation overhead",
			},
		}
		pm.addAlert(alert)
	}

	// Check SIMD performance
	if pm.metrics.SIMDRate < pm.thresholds.SIMDOpsPerSec {
		alert := PerformanceAlert{
			Level:       "critical",
			Component:   "simd",
			Message:     "SIMD parsing performance below threshold",
			CurrentValue: pm.metrics.SIMDRate,
			ThresholdValue: pm.thresholds.SIMDOpsPerSec,
			Timestamp:   time.Now(),
			Suggestions: []string{
				"Ensure SIMD parser is properly initialized",
				"Check log format compatibility with SIMD optimizations",
				"Verify CPU supports required SIMD instructions",
				"Monitor for regex compilation issues",
			},
		}
		pm.addAlert(alert)
	}

	// Check memory pool performance
	if pm.metrics.MemoryPoolRate < pm.thresholds.MemoryPoolOpsPerSec {
		alert := PerformanceAlert{
			Level:       "warning",
			Component:   "memory",
			Message:     "Memory pool performance below threshold",
			CurrentValue: pm.metrics.MemoryPoolRate,
			ThresholdValue: pm.thresholds.MemoryPoolOpsPerSec,
			Timestamp:   time.Now(),
			Suggestions: []string{
				"Check pool reuse rates (should be >80%)",
				"Consider increasing pool sizes",
				"Monitor for pool contention under high concurrency",
				"Pre-warm pools at application startup",
			},
		}
		pm.addAlert(alert)
	}

	// Check regex cache performance
	if pm.metrics.RegexCacheRate < pm.thresholds.RegexCacheOpsPerSec {
		alert := PerformanceAlert{
			Level:       "warning",
			Component:   "cache",
			Message:     "Regex cache performance below threshold",
			CurrentValue: pm.metrics.RegexCacheRate,
			ThresholdValue: pm.thresholds.RegexCacheOpsPerSec,
			Timestamp:   time.Now(),
			Suggestions: []string{
				"Check cache hit rate (should be >90%)",
				"Consider increasing cache size",
				"Monitor for cache eviction patterns",
				"Verify common regex patterns are cached",
			},
		}
		pm.addAlert(alert)
	}

	// Check cache hit rate
	if pm.metrics.CacheHitRate < 0.9 {
		alert := PerformanceAlert{
			Level:       "warning",
			Component:   "cache",
			Message:     "Cache hit rate is low",
			CurrentValue: pm.metrics.CacheHitRate * 100,
			ThresholdValue: 90.0,
			Timestamp:   time.Now(),
			Suggestions: []string{
				"Increase cache size for better hit rates",
				"Analyze cache usage patterns",
				"Pre-populate cache with common patterns",
			},
		}
		pm.addAlert(alert)
	}

	// Check memory usage
	if pm.metrics.MemoryUsageMB > pm.thresholds.MaxMemoryUsageMB {
		alert := PerformanceAlert{
			Level:       "critical",
			Component:   "memory",
			Message:     "Memory usage exceeds threshold",
			CurrentValue: pm.metrics.MemoryUsageMB,
			ThresholdValue: pm.thresholds.MaxMemoryUsageMB,
			Timestamp:   time.Now(),
			Suggestions: []string{
				"Check for memory leaks",
				"Increase pool usage to reduce allocations",
				"Monitor GC frequency and pressure",
				"Consider reducing batch sizes",
			},
		}
		pm.addAlert(alert)
	}
}

// addAlert adds a new alert and triggers the callback
func (pm *PerformanceMonitor) addAlert(alert PerformanceAlert) {
	pm.alerts = append(pm.alerts, alert)
	
	// Keep only the last 100 alerts
	if len(pm.alerts) > 100 {
		pm.alerts = pm.alerts[len(pm.alerts)-100:]
	}
	
	// Trigger callback if set
	if pm.alertCallback != nil {
		go pm.alertCallback(alert)
	}
}

// GetCurrentMetrics returns the current performance metrics
func (pm *PerformanceMonitor) GetCurrentMetrics() PerformanceMetrics {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return *pm.metrics
}

// GetRecentAlerts returns recent performance alerts
func (pm *PerformanceMonitor) GetRecentAlerts(since time.Duration) []PerformanceAlert {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	
	cutoff := time.Now().Add(-since)
	recent := make([]PerformanceAlert, 0)
	
	for _, alert := range pm.alerts {
		if alert.Timestamp.After(cutoff) {
			recent = append(recent, alert)
		}
	}
	
	return recent
}

// GetHealthStatus returns overall system health based on current metrics
func (pm *PerformanceMonitor) GetHealthStatus() string {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	
	criticalAlerts := 0
	warningAlerts := 0
	
	recentAlerts := pm.getRecentAlertsInternal(time.Hour)
	for _, alert := range recentAlerts {
		if alert.Level == "critical" {
			criticalAlerts++
		} else if alert.Level == "warning" {
			warningAlerts++
		}
	}
	
	if criticalAlerts > 0 {
		return "critical"
	} else if warningAlerts > 3 {
		return "warning"
	} else {
		return "healthy"
	}
}

func (pm *PerformanceMonitor) getRecentAlertsInternal(since time.Duration) []PerformanceAlert {
	cutoff := time.Now().Add(-since)
	recent := make([]PerformanceAlert, 0)
	
	for _, alert := range pm.alerts {
		if alert.Timestamp.After(cutoff) {
			recent = append(recent, alert)
		}
	}
	
	return recent
}

// ExportMetrics exports current metrics as JSON
func (pm *PerformanceMonitor) ExportMetrics() ([]byte, error) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	
	export := struct {
		Metrics    PerformanceMetrics   `json:"metrics"`
		Thresholds PerformanceThresholds `json:"thresholds"`
		Alerts     []PerformanceAlert   `json:"recent_alerts"`
		Health     string               `json:"health_status"`
	}{
		Metrics:    *pm.metrics,
		Thresholds: *pm.thresholds,
		Alerts:     pm.getRecentAlertsInternal(time.Hour),
		Health:     pm.GetHealthStatus(),
	}
	
	return json.MarshalIndent(export, "", "  ")
}

// DefaultAlertHandler provides a default implementation for handling alerts
func DefaultAlertHandler(alert PerformanceAlert) {
	log.Printf("PERFORMANCE ALERT [%s/%s]: %s (Current: %.2f, Threshold: %.2f)",
		alert.Level, alert.Component, alert.Message, alert.CurrentValue, alert.ThresholdValue)
	
	for _, suggestion := range alert.Suggestions {
		log.Printf("  Suggestion: %s", suggestion)
	}
}

// Example usage functions

// StartOptimizationMonitoring starts monitoring with default configuration
func StartOptimizationMonitoring(ctx context.Context) *PerformanceMonitor {
	monitor := NewPerformanceMonitor(DefaultPerformanceThresholds())
	monitor.SetAlertCallback(DefaultAlertHandler)
	
	// Start monitoring every 30 seconds
	go monitor.StartMonitoring(ctx, 30*time.Second)
	
	return monitor
}

// GetPerformanceReport generates a comprehensive performance report
func GetPerformanceReport(monitor *PerformanceMonitor) string {
	metrics := monitor.GetCurrentMetrics()
	recentAlerts := monitor.GetRecentAlerts(time.Hour)
	health := monitor.GetHealthStatus()
	
	report := fmt.Sprintf(`
=== NGINX-UI LOG PROCESSING PERFORMANCE REPORT ===

Health Status: %s
Report Generated: %s

PERFORMANCE METRICS:
├─ ParseStream Performance: %.2f ops/sec
├─ SIMD Parser Performance: %.2f ops/sec  
├─ Memory Pool Performance: %.2f ops/sec
├─ Regex Cache Performance: %.2f ops/sec
├─ Cache Hit Rate: %.2f%%
├─ Memory Usage: %.2f MB
├─ Response Time: %.2f ms
└─ Total Operations: %d

RECENT ALERTS (%d):`, 
		health, metrics.Timestamp.Format(time.RFC3339),
		metrics.ParseStreamRate, metrics.SIMDRate, metrics.MemoryPoolRate,
		metrics.RegexCacheRate, metrics.CacheHitRate*100, metrics.MemoryUsageMB,
		metrics.ResponseTimeMS, metrics.TotalOperations, len(recentAlerts))
	
	if len(recentAlerts) == 0 {
		report += "\n└─ No recent alerts - System performing well!"
	} else {
		for i, alert := range recentAlerts {
			prefix := "├─"
			if i == len(recentAlerts)-1 {
				prefix = "└─"
			}
			report += fmt.Sprintf("\n%s [%s] %s: %.2f (threshold: %.2f)",
				prefix, alert.Level, alert.Message, alert.CurrentValue, alert.ThresholdValue)
		}
	}
	
	return report
}

// Helper functions and mock implementations

func generateMonitoringTestData(lines int) *strings.Reader {
	// Simple test data generation for monitoring
	var testData strings.Builder
	for i := 0; i < lines; i++ {
		testData.WriteString(`192.168.1.100 - - [06/Sep/2025:10:00:00 +0000] "GET /test HTTP/1.1" 200 1024 "https://test.com" "Monitor/1.0"`)
		if i < lines-1 {
			testData.WriteString("\n")
		}
	}
	return strings.NewReader(testData.String())
}

type mockMonitorGeoIPService struct{}

func (m *mockMonitorGeoIPService) Search(ip string) (*parser.GeoLocation, error) {
	return &parser.GeoLocation{
		CountryCode: "US",
		RegionCode:  "CA",
		Province:    "California",
		City:        "San Francisco",
	}, nil
}