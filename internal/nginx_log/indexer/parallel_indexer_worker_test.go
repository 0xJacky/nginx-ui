package indexer

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/blevesearch/bleve/v2"
)

// Mock shard manager for parallel indexer tests
type mockShardManagerForWorkerTest struct{}

func (m *mockShardManagerForWorkerTest) GetShard(key string) (bleve.Index, int, error) {
	return nil, 0, nil
}

func (m *mockShardManagerForWorkerTest) GetShardForDocument(mainLogPath string, key string) (bleve.Index, int, error) {
	return m.GetShard(key)
}

func (m *mockShardManagerForWorkerTest) GetShardByID(id int) (bleve.Index, error) {
	return nil, nil
}

func (m *mockShardManagerForWorkerTest) GetAllShards() []bleve.Index {
	return []bleve.Index{}
}

func (m *mockShardManagerForWorkerTest) GetShardCount() int {
	return 1
}

func (m *mockShardManagerForWorkerTest) Initialize() error {
	return nil
}

func (m *mockShardManagerForWorkerTest) GetShardStats() []*ShardInfo {
	return []*ShardInfo{}
}

func (m *mockShardManagerForWorkerTest) CreateShard(id int, path string) error {
	return nil
}

func (m *mockShardManagerForWorkerTest) Close() error {
	return nil
}

func (m *mockShardManagerForWorkerTest) CloseShard(id int) error {
	return nil
}

func (m *mockShardManagerForWorkerTest) HealthCheck() error {
	return nil
}

func (m *mockShardManagerForWorkerTest) OptimizeShard(id int) error {
	return nil
}

// Test helper to create parallel indexer for worker tests
func createTestParallelIndexer(workerCount int) *ParallelIndexer {
	config := &Config{
		WorkerCount:  workerCount,
		BatchSize:    100,
		MaxQueueSize: 1000,
	}

	shardManager := &mockShardManagerForWorkerTest{}
	return NewParallelIndexer(config, shardManager)
}

func TestParallelIndexer_handleWorkerCountChange_Increase(t *testing.T) {
	pi := createTestParallelIndexer(4)

	// Start the indexer
	ctx := context.Background()
	err := pi.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start parallel indexer: %v", err)
	}
	defer pi.Stop()

	// Allow time for initialization
	time.Sleep(100 * time.Millisecond)

	initialWorkerCount := len(pi.workers)
	if initialWorkerCount != 4 {
		t.Fatalf("Expected 4 initial workers, got %d", initialWorkerCount)
	}

	// Test increasing worker count
	pi.handleWorkerCountChange(4, 6)

	// Verify worker count increased
	newWorkerCount := len(pi.workers)
	if newWorkerCount != 6 {
		t.Errorf("Expected 6 workers after increase, got %d", newWorkerCount)
	}

	// Verify config was updated
	if pi.config.WorkerCount != 6 {
		t.Errorf("Expected config worker count to be 6, got %d", pi.config.WorkerCount)
	}

	// Verify stats were updated
	if len(pi.stats.WorkerStats) != 6 {
		t.Errorf("Expected 6 worker stats, got %d", len(pi.stats.WorkerStats))
	}
}

func TestParallelIndexer_handleWorkerCountChange_Decrease(t *testing.T) {
	pi := createTestParallelIndexer(6)

	// Start the indexer
	ctx := context.Background()
	err := pi.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start parallel indexer: %v", err)
	}
	defer pi.Stop()

	// Allow time for initialization
	time.Sleep(100 * time.Millisecond)

	initialWorkerCount := len(pi.workers)
	if initialWorkerCount != 6 {
		t.Fatalf("Expected 6 initial workers, got %d", initialWorkerCount)
	}

	// Test decreasing worker count
	pi.handleWorkerCountChange(6, 4)

	// Verify worker count decreased
	newWorkerCount := len(pi.workers)
	if newWorkerCount != 4 {
		t.Errorf("Expected 4 workers after decrease, got %d", newWorkerCount)
	}

	// Verify config was updated
	if pi.config.WorkerCount != 4 {
		t.Errorf("Expected config worker count to be 4, got %d", pi.config.WorkerCount)
	}

	// Verify stats were updated
	if len(pi.stats.WorkerStats) != 4 {
		t.Errorf("Expected 4 worker stats, got %d", len(pi.stats.WorkerStats))
	}
}

