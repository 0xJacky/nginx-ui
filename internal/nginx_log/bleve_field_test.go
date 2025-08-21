package nginx_log

import (
	"testing"
	"time"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/mapping"
)

// TestBleveFieldMapping tests different field mapping approaches for file_path
func TestBleveFieldMapping(t *testing.T) {
	// Create a temporary index in memory
	index, err := bleve.NewMemOnly(createTestIndexMapping())
	if err != nil {
		t.Fatalf("Failed to create test index: %v", err)
	}
	defer index.Close()

	// Create test data similar to what we have
	testEntry := &IndexedLogEntry{
		ID:        "test_1",
		FilePath:  "/var/log/nginx/access.log",
		Timestamp: time.Now().Unix(),
		IP:        "135.220.172.38",
		Method:    "GET",
		Path:      "/test",
		Status:    200,
	}

	// Index the test entry
	err = index.Index(testEntry.ID, testEntry)
	if err != nil {
		t.Fatalf("Failed to index test entry: %v", err)
	}

	// Test 1: MatchAllQuery should work
	t.Run("MatchAllQuery", func(t *testing.T) {
		query := bleve.NewMatchAllQuery()
		searchReq := bleve.NewSearchRequest(query)
		searchReq.Size = 10
		searchReq.Fields = []string{"file_path", "ip"}

		result, err := index.Search(searchReq)
		if err != nil {
			t.Errorf("MatchAllQuery failed: %v", err)
			return
		}

		t.Logf("MatchAllQuery returned %d hits", result.Total)
		if result.Total == 0 {
			t.Error("MatchAllQuery should return at least 1 hit")
		}

		for i, hit := range result.Hits {
			t.Logf("Hit %d: ID=%s, Fields=%+v", i, hit.ID, hit.Fields)
		}
	})

	// Test 2: IP field query (should work based on logs)
	t.Run("IPFieldQuery", func(t *testing.T) {
		query := bleve.NewMatchQuery("135.220.172.38")
		query.SetField("ip")
		searchReq := bleve.NewSearchRequest(query)
		searchReq.Size = 10

		result, err := index.Search(searchReq)
		if err != nil {
			t.Errorf("IP field query failed: %v", err)
			return
		}

		t.Logf("IP field query returned %d hits", result.Total)
		if result.Total == 0 {
			t.Error("IP field query should return at least 1 hit")
		}
	})

	// Test 3: file_path field with TermQuery
	t.Run("FilePathTermQuery", func(t *testing.T) {
		query := bleve.NewTermQuery("/var/log/nginx/access.log")
		query.SetField("file_path")
		searchReq := bleve.NewSearchRequest(query)
		searchReq.Size = 10

		result, err := index.Search(searchReq)
		if err != nil {
			t.Errorf("file_path TermQuery failed: %v", err)
			return
		}

		t.Logf("file_path TermQuery returned %d hits", result.Total)
		if result.Total == 0 {
			t.Error("file_path TermQuery should return at least 1 hit")
		}
	})

	// Test 4: file_path field with MatchQuery
	t.Run("FilePathMatchQuery", func(t *testing.T) {
		query := bleve.NewMatchQuery("/var/log/nginx/access.log")
		query.SetField("file_path")
		searchReq := bleve.NewSearchRequest(query)
		searchReq.Size = 10

		result, err := index.Search(searchReq)
		if err != nil {
			t.Errorf("file_path MatchQuery failed: %v", err)
			return
		}

		t.Logf("file_path MatchQuery returned %d hits", result.Total)
		if result.Total == 0 {
			t.Error("file_path MatchQuery should return at least 1 hit")
		}
	})

	// Test 5: Different file_path mapping approaches
	t.Run("AlternativeFilepathMapping", func(t *testing.T) {
		// Create index with TextFieldMapping for file_path instead of KeywordFieldMapping
		altMapping := createAlternativeIndexMapping()
		altIndex, err := bleve.NewMemOnly(altMapping)
		if err != nil {
			t.Fatalf("Failed to create alternative index: %v", err)
		}
		defer altIndex.Close()

		// Index the same data
		err = altIndex.Index(testEntry.ID, testEntry)
		if err != nil {
			t.Fatalf("Failed to index test entry in alternative index: %v", err)
		}

		// Test with MatchQuery
		query := bleve.NewMatchQuery("/var/log/nginx/access.log")
		query.SetField("file_path")
		searchReq := bleve.NewSearchRequest(query)
		searchReq.Size = 10
		searchReq.Fields = []string{"file_path", "ip"}

		result, err := altIndex.Search(searchReq)
		if err != nil {
			t.Errorf("Alternative file_path MatchQuery failed: %v", err)
			return
		}

		t.Logf("Alternative file_path MatchQuery returned %d hits", result.Total)
		for i, hit := range result.Hits {
			t.Logf("Alt Hit %d: ID=%s, Fields=%+v", i, hit.ID, hit.Fields)
		}
	})

	// Test 6: PhraseQuery approach
	t.Run("FilePathPhraseQuery", func(t *testing.T) {
		query := bleve.NewPhraseQuery([]string{"/var/log/nginx/access.log"}, "file_path")
		searchReq := bleve.NewSearchRequest(query)
		searchReq.Size = 10

		result, err := index.Search(searchReq)
		if err != nil {
			t.Errorf("file_path PhraseQuery failed: %v", err)
			return
		}

		t.Logf("file_path PhraseQuery returned %d hits", result.Total)
	})

	// Test 7: No field specification (search all fields)
	t.Run("NoFieldSpecification", func(t *testing.T) {
		query := bleve.NewMatchQuery("/var/log/nginx/access.log")
		// Don't set field - search all fields
		searchReq := bleve.NewSearchRequest(query)
		searchReq.Size = 10
		searchReq.Fields = []string{"file_path", "ip"}

		result, err := index.Search(searchReq)
		if err != nil {
			t.Errorf("No field specification query failed: %v", err)
			return
		}

		t.Logf("No field specification query returned %d hits", result.Total)
		for i, hit := range result.Hits {
			t.Logf("NoField Hit %d: ID=%s, Fields=%+v", i, hit.ID, hit.Fields)
		}
	})
}

