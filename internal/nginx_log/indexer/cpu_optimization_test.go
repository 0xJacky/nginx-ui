package indexer

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"testing"
	"time"
)

// BenchmarkCPUUtilization tests different worker configurations for CPU utilization
func BenchmarkCPUUtilization(b *testing.B) {
	configs := []struct {
		name        string
		workerCount int
		batchSize   int
		queueSize   int
	}{
		{"Current_8W_1000B", 8, 1000, 10000},
		{"CPU_Match", runtime.GOMAXPROCS(0), 1000, 10000},
		{"CPU_Double", runtime.GOMAXPROCS(0) * 2, 1000, 10000},
		{"CPU_Triple", runtime.GOMAXPROCS(0) * 3, 1000, 10000},
		{"HighBatch_8W_2000B", 8, 2000, 10000},
		{"HighBatch_12W_2000B", 12, 2000, 20000},
		{"LowLatency_16W_500B", 16, 500, 20000},
	}

	for _, config := range configs {
		b.Run(config.name, func(b *testing.B) {
			benchmarkWorkerConfiguration(b, config.workerCount, config.batchSize, config.queueSize)
		})
	}
}

func benchmarkWorkerConfiguration(b *testing.B, workerCount, batchSize, queueSize int) {
	b.Helper()
	
	// Create test configuration
	cfg := &Config{
		WorkerCount:  workerCount,
		BatchSize:    batchSize,
		MaxQueueSize: queueSize,
	}

	// Track CPU utilization during benchmark
	var totalCPUTime time.Duration
	var measurements int

	b.ResetTimer()
	b.SetBytes(int64(batchSize * 100)) // Approximate bytes per operation

	for i := 0; i < b.N; i++ {
		start := time.Now()
		
		// Simulate worker pipeline processing
		simulateWorkerPipeline(cfg)
		
		elapsed := time.Since(start)
		totalCPUTime += elapsed
		measurements++
	}

	// Report CPU utilization metrics
	avgProcessingTime := totalCPUTime / time.Duration(measurements)
	b.ReportMetric(float64(avgProcessingTime.Nanoseconds()), "ns/pipeline")
	b.ReportMetric(float64(workerCount), "workers")
	b.ReportMetric(float64(batchSize), "batch_size")
}