func TestParallelIndexer_handleWorkerCountChange_NoChange(t *testing.T) {
	pi := createTestParallelIndexer(4)

	// Start the indexer
	ctx := context.Background()
	err := pi.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start parallel indexer: %v", err)
	}
	defer pi.Stop()

	// Allow time for initialization
	time.Sleep(100 * time.Millisecond)

	initialWorkerCount := len(pi.workers)

	// Test no change scenario
	pi.handleWorkerCountChange(4, 4)

	// Verify worker count didn't change
	newWorkerCount := len(pi.workers)
	if newWorkerCount != initialWorkerCount {
		t.Errorf("Expected worker count to remain %d, got %d", initialWorkerCount, newWorkerCount)
	}
}

func TestParallelIndexer_handleWorkerCountChange_NotRunning(t *testing.T) {
	pi := createTestParallelIndexer(4)

	// Don't start the indexer - it should be in stopped state

	initialWorkerCount := len(pi.workers)

	// Test worker count change when not running
	pi.handleWorkerCountChange(4, 6)

	// Verify no change occurred
	newWorkerCount := len(pi.workers)
	if newWorkerCount != initialWorkerCount {
		t.Errorf("Expected no worker change when not running, initial: %d, new: %d",
			initialWorkerCount, newWorkerCount)
	}

	// Verify config wasn't updated
	if pi.config.WorkerCount != 4 {
		t.Errorf("Expected config worker count to remain 4, got %d", pi.config.WorkerCount)
	}
}

func TestParallelIndexer_addWorkers(t *testing.T) {
	pi := createTestParallelIndexer(2)

	// Start the indexer
	ctx := context.Background()
	err := pi.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start parallel indexer: %v", err)
	}
	defer pi.Stop()

	// Allow time for initialization
	time.Sleep(100 * time.Millisecond)

	initialCount := len(pi.workers)
	if initialCount != 2 {
		t.Fatalf("Expected 2 initial workers, got %d", initialCount)
	}

	// Add 3 workers
	pi.addWorkers(3)

	// Verify workers were added
	newCount := len(pi.workers)
	if newCount != 5 {
		t.Errorf("Expected 5 workers after adding 3, got %d", newCount)
	}

	// Verify worker IDs are sequential
	for i, worker := range pi.workers {
		if worker.id != i {
			t.Errorf("Expected worker %d to have ID %d, got %d", i, i, worker.id)
		}
	}

	// Verify stats were updated
	if len(pi.stats.WorkerStats) != 5 {
		t.Errorf("Expected 5 worker stats, got %d", len(pi.stats.WorkerStats))
	}
}

func TestParallelIndexer_removeWorkers(t *testing.T) {
	pi := createTestParallelIndexer(5)

	// Start the indexer
	ctx := context.Background()
	err := pi.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start parallel indexer: %v", err)
	}
	defer pi.Stop()

	// Allow time for initialization
	time.Sleep(100 * time.Millisecond)

	initialCount := len(pi.workers)
	if initialCount != 5 {
		t.Fatalf("Expected 5 initial workers, got %d", initialCount)
	}

	// Remove 2 workers
	pi.removeWorkers(2)

	// Verify workers were removed
	newCount := len(pi.workers)
	if newCount != 3 {
		t.Errorf("Expected 3 workers after removing 2, got %d", newCount)
	}

	// Verify stats were updated
	if len(pi.stats.WorkerStats) != 3 {
		t.Errorf("Expected 3 worker stats, got %d", len(pi.stats.WorkerStats))
	}
}

func TestParallelIndexer_removeWorkers_KeepMinimum(t *testing.T) {
	pi := createTestParallelIndexer(2)

	// Start the indexer
	ctx := context.Background()
	err := pi.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start parallel indexer: %v", err)
	}
	defer pi.Stop()

	// Allow time for initialization
	time.Sleep(100 * time.Millisecond)

	initialCount := len(pi.workers)
	if initialCount != 2 {
		t.Fatalf("Expected 2 initial workers, got %d", initialCount)
	}

	// Try to remove all workers (should keep at least one)
	pi.removeWorkers(2)

	// Verify at least one worker remains
	newCount := len(pi.workers)
	if newCount != 1 {
		t.Errorf("Expected 1 worker to remain after trying to remove all, got %d", newCount)
	}

	// Verify stats were updated
	if len(pi.stats.WorkerStats) != 1 {
		t.Errorf("Expected 1 worker stat, got %d", len(pi.stats.WorkerStats))
	}
}

