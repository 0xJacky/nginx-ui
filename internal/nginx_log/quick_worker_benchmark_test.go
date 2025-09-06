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

// BenchmarkQuickWorkerComparison does a quick comparison of worker configurations
func BenchmarkQuickWorkerComparison(b *testing.B) {
	recordCount := 5000 // Smaller dataset for quick testing
	
	// Test configurations
	testConfigs := []struct {
		name        string
		workerMultiplier int
	}{
		{"1x_CPU", 1},
		{"2x_CPU_Old", 2},
		{"3x_CPU_New", 3},
		{"4x_CPU_HighThroughput", 4},
		{"6x_CPU_Max", 6},
	}

	// Generate test data once
	tempDir := b.TempDir()
	testLogFile := filepath.Join(tempDir, "access.log")
	if err := generateQuickTestData(testLogFile, recordCount); err != nil {
		b.Fatalf("Failed to generate test data: %v", err)
	}

	for _, tc := range testConfigs {
		b.Run(tc.name, func(b *testing.B) {
			workerCount := runtime.GOMAXPROCS(0) * tc.workerMultiplier
			
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				indexDir := filepath.Join(tempDir, fmt.Sprintf("index_%s_%d", tc.name, i))
				
				// Create config with specific worker count
				config := indexer.DefaultIndexerConfig()
				config.IndexPath = indexDir
				config.WorkerCount = workerCount
				config.EnableMetrics = false
				
				// Run test
				start := time.Now()
				result := runQuickWorkerTest(b, config, testLogFile)
				duration := time.Since(start)
				
				// Report metrics
				throughput := float64(result.Processed) / duration.Seconds()
				b.ReportMetric(throughput, "records/sec")
				b.ReportMetric(float64(workerCount), "workers")
				b.ReportMetric(float64(result.Processed), "processed")
				
				if i == 0 {
					b.Logf("Workers=%d, Processed=%d, Duration=%v, Throughput=%.0f rec/s", 
						workerCount, result.Processed, duration, throughput)
				}
			}
		})
	}
}

type QuickTestResult struct {
	Processed int
	Duration  time.Duration
}

func runQuickWorkerTest(b *testing.B, config *indexer.Config, logFile string) *QuickTestResult {
	ctx := context.Background()
	
	// Simple parser setup
	parserConfig := &parser.Config{
		MaxLineLength: 4 * 1024,
		WorkerCount:   config.WorkerCount / 3, // Parser uses 1/3 of indexer workers
		BatchSize:     500,
	}
	
	optimizedParser := parser.NewOptimizedParser(
		parserConfig,
		nil, // No UA parser for speed
		nil, // No Geo service for speed
	)
	
	// Parse only
	file, err := os.Open(logFile)
	if err != nil {
		b.Fatalf("Failed to open log file: %v", err)
	}
	defer file.Close()
	
	parseResult, err := optimizedParser.OptimizedParseStream(ctx, file)
	if err != nil {
		b.Fatalf("Parsing failed: %v", err)
	}
	
	return &QuickTestResult{
		Processed: parseResult.Processed,
		Duration:  parseResult.Duration,
	}
}

func generateQuickTestData(filename string, recordCount int) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()
	
	baseTime := time.Now().Unix()
	
	for i := 0; i < recordCount; i++ {
		logLine := fmt.Sprintf(
			`192.168.1.%d - - [%s] "GET /api/test%d HTTP/1.1" 200 %d "-" "Test/1.0" 0.001`,
			i%256,
			time.Unix(baseTime+int64(i), 0).Format("02/Jan/2006:15:04:05 -0700"),
			i%100,
			1000+i%1000,
		)
		fmt.Fprintln(file, logLine)
	}
	
	return nil
}