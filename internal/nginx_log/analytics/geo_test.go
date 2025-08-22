package analytics

import (
	"context"
	"testing"

	"github.com/0xJacky/Nginx-UI/internal/nginx_log/searcher"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestService_GetGeoDistribution_Success(t *testing.T) {
	mockSearcher := &MockSearcher{}
	s := NewService(mockSearcher)

	ctx := context.Background()
	req := &GeoQueryRequest{
		StartTime: 1000,
		EndTime:   2000,
		LogPaths:  []string{"/var/log/nginx/access.log"},
	}

	expectedResult := &searcher.SearchResult{
		TotalHits: 4,
		Facets: map[string]*searcher.Facet{
			"region_code": {
				Terms: []*searcher.FacetTerm{
					{Term: "US", Count: 2},
					{Term: "CA", Count: 1},
					{Term: "GB", Count: 1},
				},
			},
		},
	}

	mockSearcher.On("Search", ctx, mock.AnythingOfType("*searcher.SearchRequest")).Return(expectedResult, nil)

	result, err := s.GetGeoDistribution(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result.Countries, 3)
	assert.Equal(t, 2, result.Countries["US"])
	assert.Equal(t, 1, result.Countries["CA"])
	assert.Equal(t, 1, result.Countries["GB"])

	mockSearcher.AssertExpectations(t)
}

func TestService_GetGeoDistributionByCountry_Success(t *testing.T) {
	mockSearcher := &MockSearcher{}
	s := NewService(mockSearcher)

	ctx := context.Background()
	req := &GeoQueryRequest{
		StartTime: 1000,
		EndTime:   2000,
		LogPaths:  []string{"/var/log/nginx/access.log"},
	}

	// Mock search result with province data for CN
	expectedResult := &searcher.SearchResult{
		TotalHits: 4185, // Same as WorldMap CN count
		Facets: map[string]*searcher.Facet{
			"province": {
				Total:   4185,
				Missing: 0,
				Terms: []*searcher.FacetTerm{
					{Term: "广东", Count: 2000},
					{Term: "北京", Count: 1500},
					{Term: "上海", Count: 500},
					{Term: "其它", Count: 185},
				},
			},
		},
	}

	// Verify that the search request uses Countries filter correctly
	mockSearcher.On("Search", ctx, mock.MatchedBy(func(searchReq *searcher.SearchRequest) bool {
		// Check that Countries filter is set correctly
		return len(searchReq.Countries) == 1 && 
			   searchReq.Countries[0] == "CN" &&
			   len(searchReq.FacetFields) == 1 &&
			   searchReq.FacetFields[0] == "province"
	})).Return(expectedResult, nil)

	result, err := s.GetGeoDistributionByCountry(ctx, req, "CN")

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result.Countries, 4) // 4 provinces
	assert.Equal(t, 2000, result.Countries["广东"])
	assert.Equal(t, 1500, result.Countries["北京"])
	assert.Equal(t, 500, result.Countries["上海"])
	assert.Equal(t, 185, result.Countries["其它"])

	mockSearcher.AssertExpectations(t)
}

func TestService_GetGeoDistributionByCountry_EmptyProvinceFacet(t *testing.T) {
	mockSearcher := &MockSearcher{}
	s := NewService(mockSearcher)

	ctx := context.Background()
	req := &GeoQueryRequest{
		StartTime: 1000,
		EndTime:   2000,
		LogPaths:  []string{"/var/log/nginx/access.log"},
	}

	// Mock search result with no province facet (current real-world behavior)
	expectedResult := &searcher.SearchResult{
		TotalHits: 4185,
		Facets: map[string]*searcher.Facet{
			"region_code": {
				Total: 1,
				Terms: []*searcher.FacetTerm{
					{Term: "CN", Count: 4185},
				},
			},
		},
	}

	mockSearcher.On("Search", ctx, mock.MatchedBy(func(searchReq *searcher.SearchRequest) bool {
		return len(searchReq.Countries) == 1 && searchReq.Countries[0] == "CN"
	})).Return(expectedResult, nil)

	result, err := s.GetGeoDistributionByCountry(ctx, req, "CN")

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result.Countries, 0) // No provinces returned
	
	mockSearcher.AssertExpectations(t)
}

