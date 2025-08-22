package analytics

import (
	"context"
	"testing"

	"github.com/0xJacky/Nginx-UI/internal/nginx_log/searcher"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestService_GetLogEntriesStats_Success(t *testing.T) {
	mockSearcher := &MockSearcher{}
	s := NewService(mockSearcher)

	ctx := context.Background()
	req := &searcher.SearchRequest{
		Limit:  100,
		Offset: 0,
	}

	expectedResult := &searcher.SearchResult{
		TotalHits: 5,
		Facets: map[string]*searcher.Facet{
			"status": {
				Terms: []*searcher.FacetTerm{
					{Term: "200", Count: 3},
					{Term: "404", Count: 1},
					{Term: "500", Count: 1},
				},
			},
			"method": {
				Terms: []*searcher.FacetTerm{
					{Term: "GET", Count: 4},
					{Term: "POST", Count: 1},
				},
			},
			"path": {
				Terms: []*searcher.FacetTerm{
					{Term: "/a", Count: 3},
					{Term: "/b", Count: 1},
					{Term: "/c", Count: 1},
				},
			},
			"ip": {
				Terms: []*searcher.FacetTerm{
					{Term: "1.1.1.1", Count: 3},
					{Term: "1.1.1.2", Count: 1},
					{Term: "1.1.1.3", Count: 1},
				},
			},
			"user_agent": {
				Terms: []*searcher.FacetTerm{
					{Term: "Chrome", Count: 3},
					{Term: "Firefox", Count: 1},
					{Term: "Curl", Count: 1},
				},
			},
		},
	}

	mockSearcher.On("Search", ctx, mock.AnythingOfType("*searcher.SearchRequest")).Return(expectedResult, nil)

	result, err := s.GetLogEntriesStats(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, result)

	// Check total entries
	assert.Equal(t, int64(5), result.TotalEntries)

	// Check status code distribution
	assert.Equal(t, 3, result.StatusCodeDist["200"])
	assert.Equal(t, 1, result.StatusCodeDist["404"])
	assert.Equal(t, 1, result.StatusCodeDist["500"])

	// Check method distribution
	assert.Equal(t, 4, result.MethodDist["GET"])
	assert.Equal(t, 1, result.MethodDist["POST"])

	// Check top paths
	assert.Len(t, result.TopPaths, 3)
	assert.Equal(t, "/a", result.TopPaths[0].Key)
	assert.Equal(t, 3, result.TopPaths[0].Value)

	// Check top IPs
	assert.Len(t, result.TopIPs, 3)
	assert.Equal(t, "1.1.1.1", result.TopIPs[0].Key)
	assert.Equal(t, 3, result.TopIPs[0].Value)

	// Check top user agents
	assert.Len(t, result.TopUserAgents, 3)
	assert.Equal(t, "Chrome", result.TopUserAgents[0].Key)
	assert.Equal(t, 3, result.TopUserAgents[0].Value)

	mockSearcher.AssertExpectations(t)
}

func TestService_GetLogEntriesStats_NilRequest(t *testing.T) {
	mockSearcher := &MockSearcher{}
	s := NewService(mockSearcher)

	ctx := context.Background()
	result, err := s.GetLogEntriesStats(ctx, nil)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "request cannot be nil")

	mockSearcher.AssertExpectations(t)
}

