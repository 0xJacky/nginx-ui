package indexer

import (
	"context"
	"runtime"
	"testing"
	"time"

	"github.com/blevesearch/bleve/v2"
)

// TestDynamicShardAwareness tests the dynamic shard awareness system
func TestDynamicShardAwareness(t *testing.T) {
	config := DefaultIndexerConfig()

	// Create dynamic awareness
	dsa := NewDynamicShardAwareness(config)

	// Test environment factor analysis
	factors := dsa.analyzeEnvironmentFactors()

	t.Logf("Environment Analysis:")
	t.Logf("  CPU Cores: %d", factors.CPUCores)
	t.Logf("  Memory GB: %.2f", factors.MemoryGB)
	t.Logf("  Expected Load: %s", factors.ExpectedLoad)
	t.Logf("  Data Volume: %s", factors.DataVolume)
	t.Logf("  Query Patterns: %s", factors.QueryPatterns)

	// Test shard manager selection
	shouldUseDynamic := dsa.shouldUseDynamicShards(factors)
	t.Logf("Should use dynamic shards: %v", shouldUseDynamic)

	// Test manager setup
	manager, err := dsa.DetectAndSetupShardManager()
	if err != nil {
		t.Fatalf("Failed to setup shard manager: %v", err)
	}

	isDynamic := dsa.IsDynamic()
	t.Logf("Dynamic shard management active: %v", isDynamic)

	// Verify manager type
	if isDynamic {
		if _, ok := manager.(*EnhancedDynamicShardManager); !ok {
			t.Errorf("Expected EnhancedDynamicShardManager, got %T", manager)
		}
		t.Logf("✅ Dynamic shard manager successfully created")
	} else {
		if _, ok := manager.(*DefaultShardManager); !ok {
			t.Errorf("Expected DefaultShardManager, got %T", manager)
		}
		t.Logf("✅ Static shard manager successfully created")
	}
}

// TestEnhancedDynamicShardManager tests the enhanced shard manager functionality
func TestEnhancedDynamicShardManager(t *testing.T) {
	config := DefaultIndexerConfig()
	config.IndexPath = t.TempDir()

	// Create enhanced dynamic shard manager
	dsm := NewEnhancedDynamicShardManager(config)

	// Initialize
	if err := dsm.Initialize(); err != nil {
		t.Fatalf("Failed to initialize enhanced shard manager: %v", err)
	}

	// Check initial shard count
	initialCount := dsm.GetShardCount()
	t.Logf("Initial shard count: %d", initialCount)

	if initialCount != config.ShardCount {
		t.Errorf("Expected initial shard count %d, got %d", config.ShardCount, initialCount)
	}

	// Test scaling up
	targetCount := initialCount + 2
	t.Logf("Testing scale up to %d shards", targetCount)

	if err := dsm.ScaleShards(targetCount); err != nil {
		t.Errorf("Failed to scale up shards: %v", err)
	} else {
		newCount := dsm.GetShardCount()
		t.Logf("After scaling up: %d shards", newCount)

		if newCount != targetCount {
			t.Errorf("Expected %d shards after scaling up, got %d", targetCount, newCount)
		} else {
			t.Logf("✅ Scale up successful: %d → %d shards", initialCount, newCount)
		}
	}

	// Test scaling down
	targetCount = initialCount
	t.Logf("Testing scale down to %d shards", targetCount)

	if err := dsm.ScaleShards(targetCount); err != nil {
		t.Errorf("Failed to scale down shards: %v", err)
	} else {
		newCount := dsm.GetShardCount()
		t.Logf("After scaling down: %d shards", newCount)

		if newCount != targetCount {
			t.Errorf("Expected %d shards after scaling down, got %d", targetCount, newCount)
		} else {
			t.Logf("✅ Scale down successful: %d → %d shards", initialCount+2, newCount)
		}
	}

	// Cleanup
	if err := dsm.Close(); err != nil {
		t.Errorf("Failed to close shard manager: %v", err)
	}
}