func TestParallelIndexer_AdaptiveOptimizerIntegration(t *testing.T) {
	pi := createTestParallelIndexer(4)

	// Enable optimization
	pi.optimizationEnabled = true
	pi.adaptiveOptimizer = NewAdaptiveOptimizer(pi.config)

	// Start the indexer
	ctx := context.Background()
	err := pi.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start parallel indexer: %v", err)
	}
	defer pi.Stop()

	// Allow time for initialization
	time.Sleep(100 * time.Millisecond)

	// Verify adaptive optimizer callback was set
	if pi.adaptiveOptimizer.onWorkerCountChange == nil {
		t.Error("Expected adaptive optimizer callback to be set")
	}

	// Simulate worker count change from adaptive optimizer
	initialWorkerCount := len(pi.workers)

	// Trigger callback (simulate adaptive optimizer decision)
	if pi.adaptiveOptimizer.onWorkerCountChange != nil {
		pi.adaptiveOptimizer.onWorkerCountChange(4, 6)
	}

	// Verify worker count changed
	newWorkerCount := len(pi.workers)
	if newWorkerCount == initialWorkerCount {
		t.Error("Expected worker count to change from adaptive optimizer callback")
	}
}

func TestParallelIndexer_ConcurrentWorkerAdjustments(t *testing.T) {
	pi := createTestParallelIndexer(4)

	// Start the indexer
	ctx := context.Background()
	err := pi.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start parallel indexer: %v", err)
	}
	defer pi.Stop()

	// Allow time for initialization
	time.Sleep(100 * time.Millisecond)

	var wg sync.WaitGroup

	// Simulate concurrent worker adjustments
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(iteration int) {
			defer wg.Done()

			// Alternate between increasing and decreasing
			if iteration%2 == 0 {
				pi.handleWorkerCountChange(pi.config.WorkerCount, pi.config.WorkerCount+1)
			} else {
				if pi.config.WorkerCount > 2 {
					pi.handleWorkerCountChange(pi.config.WorkerCount, pi.config.WorkerCount-1)
				}
			}
		}(i)
	}

	wg.Wait()

	// Verify final state is consistent
	workerCount := len(pi.workers)
	configCount := pi.config.WorkerCount
	statsCount := len(pi.stats.WorkerStats)

	if workerCount != configCount {
		t.Errorf("Worker count (%d) doesn't match config count (%d)", workerCount, configCount)
	}

	if workerCount != statsCount {
		t.Errorf("Worker count (%d) doesn't match stats count (%d)", workerCount, statsCount)
	}

	// Verify worker IDs are sequential and unique
	workerIDs := make(map[int]bool)
	for i, worker := range pi.workers {
		if worker.id != i {
			t.Errorf("Expected worker at index %d to have ID %d, got %d", i, i, worker.id)
		}

		if workerIDs[worker.id] {
			t.Errorf("Duplicate worker ID found: %d", worker.id)
		}
		workerIDs[worker.id] = true
	}
}

func TestParallelIndexer_WorkerStatsConsistency(t *testing.T) {
	pi := createTestParallelIndexer(3)

	// Start the indexer
	ctx := context.Background()
	err := pi.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start parallel indexer: %v", err)
	}
	defer pi.Stop()

	// Allow time for initialization
	time.Sleep(100 * time.Millisecond)

	// Test adding workers
	pi.addWorkers(2)

	// Verify stats consistency
	workerCount := len(pi.workers)
	statsCount := len(pi.stats.WorkerStats)

	if workerCount != statsCount {
		t.Errorf("Worker count (%d) doesn't match stats count (%d)", workerCount, statsCount)
	}

	// Verify each worker has corresponding stats
	for i, worker := range pi.workers {
		if pi.stats.WorkerStats[i].ID != worker.id {
			t.Errorf("Worker %d ID (%d) doesn't match stats ID (%d)",
				i, worker.id, pi.stats.WorkerStats[i].ID)
		}

		if worker.stats != pi.stats.WorkerStats[i] {
			t.Errorf("Worker %d stats pointer doesn't match global stats", i)
		}
	}

	// Test removing workers
	pi.removeWorkers(1)

	// Verify stats consistency after removal
	workerCount = len(pi.workers)
	statsCount = len(pi.stats.WorkerStats)

	if workerCount != statsCount {
		t.Errorf("After removal, worker count (%d) doesn't match stats count (%d)",
			workerCount, statsCount)
	}
}