func TestService_GetGeoDistributionByCountry_CountriesFilterValidation(t *testing.T) {
	mockSearcher := &MockSearcher{}
	s := NewService(mockSearcher)

	ctx := context.Background()
	req := &GeoQueryRequest{
		StartTime: 1000,
		EndTime:   2000,
		LogPaths:  []string{"/var/log/nginx/access.log"},
	}

	tests := []struct {
		name        string
		countryCode string
		expectError bool
	}{
		{
			name:        "valid country code CN",
			countryCode: "CN",
			expectError: false,
		},
		{
			name:        "valid country code US", 
			countryCode: "US",
			expectError: false,
		},
		{
			name:        "empty country code",
			countryCode: "",
			expectError: false, // Should work, just return empty results
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expectedResult := &searcher.SearchResult{
				TotalHits: 100,
				Facets:    map[string]*searcher.Facet{},
			}

			mockSearcher.On("Search", ctx, mock.MatchedBy(func(searchReq *searcher.SearchRequest) bool {
				if tt.countryCode == "" {
					return len(searchReq.Countries) == 1 && searchReq.Countries[0] == ""
				}
				return len(searchReq.Countries) == 1 && searchReq.Countries[0] == tt.countryCode
			})).Return(expectedResult, nil).Once()

			result, err := s.GetGeoDistributionByCountry(ctx, req, tt.countryCode)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
			}
		})
	}

	mockSearcher.AssertExpectations(t)
}

func TestService_GeoDataConsistency_ChinaVsWorld(t *testing.T) {
	// This test verifies that ChinaMap total matches WorldMap CN count
	mockSearcher := &MockSearcher{}
	s := NewService(mockSearcher)

	ctx := context.Background()
	req := &GeoQueryRequest{
		StartTime: 1755014400,
		EndTime:   1755705599,
		LogPaths:  []string{"/var/log/nginx/access.log"},
	}

	// Mock WorldMap result
	worldMapResult := &searcher.SearchResult{
		TotalHits: 12845,
		Facets: map[string]*searcher.Facet{
			"region_code": {
				Total: 53,
				Terms: []*searcher.FacetTerm{
					{Term: "CN", Count: 4185}, // Key: CN count should match ChinaMap total
					{Term: "FR", Count: 3056},
					{Term: "US", Count: 1456},
					{Term: "DE", Count: 1152},
					// ... other countries
				},
			},
		},
	}

	// Mock ChinaMap result with same total
	chinaMapResult := &searcher.SearchResult{
		TotalHits: 4185, // Should match CN count from WorldMap
		Facets: map[string]*searcher.Facet{
			"province": {
				Total:   4185,
				Missing: 0,
				Terms: []*searcher.FacetTerm{
					{Term: "广东", Count: 2000},
					{Term: "北京", Count: 1500},
					{Term: "上海", Count: 500},
					{Term: "其它", Count: 185},
				},
			},
		},
	}

	// Setup mock expectations
	// First call: GetGeoDistribution (WorldMap)
	mockSearcher.On("Search", ctx, mock.MatchedBy(func(searchReq *searcher.SearchRequest) bool {
		return len(searchReq.Countries) == 0 && // No country filter for world map
			   len(searchReq.FacetFields) == 1 &&
			   searchReq.FacetFields[0] == "region_code"
	})).Return(worldMapResult, nil).Once()

	// Second call: GetGeoDistributionByCountry (ChinaMap)
	mockSearcher.On("Search", ctx, mock.MatchedBy(func(searchReq *searcher.SearchRequest) bool {
		return len(searchReq.Countries) == 1 &&
			   searchReq.Countries[0] == "CN" &&
			   len(searchReq.FacetFields) == 1 &&
			   searchReq.FacetFields[0] == "province"
	})).Return(chinaMapResult, nil).Once()

	// Test WorldMap
	worldResult, err := s.GetGeoDistribution(ctx, req)
	assert.NoError(t, err)
	assert.NotNil(t, worldResult)
	
	cnCountInWorld := worldResult.Countries["CN"]
	assert.Equal(t, 4185, cnCountInWorld)

	// Test ChinaMap
	chinaResult, err := s.GetGeoDistributionByCountry(ctx, req, "CN")
	assert.NoError(t, err)
	assert.NotNil(t, chinaResult)

	// Calculate total from provinces
	totalChinaVisits := 0
	for _, count := range chinaResult.Countries {
		totalChinaVisits += count
	}

	// Verify consistency: WorldMap CN count should equal ChinaMap total
	assert.Equal(t, cnCountInWorld, totalChinaVisits, 
		"WorldMap CN count (%d) should equal ChinaMap total (%d)", 
		cnCountInWorld, totalChinaVisits)

	mockSearcher.AssertExpectations(t)
}

