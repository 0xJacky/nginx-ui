package nginx_log

import (
	"bufio"
	"context"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/dgraph-io/ristretto/v2"
)

// Test data generators for realistic nginx log simulation
var (
	ips = []string{
		"192.168.1.1", "10.0.0.1", "172.16.0.1", "203.0.113.1", "198.51.100.1",
		"192.168.2.100", "10.10.10.10", "172.31.255.255", "8.8.8.8", "1.1.1.1",
	}

	userAgents = []string{
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		"Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Mobile/15E148 Safari/604.1",
		"Mozilla/5.0 (Linux; Android 13; SM-G991B) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Mobile Safari/537.36",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:120.0) Gecko/20100101 Firefox/120.0",
	}

	methods = []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"}

	paths = []string{
		"/api/v1/users", "/api/v1/orders", "/api/v1/products", "/api/v1/auth/login",
		"/static/js/app.js", "/static/css/main.css", "/images/logo.png",
		"/admin/dashboard", "/admin/users", "/admin/settings",
		"/health", "/metrics", "/favicon.ico", "/robots.txt",
	}

	statuses = []int{200, 201, 400, 401, 403, 404, 500, 502, 503}
	
	referers = []string{
		"https://example.com", "https://google.com", "https://github.com", 
		"-", "https://stackoverflow.com", "https://reddit.com",
	}
)

func generateRandomLogLine(timestamp time.Time) string {
	ip := ips[rand.Intn(len(ips))]
	method := methods[rand.Intn(len(methods))]
	path := paths[rand.Intn(len(paths))]
	if rand.Float32() < 0.3 {
		path += fmt.Sprintf("/%d", rand.Intn(10000))
	}
	status := statuses[rand.Intn(len(statuses))]
	size := rand.Intn(50000) + 100
	referer := referers[rand.Intn(len(referers))]
	userAgent := userAgents[rand.Intn(len(userAgents))]
	
	timeStr := timestamp.Format("02/Jan/2006:15:04:05 -0700")
	
	return fmt.Sprintf(`%s - - [%s] "%s %s HTTP/1.1" %d %d "%s" "%s"`,
		ip, timeStr, method, path, status, size, referer, userAgent)
}

func generateLogFile(filePath string, count int) error {
	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	defer writer.Flush()

	baseTime := time.Now().Add(-24 * time.Hour)
	
	for i := 0; i < count; i++ {
		timestamp := baseTime.Add(time.Duration(i) * time.Second / time.Duration(count))
		line := generateRandomLogLine(timestamp)
		
		if _, err := writer.WriteString(line + "\n"); err != nil {
			return err
		}
		
		if i%100000 == 0 {
			writer.Flush()
		}
	}
	
	return nil
}

func BenchmarkLogGeneration_1M(b *testing.B) {
	tempDir, err := os.MkdirTemp("", "nginx_log_bench")
	if err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		logFile := filepath.Join(tempDir, fmt.Sprintf("access_%d.log", i))
		err := generateLogFile(logFile, 1000000)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkLogParsing_OptimizedBatch(b *testing.B) {
	tempDir, err := os.MkdirTemp("", "nginx_log_bench")
	if err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	logFile := filepath.Join(tempDir, "access.log")
	err = generateLogFile(logFile, 1000000)
	if err != nil {
		b.Fatal(err)
	}

	parser := NewOptimizedLogParser(NewSimpleUserAgentParser())
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		file, err := os.Open(logFile)
		if err != nil {
			b.Fatal(err)
		}
		
		scanner := bufio.NewScanner(file)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		
		count := 0
		for scanner.Scan() {
			line := scanner.Text()
			if strings.TrimSpace(line) == "" {
				continue
			}
			
			_, err := parser.ParseLine(line)
			if err != nil {
				continue
			}
			count++
		}
		
		file.Close()
	}
}

