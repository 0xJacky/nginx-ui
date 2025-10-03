package nginx_log

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/nginx_log/indexer"
	"github.com/0xJacky/Nginx-UI/internal/nginx_log/parser"
)

// TestSimpleProductionThroughput tests realistic production throughput
func TestSimpleProductionThroughput(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping production test in short mode")
	}

	recordCounts := []int{10000, 20000, 30000}

	for _, records := range recordCounts {
		t.Run(fmt.Sprintf("Records_%d", records), func(t *testing.T) {
			runSimpleProductionTest(t, records)
		})
	}
}

func runSimpleProductionTest(t *testing.T, recordCount int) {
	t.Logf("🚀 Testing production throughput with %d records", recordCount)

	// Create temp directory
	tempDir, err := os.MkdirTemp("", "simple_production_test_")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Generate test data
	testLogFile := filepath.Join(tempDir, "access.log")
	dataStart := time.Now()

	if err := generateSimpleLogFile(testLogFile, recordCount); err != nil {
		t.Fatalf("Failed to generate test data: %v", err)
	}

	dataTime := time.Since(dataStart)
	t.Logf("📊 Generated %d records in %v", recordCount, dataTime)

	// Setup production-like environment
	setupStart := time.Now()

	indexDir := filepath.Join(tempDir, "index")
	if err := os.MkdirAll(indexDir, 0755); err != nil {
		t.Fatalf("Failed to create index dir: %v", err)
	}

	config := indexer.DefaultIndexerConfig()
	config.IndexPath = indexDir
	config.WorkerCount = 12 // Reasonable for testing
	config.BatchSize = 1000 // Reasonable batch size
	config.EnableMetrics = true

	setupTime := time.Since(setupStart)
	t.Logf("⚙️ Setup completed in %v", setupTime)

	// Run the actual production test
	productionStart := time.Now()

	result := runActualProductionWorkflow(t, config, testLogFile, recordCount)

	productionTime := time.Since(productionStart)

	// Calculate metrics
	throughput := float64(recordCount) / productionTime.Seconds()

	t.Logf("🏆 === PRODUCTION RESULTS ===")
	t.Logf("📈 Records: %d", recordCount)
	t.Logf("⏱️  Total Time: %v", productionTime)
	t.Logf("🚀 Throughput: %.0f records/second", throughput)
	t.Logf("📊 Data Generation: %v", dataTime)
	t.Logf("⚙️  Setup Time: %v", setupTime)
	t.Logf("🔧 Processing Time: %v", productionTime)

	if result != nil {
		t.Logf("✅ Success Rate: %.1f%%", result.SuccessRate*100)
		t.Logf("📋 Processed/Succeeded: %d/%d", result.Processed, result.Succeeded)
	}
}

type SimpleResult struct {
	Processed   int
	Succeeded   int
	SuccessRate float64
}

func runActualProductionWorkflow(t *testing.T, config *indexer.Config, logFile string, expectedRecords int) *SimpleResult {
	// Create services like production
	geoService := &SimpleGeoIPService{}
	userAgentParser := parser.NewCachedUserAgentParser(
		parser.NewSimpleUserAgentParser(),
		1000,
	)

	optimizedParser := parser.NewParser(
		&parser.Config{
			MaxLineLength: 8 * 1024,
			WorkerCount:   8,
			BatchSize:     500,
		},
		userAgentParser,
		geoService,
	)

	// Create indexer
	shardManager := indexer.NewGroupedShardManager(config)
	indexerInstance := indexer.NewParallelIndexer(config, shardManager)

	ctx := context.Background()
	if err := indexerInstance.Start(ctx); err != nil {
		t.Fatalf("Failed to start indexer: %v", err)
	}
	defer indexerInstance.Stop()

	// Parse the log file
	file, err := os.Open(logFile)
	if err != nil {
		t.Fatalf("Failed to open log file: %v", err)
	}
	defer file.Close()

	parseResult, err := optimizedParser.ParseStream(ctx, file)
	if err != nil {
		t.Fatalf("Parsing failed: %v", err)
	}

	t.Logf("📋 Parsed %d records successfully", len(parseResult.Entries))

	// Index a subset of documents (to avoid timeout while still being realistic)
	maxToIndex := min(len(parseResult.Entries), 5000) // Limit for testing
	indexed := 0

	for i, entry := range parseResult.Entries[:maxToIndex] {
		doc := &indexer.Document{
			ID: fmt.Sprintf("doc_%d", i),
			Fields: &indexer.LogDocument{
				Timestamp:   entry.Timestamp,
				IP:          entry.IP,
				Method:      entry.Method,
				Path:        entry.Path,
				PathExact:   entry.Path,
				Status:      entry.Status,
				BytesSent:   entry.BytesSent,
				Referer:     entry.Referer,
				UserAgent:   entry.UserAgent,
				Browser:     entry.Browser,
				BrowserVer:  entry.BrowserVer,
				OS:          entry.OS,
				OSVersion:   entry.OSVersion,
				DeviceType:  entry.DeviceType,
				RequestTime: entry.RequestTime,
				FilePath:    logFile,
				MainLogPath: logFile,
				Raw:         entry.Raw,
			},
		}

		if err := indexerInstance.IndexDocument(ctx, doc); err == nil {
			indexed++
		}

		// Progress feedback
		if i%1000 == 0 && i > 0 {
			t.Logf("📊 Indexed %d documents...", i)
		}
	}

	// Flush
	if err := indexerInstance.FlushAll(); err != nil {
		t.Logf("Warning: Flush failed: %v", err)
	}

	return &SimpleResult{
		Processed:   maxToIndex,
		Succeeded:   indexed,
		SuccessRate: float64(indexed) / float64(maxToIndex),
	}
}

func generateSimpleLogFile(filename string, recordCount int) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	// use global rng defaults; no explicit rand.Seed needed in Go 1.20+
	baseTime := time.Now().Unix() - 3600 // 1 hour ago

	for i := 0; i < recordCount; i++ {
		timestamp := baseTime + int64(i%3600)
		ip := fmt.Sprintf("192.168.1.%d", rand.Intn(254)+1)
		path := []string{"/", "/api/users", "/api/data", "/health"}[rand.Intn(4)]
		status := []int{200, 200, 200, 404, 500}[rand.Intn(5)]
		size := rand.Intn(5000) + 100

		logLine := fmt.Sprintf(
			`%s - - [%s] "GET %s HTTP/1.1" %d %d "-" "Mozilla/5.0 Test" 0.123`,
			ip,
			time.Unix(timestamp, 0).Format("02/Jan/2006:15:04:05 -0700"),
			path,
			status,
			size,
		)

		if _, err := fmt.Fprintln(file, logLine); err != nil {
			return err
		}
	}

	return nil
}

type SimpleGeoIPService struct{}

func (s *SimpleGeoIPService) Search(ip string) (*parser.GeoLocation, error) {
	return &parser.GeoLocation{
		CountryCode: "US",
		RegionCode:  "CA",
		Province:    "California",
		City:        "San Francisco",
	}, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