func TestService_GetGeoDistribution_NilRequest(t *testing.T) {
	mockSearcher := &MockSearcher{}
	s := NewService(mockSearcher)

	ctx := context.Background()
	result, err := s.GetGeoDistribution(ctx, nil)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "request cannot be nil")
}

func TestService_GetGeoDistribution_InvalidTimeRange(t *testing.T) {
	mockSearcher := &MockSearcher{}
	s := NewService(mockSearcher)

	ctx := context.Background()
	req := &GeoQueryRequest{
		StartTime: 2000,
		EndTime:   1000, // End before start
	}

	// We don't need to mock the searcher as the time range validation should fail first.
	result, err := s.GetGeoDistribution(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "invalid time range")
}

func TestService_GetGeoDistribution_SearchError(t *testing.T) {
	mockSearcher := &MockSearcher{}
	s := NewService(mockSearcher)

	ctx := context.Background()
	req := &GeoQueryRequest{
		StartTime: 1000,
		EndTime:   2000,
	}

	mockSearcher.On("Search", ctx, mock.AnythingOfType("*searcher.SearchRequest")).Return(nil, assert.AnError)

	result, err := s.GetGeoDistribution(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to get geo distribution")

	mockSearcher.AssertExpectations(t)
}


func TestService_GetTopCountries_Success(t *testing.T) {
	mockSearcher := &MockSearcher{}
	s := NewService(mockSearcher)

	ctx := context.Background()
	req := &GeoQueryRequest{
		StartTime: 1000,
		EndTime:   2000,
		Limit:     2, // Limit to top 2
		LogPaths:  []string{"/var/log/nginx/access.log"},
	}

	expectedResult := &searcher.SearchResult{
		TotalHits: 6,
		Facets: map[string]*searcher.Facet{
			"region_code": {
				Terms: []*searcher.FacetTerm{
					{Term: "US", Count: 3},
					{Term: "CN", Count: 2},
				},
			},
		},
	}

	mockSearcher.On("Search", ctx, mock.AnythingOfType("*searcher.SearchRequest")).Return(expectedResult, nil)

	result, err := s.GetTopCountries(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result, 2)

	// Check that results are sorted and limited
	assert.Equal(t, "US", result[0].Country)
	assert.Equal(t, 3, result[0].Requests)

	assert.Equal(t, "CN", result[1].Country)
	assert.Equal(t, 2, result[1].Requests)

	mockSearcher.AssertExpectations(t)
}

func TestService_GetTopCities_Success(t *testing.T) {
	mockSearcher := &MockSearcher{}
	s := NewService(mockSearcher)

	ctx := context.Background()
	req := &GeoQueryRequest{
		StartTime: 1000,
		EndTime:   2000,
		Limit:     2,
	}

	expectedResult := &searcher.SearchResult{
		TotalHits: 1000,
		Facets: map[string]*searcher.Facet{
			"city": {
				Field: "city",
				Total: 1000,
				Terms: []*searcher.FacetTerm{
					{Term: "New York", Count: 400},
					{Term: "Toronto", Count: 300},
				},
			},
		},
	}

	mockSearcher.On("Search", ctx, mock.AnythingOfType("*searcher.SearchRequest")).Return(expectedResult, nil)

	result, err := s.GetTopCities(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result, 2)

	assert.Equal(t, "New York", result[0].City)
	assert.Equal(t, 400, result[0].Count)

	mockSearcher.AssertExpectations(t)
}

func TestService_GetGeoStatsForIP_Success(t *testing.T) {
	mockSearcher := &MockSearcher{}
	s := NewService(mockSearcher)

	ctx := context.Background()
	req := &GeoQueryRequest{
		StartTime: 1000,
		EndTime:   2000,
	}
	ip := "192.168.1.1"

	expectedResult := &searcher.SearchResult{
		TotalHits: 150,
		Facets: map[string]*searcher.Facet{
			"country": {
				Field: "country",
				Terms: []*searcher.FacetTerm{
					{Term: "United States", Count: 150},
				},
			},
			"country_code": {
				Field: "country_code",
				Terms: []*searcher.FacetTerm{
					{Term: "US", Count: 150},
				},
			},
			"city": {
				Field: "city",
				Terms: []*searcher.FacetTerm{
					{Term: "New York", Count: 150},
				},
			},
		},
	}

	mockSearcher.On("Search", ctx, mock.AnythingOfType("*searcher.SearchRequest")).Return(expectedResult, nil)

	result, err := s.GetGeoStatsForIP(ctx, req, ip)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "New York", result.City)
	assert.Equal(t, "United States", result.Country)
	assert.Equal(t, "US", result.CountryCode)
	assert.Equal(t, 150, result.Count)
	assert.Equal(t, 100.0, result.Percent) // 100% for single IP

	mockSearcher.AssertExpectations(t)
}

