package searcher

import (
	"context"
	"testing"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/nginx_log/indexer"
	"github.com/blevesearch/bleve/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// filterTestDoc describes a single log document used by the filter tests.
type filterTestDoc struct {
	id     string
	fields map[string]interface{}
}

// newFilterTestSearcher builds an in-memory Bleve index using the production
// log index mapping, indexes a fixed set of documents, and returns a Searcher
// over that index. This lets the filter tests exercise the real query/index
// behavior instead of only checking that a query object is non-nil.
func newFilterTestSearcher(t *testing.T) *Searcher {
	t.Helper()

	index, err := bleve.NewMemOnly(indexer.CreateLogIndexMapping())
	require.NoError(t, err)

	baseTime := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC).Unix()
	docs := []filterTestDoc{
		{
			id: "doc1",
			fields: map[string]interface{}{
				"timestamp": baseTime, "ip": "192.168.1.1", "method": "GET", "status": 200,
				"path": "/api/products", "path_exact": "/api/products", "bytes_sent": 1024,
				"browser": "Chrome", "os": "Windows", "device_type": "Desktop",
				"referer": "examplereferer", "user_agent": "chromeua",
				"file_path": "/var/log/nginx/access.log", "main_log_path": "/var/log/nginx/access.log",
				"raw": "doc1",
			},
		},
		{
			id: "doc2",
			fields: map[string]interface{}{
				"timestamp": baseTime, "ip": "192.168.1.2", "method": "POST", "status": 200,
				"path": "/api/login", "path_exact": "/api/login", "bytes_sent": 2048,
				"browser": "Firefox", "os": "Ubuntu", "device_type": "Desktop",
				"referer": "otherreferer", "user_agent": "firefoxua",
				"file_path": "/var/log/nginx/access.log", "main_log_path": "/var/log/nginx/access.log",
				"raw": "doc2",
			},
		},
		{
			id: "doc3",
			fields: map[string]interface{}{
				"timestamp": baseTime, "ip": "10.0.0.1", "method": "GET", "status": 404,
				"path": "/missing/page", "path_exact": "/missing/page", "bytes_sent": 512,
				"browser": "Chrome", "os": "Android", "device_type": "Mobile",
				"referer": "otherreferer", "user_agent": "chromeua",
				"file_path": "/var/log/nginx/access.log", "main_log_path": "/var/log/nginx/access.log",
				"raw": "doc3",
			},
		},
		{
			id: "doc4",
			fields: map[string]interface{}{
				"timestamp": baseTime, "ip": "10.0.0.2", "method": "DELETE", "status": 500,
				"path": "/error", "path_exact": "/error", "bytes_sent": 256,
				"browser": "Safari", "os": "iOS", "device_type": "Mobile",
				"referer": "otherreferer", "user_agent": "safariua",
				"file_path": "/var/log/nginx/access.log", "main_log_path": "/var/log/nginx/access.log",
				"raw": "doc4",
			},
		},
	}

	for _, doc := range docs {
		require.NoError(t, index.Index(doc.id, doc.fields))
	}
	// Searcher.Stop does not close the underlying shards, so close the
	// in-memory index here to avoid leaking it across tests.
	t.Cleanup(func() { _ = index.Close() })

	config := DefaultSearcherConfig()
	config.EnableCache = false
	return NewSearcher(config, []bleve.Index{index})
}

// TestSearcherFieldFilters verifies that every advanced-search filter narrows
// the result set to the matching documents. It is a regression test for the
// log filter returning no entries (GitHub issue #1669).
func TestSearcherFieldFilters(t *testing.T) {
	s := newFilterTestSearcher(t)
	defer func() { _ = s.Stop() }()

	tests := []struct {
		name    string
		req     *SearchRequest
		wantIDs []string
	}{
		{"status code 200", &SearchRequest{StatusCodes: []int{200}}, []string{"doc1", "doc2"}},
		{"status code 404", &SearchRequest{StatusCodes: []int{404}}, []string{"doc3"}},
		{"multiple status codes", &SearchRequest{StatusCodes: []int{200, 500}}, []string{"doc1", "doc2", "doc4"}},
		{"ip address", &SearchRequest{IPAddresses: []string{"192.168.1.1"}}, []string{"doc1"}},
		{"http method", &SearchRequest{Methods: []string{"GET"}}, []string{"doc1", "doc3"}},
		{"browser", &SearchRequest{Browsers: []string{"Chrome"}}, []string{"doc1", "doc3"}},
		{"operating system", &SearchRequest{OSs: []string{"Android"}}, []string{"doc3"}},
		{"device type", &SearchRequest{Devices: []string{"Mobile"}}, []string{"doc3", "doc4"}},
		// doc1 path "/api/products" shares the "api" token with "/api/login",
		// so this also verifies the path filter matches the token sequence
		// rather than loosely matching any single shared token.
		{"request path", &SearchRequest{Paths: []string{"/api/login"}}, []string{"doc2"}},
		{"referer", &SearchRequest{Referers: []string{"examplereferer"}}, []string{"doc1"}},
		{"user agent", &SearchRequest{UserAgents: []string{"chromeua"}}, []string{"doc1", "doc3"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.req.Limit = 100
			result, err := s.Search(context.Background(), tt.req)
			require.NoError(t, err)

			gotIDs := make([]string, 0, len(result.Hits))
			for _, hit := range result.Hits {
				gotIDs = append(gotIDs, hit.ID)
			}
			assert.ElementsMatch(t, tt.wantIDs, gotIDs,
				"filter %q should return only the matching documents", tt.name)
		})
	}
}

// TestSearcherStatusFacet verifies that faceting on the numeric status field
// produces correct per-code buckets. A plain terms facet cannot bucket a
// numeric field, so the searcher must facet it with numeric ranges.
func TestSearcherStatusFacet(t *testing.T) {
	s := newFilterTestSearcher(t)
	defer func() { _ = s.Stop() }()

	result, err := s.Search(context.Background(), &SearchRequest{
		Limit:         100,
		IncludeFacets: true,
		FacetFields:   []string{"status"},
	})
	require.NoError(t, err)

	statusFacet, ok := result.Facets["status"]
	require.True(t, ok, "status facet should be present in the result")

	got := make(map[string]int)
	for _, term := range statusFacet.Terms {
		got[term.Term] = term.Count
	}
	assert.Equal(t, map[string]int{"200": 2, "404": 1, "500": 1}, got)
}