// TestParallelIndexerWithDynamicShards tests ParallelIndexer with dynamic shard awareness
func TestParallelIndexerWithDynamicShards(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping long-running test in short mode")
	}

	config := DefaultIndexerConfig()
	config.IndexPath = t.TempDir()
	config.WorkerCount = runtime.GOMAXPROCS(0) * 2 // Ensure high worker count for dynamic detection

	// Create indexer with nil shard manager to trigger dynamic detection
	indexer := NewParallelIndexer(config, nil)

	// Check if dynamic awareness is working
	if indexer.dynamicAwareness == nil {
		t.Fatal("Dynamic awareness should be initialized")
	}

	isDynamic := indexer.dynamicAwareness.IsDynamic()
	t.Logf("Dynamic shard management detected: %v", isDynamic)

	currentManager := indexer.dynamicAwareness.GetCurrentShardManager()
	t.Logf("Current shard manager type: %T", currentManager)

	// For M2 Pro with 12 cores, 24 workers, should detect dynamic management
	if runtime.GOMAXPROCS(0) >= 8 {
		if !isDynamic {
			t.Errorf("Expected dynamic shard management on high-core system (Procs: %d)", runtime.GOMAXPROCS(0))
		} else {
			t.Logf("✅ Dynamic shard management correctly detected on high-core system")
		}
	}

	// Test starting and stopping
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := indexer.Start(ctx); err != nil {
		t.Fatalf("Failed to start indexer: %v", err)
	}

	// Let it run briefly
	time.Sleep(1 * time.Second)

	if err := indexer.Stop(); err != nil {
		t.Errorf("Failed to stop indexer: %v", err)
	}

	t.Logf("✅ ParallelIndexer with dynamic shard awareness started and stopped successfully")
}

// TestDataMigrationDuringScaleDown tests that data is properly migrated during shard scale-down
func TestDataMigrationDuringScaleDown(t *testing.T) {
	config := DefaultIndexerConfig()
	config.IndexPath = t.TempDir()
	config.ShardCount = 4 // Start with 4 shards

	// Create enhanced dynamic shard manager
	dsm := NewEnhancedDynamicShardManager(config)

	// Initialize
	if err := dsm.Initialize(); err != nil {
		t.Fatalf("Failed to initialize enhanced shard manager: %v", err)
	}
	defer dsm.Close()

	t.Logf("✅ Initialized shard manager with %d shards", dsm.GetShardCount())

	// Add test documents to different shards
	testDocs := []struct {
		id   string
		data map[string]interface{}
	}{
		{"doc1", map[string]interface{}{"content": "test document 1", "type": "log"}},
		{"doc2", map[string]interface{}{"content": "test document 2", "type": "log"}},
		{"doc3", map[string]interface{}{"content": "test document 3", "type": "log"}},
		{"doc4", map[string]interface{}{"content": "test document 4", "type": "log"}},
		{"doc5", map[string]interface{}{"content": "test document 5", "type": "log"}},
		{"doc6", map[string]interface{}{"content": "test document 6", "type": "log"}},
	}

	// Index documents across shards
	var totalDocs int64
	shardDocCounts := make(map[int]int64)

	for _, testDoc := range testDocs {
		// Determine which shard this document belongs to
		shardID := dsm.DefaultShardManager.hashFunc(testDoc.id, config.ShardCount)
		shard, err := dsm.GetShardByID(shardID)
		if err != nil {
			t.Fatalf("Failed to get shard %d: %v", shardID, err)
		}

		// Index the document
		if err := shard.Index(testDoc.id, testDoc.data); err != nil {
			t.Fatalf("Failed to index document %s in shard %d: %v", testDoc.id, shardID, err)
		}

		shardDocCounts[shardID]++
		totalDocs++
	}

	t.Logf("✅ Indexed %d documents across shards", totalDocs)

	// Log distribution before scaling
	for shardID, count := range shardDocCounts {
		t.Logf("Shard %d: %d documents", shardID, count)
	}

	// Count total documents before scaling
	var beforeCount uint64
	for i := 0; i < config.ShardCount; i++ {
		shard, err := dsm.GetShardByID(i)
		if err != nil {
			continue
		}
		count, _ := shard.DocCount()
		beforeCount += count
	}
	t.Logf("Total documents before scaling: %d", beforeCount)

	// Scale down from 4 to 2 shards (should migrate data from shards 2 and 3)
	targetShards := 2
	t.Logf("🔄 Scaling down from %d to %d shards", config.ShardCount, targetShards)

	err := dsm.ScaleShards(targetShards)
	if err != nil {
		t.Fatalf("Failed to scale down shards: %v", err)
	}

	// Verify final shard count
	finalShardCount := dsm.GetShardCount()
	if finalShardCount != targetShards {
		t.Fatalf("Expected %d shards after scaling, got %d", targetShards, finalShardCount)
	}

	// Count total documents after scaling
	var afterCount uint64
	for i := 0; i < targetShards; i++ {
		shard, err := dsm.GetShardByID(i)
		if err != nil {
			t.Errorf("Failed to get shard %d after scaling: %v", i, err)
			continue
		}
		count, _ := shard.DocCount()
		afterCount += count
		t.Logf("Shard %d after scaling: %d documents", i, count)
	}

	t.Logf("Total documents after scaling: %d", afterCount)

	// Verify no data loss
	if afterCount != beforeCount {
		t.Errorf("Data loss detected! Before: %d documents, After: %d documents", beforeCount, afterCount)
	} else {
		t.Logf("✅ No data loss: %d documents preserved", afterCount)
	}

	// Verify all original documents are still searchable
	for _, testDoc := range testDocs {
		found := false
		for i := 0; i < targetShards; i++ {
			shard, err := dsm.GetShardByID(i)
			if err != nil {
				continue
			}

			// Try to find the document
			doc, err := shard.Document(testDoc.id)
			if err == nil && doc != nil {
				found = true
				t.Logf("✅ Document %s found in shard %d after migration", testDoc.id, i)
				break
			}
		}

		if !found {
			t.Errorf("❌ Document %s not found after migration", testDoc.id)
		}
	}

	// Test searching across all remaining shards
	for i := 0; i < targetShards; i++ {
		shard, err := dsm.GetShardByID(i)
		if err != nil {
			continue
		}

		// Search for all documents
		query := bleve.NewMatchAllQuery()
		searchReq := bleve.NewSearchRequest(query)
		searchReq.Size = 100

		results, err := shard.Search(searchReq)
		if err != nil {
			t.Errorf("Search failed in shard %d: %v", i, err)
			continue
		}

		t.Logf("Shard %d search results: %d hits", i, len(results.Hits))
	}

	t.Logf("✅ Data migration during scale-down completed successfully")
}

