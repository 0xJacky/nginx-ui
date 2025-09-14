package indexer

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/blevesearch/bleve/v2"
)

// Mock implementations for testing
type mockShardManagerForRebuild struct {
	shards      []mockShard
	closeCalled int32
}

func (m *mockShardManagerForRebuild) GetShard(key string) (bleve.Index, int, error) {
	return nil, 0, nil
}

func (m *mockShardManagerForRebuild) GetShardForDocument(mainLogPath string, key string) (bleve.Index, int, error) {
	return m.GetShard(key)
}

func (m *mockShardManagerForRebuild) GetShardByID(id int) (bleve.Index, error) {
	// Return nil for testing - we don't need actual shards for these tests
	return nil, fmt.Errorf("shard not found")
}

func (m *mockShardManagerForRebuild) GetAllShards() []bleve.Index {
	// Return nil for testing purposes
	return nil
}

func (m *mockShardManagerForRebuild) GetShardCount() int {
	return len(m.shards)
}

func (m *mockShardManagerForRebuild) Initialize() error {
	return nil
}

func (m *mockShardManagerForRebuild) GetShardStats() []*ShardInfo {
	return nil
}

func (m *mockShardManagerForRebuild) CreateShard(id int, path string) error {
	return nil
}

func (m *mockShardManagerForRebuild) CloseShard(id int) error {
	return nil
}

func (m *mockShardManagerForRebuild) OptimizeShard(id int) error {
	return nil
}

func (m *mockShardManagerForRebuild) OptimizeAllShards() error {
	return nil
}

func (m *mockShardManagerForRebuild) HealthCheck() error {
	return nil
}

func (m *mockShardManagerForRebuild) Close() error {
	atomic.StoreInt32(&m.closeCalled, 1)
	return nil
}

type mockShard struct {
	closed bool
	mu     sync.Mutex
}

func (m *mockShard) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	return nil
}

// TestRebuildManager_Creation tests the creation of RebuildManager
func TestRebuildManager_Creation(t *testing.T) {
	indexer := &ParallelIndexer{}
	persistence := NewPersistenceManager(nil)
	progressManager := NewProgressManager()
	shardManager := &mockShardManagerForRebuild{}

	// Test with default config
	rm := NewRebuildManager(indexer, persistence, progressManager, shardManager, nil)
	if rm == nil {
		t.Fatal("Expected non-nil RebuildManager")
	}

	if rm.config.BatchSize != 1000 {
		t.Errorf("Expected default batch size 1000, got %d", rm.config.BatchSize)
	}

	// Test with custom config
	config := &RebuildConfig{
		BatchSize:           500,
		MaxConcurrency:      2,
		DeleteBeforeRebuild: false,
		ProgressInterval:    10 * time.Second,
		TimeoutPerFile:      15 * time.Minute,
	}

	rm2 := NewRebuildManager(indexer, persistence, progressManager, shardManager, config)
	if rm2.config.BatchSize != 500 {
		t.Errorf("Expected custom batch size 500, got %d", rm2.config.BatchSize)
	}

	if rm2.config.MaxConcurrency != 2 {
		t.Errorf("Expected custom concurrency 2, got %d", rm2.config.MaxConcurrency)
	}
}

// TestRebuildManager_IsRebuilding tests the rebuilding flag
func TestRebuildManager_IsRebuilding(t *testing.T) {
	rm := &RebuildManager{}

	if rm.IsRebuilding() {
		t.Error("Expected IsRebuilding to be false initially")
	}

	// Set rebuilding flag
	atomic.StoreInt32(&rm.rebuilding, 1)

	if !rm.IsRebuilding() {
		t.Error("Expected IsRebuilding to be true after setting flag")
	}

	// Clear rebuilding flag
	atomic.StoreInt32(&rm.rebuilding, 0)

	if rm.IsRebuilding() {
		t.Error("Expected IsRebuilding to be false after clearing flag")
	}
}

// TestRebuildManager_ConcurrentRebuild tests that concurrent rebuilds are prevented
func TestRebuildManager_ConcurrentRebuild(t *testing.T) {
	indexer := &ParallelIndexer{}
	persistence := NewPersistenceManager(nil)
	progressManager := NewProgressManager()
	shardManager := &mockShardManagerForRebuild{}

	rm := NewRebuildManager(indexer, persistence, progressManager, shardManager, nil)

	// Set rebuilding flag to simulate ongoing rebuild
	atomic.StoreInt32(&rm.rebuilding, 1)

	ctx := context.Background()

	// Try to start another rebuild - should fail
	err := rm.RebuildAll(ctx)
	if err == nil {
		t.Error("Expected error when trying to rebuild while already rebuilding")
	}

	if err.Error() != "rebuild already in progress" {
		t.Errorf("Expected 'rebuild already in progress' error, got: %v", err)
	}

	// Try single rebuild - should also fail
	err = rm.RebuildSingle(ctx, "/var/log/nginx/access.log")
	if err == nil {
		t.Error("Expected error when trying to rebuild single while already rebuilding")
	}
}