func BenchmarkIndexing_LargeDataset(b *testing.B) {
	tempDir, err := os.MkdirTemp("", "nginx_log_bench")
	if err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	logFile := filepath.Join(tempDir, "access.log")
	err = generateLogFile(logFile, 1000000)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		
		indexPath := filepath.Join(tempDir, fmt.Sprintf("index_%d", i))
		index, err := createOrOpenIndex(indexPath)
		if err != nil {
			b.Fatal(err)
		}

		cache, err := ristretto.NewCache(&ristretto.Config[string, *CachedSearchResult]{
			NumCounters: 1e7,
			MaxCost:     1 << 30,
			BufferItems: 64,
		})
		if err != nil {
			b.Fatal(err)
		}

		indexer := &LogIndexer{
			index:      index,
			indexPath:  indexPath,
			parser:     NewOptimizedLogParser(NewSimpleUserAgentParser()),
			logPaths:   make(map[string]*LogFileInfo),
			indexBatch: 50000,
			cache:      cache,
		}
		
		err = indexer.AddLogPath(logFile)
		if err != nil {
			b.Fatal(err)
		}
		
		b.StartTimer()
		
		err = indexer.IndexLogFile(logFile)
		if err != nil {
			b.Fatal(err)
		}
		
		b.StopTimer()
		indexer.Close()
	}
}

