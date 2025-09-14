package nginx_log

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/nginx_log/indexer"
)

// TestAccurateProductionPerformance tests the exact same workflow as production rebuild
func TestAccurateProductionPerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping accurate production performance test in short mode")
	}

	// Test with realistic scales matching your production usage
	testSizes := []struct {
		name    string
		records int
	}{
		{"Production_50K", 50000},   // Smaller scale for quick validation
		{"Production_100K", 100000}, // Medium scale
	}

	for _, testSize := range testSizes {
		t.Run(testSize.name, func(t *testing.T) {
			runAccurateProductionTest(t, testSize.records)
		})
	}
}

func runAccurateProductionTest(t *testing.T, recordCount int) {
	t.Logf("🚀 Starting ACCURATE production test with %d records (same as production rebuild)", recordCount)

	tempDir := t.TempDir()

	// Generate test data with production-like log entries
	testLogFile := filepath.Join(tempDir, "access.log")
	dataGenStart := time.Now()

	if err := generateProductionLikeLogFile(testLogFile, recordCount); err != nil {
		t.Fatalf("Failed to generate test data: %v", err)
	}

	dataGenTime := time.Since(dataGenStart)
	t.Logf("📊 Generated %d records in %v", recordCount, dataGenTime)

	// Create indexer with PRODUCTION configuration (not test configuration)
	indexDir := filepath.Join(tempDir, "index")
	if err := os.MkdirAll(indexDir, 0755); err != nil {
		t.Fatalf("Failed to create index dir: %v", err)
	}

	// Use production default configuration
	config := indexer.DefaultIndexerConfig()
	config.IndexPath = indexDir
	// Don't override the optimized defaults - use them as-is

	shardManager := indexer.NewGroupedShardManager(config)
	parallelIndexer := indexer.NewParallelIndexer(config, shardManager)

	ctx := context.Background()
	if err := parallelIndexer.Start(ctx); err != nil {
		t.Fatalf("Failed to start indexer: %v", err)
	}
	defer parallelIndexer.Stop()

	// Now use EXACT same method as production: IndexLogGroupWithProgress
	t.Logf("🔄 Starting production rebuild using IndexLogGroupWithProgress")

	productionStart := time.Now()

	// Create progress config (similar to production but with test logging)
	progressConfig := &indexer.ProgressConfig{
		NotifyInterval: 1 * time.Second,
		OnProgress: func(progress indexer.ProgressNotification) {
			t.Logf("📈 Progress: %.1f%% - Files: %d/%d, Lines: %d/%d",
				progress.Percentage, progress.CompletedFiles, progress.TotalFiles,
				progress.ProcessedLines, progress.EstimatedLines)
		},
		OnCompletion: func(completion indexer.CompletionNotification) {
			t.Logf("✅ Completed: %s - Success: %t, Duration: %s, Lines: %d",
				completion.LogGroupPath, completion.Success, completion.Duration, completion.TotalLines)
		},
	}

	// Call the EXACT same method as production
	docsCountMap, minTime, maxTime, err := parallelIndexer.IndexLogGroupWithProgress(testLogFile, progressConfig)

	productionTime := time.Since(productionStart)

	if err != nil {
		t.Fatalf("IndexLogGroupWithProgress failed: %v", err)
	}

	// Calculate metrics (same as production rebuild reporting)
	var totalIndexedDocs uint64
	for _, count := range docsCountMap {
		totalIndexedDocs += count
	}

	throughput := float64(totalIndexedDocs) / productionTime.Seconds()

	t.Logf("🏆 === ACCURATE PRODUCTION RESULTS ===")
	t.Logf("📊 Input Records: %d", recordCount)
	t.Logf("📋 Documents Indexed: %d", totalIndexedDocs)
	t.Logf("⏱️  Total Time: %s", productionTime)
	t.Logf("🚀 Throughput: %.0f records/second", throughput)
	t.Logf("📈 Files Processed: %d", len(docsCountMap))

	if minTime != nil && maxTime != nil {
		t.Logf("📅 Time Range: %s to %s", minTime.Format(time.RFC3339), maxTime.Format(time.RFC3339))
	}

	// Flush all data (same as production)
	if err := parallelIndexer.FlushAll(); err != nil {
		t.Logf("Warning: Flush failed: %v", err)
	}

	// Performance validation
	if throughput < 1000 {
		t.Errorf("⚠️  Throughput too low: %.0f records/sec (expected >1000 for production)", throughput)
	}

	if totalIndexedDocs == 0 {
		t.Errorf("❌ No documents were indexed")
	}

	t.Logf("✨ Test completed successfully - Production performance validated")
}

func generateProductionLikeLogFile(filename string, recordCount int) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	baseTime := time.Now().Unix() - 86400 // 24 hours ago

	// Production-like variety in IPs, paths, user agents, etc.
	ips := []string{
		"192.168.1.100", "10.0.0.45", "172.16.0.25", "203.0.113.45",
		"198.51.100.67", "192.0.2.89", "203.0.113.234", "198.51.100.123",
		"10.0.1.45", "192.168.2.78", "172.16.1.99", "10.0.2.156",
	}

	methods := []string{"GET", "POST", "PUT", "DELETE", "HEAD"}

	paths := []string{
		"/", "/api/users", "/api/posts", "/api/auth/login", "/api/data",
		"/static/css/main.css", "/static/js/app.js", "/images/logo.png",
		"/admin/dashboard", "/user/profile", "/search", "/api/v1/metrics",
		"/health", "/favicon.ico", "/robots.txt", "/sitemap.xml",
	}

	userAgents := []string{
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/91.0.4472.124",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 Safari/605.1.15",
		"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 Chrome/91.0.4472.124",
		"Mozilla/5.0 (iPhone; CPU iPhone OS 14_6) AppleWebKit/605.1.15 Mobile Safari/604.1",
		"Mozilla/5.0 (Android 11; Mobile) AppleWebKit/537.36 Chrome/91.0.4472.124",
	}

	statuses := []int{200, 200, 200, 200, 200, 304, 301, 404, 500} // Weighted toward 200

	for i := 0; i < recordCount; i++ {
		// Distribute timestamps over 24 hours
		timestamp := baseTime + int64(i%86400)
		ip := ips[i%len(ips)]
		method := methods[i%len(methods)]
		path := paths[i%len(paths)]
		status := statuses[i%len(statuses)]
		size := 500 + (i % 5000) // Vary response sizes
		userAgent := userAgents[i%len(userAgents)]

		referer := "-"
		if i%10 == 0 { // 10% of requests have referrer
			referer = "https://example.com/page"
		}

		requestTime := float64(i%2000) / 1000.0 // 0-2 seconds

		// Standard nginx combined log format (same as production)
		logLine := fmt.Sprintf(
			`%s - - [%s] "%s %s HTTP/1.1" %d %d "%s" "%s" %.3f`,
			ip,
			time.Unix(timestamp, 0).Format("02/Jan/2006:15:04:05 -0700"),
			method,
			path,
			status,
			size,
			referer,
			userAgent,
			requestTime,
		)

		if _, err := fmt.Fprintln(file, logLine); err != nil {
			return err
		}
	}

	return nil
}
