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

// BenchmarkConservativeWorkerScaling tests lower worker multipliers to show clearer performance differences
func BenchmarkConservativeWorkerScaling(b *testing.B) {
	recordCount := 10000 // Moderate dataset for clear differences
	
	// Test lower multipliers to show more obvious improvements
	testConfigs := []struct {
		name        string
		multiplier  float64
	}{
		{"0.5x_CPU_Conservative", 0.5},
		{"1x_CPU_Old_Default", 1.0},
		{"1.5x_CPU_Moderate", 1.5},
		{"2x_CPU_Previous_Default", 2.0},
		{"3x_CPU_New_Default", 3.0},
		{"4x_CPU_High_Throughput", 4.0},
	}

	// Generate test data once
	tempDir := b.TempDir()
	testLogFile := filepath.Join(tempDir, "access.log")
	if err := generateConservativeTestData(testLogFile, recordCount); err != nil {
		b.Fatalf("Failed to generate test data: %v", err)
	}

	for _, tc := range testConfigs {
		b.Run(tc.name, func(b *testing.B) {
			cpuCount := runtime.GOMAXPROCS(0)
			workerCount := int(float64(cpuCount) * tc.multiplier)
			if workerCount < 1 {
				workerCount = 1
			}
			
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
				result := runConservativeWorkerTest(b, config, testLogFile)
				duration := time.Since(start)
				
				// Report metrics
				throughput := float64(result.Processed) / duration.Seconds()
				b.ReportMetric(throughput, "records/sec")
				b.ReportMetric(float64(workerCount), "workers")
				b.ReportMetric(float64(result.Processed), "processed")
				
				if i == 0 {
					b.Logf("Multiplier=%.1fx, Workers=%d, Processed=%d, Duration=%v, Throughput=%.0f rec/s", 
						tc.multiplier, workerCount, result.Processed, duration, throughput)
				}
			}
		})
	}
}

type ConservativeTestResult struct {
	Processed int
	Duration  time.Duration
}

func runConservativeWorkerTest(b *testing.B, config *indexer.Config, logFile string) *ConservativeTestResult {
	ctx := context.Background()
	
	// Parser setup with proportional worker count
	parserConfig := &parser.Config{
		MaxLineLength: 4 * 1024,
		WorkerCount:   max(1, config.WorkerCount / 3), // Parser uses 1/3 of indexer workers
		BatchSize:     500,
	}
	
	optimizedParser := parser.NewOptimizedParser(
		parserConfig,
		nil, // No UA parser for speed
		nil, // No Geo service for speed
	)
	
	// Parse only for consistent measurement
	file, err := os.Open(logFile)
	if err != nil {
		b.Fatalf("Failed to open log file: %v", err)
	}
	defer file.Close()
	
	parseResult, err := optimizedParser.OptimizedParseStream(ctx, file)
	if err != nil {
		b.Fatalf("Parsing failed: %v", err)
	}
	
	return &ConservativeTestResult{
		Processed: parseResult.Processed,
		Duration:  parseResult.Duration,
	}
}

func generateConservativeTestData(filename string, recordCount int) error {
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

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}