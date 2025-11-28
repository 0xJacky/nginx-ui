package indexer

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/0xJacky/Nginx-UI/settings"
)

// TestIntegrationValidation validates the complete integration of optimizations
func TestIntegrationValidation(t *testing.T) {
	t.Log("=== Validating Optimized Indexer Integration ===")

	// Create test log content
	testLogContent := `127.0.0.1 - - [25/Dec/2023:10:00:00 +0000] "GET /api/test HTTP/1.1" 200 1234 "-" "test-agent"
127.0.0.2 - - [25/Dec/2023:10:01:00 +0000] "POST /api/data HTTP/1.1" 201 5678 "http://example.com" "another-agent"
127.0.0.3 - - [25/Dec/2023:10:02:00 +0000] "PUT /api/update HTTP/1.1" 204 0 "-" "update-agent"`

	// Create temporary test file
	tmpFile, err := os.CreateTemp("", "test_nginx_*.log")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	// Allow tests to operate on the temporary log path by whitelisting its directory.
	settings.NginxSettings.LogDirWhiteList = []string{filepath.Dir(tmpFile.Name())}

	if _, err := tmpFile.WriteString(testLogContent); err != nil {
		t.Fatalf("Failed to write test content: %v", err)
	}
	tmpFile.Close()

	// Test 1: Validate optimized parsing
	t.Log("Testing optimized parsing...")
	ctx := context.Background()

	file, err := os.Open(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to open test file: %v", err)
	}
	defer file.Close()

	logDocs, err := ParseLogStream(ctx, file, tmpFile.Name())
	if err != nil {
		t.Fatalf("Optimized parsing failed: %v", err)
	}

	if len(logDocs) != 3 {
		t.Errorf("Expected 3 parsed documents, got %d", len(logDocs))
	}

	// Test 2: Validate single line parsing with SIMD optimization
	t.Log("Testing SIMD-optimized single line parsing...")
	testLine := `127.0.0.1 - - [25/Dec/2023:10:00:00 +0000] "GET /api/test HTTP/1.1" 200 1234 "-" "test-agent"`

	logDoc, err := ParseLogLine(testLine)
	if err != nil {
		t.Fatalf("SIMD-optimized parsing failed: %v", err)
	}

	if logDoc.IP != "127.0.0.1" {
		t.Errorf("Expected IP 127.0.0.1, got %s", logDoc.IP)
	}
	if logDoc.Method != "GET" {
		t.Errorf("Expected method GET, got %s", logDoc.Method)
	}
	if logDoc.Path != "/api/test" {
		t.Errorf("Expected path /api/test, got %s", logDoc.Path)
	}

	// Test 3: Validate optimized indexer with ProgressTracker
	t.Log("Testing optimized indexer with ProgressTracker...")

	config := DefaultIndexerConfig()
	shardManager := NewGroupedShardManager(config)
	indexer := NewParallelIndexer(config, shardManager)

	// Start the indexer
	err = indexer.Start(context.Background())
	if err != nil {
		t.Fatalf("Failed to start indexer: %v", err)
	}
	defer indexer.Stop()

	// Create progress tracker
	progressConfig := &ProgressConfig{
		NotifyInterval: time.Millisecond * 100,
		OnProgress: func(notification ProgressNotification) {
			t.Logf("Progress: %+v", notification)
		},
		OnCompletion: func(notification CompletionNotification) {
			t.Logf("Completion: %+v", notification)
		},
	}
	progressTracker := NewProgressTracker("test-group", progressConfig)

	// Test optimized indexing with progress tracking
	docCount, minTime, maxTime, err := indexer.IndexSingleFileWithProgress(tmpFile.Name(), progressTracker)
	if err != nil {
		t.Fatalf("IndexSingleFileWithProgress failed: %v", err)
	}

	if docCount != 3 {
		t.Errorf("Expected 3 indexed documents, got %d", docCount)
	}

	if minTime == nil || maxTime == nil {
		t.Errorf("Expected time ranges to be calculated, got minTime=%v, maxTime=%v", minTime, maxTime)
	}

	if minTime != nil && maxTime != nil {
		if minTime.After(*maxTime) {
			t.Errorf("minTime (%v) should be before maxTime (%v)", minTime, maxTime)
		}
		t.Logf("Calculated time range: %v to %v", minTime, maxTime)
	}

	// Test 4: Validate incremental indexing method
	t.Log("Testing IndexSingleFileIncrementally...")

	docsCountMap, minTime2, maxTime2, err := indexer.IndexSingleFileIncrementally(tmpFile.Name(), progressConfig)
	if err != nil {
		t.Fatalf("IndexSingleFileIncrementally failed: %v", err)
	}

	if len(docsCountMap) != 1 {
		t.Errorf("Expected 1 entry in docsCountMap, got %d", len(docsCountMap))
	}

	if docsCount, exists := docsCountMap[tmpFile.Name()]; !exists || docsCount != 3 {
		t.Errorf("Expected 3 documents for file %s, got %d (exists: %v)", tmpFile.Name(), docsCount, exists)
	}

	if minTime2 == nil || maxTime2 == nil {
		t.Errorf("Expected time ranges from incremental indexing, got minTime=%v, maxTime=%v", minTime2, maxTime2)
	}

	// Test 5: Validate optimization status
	t.Log("Testing optimization status...")

	status := GetOptimizationStatus()
	expectedKeys := []string{"parser_optimized", "simd_enabled", "memory_pools_enabled", "batch_processing"}
	for _, key := range expectedKeys {
		if _, exists := status[key]; !exists {
			t.Errorf("Expected optimization status key %s to exist", key)
		}
	}

	t.Logf("Optimization status: %+v", status)

	// Test 6: Validate production configuration
	t.Log("Testing production configuration...")

	// Test the current indexer's configuration (which should have production optimizations)
	currentConfig := indexer.GetConfig()
	if currentConfig.WorkerCount <= 0 {
		t.Errorf("Expected positive WorkerCount in config, got %d", currentConfig.WorkerCount)
	}
	if currentConfig.BatchSize <= 0 {
		t.Errorf("Expected positive BatchSize in config, got %d", currentConfig.BatchSize)
	}

	t.Log("=== All Optimized Indexer Integration Tests Passed ===")
}