func BenchmarkSearch_ComplexQueries(b *testing.B) {
	tempDir, err := os.MkdirTemp("", "nginx_log_bench")
	if err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	logFile := filepath.Join(tempDir, "access.log")
	err = generateLogFile(logFile, 500000)
	if err != nil {
		b.Fatal(err)
	}

	indexPath := filepath.Join(tempDir, "index")
	index, err := createOrOpenIndex(indexPath)
	if err != nil {
		b.Fatal(err)
	}

	cache, err := ristretto.NewCache(&ristretto.Config[string, *CachedSearchResult]{
		NumCounters: 1e7,
		MaxCost:     1 << 29,
		BufferItems: 64,
	})
	if err != nil {
		b.Fatal(err)
	}

	indexer := &LogIndexer{
		index:      index,
		indexPath:  indexPath,
		parser:     NewOptimizedLogParser(NewSimpleUserAgentParser()),
		logPaths:   make(map[string]*LogFileInfo),
		indexBatch: 25000,
		cache:      cache,
	}
	defer indexer.Close()

	err = indexer.AddLogPath(logFile)
	if err != nil {
		b.Fatal(err)
	}

	err = indexer.IndexLogFile(logFile)
	if err != nil {
		b.Fatal(err)
	}

	time.Sleep(2 * time.Second)

	queries := []*QueryRequest{
		{Method: "GET", Limit: 1000},
		{Status: []int{200, 201}, Limit: 1000},
		{IP: "192.168.1.1", Limit: 1000},
		{Path: "/api/v1/users", Limit: 1000},
		{Method: "POST", Status: []int{400, 401, 403}, Limit: 1000},
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		query := queries[i%len(queries)]
		_, err := indexer.SearchLogs(context.Background(), query)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkAnalytics_IndexStatus(b *testing.B) {
	tempDir, err := os.MkdirTemp("", "nginx_log_bench")
	if err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	logFile := filepath.Join(tempDir, "access.log")
	err = generateLogFile(logFile, 500000)
	if err != nil {
		b.Fatal(err)
	}

	indexPath := filepath.Join(tempDir, "index")
	index, err := createOrOpenIndex(indexPath)
	if err != nil {
		b.Fatal(err)
	}

	cache, err := ristretto.NewCache(&ristretto.Config[string, *CachedSearchResult]{
		NumCounters: 1e7,
		MaxCost:     1 << 29,
		BufferItems: 64,
	})
	if err != nil {
		b.Fatal(err)
	}

	indexer := &LogIndexer{
		index:      index,
		indexPath:  indexPath,
		parser:     NewOptimizedLogParser(NewSimpleUserAgentParser()),
		logPaths:   make(map[string]*LogFileInfo),
		indexBatch: 25000,
		cache:      cache,
	}
	defer indexer.Close()

	err = indexer.AddLogPath(logFile)
	if err != nil {
		b.Fatal(err)
	}

	err = indexer.IndexLogFile(logFile)
	if err != nil {
		b.Fatal(err)
	}

	time.Sleep(2 * time.Second)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := indexer.GetIndexStatus()
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkMemoryEfficiency_LargeDataset(b *testing.B) {
	tempDir, err := os.MkdirTemp("", "nginx_log_bench")
	if err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	logFile := filepath.Join(tempDir, "access.log")
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		
		var m1, m2 runtime.MemStats
		runtime.GC()
		runtime.ReadMemStats(&m1)
		
		b.StartTimer()
		
		err := generateLogFile(logFile, 1000000)
		if err != nil {
			b.Fatal(err)
		}

		parser := NewOptimizedLogParser(NewSimpleUserAgentParser())
		file, err := os.Open(logFile)
		if err != nil {
			b.Fatal(err)
		}

		scanner := bufio.NewScanner(file)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		
		count := 0
		for scanner.Scan() {
			line := scanner.Text()
			if strings.TrimSpace(line) == "" {
				continue
			}
			
			_, err := parser.ParseLine(line)
			if err != nil {
				continue
			}
			count++
			
			if count%100000 == 0 {
				runtime.GC()
			}
		}
		
		file.Close()
		
		b.StopTimer()
		
		runtime.GC()
		runtime.ReadMemStats(&m2)
		
		b.ReportMetric(float64(m2.Alloc-m1.Alloc)/1024/1024, "MB/processed")
		b.ReportMetric(float64(count), "lines/processed")
		
		os.Remove(logFile)
	}
}

func BenchmarkConcurrentParsing_MultiCore(b *testing.B) {
	tempDir, err := os.MkdirTemp("", "nginx_log_bench")
	if err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	numWorkers := runtime.NumCPU()
	linesPerFile := 200000
	
	logFiles := make([]string, numWorkers)
	for i := 0; i < numWorkers; i++ {
		logFile := filepath.Join(tempDir, fmt.Sprintf("access_%d.log", i))
		err := generateLogFile(logFile, linesPerFile)
		if err != nil {
			b.Fatal(err)
		}
		logFiles[i] = logFile
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		done := make(chan int, numWorkers)
		
		for j := 0; j < numWorkers; j++ {
			go func(fileIndex int) {
				parser := NewOptimizedLogParser(NewSimpleUserAgentParser())
				file, err := os.Open(logFiles[fileIndex])
				if err != nil {
					done <- 0
					return
				}
				defer file.Close()

				scanner := bufio.NewScanner(file)
				scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
				
				count := 0
				for scanner.Scan() {
					line := scanner.Text()
					if strings.TrimSpace(line) == "" {
						continue
					}
					
					_, err := parser.ParseLine(line)
					if err != nil {
						continue
					}
					count++
				}
				
				done <- count
			}(j)
		}
		
		totalProcessed := 0
		for j := 0; j < numWorkers; j++ {
			totalProcessed += <-done
		}
		
		b.ReportMetric(float64(totalProcessed), "total_lines_processed")
	}
}

func BenchmarkOptimizedParser_vs_Standard(b *testing.B) {
	logLine := `192.168.1.1 - - [25/Dec/2023:10:00:00 +0000] "GET /api/v1/users/123 HTTP/1.1" 200 1024 "https://example.com" "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"`

	b.Run("StandardParser", func(b *testing.B) {
		parser := NewOptimizedLogParser(NewSimpleUserAgentParser())
		b.ResetTimer()
		b.ReportAllocs()
		
		for i := 0; i < b.N; i++ {
			_, _ = parser.ParseLine(logLine)
		}
	})

	b.Run("OptimizedParser", func(b *testing.B) {
		parser := NewOptimizedLogParser(NewSimpleUserAgentParser())
		b.ResetTimer()
		b.ReportAllocs()
		
		for i := 0; i < b.N; i++ {
			_, _ = parser.ParseLine(logLine)
		}
	})
}

func BenchmarkBatchIndexing_OptimizedSizes(b *testing.B) {
	tempDir, err := os.MkdirTemp("", "nginx_log_batch_bench")
	if err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	logFile := filepath.Join(tempDir, "access.log")
	err = generateLogFile(logFile, 1000000)
	if err != nil {
		b.Fatal(err)
	}

	batchSizes := []int{1000, 5000, 10000, 25000, 50000, 100000}

	for _, batchSize := range batchSizes {
		b.Run(fmt.Sprintf("BatchSize_%d", batchSize), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				b.StopTimer()
				
				indexPath := filepath.Join(tempDir, fmt.Sprintf("index_batch_%d_%d", batchSize, i))
				index, err := createOrOpenIndex(indexPath)
				if err != nil {
					b.Fatal(err)
				}

				cache, err := ristretto.NewCache(&ristretto.Config[string, *CachedSearchResult]{
					NumCounters: 1e7,
					MaxCost:     1 << 28,
					BufferItems: 64,
				})
				if err != nil {
					b.Fatal(err)
				}

				indexer := &LogIndexer{
					index:      index,
					indexPath:  indexPath,
					parser:     NewOptimizedLogParser(NewSimpleUserAgentParser()),
					logPaths:   make(map[string]*LogFileInfo),
					indexBatch: batchSize,
					cache:      cache,
				}
				
				err = indexer.AddLogPath(logFile)
				if err != nil {
					b.Fatal(err)
				}
				
				b.StartTimer()
				
				err = indexer.IndexLogFile(logFile)
				if err != nil {
					b.Fatal(err)
				}
				
				b.StopTimer()
				indexer.Close()
				os.RemoveAll(indexPath)
			}
		})
	}
}

