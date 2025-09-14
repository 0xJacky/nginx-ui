package nginx_log

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/nginx_log/indexer"
	"github.com/0xJacky/Nginx-UI/internal/nginx_log/parser"
)

// BenchmarkRealisticProduction benchmarks the complete production pipeline
func BenchmarkRealisticProduction(b *testing.B) {
	// Test different scales
	scales := []struct {
		name    string
		records int
	}{
		{"Small_1K", 1000},
		{"Medium_5K", 5000},
	}

	for _, scale := range scales {
		b.Run(scale.name, func(b *testing.B) {
			benchmarkCompleteProduction(b, scale.records)
		})
	}
}

func benchmarkCompleteProduction(b *testing.B, recordCount int) {
	// Setup once
	tempDir, err := os.MkdirTemp("", "benchmark_production_")
	if err != nil {
		b.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Generate test data once
	testLogFile := filepath.Join(tempDir, "access.log")
	if err := generateBenchmarkLogFile(testLogFile, recordCount); err != nil {
		b.Fatalf("Failed to generate test data: %v", err)
	}

	// Setup production environment
	indexDir := filepath.Join(tempDir, "index")

	config := indexer.DefaultIndexerConfig()
	config.IndexPath = indexDir
	config.WorkerCount = 8
	config.BatchSize = 500
	config.EnableMetrics = false // Disable for cleaner benchmarking

	// Create production-like services
	geoService := &BenchGeoIPService{}
	userAgentParser := parser.NewSimpleUserAgentParser()

	optimizedParser := parser.NewOptimizedParser(
		&parser.Config{
			MaxLineLength: 4 * 1024,
			WorkerCount:   4,
			BatchSize:     200,
		},
		userAgentParser,
		geoService,
	)

	b.ResetTimer()
	b.ReportAllocs()

	// Run the benchmark
	for i := 0; i < b.N; i++ {
		// Create fresh index for each iteration
		iterIndexDir := filepath.Join(tempDir, fmt.Sprintf("index_%d", i))
		iterConfig := *config
		iterConfig.IndexPath = iterIndexDir

		result := runBenchmarkProduction(b, &iterConfig, optimizedParser, testLogFile)

		// Custom metrics
		throughput := float64(recordCount) / result.Duration.Seconds()
		b.ReportMetric(throughput, "records/sec")
		b.ReportMetric(float64(result.Processed), "records_processed")
		b.ReportMetric(float64(result.Indexed), "records_indexed")
		b.ReportMetric(result.SuccessRate*100, "success_rate_%")
	}
}

type BenchResult struct {
	Duration    time.Duration
	Processed   int
	Indexed     int
	SuccessRate float64
}

func runBenchmarkProduction(b *testing.B, config *indexer.Config, optimizedParser *parser.OptimizedParser, logFile string) *BenchResult {
	start := time.Now()

	// Create indexer
	if err := os.MkdirAll(config.IndexPath, 0755); err != nil {
		b.Fatalf("Failed to create index dir: %v", err)
	}

	shardManager := indexer.NewGroupedShardManager(config)
	indexerInstance := indexer.NewParallelIndexer(config, shardManager)

	ctx := context.Background()
	if err := indexerInstance.Start(ctx); err != nil {
		b.Fatalf("Failed to start indexer: %v", err)
	}
	defer indexerInstance.Stop()

	// Parse
	file, err := os.Open(logFile)
	if err != nil {
		b.Fatalf("Failed to open log file: %v", err)
	}
	defer file.Close()

	parseResult, err := optimizedParser.OptimizedParseStream(ctx, file)
	if err != nil {
		b.Fatalf("Parsing failed: %v", err)
	}

	// Index (limit to avoid timeout in benchmarking)
	maxToIndex := minVal(len(parseResult.Entries), 1000)
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
				UserAgent:   entry.UserAgent,
				FilePath:    logFile,
				MainLogPath: logFile,
				Raw:         entry.Raw,
			},
		}

		if err := indexerInstance.IndexDocument(ctx, doc); err == nil {
			indexed++
		}
	}

	// Flush
	indexerInstance.FlushAll()

	duration := time.Since(start)

	return &BenchResult{
		Duration:    duration,
		Processed:   parseResult.Processed,
		Indexed:     indexed,
		SuccessRate: float64(indexed) / float64(maxToIndex),
	}
}

func generateBenchmarkLogFile(filename string, recordCount int) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	baseTime := time.Now().Unix() - 3600

	for i := 0; i < recordCount; i++ {
		timestamp := baseTime + int64(i%3600)
		ip := fmt.Sprintf("10.0.%d.%d", (i/254)%256, i%254+1)
		path := []string{"/", "/api", "/health", "/metrics"}[i%4]
		status := []int{200, 200, 200, 404}[i%4]
		size := 1000 + i%2000

		logLine := fmt.Sprintf(
			`%s - - [%s] "GET %s HTTP/1.1" %d %d "-" "TestAgent/1.0" 0.%03d`,
			ip,
			time.Unix(timestamp, 0).Format("02/Jan/2006:15:04:05 -0700"),
			path,
			status,
			size,
			i%1000,
		)

		if _, err := fmt.Fprintln(file, logLine); err != nil {
			return err
		}
	}

	return nil
}

type BenchGeoIPService struct{}

func (s *BenchGeoIPService) Search(ip string) (*parser.GeoLocation, error) {
	return &parser.GeoLocation{
		CountryCode: "US",
		RegionCode:  "CA",
		Province:    "California",
		City:        "San Francisco",
	}, nil
}

func minVal(a, b int) int {
	if a < b {
		return a
	}
	return b
}