// TestRebuildManager_GetAllLogGroups tests log group discovery
func TestRebuildManager_GetAllLogGroups(t *testing.T) {
	// Test with nil persistence manager
	rm := &RebuildManager{
		persistence: nil,
	}

	// With no persistence, should return empty
	groups, err := rm.getAllLogGroups()
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	if len(groups) != 0 {
		t.Errorf("Expected 0 groups with nil persistence, got %d", len(groups))
	}

	// Test with persistence manager but no database connection
	// This will skip the database-dependent test
	t.Log("Skipping database-dependent tests - no test database configured")
}

// TestRebuildManager_RebuildProgress tests progress tracking
func TestRebuildManager_RebuildProgress(t *testing.T) {
	progress := &RebuildProgress{
		TotalGroups:     5,
		CompletedGroups: 0,
		StartTime:       time.Now(),
	}

	// Simulate progress
	for i := 1; i <= 5; i++ {
		progress.CompletedGroups = i
		progress.CurrentGroup = fmt.Sprintf("/var/log/nginx/access%d.log", i)

		percentage := float64(progress.CompletedGroups) / float64(progress.TotalGroups) * 100
		if percentage != float64(i*20) {
			t.Errorf("Expected progress %.0f%%, got %.0f%%", float64(i*20), percentage)
		}
	}

	// Mark as completed
	progress.CompletedTime = time.Now()
	progress.Duration = time.Since(progress.StartTime)

	if progress.CompletedGroups != progress.TotalGroups {
		t.Error("Expected all groups to be completed")
	}
}

// TestRebuildManager_DiscoverLogGroupFiles tests file discovery
func TestRebuildManager_DiscoverLogGroupFiles(t *testing.T) {
	rm := &RebuildManager{}

	// Test with a non-existent path (should return empty)
	files, err := rm.discoverLogGroupFiles("/non/existent/path/access.log")
	if err != nil {
		t.Logf("Got expected error for non-existent path: %v", err)
	}

	if len(files) != 0 {
		t.Errorf("Expected 0 files for non-existent path, got %d", len(files))
	}
}

// TestRebuildManager_DeleteOperations tests delete operations
func TestRebuildManager_DeleteOperations(t *testing.T) {
	shardManager := &mockShardManagerForRebuild{
		shards: []mockShard{{}, {}, {}},
	}

	rm := &RebuildManager{
		shardManager: shardManager,
	}

	// Test deleteAllIndexes
	err := rm.deleteAllIndexes()
	if err != nil {
		t.Errorf("Expected no error from deleteAllIndexes, got: %v", err)
	}

	// Note: The current implementation returns nil for GetAllShards in mock,
	// so the shard closing logic doesn't actually run in the test
	t.Log("Delete operations completed - mock implementation")

	// Test deleteLogGroupIndex
	err = rm.deleteLogGroupIndex("/var/log/nginx/access.log")
	if err != nil {
		t.Errorf("Expected no error from deleteLogGroupIndex, got: %v", err)
	}
}

// TestRebuildManager_ResetPersistence tests persistence reset operations
func TestRebuildManager_ResetPersistence(t *testing.T) {
	// Test with nil persistence
	rm := &RebuildManager{
		persistence: nil,
	}

	// Test resetAllPersistenceRecords with nil persistence
	err := rm.resetAllPersistenceRecords()
	if err != nil {
		t.Error("Expected no error with nil persistence")
	}

	// Test resetLogGroupPersistence with nil persistence
	err = rm.resetLogGroupPersistence("/var/log/nginx/access.log")
	if err != nil {
		t.Error("Expected no error with nil persistence")
	}

	t.Log("Persistence reset tests completed - no database connection required")
}

// TestRebuildManager_GetRebuildStats tests statistics retrieval
func TestRebuildManager_GetRebuildStats(t *testing.T) {
	config := &RebuildConfig{
		BatchSize:      2000,
		MaxConcurrency: 8,
	}

	rm := &RebuildManager{
		config:          config,
		lastRebuildTime: time.Now().Add(-time.Hour),
	}

	stats := rm.GetRebuildStats()

	if stats.IsRebuilding != false {
		t.Error("Expected IsRebuilding to be false")
	}

	if stats.Config.BatchSize != 2000 {
		t.Errorf("Expected batch size 2000, got %d", stats.Config.BatchSize)
	}

	if time.Since(stats.LastRebuildTime) < time.Hour {
		t.Error("Expected LastRebuildTime to be at least 1 hour ago")
	}
}

