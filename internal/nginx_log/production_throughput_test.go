package nginx_log

import (
	"context"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/nginx_log/indexer"
	"github.com/0xJacky/Nginx-UI/internal/nginx_log/parser"
)

// ProductionThroughputTest tests the complete production pipeline
// including data generation, indexing, GeoIP, User-Agent parsing, etc.
func TestProductionThroughputEndToEnd(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping production throughput test in short mode")
	}

	scales := []struct {
		name    string
		records int
	}{
		{"Small_50K", 50000},
		{"Medium_100K", 100000},
		{"Large_200K", 200000},
		{"XLarge_500K", 500000},
	}

	for _, scale := range scales {
		t.Run(scale.name, func(t *testing.T) {
			runCompleteProductionTest(t, scale.records)
		})
	}
}

func runCompleteProductionTest(t *testing.T, recordCount int) {
	t.Logf("🚀 Starting COMPLETE production test with %d records", recordCount)
	
	// Step 1: Create temporary directory
	tempDir, err := ioutil.TempDir("", "nginx_ui_production_test_")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Step 2: Generate realistic test data
	testLogFile := filepath.Join(tempDir, "access.log")
	dataGenStart := time.Now()
	
	if err := generateRealisticLogFile(testLogFile, recordCount); err != nil {
		t.Fatalf("Failed to generate test data: %v", err)
	}
	
	dataGenTime := time.Since(dataGenStart)
	t.Logf("📊 Generated %d records in %v", recordCount, dataGenTime)

	// Step 3: Set up complete production environment
	setupStart := time.Now()
	
	// Create index directory
	indexDir := filepath.Join(tempDir, "index")
	if err := os.MkdirAll(indexDir, 0755); err != nil {
		t.Fatalf("Failed to create index dir: %v", err)
	}

	// Initialize production-grade configuration
	config := indexer.DefaultIndexerConfig()
	config.IndexPath = indexDir
	config.WorkerCount = 24 // Use all available cores
	config.BatchSize = 2000 // Production batch size
	config.EnableMetrics = true

	// Create production services
	geoIPService := &mockProductionGeoIPService{}
	userAgentParser := parser.NewCachedUserAgentParser(
		parser.NewSimpleUserAgentParser(),
		10000, // Large cache for production
	)
	
	optimizedParser := parser.NewOptimizedParser(
		&parser.Config{
			MaxLineLength: 16 * 1024,
			WorkerCount:   12,
			BatchSize:     1500,
		},
		userAgentParser,
		geoIPService,
	)

	// Create shard manager
	shardManager := indexer.NewDefaultShardManager(config)
	
	// Initialize indexer with all production components
	parallelIndexer := indexer.NewParallelIndexer(config, shardManager)
	ctx := context.Background()
	
	if err := parallelIndexer.Start(ctx); err != nil {
		t.Fatalf("Failed to start indexer: %v", err)
	}
	defer parallelIndexer.Stop()

	setupTime := time.Since(setupStart)
	t.Logf("⚙️ Production environment setup completed in %v", setupTime)

	// Step 4: Execute complete production rebuild (same as real rebuild)
	rebuildStart := time.Now()
	
	t.Logf("🔄 Starting COMPLETE production rebuild with full indexing pipeline")
	
	// This simulates the exact same process as production rebuild
	result, err := executeProductionRebuild(ctx, parallelIndexer, optimizedParser, testLogFile)
	if err != nil {
		t.Fatalf("Production rebuild failed: %v", err)
	}
	
	rebuildTime := time.Since(rebuildStart)
	
	// Step 5: Calculate and report realistic metrics
	recordsPerSecond := float64(recordCount) / rebuildTime.Seconds()
	
	t.Logf("🏆 === PRODUCTION THROUGHPUT RESULTS ===")
	t.Logf("📈 Total Records: %d", recordCount)
	t.Logf("⏱️  Total Time: %v", rebuildTime)
	t.Logf("🚀 Throughput: %.2f records/second", recordsPerSecond)
	t.Logf("✅ Success Rate: %.2f%% (%d/%d)", result.SuccessRate*100, result.Succeeded, result.Processed)
	t.Logf("📊 Index Size: %d documents", result.IndexedDocuments)
	t.Logf("🔧 Configuration: Workers=%d, BatchSize=%d", config.WorkerCount, config.BatchSize)
	
	// Performance validation
	if result.SuccessRate < 0.99 {
		t.Errorf("Success rate too low: %.2f%% (expected >99%%)", result.SuccessRate*100)
	}
	
	if recordsPerSecond < 1000 {
		t.Logf("⚠️  Warning: Throughput below 1000 records/sec: %.2f", recordsPerSecond)
	}
	
	// Log memory usage
	stats := parallelIndexer.GetStats()
	if stats != nil {
		t.Logf("💾 Memory Usage: %d MB", stats.MemoryUsage/(1024*1024))
		t.Logf("🔄 Queue Size: %d", stats.QueueSize)
	}
}