// TestOptimizationCompatibility ensures backward compatibility
func TestOptimizationCompatibility(t *testing.T) {
	t.Log("=== Testing Optimization Backward Compatibility ===")

	// Create test log content
	testLogContent := `127.0.0.1 - - [25/Dec/2023:10:00:00 +0000] "GET /test HTTP/1.1" 200 1234`

	tmpFile, err := os.CreateTemp("", "compat_test_*.log")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	settings.NginxSettings.LogDirWhiteList = []string{filepath.Dir(tmpFile.Name())}

	if _, err := tmpFile.WriteString(testLogContent); err != nil {
		t.Fatalf("Failed to write test content: %v", err)
	}
	tmpFile.Close()

	config := DefaultIndexerConfig()
	shardManager := NewGroupedShardManager(config)
	indexer := NewParallelIndexer(config, shardManager)

	err = indexer.Start(context.Background())
	if err != nil {
		t.Fatalf("Failed to start indexer: %v", err)
	}
	defer indexer.Stop()

	// Test that original methods still work (they should delegate to optimized versions)
	t.Log("Testing IndexLogFile (should use optimized implementation)...")
	err = indexer.IndexLogFile(tmpFile.Name())
	if err != nil {
		t.Fatalf("IndexLogFile failed: %v", err)
	}

	t.Log("Testing indexSingleFile (should use optimized implementation)...")
	docCount, minTime, maxTime, err := indexer.indexSingleFile(tmpFile.Name())
	if err != nil {
		t.Fatalf("indexSingleFile failed: %v", err)
	}

	if docCount != 1 {
		t.Errorf("Expected 1 document, got %d", docCount)
	}

	if minTime == nil || maxTime == nil {
		t.Errorf("Expected time ranges, got minTime=%v, maxTime=%v", minTime, maxTime)
	}

	t.Log("=== Backward Compatibility Tests Passed ===")
}