func TestService_GetLogEntriesStats_SearchError(t *testing.T) {
	mockSearcher := &MockSearcher{}
	s := NewService(mockSearcher)

	ctx := context.Background()
	req := &searcher.SearchRequest{
		Limit:  100,
		Offset: 0,
	}

	mockSearcher.On("Search", ctx, mock.AnythingOfType("*searcher.SearchRequest")).Return(nil, assert.AnError)

	result, err := s.GetLogEntriesStats(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to search logs")

	mockSearcher.AssertExpectations(t)
}

func TestService_GetLogEntriesStats_NoFacets(t *testing.T) {
	mockSearcher := &MockSearcher{}
	s := NewService(mockSearcher)

	ctx := context.Background()
	req := &searcher.SearchRequest{
		Limit:  100,
		Offset: 0,
	}

	expectedResult := &searcher.SearchResult{
		TotalHits: 1000,
		Facets:    nil, // No facets
		Stats:     nil, // No stats
	}

	mockSearcher.On("Search", ctx, mock.AnythingOfType("*searcher.SearchRequest")).Return(expectedResult, nil)

	result, err := s.GetLogEntriesStats(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, result)

	// Should have initialized empty maps and slices
	assert.Equal(t, int64(1000), result.TotalEntries)
	assert.NotNil(t, result.StatusCodeDist)
	assert.NotNil(t, result.MethodDist)
	assert.NotNil(t, result.TopPaths)
	assert.NotNil(t, result.TopIPs)
	assert.NotNil(t, result.TopUserAgents)
	
	// Should be empty
	assert.Len(t, result.StatusCodeDist, 0)
	assert.Len(t, result.MethodDist, 0)
	assert.Len(t, result.TopPaths, 0)
	assert.Len(t, result.TopIPs, 0)
	assert.Len(t, result.TopUserAgents, 0)

	// Stats should be nil
	assert.Nil(t, result.BytesStats)
	assert.Nil(t, result.ResponseTimeStats)

	mockSearcher.AssertExpectations(t)
}

func TestService_GetTopPaths_Success(t *testing.T) {
	mockSearcher := &MockSearcher{}
	s := NewService(mockSearcher)

	ctx := context.Background()
	req := &TopListRequest{
		StartTime: 1000,
		EndTime:   2000,
		Limit:     5,
		Field:     FieldPath,
		LogPath:   "/var/log/nginx/access.log",
	}

	expectedResult := &searcher.SearchResult{
		TotalHits: 4,
		Facets: map[string]*searcher.Facet{
			"path_exact": {
				Terms: []*searcher.FacetTerm{
					{Term: "/a", Count: 2},
					{Term: "/b", Count: 1},
					{Term: "/c", Count: 1},
				},
			},
		},
	}

	mockSearcher.On("Search", ctx, mock.AnythingOfType("*searcher.SearchRequest")).Return(expectedResult, nil)

	result, err := s.GetTopPaths(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result, 3)

	// Should be ordered by count descending
	assert.Equal(t, "/a", result[0].Key)
	assert.Equal(t, 2, result[0].Value)
	assert.Equal(t, "/b", result[1].Key)
	assert.Equal(t, 1, result[1].Value)

	mockSearcher.AssertExpectations(t)
}

func TestService_GetTopPaths_WithLimit(t *testing.T) {
	mockSearcher := &MockSearcher{}
	s := NewService(mockSearcher)

	ctx := context.Background()
	req := &TopListRequest{
		StartTime: 1000,
		EndTime:   2000,
		Limit:     1,
		Field:     FieldPath,
		LogPath:   "/var/log/nginx/access.log",
	}

	expectedResult := &searcher.SearchResult{
		TotalHits: 4,
		Facets: map[string]*searcher.Facet{
			"path_exact": {
				Terms: []*searcher.FacetTerm{
					{Term: "/a", Count: 2},
				},
			},
		},
	}

	mockSearcher.On("Search", ctx, mock.AnythingOfType("*searcher.SearchRequest")).Return(expectedResult, nil)

	result, err := s.GetTopPaths(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result, 1) // Limited to 1

	assert.Equal(t, "/a", result[0].Key)
	assert.Equal(t, 2, result[0].Value)

	mockSearcher.AssertExpectations(t)
}

func TestService_GetTopIPs_Success(t *testing.T) {
	mockSearcher := &MockSearcher{}
	s := NewService(mockSearcher)

	ctx := context.Background()
	req := &TopListRequest{
		StartTime: 1000,
		EndTime:   2000,
		Limit:     3,
		Field:     FieldIP,
		LogPath:   "/var/log/nginx/access.log",
	}

	expectedResult := &searcher.SearchResult{
		TotalHits: 3,
		Facets: map[string]*searcher.Facet{
			"ip": {
				Terms: []*searcher.FacetTerm{
					{Term: "1.1.1.1", Count: 2},
					{Term: "1.1.1.2", Count: 1},
				},
			},
		},
	}

	mockSearcher.On("Search", ctx, mock.AnythingOfType("*searcher.SearchRequest")).Return(expectedResult, nil)

	result, err := s.GetTopIPs(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result, 2)

	assert.Equal(t, "1.1.1.1", result[0].Key)
	assert.Equal(t, 2, result[0].Value)
	assert.Equal(t, "1.1.1.2", result[1].Key)
	assert.Equal(t, 1, result[1].Value)

	mockSearcher.AssertExpectations(t)
}

func TestService_GetTopUserAgents_Success(t *testing.T) {
	mockSearcher := &MockSearcher{}
	s := NewService(mockSearcher)

	ctx := context.Background()
	req := &TopListRequest{
		StartTime: 1000,
		EndTime:   2000,
		Limit:     2,
		Field:     FieldUserAgent,
		LogPath:   "/var/log/nginx/access.log",
	}

	expectedResult := &searcher.SearchResult{
		TotalHits: 2,
		Facets: map[string]*searcher.Facet{
			"user_agent": {
				Terms: []*searcher.FacetTerm{
					{Term: "Chrome", Count: 1},
					{Term: "Firefox", Count: 1},
				},
			},
		},
	}

	mockSearcher.On("Search", ctx, mock.AnythingOfType("*searcher.SearchRequest")).Return(expectedResult, nil)

	result, err := s.GetTopUserAgents(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result, 2)

	assert.Equal(t, "Chrome", result[0].Key)
	assert.Equal(t, 1, result[0].Value)
	assert.Equal(t, "Firefox", result[1].Key)
	assert.Equal(t, 1, result[1].Value)

	mockSearcher.AssertExpectations(t)
}

func TestService_GetTopPaths_InvalidTimeRange(t *testing.T) {
	mockSearcher := &MockSearcher{}
	s := NewService(mockSearcher)

	ctx := context.Background()
	req := &TopListRequest{
		StartTime: 2000,
		EndTime:   1000, // End before start
		Limit:     10,
		Field:     FieldPath,
		LogPath:   "/var/log/nginx/access.log",
	}

	result, err := s.GetTopPaths(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "time")

	mockSearcher.AssertExpectations(t)
}

func TestService_GetTopPaths_NoFacets(t *testing.T) {
	mockSearcher := &MockSearcher{}
	s := NewService(mockSearcher)

	ctx := context.Background()
	req := &TopListRequest{
		StartTime: 1000,
		EndTime:   2000,
		Limit:     10,
		Field:     FieldPath,
	}

	expectedResult := &searcher.SearchResult{
		TotalHits: 1000,
		Facets:    nil, // No facets
	}

	mockSearcher.On("Search", ctx, mock.AnythingOfType("*searcher.SearchRequest")).Return(expectedResult, nil)

	result, err := s.GetTopPaths(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result, 0) // Should be empty

	mockSearcher.AssertExpectations(t)
}

func TestService_GetTopPaths_DefaultLimit(t *testing.T) {
	mockSearcher := &MockSearcher{}
	s := NewService(mockSearcher)

	ctx := context.Background()
	req := &TopListRequest{
		StartTime: 1000,
		EndTime:   2000,
		Limit:     0, // Should use default
		Field:     FieldPath,
	}

	expectedResult := &searcher.SearchResult{
		TotalHits: 1000,
		Facets: map[string]*searcher.Facet{
			"path_exact": {
				Field: "path_exact",
				Total: 1000,
				Terms: []*searcher.FacetTerm{
					{Term: "/api/users", Count: 400},
				},
			},
		},
	}

	mockSearcher.On("Search", ctx, mock.AnythingOfType("*searcher.SearchRequest")).Return(expectedResult, nil)

	result, err := s.GetTopPaths(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	// Should work with default limit

	mockSearcher.AssertExpectations(t)
}

func TestService_GetTopPaths_MaxLimit(t *testing.T) {
	mockSearcher := &MockSearcher{}
	s := NewService(mockSearcher)

	ctx := context.Background()
	req := &TopListRequest{
		StartTime: 1000,
		EndTime:   2000,
		Limit:     99999, // Should be capped to MaxLimit
		Field:     FieldPath,
	}

	expectedResult := &searcher.SearchResult{
		TotalHits: 1000,
		Facets: map[string]*searcher.Facet{
			"path_exact": {
				Field: "path_exact",
				Total: 1000,
				Terms: []*searcher.FacetTerm{
					{Term: "/api/users", Count: 400},
				},
			},
		},
	}

	mockSearcher.On("Search", ctx, mock.AnythingOfType("*searcher.SearchRequest")).Return(expectedResult, nil)

	result, err := s.GetTopPaths(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	// Should work with capped limit

	mockSearcher.AssertExpectations(t)
}