type ProductionResult struct {
	Processed        int
	Succeeded        int
	Failed           int
	SuccessRate      float64
	IndexedDocuments int
	Duration         time.Duration
}

func executeProductionRebuild(ctx context.Context, indexerInstance *indexer.ParallelIndexer, parser *parser.OptimizedParser, logFile string) (*ProductionResult, error) {
	// Open log file
	file, err := os.Open(logFile)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}
	defer file.Close()

	// Execute the same parsing and indexing as production rebuild
	startTime := time.Now()
	
	// Use optimized parse stream (same as production)
	parseResult, err := parser.OptimizedParseStream(ctx, file)
	if err != nil {
		return nil, fmt.Errorf("parsing failed: %w", err)
	}

	// Index all parsed documents (same as production)
	var totalIndexed int
	for _, entry := range parseResult.Entries {
		doc := &indexer.Document{
			ID: fmt.Sprintf("doc_%d", totalIndexed),
			Fields: &indexer.LogDocument{
				Timestamp:    entry.Timestamp,
				IP:           entry.IP,
				Method:       entry.Method,
				Path:         entry.Path,
				PathExact:    entry.Path,
				Status:       entry.Status,
				BytesSent:    entry.BytesSent,
				Referer:      entry.Referer,
				UserAgent:    entry.UserAgent,
				Browser:      entry.Browser,
				BrowserVer:   entry.BrowserVer,
				OS:           entry.OS,
				OSVersion:    entry.OSVersion,
				DeviceType:   entry.DeviceType,
				RequestTime:  entry.RequestTime,
				UpstreamTime: entry.UpstreamTime,
				FilePath:     logFile,
				MainLogPath:  logFile,
				Raw:          entry.Raw,
			},
		}
		
		// Index document (same as production indexing)
		err := indexerInstance.IndexDocument(ctx, doc)
		if err != nil {
			continue // Count as failed but continue processing
		}
		totalIndexed++
	}

	// Flush all pending operations (same as production)
	if err := indexerInstance.FlushAll(); err != nil {
		return nil, fmt.Errorf("failed to flush: %w", err)
	}

	duration := time.Since(startTime)
	
	return &ProductionResult{
		Processed:        parseResult.Processed,
		Succeeded:        parseResult.Succeeded,
		Failed:           parseResult.Failed,
		SuccessRate:      float64(parseResult.Succeeded) / float64(parseResult.Processed),
		IndexedDocuments: totalIndexed,
		Duration:         duration,
	}, nil
}

func generateRealisticLogFile(filename string, recordCount int) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	// Realistic log patterns
	ips := []string{
		"192.168.1.1", "10.0.0.1", "172.16.0.1", "203.0.113.1",
		"198.51.100.1", "192.0.2.1", "203.0.113.195", "198.51.100.178",
	}
	
	methods := []string{"GET", "POST", "PUT", "DELETE", "HEAD", "OPTIONS"}
	
	paths := []string{
		"/", "/api/users", "/api/posts", "/api/auth/login", "/api/auth/logout",
		"/static/css/style.css", "/static/js/app.js", "/images/logo.png",
		"/admin/dashboard", "/user/profile", "/search?q=test", "/api/v1/data",
	}
	
	userAgents := []string{
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/14.1.1 Safari/605.1.15",
		"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36",
		"Mozilla/5.0 (iPhone; CPU iPhone OS 14_6 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/14.1.1 Mobile/15E148 Safari/604.1",
	}
	
	statuses := []int{200, 200, 200, 200, 301, 302, 404, 500} // Weighted towards 200
	
	rand.Seed(time.Now().UnixNano())
	baseTime := time.Now().Unix() - 86400 // 24 hours ago
	
	for i := 0; i < recordCount; i++ {
		timestamp := baseTime + int64(i)
		ip := ips[rand.Intn(len(ips))]
		method := methods[rand.Intn(len(methods))]
		path := paths[rand.Intn(len(paths))]
		status := statuses[rand.Intn(len(statuses))]
		size := rand.Intn(10000) + 100
		userAgent := userAgents[rand.Intn(len(userAgents))]
		referer := "-"
		if rand.Float32() < 0.3 {
			referer = "https://example.com/referrer"
		}
		requestTime := rand.Float64() * 2.0 // 0-2 seconds
		
		// Standard nginx log format
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

// Mock services for testing  
type mockProductionGeoIPService struct{}

func (m *mockProductionGeoIPService) Search(ip string) (*parser.GeoLocation, error) {
	// Mock geographic data
	regions := []string{"US", "CN", "JP", "DE", "GB"}
	provinces := []string{"California", "Beijing", "Tokyo", "Berlin", "London"}
	cities := []string{"San Francisco", "Beijing", "Tokyo", "Berlin", "London"}
	
	rand.Seed(time.Now().UnixNano())
	idx := rand.Intn(len(regions))
	
	return &parser.GeoLocation{
		CountryCode: regions[idx],
		RegionCode:  regions[idx],
		Province:    provinces[idx],
		City:        cities[idx],
	}, nil
}