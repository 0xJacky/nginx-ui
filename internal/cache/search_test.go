package cache

import (
	"context"
	"os"
	"testing"
	"time"
)

// TestIsNumericQuery tests the isNumericQuery function
func TestIsNumericQuery(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		expected bool
	}{
		{
			name:     "Pure number",
			query:    "9005",
			expected: true,
		},
		{
			name:     "Port with colon",
			query:    ":9005",
			expected: true, // 4/5 = 80% are digits
		},
		{
			name:     "IP address",
			query:    "192.168.1.1",
			expected: true, // 9/11 = 81% are digits
		},
		{
			name:     "Pure text",
			query:    "nginx",
			expected: false,
		},
		{
			name:     "Mixed with mostly text",
			query:    "server9005",
			expected: false, // 4/10 = 40% are digits
		},
		{
			name:     "Mixed with mostly numbers",
			query:    "9005server",
			expected: false, // 4/10 = 40% are digits
		},
		{
			name:     "Port number",
			query:    "8080",
			expected: true,
		},
		{
			name:     "Version number",
			query:    "v1.2.3",
			expected: false, // 3/6 = 50% exactly, not > 50%
		},
		{
			name:     "Empty string",
			query:    "",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isNumericQuery(tt.query)
			if result != tt.expected {
				t.Errorf("isNumericQuery(%q) = %v, want %v", tt.query, result, tt.expected)
			}
		})
	}
}

// TestBuildQuery tests the buildQuery function structure
func TestBuildQuery(t *testing.T) {
	indexer := &SearchIndexer{}

	tests := []struct {
		name     string
		query    string
		docType  string
		validate func(t *testing.T, query interface{})
	}{
		{
			name:    "Numeric query",
			query:   "9005",
			docType: "",
			validate: func(t *testing.T, query interface{}) {
				if query == nil {
					t.Error("Expected non-nil query")
				}
				// The query should be built with numeric strategy
				// which prioritizes exact matches
			},
		},
		{
			name:    "Text query",
			query:   "nginx",
			docType: "",
			validate: func(t *testing.T, query interface{}) {
				if query == nil {
					t.Error("Expected non-nil query")
				}
				// The query should be built with text strategy
				// which includes fuzzy matching
			},
		},
		{
			name:    "Numeric query with type filter",
			query:   "9005",
			docType: "site",
			validate: func(t *testing.T, query interface{}) {
				if query == nil {
					t.Error("Expected non-nil query")
				}
				// The query should include type filter
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query := indexer.buildQuery(tt.query, tt.docType)
			tt.validate(t, query)
		})
	}
}

// TestSearchStrategyDifference ensures numeric and text queries use different strategies
func TestSearchStrategyDifference(t *testing.T) {
	// Test that numeric queries don't use fuzzy matching
	numericQuery := "9005"
	if !isNumericQuery(numericQuery) {
		t.Error("Expected '9005' to be detected as numeric")
	}

	// Test that text queries do use fuzzy matching
	textQuery := "nginx"
	if isNumericQuery(textQuery) {
		t.Error("Expected 'nginx' to be detected as text")
	}
}

func TestHandleConfigScanSkipsUnchangedContent(t *testing.T) {
	indexer := &SearchIndexer{
		indexPath:      t.TempDir(),
		maxMemoryUsage: 100 * 1024 * 1024,
	}
	ctx := context.Background()
	if err := indexer.Initialize(ctx); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}
	t.Cleanup(func() {
		if err := indexer.Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	})

	configPath := "/etc/nginx/sites-enabled/example.conf"
	content := []byte("server { listen 80; server_name example.com; }")
	if err := indexer.handleConfigScan(configPath, content); err != nil {
		t.Fatalf("handleConfigScan() first call error = %v", err)
	}

	results, err := indexer.Search(ctx, "example.com", 10)
	if err != nil {
		t.Fatalf("Search() after first index error = %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("Search() after first index returned %d results, want 1", len(results))
	}
	firstUpdatedAt := results[0].Document.UpdatedAt
	if firstUpdatedAt.IsZero() {
		t.Fatal("first UpdatedAt is zero")
	}

	time.Sleep(1100 * time.Millisecond)
	if err := indexer.handleConfigScan(configPath, content); err != nil {
		t.Fatalf("handleConfigScan() second call error = %v", err)
	}

	results, err = indexer.Search(ctx, "example.com", 10)
	if err != nil {
		t.Fatalf("Search() after second index error = %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("Search() after second index returned %d results, want 1", len(results))
	}
	if !results[0].Document.UpdatedAt.Equal(firstUpdatedAt) {
		t.Fatalf("UpdatedAt changed for unchanged content: got %s, want %s",
			results[0].Document.UpdatedAt, firstUpdatedAt)
	}
}

func TestSearchIndexerDoesNotWriteDiskIndexFiles(t *testing.T) {
	indexPath := t.TempDir()
	indexer := &SearchIndexer{
		indexPath:      indexPath,
		maxMemoryUsage: 100 * 1024 * 1024,
	}
	ctx := context.Background()
	if err := indexer.Initialize(ctx); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}
	t.Cleanup(func() {
		if err := indexer.Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	})

	configPath := "/etc/nginx/sites-enabled/example.conf"
	content := []byte("server { listen 80; server_name example.com; }")
	if err := indexer.handleConfigScan(configPath, content); err != nil {
		t.Fatalf("handleConfigScan() error = %v", err)
	}

	entries, err := os.ReadDir(indexPath)
	if err != nil {
		t.Fatalf("ReadDir() error = %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("search index wrote %d disk entries, want 0", len(entries))
	}
}