func TestService_GetGeoStatsForIP_EmptyIP(t *testing.T) {
	mockSearcher := &MockSearcher{}
	s := NewService(mockSearcher)

	ctx := context.Background()
	req := &GeoQueryRequest{
		StartTime: 1000,
		EndTime:   2000,
	}

	result, err := s.GetGeoStatsForIP(ctx, req, "")

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "IP address cannot be empty")
}

func TestService_GetGeoStatsForIP_NoData(t *testing.T) {
	mockSearcher := &MockSearcher{}
	s := NewService(mockSearcher)

	ctx := context.Background()
	req := &GeoQueryRequest{
		StartTime: 1000,
		EndTime:   2000,
	}
	ip := "192.168.1.1"

	expectedResult := &searcher.SearchResult{
		TotalHits: 0, // No data found
		Facets:    make(map[string]*searcher.Facet),
	}

	mockSearcher.On("Search", ctx, mock.AnythingOfType("*searcher.SearchRequest")).Return(expectedResult, nil)

	result, err := s.GetGeoStatsForIP(ctx, req, ip)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "no data found for IP")

	mockSearcher.AssertExpectations(t)
}

func TestService_GetGeoStatsForIP_NoGeoData(t *testing.T) {
	mockSearcher := &MockSearcher{}
	s := NewService(mockSearcher)

	ctx := context.Background()
	req := &GeoQueryRequest{
		StartTime: 1000,
		EndTime:   2000,
	}
	ip := "192.168.1.1"

	expectedResult := &searcher.SearchResult{
		TotalHits: 100,
		Facets:    nil, // No geo facets
	}

	mockSearcher.On("Search", ctx, mock.AnythingOfType("*searcher.SearchRequest")).Return(expectedResult, nil)

	result, err := s.GetGeoStatsForIP(ctx, req, ip)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "could not extract geo information")

	mockSearcher.AssertExpectations(t)
}

func TestService_GetGeoDistribution_DefaultLimit(t *testing.T) {
	mockSearcher := &MockSearcher{}
	s := NewService(mockSearcher)

	ctx := context.Background()
	req := &GeoQueryRequest{
		StartTime: 1000,
		EndTime:   2000,
		Limit:     0, // Should use default
	}

	expectedResult := &searcher.SearchResult{
		TotalHits: 1000,
		Facets: map[string]*searcher.Facet{
			"country": {
				Field: "country",
				Total: 1000,
				Terms: []*searcher.FacetTerm{
					{Term: "United States", Count: 600},
					{Term: "Canada", Count: 400},
				},
			},
		},
	}

	mockSearcher.On("Search", ctx, mock.AnythingOfType("*searcher.SearchRequest")).Return(expectedResult, nil)

	result, err := s.GetGeoDistribution(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	// Should work with default limit

	mockSearcher.AssertExpectations(t)
}

func TestService_GetGeoDistribution_MaxLimit(t *testing.T) {
	mockSearcher := &MockSearcher{}
	s := NewService(mockSearcher)

	ctx := context.Background()
	req := &GeoQueryRequest{
		StartTime: 1000,
		EndTime:   2000,
		Limit:     99999, // Should be capped to MaxLimit
	}

	expectedResult := &searcher.SearchResult{
		TotalHits: 1000,
		Facets: map[string]*searcher.Facet{
			"country": {
				Field: "country",
				Total: 1000,
				Terms: []*searcher.FacetTerm{
					{Term: "United States", Count: 600},
					{Term: "Canada", Count: 400},
				},
			},
		},
	}

	mockSearcher.On("Search", ctx, mock.AnythingOfType("*searcher.SearchRequest")).Return(expectedResult, nil)

	result, err := s.GetGeoDistribution(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	// Should work with capped limit

	mockSearcher.AssertExpectations(t)
}