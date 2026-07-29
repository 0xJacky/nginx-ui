package indexer

import (
	"testing"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/mapping"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateLogIndexMappingUsesOnlyRequiredIndexFeatures(t *testing.T) {
	indexMapping, ok := CreateLogIndexMapping().(*mapping.IndexMappingImpl)
	require.True(t, ok)

	assert.Equal(t, "raw", indexMapping.DefaultField)
	assert.False(t, indexMapping.IndexDynamic)
	assert.False(t, indexMapping.StoreDynamic)
	assert.False(t, indexMapping.DocValuesDynamic)

	documentMapping := indexMapping.TypeMapping["_default"]
	require.NotNil(t, documentMapping)
	assert.False(t, documentMapping.Dynamic)

	tests := []struct {
		name               string
		store              bool
		index              bool
		includeTermVectors bool
		docValues          bool
	}{
		{name: "timestamp", store: true, index: true, docValues: true},
		{name: "ip", store: true, index: true, docValues: true},
		{name: "region_code", store: true, index: true, docValues: true},
		{name: "province", store: true, index: true, docValues: true},
		{name: "city", store: true, index: true},
		{name: "method", store: true, index: true, docValues: true},
		{name: "path", store: true, index: true, includeTermVectors: true},
		{name: "path_exact", index: true, docValues: true},
		{name: "protocol", store: true},
		{name: "status", store: true, index: true, docValues: true},
		{name: "bytes_sent", store: true, index: true, docValues: true},
		{name: "referer", store: true, index: true, includeTermVectors: true},
		{name: "user_agent", store: true, index: true, includeTermVectors: true, docValues: true},
		{name: "browser", store: true, index: true, docValues: true},
		{name: "browser_version", store: true, index: true},
		{name: "os", store: true, index: true, docValues: true},
		{name: "os_version", store: true, index: true},
		{name: "device_type", store: true, index: true, docValues: true},
		{name: "request_time", store: true, index: true},
		{name: "upstream_time", store: true, index: true},
		{name: "raw", store: true, index: true},
		{name: "file_path", store: true, index: true},
		{name: "main_log_path", store: true, index: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fieldMapping := requireFieldMapping(t, documentMapping, tt.name)
			assert.Equal(t, tt.store, fieldMapping.Store)
			assert.Equal(t, tt.index, fieldMapping.Index)
			assert.Equal(t, tt.includeTermVectors, fieldMapping.IncludeTermVectors)
			assert.False(t, fieldMapping.IncludeInAll)
			assert.Equal(t, tt.docValues, fieldMapping.DocValues)
		})
	}
}

func TestCreateLogIndexMappingPreservesSearchBehavior(t *testing.T) {
	index, err := bleve.NewMemOnly(CreateLogIndexMapping())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, index.Close()) })

	documents := map[string]map[string]interface{}{
		"older": {
			"timestamp": int64(100), "ip": "192.0.2.1", "region_code": "US", "province": "California", "city": "Los Angeles",
			"method": "GET", "path": "/api/v1/orders", "path_exact": "/api/v1/orders", "protocol": "HTTP/2.0",
			"status": 200, "bytes_sent": int64(1024), "referer": "campaign source", "user_agent": "Mozilla Firefox",
			"browser": "Firefox", "browser_version": "128", "os": "Linux", "os_version": "6.8", "device_type": "Desktop",
			"request_time": 0.1, "upstream_time": 0.05, "file_path": "/var/log/nginx/access.log", "main_log_path": "/var/log/nginx/access.log",
			"raw": `192.0.2.1 - - [date] "GET /api/v1/orders HTTP/2.0" compressionprobe`,
		},
		"newer": {
			"timestamp": int64(200), "ip": "192.0.2.2", "method": "POST", "path": "/login", "path_exact": "/login",
			"protocol": "HTTP/1.1", "status": 201, "bytes_sent": int64(512), "browser": "Chrome", "os": "macOS", "device_type": "Desktop",
			"file_path": "/var/log/nginx/access.log", "main_log_path": "/var/log/nginx/access.log", "raw": "routine request",
		},
	}
	for id, document := range documents {
		require.NoError(t, index.Index(id, document))
	}

	t.Run("default full text search", func(t *testing.T) {
		result, err := index.Search(bleve.NewSearchRequest(bleve.NewMatchQuery("compressionprobe")))
		require.NoError(t, err)
		require.Len(t, result.Hits, 1)
		assert.Equal(t, "older", result.Hits[0].ID)
	})

	for _, tt := range []struct {
		field  string
		phrase string
	}{
		{field: "path", phrase: "/api/v1/orders"},
		{field: "referer", phrase: "campaign source"},
		{field: "user_agent", phrase: "Mozilla Firefox"},
	} {
		t.Run("phrase query "+tt.field, func(t *testing.T) {
			query := bleve.NewMatchPhraseQuery(tt.phrase)
			query.SetField(tt.field)
			result, err := index.Search(bleve.NewSearchRequest(query))
			require.NoError(t, err)
			require.Len(t, result.Hits, 1)
			assert.Equal(t, "older", result.Hits[0].ID)
		})
	}

	t.Run("stored fields", func(t *testing.T) {
		request := bleve.NewSearchRequest(bleve.NewDocIDQuery([]string{"older"}))
		request.Fields = []string{"*"}
		result, err := index.Search(request)
		require.NoError(t, err)
		require.Len(t, result.Hits, 1)
		assert.Equal(t, "HTTP/2.0", result.Hits[0].Fields["protocol"])
		assert.Contains(t, result.Hits[0].Fields["raw"], "compressionprobe")
	})

	t.Run("sorting and faceting", func(t *testing.T) {
		request := bleve.NewSearchRequest(bleve.NewMatchAllQuery())
		request.Size = 10
		request.SortBy([]string{"-timestamp"})
		request.AddFacet("path_exact", bleve.NewFacetRequest("path_exact", 10))
		request.AddFacet("path", bleve.NewFacetRequest("path", 10))
		request.AddFacet("user_agent", bleve.NewFacetRequest("user_agent", 10))
		result, err := index.Search(request)
		require.NoError(t, err)
		require.Len(t, result.Hits, 2)
		assert.Equal(t, "newer", result.Hits[0].ID)
		require.NotNil(t, result.Facets["path_exact"])
		assert.Len(t, result.Facets["path_exact"].Terms.Terms(), 2)
		require.NotNil(t, result.Facets["path"])
		assert.NotEmpty(t, result.Facets["path"].Terms.Terms())
		require.NotNil(t, result.Facets["user_agent"])
		assert.NotEmpty(t, result.Facets["user_agent"].Terms.Terms())
	})
}

func requireFieldMapping(t *testing.T, documentMapping *mapping.DocumentMapping, name string) *mapping.FieldMapping {
	t.Helper()

	propertyMapping := documentMapping.Properties[name]
	require.NotNil(t, propertyMapping, "field %q should be explicitly mapped", name)
	require.Len(t, propertyMapping.Fields, 1, "field %q should have exactly one mapping", name)
	return propertyMapping.Fields[0]
}