// createTestIndexMapping creates the same index mapping as the main code
func createTestIndexMapping() mapping.IndexMapping {
	logMapping := bleve.NewDocumentMapping()

	// Timestamp
	timestampMapping := bleve.NewNumericFieldMapping()
	logMapping.AddFieldMappingsAt("timestamp", timestampMapping)

	// File path with TextFieldMapping + keyword analyzer (current approach)
	filePathMapping := bleve.NewTextFieldMapping()
	filePathMapping.Store = true
	filePathMapping.Index = true
	filePathMapping.Analyzer = "keyword"  // Use keyword analyzer for exact matching
	logMapping.AddFieldMappingsAt("file_path", filePathMapping)

	// Other text fields
	textMapping := bleve.NewTextFieldMapping()
	textMapping.Store = true
	textMapping.Index = true
	logMapping.AddFieldMappingsAt("ip", textMapping)
	logMapping.AddFieldMappingsAt("method", textMapping)
	logMapping.AddFieldMappingsAt("path", textMapping)

	// Numeric fields
	numericMapping := bleve.NewNumericFieldMapping()
	numericMapping.Store = true
	numericMapping.Index = true
	logMapping.AddFieldMappingsAt("status", numericMapping)

	// Create index mapping
	indexMapping := bleve.NewIndexMapping()
	// Use the default mapping instead of creating a separate document type
	indexMapping.DefaultMapping = logMapping

	return indexMapping
}

// createAlternativeIndexMapping uses TextFieldMapping for file_path
func createAlternativeIndexMapping() mapping.IndexMapping {
	logMapping := bleve.NewDocumentMapping()

	// Timestamp
	timestampMapping := bleve.NewNumericFieldMapping()
	logMapping.AddFieldMappingsAt("timestamp", timestampMapping)

	// File path with TextFieldMapping instead of KeywordFieldMapping
	filePathMapping := bleve.NewTextFieldMapping()
	filePathMapping.Store = true
	filePathMapping.Index = true
	// Use keyword analyzer for exact matching
	filePathMapping.Analyzer = "keyword"
	logMapping.AddFieldMappingsAt("file_path", filePathMapping)

	// Other text fields
	textMapping := bleve.NewTextFieldMapping()
	textMapping.Store = true
	textMapping.Index = true
	logMapping.AddFieldMappingsAt("ip", textMapping)
	logMapping.AddFieldMappingsAt("method", textMapping)
	logMapping.AddFieldMappingsAt("path", textMapping)

	// Numeric fields
	numericMapping := bleve.NewNumericFieldMapping()
	numericMapping.Store = true
	numericMapping.Index = true
	logMapping.AddFieldMappingsAt("status", numericMapping)

	// Create index mapping
	indexMapping := bleve.NewIndexMapping()
	// Use the default mapping instead of creating a separate document type
	indexMapping.DefaultMapping = logMapping

	return indexMapping
}