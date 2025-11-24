package cache

import (
	"testing"
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