func BenchmarkStreamingProcessor_HighThroughput(b *testing.B) {
	tempDir, err := os.MkdirTemp("", "nginx_log_streaming_bench")
	if err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	logFile := filepath.Join(tempDir, "access.log")
	err = generateLogFile(logFile, 1000000)
	if err != nil {
		b.Fatal(err)
	}

	workerCounts := []int{1, 2, 4, 8, 16}
	
	for _, workers := range workerCounts {
		b.Run(fmt.Sprintf("Workers_%d", workers), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				b.StopTimer()
				
				indexPath := filepath.Join(tempDir, fmt.Sprintf("index_stream_%d_%d", workers, i))
				index, err := createOrOpenIndex(indexPath)
				if err != nil {
					b.Fatal(err)
				}

				cache, err := ristretto.NewCache(&ristretto.Config[string, *CachedSearchResult]{
					NumCounters: 1e7,
					MaxCost:     1 << 28,
					BufferItems: 64,
				})
				if err != nil {
					b.Fatal(err)
				}

				indexer := &LogIndexer{
					index:      index,
					indexPath:  indexPath,
					parser:     NewOptimizedLogParser(NewSimpleUserAgentParser()),
					logPaths:   make(map[string]*LogFileInfo),
					indexBatch: 25000,
					cache:      cache,
				}
				
				processor := NewStreamingLogProcessor(indexer, 10000, workers)
				
				file, err := os.Open(logFile)
				if err != nil {
					b.Fatal(err)
				}
				
				b.StartTimer()
				
				err = processor.ProcessFile(file)
				if err != nil {
					b.Fatal(err)
				}
				
				b.StopTimer()
				
				file.Close()
				indexer.Close()
				os.RemoveAll(indexPath)
			}
		})
	}
}

func BenchmarkSearchPerformance_LargeResults(b *testing.B) {
	tempDir, err := os.MkdirTemp("", "nginx_log_search_bench")
	if err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	logFile := filepath.Join(tempDir, "access.log")
	err = generateLogFile(logFile, 2000000)
	if err != nil {
		b.Fatal(err)
	}

	indexPath := filepath.Join(tempDir, "index")
	index, err := createOrOpenIndex(indexPath)
	if err != nil {
		b.Fatal(err)
	}

	cache, err := ristretto.NewCache(&ristretto.Config[string, *CachedSearchResult]{
		NumCounters: 1e7,
		MaxCost:     1 << 29,
		BufferItems: 64,
	})
	if err != nil {
		b.Fatal(err)
	}

	indexer := &LogIndexer{
		index:      index,
		indexPath:  indexPath,
		parser:     NewOptimizedLogParser(NewSimpleUserAgentParser()),
		logPaths:   make(map[string]*LogFileInfo),
		indexBatch: 50000,
		cache:      cache,
	}
	defer indexer.Close()

	err = indexer.AddLogPath(logFile)
	if err != nil {
		b.Fatal(err)
	}

	err = indexer.IndexLogFile(logFile)
	if err != nil {
		b.Fatal(err)
	}

	time.Sleep(3 * time.Second)

	limits := []int{100, 1000, 5000, 10000, 50000}

	for _, limit := range limits {
		b.Run(fmt.Sprintf("Limit_%d", limit), func(b *testing.B) {
			query := &QueryRequest{
				Method: "GET",
				Limit:  limit,
			}

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				result, err := indexer.SearchLogs(context.Background(), query)
				if err != nil {
					b.Fatal(err)
				}
				b.ReportMetric(float64(len(result.Entries)), "results_returned")
			}
		})
	}
}