func simulateWorkerPipeline(cfg *Config) {
	// Create job and result channels
	jobQueue := make(chan *IndexJob, cfg.MaxQueueSize)
	resultQueue := make(chan *IndexResult, cfg.WorkerCount)

	// Create worker pool
	var wg sync.WaitGroup
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Start workers
	for i := 0; i < cfg.WorkerCount; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			simulateWorker(ctx, workerID, jobQueue, resultQueue)
		}(i)
	}

	// Generate jobs
	go func() {
		defer close(jobQueue)
		for i := 0; i < cfg.BatchSize; i++ {
			select {
			case jobQueue <- &IndexJob{
				Documents: []*Document{{
					ID: fmt.Sprintf("job-%d", i),
					Fields: &LogDocument{
						Timestamp: time.Now().Unix(),
						IP:        "127.0.0.1",
					},
				}},
				Priority:  1,
			}:
			case <-ctx.Done():
				return
			}
		}
	}()

	// Process results
	resultCount := 0
	done := make(chan bool)
	go func() {
		defer close(done)
		for {
			select {
			case <-resultQueue:
				resultCount++
				if resultCount >= cfg.BatchSize {
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	// Wait for completion
	select {
	case <-done:
	case <-ctx.Done():
	}

	wg.Wait()
}

func simulateWorker(ctx context.Context, workerID int, jobQueue <-chan *IndexJob, resultQueue chan<- *IndexResult) {
	for {
		select {
		case _, ok := <-jobQueue:
			if !ok {
				return
			}
			
			// Simulate CPU-intensive work
			simulateCPUWork()
			
			// Send result
			select {
			case resultQueue <- &IndexResult{
				Processed:  1,
				Succeeded:  1,
				Failed:     0,
				Throughput: 1.0,
			}:
			case <-ctx.Done():
				return
			}
		case <-ctx.Done():
			return
		}
	}
}

func simulateCPUWork() {
	// Simulate CPU-bound operations similar to log parsing and indexing
	sum := 0
	for i := 0; i < 10000; i++ {
		sum += i * i
	}
	// Prevent compiler optimization
	if sum < 0 {
		panic("unexpected")
	}
}

// BenchmarkPipelineBottleneck identifies bottlenecks in the processing pipeline
func BenchmarkPipelineBottleneck(b *testing.B) {
	tests := []struct {
		name          string
		jobQueueSize  int
		resultQueueSize int
		workerCount   int
	}{
		{"SmallQueues", 100, 10, 8},
		{"MediumQueues", 1000, 100, 8},
		{"LargeQueues", 10000, 1000, 8},
		{"BufferedPipeline", 50000, 5000, 8},
	}

	for _, test := range tests {
		b.Run(test.name, func(b *testing.B) {
			benchmarkPipelineConfiguration(b, test.jobQueueSize, test.resultQueueSize, test.workerCount)
		})
	}
}

func benchmarkPipelineConfiguration(b *testing.B, jobQueueSize, resultQueueSize, workerCount int) {
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		// Simulate pipeline with different buffer sizes
		jobQueue := make(chan *IndexJob, jobQueueSize)
		resultQueue := make(chan *IndexResult, resultQueueSize)
		
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		
		var wg sync.WaitGroup
		
		// Start workers
		for w := 0; w < workerCount; w++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for {
					select {
					case _, ok := <-jobQueue:
						if !ok {
							return
						}
						// Simulate processing
						for j := 0; j < 1000; j++ {
							_ = j * j
						}
						select {
						case resultQueue <- &IndexResult{
					Processed:  1,
					Succeeded:  1,
					Failed:     0,
					Throughput: 1.0,
				}:
						case <-ctx.Done():
							return
						}
					case <-ctx.Done():
						return
					}
				}
			}()
		}
		
		// Feed jobs
		go func() {
			defer close(jobQueue)
			for j := 0; j < 1000; j++ {
				select {
				case jobQueue <- &IndexJob{
					Documents: []*Document{{
						ID: fmt.Sprintf("job-%d", j),
						Fields: &LogDocument{
							Timestamp: time.Now().Unix(),
							IP:        "127.0.0.1",
						},
					}},
					Priority:  1,
				}:
				case <-ctx.Done():
					return
				}
			}
		}()
		
		// Consume results
		resultCount := 0
		for resultCount < 1000 {
			select {
			case <-resultQueue:
				resultCount++
			case <-ctx.Done():
				break
			}
		}
		
		cancel()
		wg.Wait()
	}
	
	b.ReportMetric(float64(jobQueueSize), "job_queue_size")
	b.ReportMetric(float64(resultQueueSize), "result_queue_size")
}

// BenchmarkMemoryPressure tests performance under different memory conditions
func BenchmarkMemoryPressure(b *testing.B) {
	tests := []struct {
		name        string
		allocSize   int
		allocCount  int
	}{
		{"LowMemory", 1024, 100},
		{"MediumMemory", 4096, 500},
		{"HighMemory", 16384, 1000},
	}

	for _, test := range tests {
		b.Run(test.name, func(b *testing.B) {
			// Allocate memory to simulate different memory pressure
			allocations := make([][]byte, test.allocCount)
			for i := 0; i < test.allocCount; i++ {
				allocations[i] = make([]byte, test.allocSize)
			}
			
			b.ResetTimer()
			
			for i := 0; i < b.N; i++ {
				// Simulate indexing work under memory pressure
				simulateMemoryIntensiveWork()
			}
			
			// Keep allocations alive until end
			runtime.KeepAlive(allocations)
		})
	}
}

func simulateMemoryIntensiveWork() {
	// Simulate memory allocation patterns similar to log parsing
	buffers := make([][]byte, 10)
	for i := range buffers {
		buffers[i] = make([]byte, 1024)
		// Fill with some data
		for j := range buffers[i] {
			buffers[i][j] = byte(i + j)
		}
	}
	
	// Simulate some processing
	sum := 0
	for _, buf := range buffers {
		for _, b := range buf {
			sum += int(b)
		}
	}
	
	// Prevent optimization
	if sum < 0 {
		panic("unexpected")
	}
}