package nginx_log

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/nginx_log/indexer"
	"github.com/0xJacky/Nginx-UI/internal/nginx_log/parser"
)

// BenchmarkWorkerScaling tests the actual production configuration performance
func BenchmarkWorkerScaling(b *testing.B) {
	// Different worker configurations to test
	testConfigs := []struct {
		name           string
		configModifier func(*indexer.Config)
	}{
		{
			name: "Default_Config",
			configModifier: func(c *indexer.Config) {
				// Use actual default configuration - no modifications
			},
		},
		{
			name: "Old_Conservative_2x",
			configModifier: func(c *indexer.Config) {
				c.WorkerCount = runtime.GOMAXPROCS(0) * 2 // Old default
			},
		},
		{
			name: "New_Default_3x",
			configModifier: func(c *indexer.Config) {
				c.WorkerCount = runtime.GOMAXPROCS(0) * 3 // New default
			},
		},
		{
			name: "High_Throughput_4x",
			configModifier: func(c *indexer.Config) {
				c.WorkerCount = runtime.GOMAXPROCS(0) * 4 // High throughput mode
			},
		},
		{
			name: "Aggressive_6x",
			configModifier: func(c *indexer.Config) {
				c.WorkerCount = runtime.GOMAXPROCS(0) * 6 // Maximum adaptive scaling
			},
		},
	}

	recordCounts := []int{10000, 50000, 100000}

	for _, recordCount := range recordCounts {
		for _, tc := range testConfigs {
			benchName := fmt.Sprintf("Records_%d/%s", recordCount, tc.name)
			b.Run(benchName, func(b *testing.B) {
				benchmarkWorkerConfig(b, recordCount, tc.configModifier)
			})
		}
	}
}

func benchmarkWorkerConfig(b *testing.B, recordCount int, configModifier func(*indexer.Config)) {
	// Create temp directory
	tempDir := b.TempDir()
	
	// Generate test data once
	testLogFile := filepath.Join(tempDir, "access.log")
	if err := generateBenchmarkLogData(testLogFile, recordCount); err != nil {
		b.Fatalf("Failed to generate test data: %v", err)
	}

	// Get file size for metrics
	fileInfo, err := os.Stat(testLogFile)
	if err != nil {
		b.Fatalf("Failed to stat test file: %v", err)
	}
	fileSizeMB := float64(fileInfo.Size()) / (1024 * 1024)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// Create index directory for this iteration
		indexDir := filepath.Join(tempDir, fmt.Sprintf("index_%d", i))
		if err := os.MkdirAll(indexDir, 0755); err != nil {
			b.Fatalf("Failed to create index dir: %v", err)
		}

		// Use DEFAULT configuration and apply modifier
		config := indexer.DefaultIndexerConfig()
		config.IndexPath = indexDir
		config.EnableMetrics = false // Disable for cleaner benchmarking
		
		// Apply configuration modifier
		if configModifier != nil {
			configModifier(config)
		}

		// Run the actual benchmark
		result := runWorkerBenchmark(b, config, testLogFile, recordCount)
		
		// Report custom metrics
		throughput := float64(recordCount) / result.Duration.Seconds()
		mbPerSec := fileSizeMB / result.Duration.Seconds()
		
		b.ReportMetric(throughput, "records/sec")
		b.ReportMetric(mbPerSec, "MB/sec")
		b.ReportMetric(float64(config.WorkerCount), "workers")
		b.ReportMetric(float64(result.Parsed), "parsed")
		b.ReportMetric(float64(result.Indexed), "indexed")
		
		// Log configuration for verification
		if i == 0 {
			b.Logf("Config: Workers=%d, BatchSize=%d, Shards=%d", 
				config.WorkerCount, config.BatchSize, config.ShardCount)
		}
	}
}

type WorkerBenchResult struct {
	Duration time.Duration
	Parsed   int
	Indexed  int
}

func runWorkerBenchmark(b *testing.B, config *indexer.Config, logFile string, expectedRecords int) *WorkerBenchResult {
	start := time.Now()
	
	// Create production components
	shardManager := indexer.NewDefaultShardManager(config)
	parallelIndexer := indexer.NewParallelIndexer(config, shardManager)
	
	ctx := context.Background()
	if err := parallelIndexer.Start(ctx); err != nil {
		b.Fatalf("Failed to start indexer: %v", err)
	}
	defer parallelIndexer.Stop()

	// Create parser with production configuration
	parserConfig := &parser.Config{
		MaxLineLength: 8 * 1024,
		WorkerCount:   config.WorkerCount / 2, // Parser uses half of indexer workers
		BatchSize:     1000,
	}
	
	optimizedParser := parser.NewOptimizedParser(
		parserConfig,
		parser.NewSimpleUserAgentParser(),
		&MockGeoService{},
	)

	// Parse the log file
	file, err := os.Open(logFile)
	if err != nil {
		b.Fatalf("Failed to open log file: %v", err)
	}
	defer file.Close()

	parseResult, err := optimizedParser.OptimizedParseStream(ctx, file)
	if err != nil {
		b.Fatalf("Parsing failed: %v", err)
	}

	// Index documents (limit to avoid timeout)
	maxToIndex := minInt(len(parseResult.Entries), 5000)
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
		
		if err := parallelIndexer.IndexDocument(ctx, doc); err == nil {
			indexed++
		}
	}

	// Flush
	parallelIndexer.FlushAll()
	
	duration := time.Since(start)
	
	return &WorkerBenchResult{
		Duration: duration,
		Parsed:   parseResult.Processed,
		Indexed:  indexed,
	}
}

func generateBenchmarkLogData(filename string, recordCount int) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	baseTime := time.Now().Unix() - 86400
	
	for i := 0; i < recordCount; i++ {
		timestamp := baseTime + int64(i%86400)
		ip := fmt.Sprintf("192.168.%d.%d", (i/256)%256, i%256)
		path := []string{"/", "/api/users", "/api/posts", "/health"}[i%4]
		status := []int{200, 200, 200, 404}[i%4]
		size := 1000 + i%5000
		
		logLine := fmt.Sprintf(
			`%s - - [%s] "GET %s HTTP/1.1" %d %d "-" "Mozilla/5.0" 0.%03d`,
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

type MockGeoService struct{}

func (m *MockGeoService) Search(ip string) (*parser.GeoLocation, error) {
	return &parser.GeoLocation{
		CountryCode: "US",
		RegionCode:  "CA",
		Province:    "California",
		City:        "San Francisco",
	}, nil
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}