func BenchmarkAnalyticsAggregation_GeoStats(b *testing.B) {
	tempDir, err := os.MkdirTemp("", "nginx_log_analytics_bench")
	if err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	logFile := filepath.Join(tempDir, "access.log")
	err = generateLogFile(logFile, 500000)
	if err != nil {
		b.Fatal(err)
	}

	indexPath := filepath.Join(tempDir, "index")
	index, err := createOrOpenIndex(indexPath)
	if err != nil {
		b.Fatal(err)
	}

	cache, err := ristretto.NewCache(&ristretto.Config[string, *CachedSearchResult]{
		NumCounters: 1e7,
		MaxCost:     1 << 29,
		BufferItems: 64,
	})
	if err != nil {
		b.Fatal(err)
	}

	statsService := NewBleveStatsService()

	indexer := &LogIndexer{
		index:      index,
		indexPath:  indexPath,
		parser:     NewOptimizedLogParser(NewSimpleUserAgentParser()),
		logPaths:   make(map[string]*LogFileInfo),
		indexBatch: 50000,
		cache:      cache,
	}
	defer indexer.Close()

	statsService.SetIndexer(indexer)

	err = indexer.AddLogPath(logFile)
	if err != nil {
		b.Fatal(err)
	}

	err = indexer.IndexLogFile(logFile)
	if err != nil {
		b.Fatal(err)
	}

	time.Sleep(3 * time.Second)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := statsService.GetGeoStats(context.Background(), nil, 100)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func Benchmark100MRecords_FullPipeline(b *testing.B) {
	if testing.Short() {
		b.Skip("Skipping 100M records benchmark in short mode")
	}
	
	tempDir, err := os.MkdirTemp("", "nginx_log_100m_bench")
	if err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	b.Log("Starting 100M records benchmark...")
	
	logFile := filepath.Join(tempDir, "access_100m.log")
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		
		var m1, m2 runtime.MemStats
		runtime.GC()
		runtime.ReadMemStats(&m1)
		
		startTime := time.Now()
		
		b.Log("Phase 1: Generating 100M log records...")
		err := generateLogFile(logFile, 100000000)
		if err != nil {
			b.Fatal(err)
		}
		
		generationTime := time.Since(startTime)
		b.ReportMetric(generationTime.Seconds(), "generation_time_seconds")
		b.Logf("Generation completed in %.2f seconds", generationTime.Seconds())
		
		parseStartTime := time.Now()
		b.Log("Phase 2: Parsing with optimized parser...")
		
		b.StartTimer()
		
		parser := NewOptimizedLogParser(NewSimpleUserAgentParser())
		file, err := os.Open(logFile)
		if err != nil {
			b.Fatal(err)
		}

		scanner := bufio.NewScanner(file)
		scanner.Buffer(make([]byte, 0, 256*1024), 4096*1024)
		
		count := 0
		batchSize := 500000
		
		for scanner.Scan() {
			line := scanner.Text()
			if strings.TrimSpace(line) == "" {
				continue
			}
			
			_, err := parser.ParseLine(line)
			if err != nil {
				continue
			}
			count++
			
			if count%batchSize == 0 {
				runtime.GC()
				if count%(batchSize*10) == 0 {
					b.Logf("Processed %d records (%.1f%% complete)", count, float64(count)/100000000*100)
				}
			}
		}
		
		file.Close()
		
		b.StopTimer()
		
		parseTime := time.Since(parseStartTime)
		
		runtime.GC()
		runtime.ReadMemStats(&m2)
		
		b.ReportMetric(parseTime.Seconds(), "parse_time_seconds")
		b.ReportMetric(float64(count), "total_records_processed")
		b.ReportMetric(float64(count)/parseTime.Seconds(), "records_per_second")
		b.ReportMetric(float64(m2.Alloc-m1.Alloc)/1024/1024, "peak_memory_MB")
		
		b.Logf("Parse completed: %d records in %.2f seconds (%.0f records/sec)", 
			count, parseTime.Seconds(), float64(count)/parseTime.Seconds())
		b.Logf("Peak memory usage: %.2f MB", float64(m2.Alloc-m1.Alloc)/1024/1024)
		
		os.Remove(logFile)
	}
}