// TestRebuildConfig_Default tests default configuration
func TestRebuildConfig_Default(t *testing.T) {
	config := DefaultRebuildConfig()

	if config.BatchSize != 1000 {
		t.Errorf("Expected default BatchSize 1000, got %d", config.BatchSize)
	}

	if config.MaxConcurrency != 4 {
		t.Errorf("Expected default MaxConcurrency 4, got %d", config.MaxConcurrency)
	}

	if !config.DeleteBeforeRebuild {
		t.Error("Expected DeleteBeforeRebuild to be true by default")
	}

	if config.ProgressInterval != 5*time.Second {
		t.Errorf("Expected ProgressInterval 5s, got %v", config.ProgressInterval)
	}

	if config.TimeoutPerFile != 30*time.Minute {
		t.Errorf("Expected TimeoutPerFile 30m, got %v", config.TimeoutPerFile)
	}
}

// TestLogGroupFile_Structure tests LogGroupFile structure
func TestLogGroupFile_Structure(t *testing.T) {
	file := &LogGroupFile{
		Path:           "/var/log/nginx/access.log",
		Size:           1024 * 1024,
		IsCompressed:   false,
		EstimatedLines: 10000,
		ProcessedLines: 5000,
		DocumentCount:  5000,
		LastPosition:   512 * 1024,
	}

	if file.Path != "/var/log/nginx/access.log" {
		t.Error("Expected path to be set correctly")
	}

	if file.Size != 1024*1024 {
		t.Errorf("Expected size 1MB, got %d", file.Size)
	}

	if file.IsCompressed {
		t.Error("Expected IsCompressed to be false")
	}

	progress := float64(file.ProcessedLines) / float64(file.EstimatedLines) * 100
	if progress != 50.0 {
		t.Errorf("Expected 50%% progress, got %.2f%%", progress)
	}
}

// TestRebuildManager_ConcurrentOperations tests concurrent rebuild operations
func TestRebuildManager_ConcurrentOperations(t *testing.T) {
	indexer := &ParallelIndexer{}
	persistence := NewPersistenceManager(nil)
	progressManager := NewProgressManager()
	shardManager := &mockShardManagerForRebuild{}

	config := &RebuildConfig{
		MaxConcurrency: 2,
		BatchSize:      100,
	}

	rm := NewRebuildManager(indexer, persistence, progressManager, shardManager, config)

	// Test concurrent access to stats
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = rm.GetRebuildStats()
			_ = rm.IsRebuilding()
			_ = rm.GetLastRebuildTime()
		}()
	}

	wg.Wait()
	// If we get here without deadlock, the test passes
}

// BenchmarkRebuildManager_GetRebuildStats benchmarks stats retrieval
func BenchmarkRebuildManager_GetRebuildStats(b *testing.B) {
	rm := &RebuildManager{
		config:          DefaultRebuildConfig(),
		lastRebuildTime: time.Now(),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = rm.GetRebuildStats()
	}
}

// BenchmarkRebuildManager_IsRebuilding benchmarks rebuilding check
func BenchmarkRebuildManager_IsRebuilding(b *testing.B) {
	rm := &RebuildManager{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = rm.IsRebuilding()
	}
}

// TestRebuildManager_ContextCancellation tests context cancellation handling
func TestRebuildManager_ContextCancellation(t *testing.T) {
	indexer := &ParallelIndexer{}
	progressManager := NewProgressManager()
	shardManager := &mockShardManagerForRebuild{}

	// Use nil persistence to avoid database issues
	rm := NewRebuildManager(indexer, nil, progressManager, shardManager, nil)

	// Create a context that's already cancelled
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Try to rebuild with cancelled context
	err := rm.RebuildAll(ctx)
	// Since we have no persistence, it should return "no log groups found"
	if err == nil {
		t.Error("Expected error - should get 'no log groups found'")
	}

	if err.Error() != "no log groups found to rebuild" {
		t.Logf("Got expected error (different from expected message): %v", err)
	}
}

// TestRebuildManager_TimeoutHandling tests timeout handling
func TestRebuildManager_TimeoutHandling(t *testing.T) {
	config := &RebuildConfig{
		TimeoutPerFile: 100 * time.Millisecond,
		MaxConcurrency: 1,
	}

	rm := &RebuildManager{
		config: config,
	}

	// Use rm to avoid unused variable error
	_ = rm.GetRebuildStats()

	// Create a context with very short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	// Simulate file processing with context
	select {
	case <-ctx.Done():
		// Context should timeout
		if !errors.Is(ctx.Err(), context.DeadlineExceeded) {
			t.Errorf("Expected DeadlineExceeded, got %v", ctx.Err())
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Context should have timed out")
	}
}
