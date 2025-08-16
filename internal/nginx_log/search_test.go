package nginx_log

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dgraph-io/ristretto/v2"
)

func TestLogIndexer_SearchFunctionality(t *testing.T) {
	// Create temporary directory for test index
	tempDir, err := os.MkdirTemp("", "nginx_log_search_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create test log file
	logFile := filepath.Join(tempDir, "access.log")
	logContent := `192.168.1.1 - - [10/Oct/2023:13:55:36 +0000] "GET /api/test HTTP/1.1" 200 1234 "-" "Mozilla/5.0 (iPhone; CPU iPhone OS 16_0 like Mac OS X) AppleWebKit/605.1.15"
192.168.1.2 - - [10/Oct/2023:13:56:36 +0000] "POST /api/login HTTP/1.1" 401 567 "-" "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"
192.168.1.3 - - [10/Oct/2023:13:57:36 +0000] "GET /api/data HTTP/1.1" 500 890 "-" "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36"`

	err = os.WriteFile(logFile, []byte(logContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write test log file: %v", err)
	}

	// Create indexer with custom index path for testing
	// We need to create it manually since NewLogIndexer uses config directory
	indexPath := filepath.Join(tempDir, "index")
	index, err := createOrOpenIndex(indexPath)
	if err != nil {
		t.Fatalf("Failed to create index: %v", err)
	}

	uaParser := NewSimpleUserAgentParser()
	parser := NewLogParser(uaParser)

	// Initialize cache
	cache, err := ristretto.NewCache(&ristretto.Config[string, *CachedSearchResult]{
		NumCounters: 1e7,     // number of keys to track frequency of (10M)
		MaxCost:     1 << 27, // maximum cost of cache (128MB)
		BufferItems: 64,      // number of keys per Get buffer
	})
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}

	indexer := &LogIndexer{
		index:      index,
		indexPath:  indexPath,
		parser:     parser,
		logPaths:   make(map[string]*LogFileInfo),
		indexBatch: 10000,
		cache:      cache,
		// Note: We skip watcher initialization for testing
	}
	defer indexer.Close()

	// Add log path and index
	err = indexer.AddLogPath(logFile)
	if err != nil {
		t.Fatalf("Failed to add log path: %v", err)
	}

	err = indexer.IndexLogFile(logFile)
	if err != nil {
		t.Fatalf("Failed to index log file: %v", err)
	}

	// Wait a bit for indexing to complete
	time.Sleep(100 * time.Millisecond)

	// Test 1: Search all entries
	t.Run("Search all entries", func(t *testing.T) {
		req := &QueryRequest{
			Limit: 10,
		}

		result, err := indexer.SearchLogs(context.Background(), req)
		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}

		if len(result.Entries) != 3 {
			t.Errorf("Expected 3 entries, got %d", len(result.Entries))
		}
	})

	// Test 2: Search by IP
	t.Run("Search by IP", func(t *testing.T) {
		req := &QueryRequest{
			IP:    "192.168.1.1",
			Limit: 10,
		}

		result, err := indexer.SearchLogs(context.Background(), req)
		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}

		if len(result.Entries) != 1 {
			t.Errorf("Expected 1 entry for IP search, got %d", len(result.Entries))
		}

		if len(result.Entries) > 0 && result.Entries[0].IP != "192.168.1.1" {
			t.Errorf("Expected IP 192.168.1.1, got %s", result.Entries[0].IP)
		}
	})

	// Test 3: Search by method
	t.Run("Search by method", func(t *testing.T) {
		req := &QueryRequest{
			Method: "POST",
			Limit:  10,
		}

		result, err := indexer.SearchLogs(context.Background(), req)
		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}

		if len(result.Entries) != 1 {
			t.Errorf("Expected 1 entry for POST method, got %d", len(result.Entries))
		}

		if len(result.Entries) > 0 && result.Entries[0].Method != "POST" {
			t.Errorf("Expected method POST, got %s", result.Entries[0].Method)
		}
	})

	// Test 4: Search by status
	t.Run("Search by status", func(t *testing.T) {
		req := &QueryRequest{
			Status: []int{200},
			Limit:  10,
		}

		result, err := indexer.SearchLogs(context.Background(), req)
		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}

		if len(result.Entries) != 1 {
			t.Errorf("Expected 1 entry for status 200, got %d", len(result.Entries))
		}

		if len(result.Entries) > 0 && result.Entries[0].Status != 200 {
			t.Errorf("Expected status 200, got %d", result.Entries[0].Status)
		}
	})

	// Test 5: Search by path
	t.Run("Search by path", func(t *testing.T) {
		req := &QueryRequest{
			Path:  "/api/test",
			Limit: 10,
		}

		result, err := indexer.SearchLogs(context.Background(), req)
		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}

		if len(result.Entries) != 1 {
			t.Errorf("Expected 1 entry for path /api/test, got %d", len(result.Entries))
		}

		if len(result.Entries) > 0 && result.Entries[0].Path != "/api/test" {
			t.Errorf("Expected path /api/test, got %s", result.Entries[0].Path)
		}
	})

	// Test 6: Complex search with multiple criteria
	t.Run("Complex search", func(t *testing.T) {
		req := &QueryRequest{
			Method: "GET",
			Status: []int{200, 500},
			Limit:  10,
		}

		result, err := indexer.SearchLogs(context.Background(), req)
		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}

		// Should find 2 entries: one with status 200 and one with status 500, both GET
		if len(result.Entries) != 2 {
			t.Errorf("Expected 2 entries for complex search, got %d", len(result.Entries))
		}

		for _, entry := range result.Entries {
			if entry.Method != "GET" {
				t.Errorf("Expected method GET, got %s", entry.Method)
			}
			if entry.Status != 200 && entry.Status != 500 {
				t.Errorf("Expected status 200 or 500, got %d", entry.Status)
			}
		}
	})
}

func TestLogIndexer_GetIndexStatus(t *testing.T) {
	// Create temporary directory for test index
	tempDir, err := os.MkdirTemp("", "nginx_log_status_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create indexer with custom index path for testing
	indexPath := filepath.Join(tempDir, "index")
	index, err := createOrOpenIndex(indexPath)
	if err != nil {
		t.Fatalf("Failed to create index: %v", err)
	}

	uaParser := NewSimpleUserAgentParser()
	parser := NewLogParser(uaParser)

	// Initialize cache
	cache, err := ristretto.NewCache(&ristretto.Config[string, *CachedSearchResult]{
		NumCounters: 1e7,     // number of keys to track frequency of (10M)
		MaxCost:     1 << 27, // maximum cost of cache (128MB)
		BufferItems: 64,      // number of keys per Get buffer
	})
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}

	indexer := &LogIndexer{
		index:      index,
		indexPath:  indexPath,
		parser:     parser,
		logPaths:   make(map[string]*LogFileInfo),
		indexBatch: 10000,
		cache:      cache,
		// Note: We skip watcher initialization for testing
	}
	defer indexer.Close()

	// Get status for empty index
	status, err := indexer.GetIndexStatus()
	if err != nil {
		t.Fatalf("Failed to get index status: %v", err)
	}

	if status.DocumentCount != 0 {
		t.Errorf("Expected document count 0, got %v", status.DocumentCount)
	}

	if status.LogPathsCount != 0 {
		t.Errorf("Expected log paths count 0, got %v", status.LogPathsCount)
	}

	if status.TotalFiles != 0 {
		t.Errorf("Expected total files 0, got %v", status.TotalFiles)
	}

	if len(status.Files) != 0 {
		t.Errorf("Expected empty files array, got %d files", len(status.Files))
	}
}