// TestDataMigrationBasicValidation tests the core data migration logic
func TestDataMigrationBasicValidation(t *testing.T) {
	config := DefaultIndexerConfig()
	config.IndexPath = t.TempDir()
	config.ShardCount = 3 // Start with 3 shards

	// Create enhanced dynamic shard manager
	dsm := NewEnhancedDynamicShardManager(config)

	// Initialize
	if err := dsm.Initialize(); err != nil {
		t.Fatalf("Failed to initialize enhanced shard manager: %v", err)
	}
	defer dsm.Close()

	t.Logf("✅ Initialized with %d shards", dsm.GetShardCount())

	// Add test documents to shard 2 (which we'll migrate)
	testDocs := []struct {
		id   string
		data map[string]interface{}
	}{
		{"test1", map[string]interface{}{"content": "migration test 1", "type": "test"}},
		{"test2", map[string]interface{}{"content": "migration test 2", "type": "test"}},
	}

	// Index documents directly to shard 2
	shard2, err := dsm.GetShardByID(2)
	if err != nil {
		t.Fatalf("Failed to get shard 2: %v", err)
	}

	for _, doc := range testDocs {
		if err := shard2.Index(doc.id, doc.data); err != nil {
			t.Fatalf("Failed to index %s: %v", doc.id, err)
		}
	}

	// Verify documents are in shard 2
	count, err := shard2.DocCount()
	if err != nil {
		t.Fatalf("Failed to get shard 2 doc count: %v", err)
	}
	t.Logf("Shard 2 has %d documents before migration", count)

	// Test direct data migration function (bypass ScaleShards to avoid lock issues)
	migratedCount, err := dsm.migrateShardData(2, 2) // Migrate shard 2 to shards 0-1
	if err != nil {
		t.Fatalf("Data migration failed: %v", err)
	}

	t.Logf("✅ Successfully migrated %d documents from shard 2", migratedCount)

	// Verify source shard is now empty
	count, err = shard2.DocCount()
	if err != nil {
		t.Fatalf("Failed to get shard 2 doc count after migration: %v", err)
	}
	if count != 0 {
		t.Errorf("Expected shard 2 to be empty, but has %d documents", count)
	} else {
		t.Logf("✅ Source shard 2 is now empty")
	}

	// Verify target shards received the documents
	var totalFound uint64
	for i := 0; i < 2; i++ {
		shard, err := dsm.GetShardByID(i)
		if err != nil {
			continue
		}
		count, _ := shard.DocCount()
		totalFound += count
		if count > 0 {
			t.Logf("Shard %d now has %d documents", i, count)
		}
	}

	if totalFound < uint64(len(testDocs)) {
		t.Errorf("Expected at least %d documents in target shards, found %d", len(testDocs), totalFound)
	} else {
		t.Logf("✅ All %d documents successfully migrated to target shards", totalFound)
	}

	// Verify documents are searchable in target shards
	foundDocs := make(map[string]bool)
	for i := 0; i < 2; i++ {
		shard, err := dsm.GetShardByID(i)
		if err != nil {
			continue
		}

		for _, testDoc := range testDocs {
			_, err := shard.Document(testDoc.id)
			if err == nil && !foundDocs[testDoc.id] {
				foundDocs[testDoc.id] = true
				t.Logf("✅ Document %s found in shard %d", testDoc.id, i)
			}
		}
	}

	if len(foundDocs) != len(testDocs) {
		t.Errorf("Expected to find %d unique documents, found %d", len(testDocs), len(foundDocs))
	} else {
		t.Logf("✅ All %d documents are searchable after migration", len(foundDocs))
	}
}
