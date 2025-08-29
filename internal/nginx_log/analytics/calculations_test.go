package analytics

import (
	"context"
	"testing"

	"github.com/0xJacky/Nginx-UI/internal/nginx_log/searcher"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestService_GetDashboardAnalytics_HourlyStats(t *testing.T) {
	mockSearcher := &MockSearcher{}
	s := NewService(mockSearcher)

	ctx := context.Background()
	req := &DashboardQueryRequest{
		StartTime: 1640995200, // 2022-01-01 00:00:00 UTC
		EndTime:   1641006000, // 2022-01-01 03:00:00 UTC (same day as test data)
		LogPaths:  []string{"/var/log/nginx/access.log"},
	}

	expectedResult := &searcher.SearchResult{
		TotalHits: 3,
		Hits: []*searcher.SearchHit{
			{
				Fields: map[string]interface{}{
					"timestamp": float64(1640995800), // 2022-01-01 00:10:00
					"ip":        "192.168.1.1",
					"bytes":     int64(1024),
				},
			},
			{
				Fields: map[string]interface{}{
					"timestamp": float64(1640999400), // 2022-01-01 01:10:00
					"ip":        "192.168.1.2",
					"bytes":     int64(2048),
				},
			},
			{
				Fields: map[string]interface{}{
					"timestamp": float64(1640999500), // 2022-01-01 01:11:40
					"ip":        "192.168.1.1",     // Same IP as first hit
					"bytes":     int64(512),
				},
			},
		},
		Facets: map[string]*searcher.Facet{
			"ip": {
				Terms: []*searcher.FacetTerm{
					{Term: "192.168.1.1", Count: 2},
					{Term: "192.168.1.2", Count: 1},
				},
			},
		},
	}

	mockSearcher.On("Search", ctx, mock.AnythingOfType("*searcher.SearchRequest")).Return(expectedResult, nil)

	result, err := s.GetDashboardAnalytics(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotEmpty(t, result.HourlyStats)

	// Check that we have some hourly data - the specific hours depend on the test data timestamps
	var totalPV, totalUV int
	for _, stat := range result.HourlyStats {
		totalPV += stat.PV
		totalUV += stat.UV
	}

	// We should have some aggregated data
	assert.Greater(t, totalPV, 0)
	assert.Greater(t, totalUV, 0)

	mockSearcher.AssertExpectations(t)
}

// Duplicate test functions removed - they exist in dashboard_test.go

func TestService_calculateHourlyStats_HourlyInterval(t *testing.T) {
	mockSearcher := &MockSearcher{}
	s := NewService(mockSearcher).(*service)

	result := &searcher.SearchResult{
		TotalHits: 2,
		Hits: []*searcher.SearchHit{
			{
				Fields: map[string]interface{}{
					"timestamp": float64(1640995800), // 2022-01-01 00:10:00 UTC
					"ip":        "192.168.1.1",
				},
			},
			{
				Fields: map[string]interface{}{
					"timestamp": float64(1641002400), // 2022-01-01 02:00:00 UTC
					"ip":        "192.168.1.2",
				},
			},
		},
	}

	startTime := int64(1640995200) // 2022-01-01 00:00:00 UTC
	endTime := int64(1641002400)   // 2022-01-01 02:00:00 UTC

	stats := s.calculateHourlyStats(result, startTime, endTime)

	assert.NotNil(t, stats)
	assert.GreaterOrEqual(t, len(stats), 2) // Should have at least 2 hours

	// Check that stats are sorted by timestamp (not just hour, since we have 48 hours of data)
	for i := 1; i < len(stats); i++ {
		assert.LessOrEqual(t, stats[i-1].Timestamp, stats[i].Timestamp)
	}
}

func TestService_calculateDailyStats_DailyInterval(t *testing.T) {
	mockSearcher := &MockSearcher{}
	s := NewService(mockSearcher).(*service)

	result := &searcher.SearchResult{
		TotalHits: 2,
		Hits: []*searcher.SearchHit{
			{
				Fields: map[string]interface{}{
					"timestamp": float64(1640995800), // 2022-01-01 00:10:00 UTC
					"ip":        "192.168.1.1",
				},
			},
			{
				Fields: map[string]interface{}{
					"timestamp": float64(1641168000), // 2022-01-03 00:00:00 UTC
					"ip":        "192.168.1.2",
				},
			},
		},
	}

	startTime := int64(1640995200) // 2022-01-01 00:00:00 UTC
	endTime := int64(1641168000)   // 2022-01-03 00:00:00 UTC

	stats := s.calculateDailyStats(result, startTime, endTime)

	assert.NotNil(t, stats)
	assert.GreaterOrEqual(t, len(stats), 2) // Should have at least 2 days

	// Check that stats are sorted by timestamp
	for i := 1; i < len(stats); i++ {
		assert.LessOrEqual(t, stats[i-1].Timestamp, stats[i].Timestamp)
	}
}

func TestService_calculateDashboardSummary_MonthlyData(t *testing.T) {
	mockSearcher := &MockSearcher{}
	s := NewService(mockSearcher).(*service)

	analytics := &DashboardAnalytics{
		HourlyStats: []HourlyAccessStats{
			{Hour: 0, UV: 10, PV: 100},
			{Hour: 1, UV: 20, PV: 200},
			{Hour: 2, UV: 15, PV: 150},
		},
		DailyStats: []DailyAccessStats{
			{Date: "2022-01-01", UV: 30, PV: 300, Timestamp: 1640995200},
			{Date: "2022-01-02", UV: 25, PV: 250, Timestamp: 1641081600},
			{Date: "2022-01-03", UV: 28, PV: 280, Timestamp: 1641168000},
		},
	}

	result := &searcher.SearchResult{
		TotalHits: 830,
		Facets: map[string]*searcher.Facet{
			"ip": {
				Total: 50, // 50 unique IPs
			},
		},
	}

	summary := s.calculateDashboardSummary(analytics, result)

	assert.Equal(t, 50, summary.TotalUV)
	assert.Equal(t, 830, summary.TotalPV)
	assert.InDelta(t, 16.67, summary.AvgDailyUV, 0.01) // 50 total UV / 3 days
	assert.InDelta(t, 276.67, summary.AvgDailyPV, 0.01) // (300+250+280)/3

	// Peak hour should be hour 1 with 200 PV
	assert.Equal(t, 1, summary.PeakHour)
	assert.Equal(t, 200, summary.PeakHourTraffic)
}

func TestService_calculateTopFieldStats_Generic(t *testing.T) {
	facet := &searcher.Facet{
		Terms: []*searcher.FacetTerm{
			{Term: "/api/users", Count: 100},
			{Term: "/api/posts", Count: 50},
			{Term: "/", Count: 25},
		},
	}

	totalHits := 200

	result := calculateTopFieldStats(facet, totalHits, func(term string, count int, percent float64) URLAccessStats {
		return URLAccessStats{URL: term, Visits: count, Percent: percent}
	})

	assert.NotNil(t, result)
	assert.Len(t, result, 3)

	// Check first item
	assert.Equal(t, "/api/users", result[0].URL)
	assert.Equal(t, 100, result[0].Visits)
	assert.Equal(t, 50.0, result[0].Percent) // 100/200 * 100

	// Check second item
	assert.Equal(t, "/api/posts", result[1].URL)
	assert.Equal(t, 50, result[1].Visits)
	assert.Equal(t, 25.0, result[1].Percent) // 50/200 * 100
}

func TestCalculateTopFieldStats_EmptyFacet(t *testing.T) {
	result := calculateTopFieldStats[URLAccessStats](nil, 100, func(term string, count int, percent float64) URLAccessStats {
		return URLAccessStats{URL: term, Visits: count, Percent: percent}
	})

	assert.NotNil(t, result)
	assert.Len(t, result, 0)
}

func TestCalculateTopFieldStats_ZeroHits(t *testing.T) {
	facet := &searcher.Facet{
		Terms: []*searcher.FacetTerm{
			{Term: "/api/users", Count: 100},
		},
	}

	result := calculateTopFieldStats(facet, 0, func(term string, count int, percent float64) URLAccessStats {
		return URLAccessStats{URL: term, Visits: count, Percent: percent}
	})

	assert.NotNil(t, result)
	assert.Len(t, result, 0)
}

func TestService_ValidateTimeRange_Comprehensive(t *testing.T) {
	mockSearcher := &MockSearcher{}
	s := NewService(mockSearcher)

	tests := []struct {
		name      string
		startTime int64
		endTime   int64
		expected  bool
	}{
		{"valid range", 1000, 2000, true},
		{"invalid range - same", 1000, 1000, false},
		{"invalid range - backwards", 2000, 1000, false},
		{"zero range", 0, 0, true},
		{"negative start", -1000, 2000, false},
		{"negative end", 1000, -2000, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := s.ValidateTimeRange(tt.startTime, tt.endTime)
			if tt.expected {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

func TestGetTopKeyValuesFromMap(t *testing.T) {
	counts := map[string]int{
		"200": 2,
		"404": 1,
		"500": 3,
	}

	result := getTopKeyValuesFromMap(counts, 10) // Set a reasonable limit

	assert.NotNil(t, result)
	assert.Len(t, result, 3)

	// Should be sorted by value descending
	assert.Equal(t, "500", result[0].Key)
	assert.Equal(t, 3, result[0].Value)
	assert.Equal(t, "200", result[1].Key)
	assert.Equal(t, 2, result[1].Value)
	assert.Equal(t, "404", result[2].Key)
	assert.Equal(t, 1, result[2].Value)
}

func TestGetTopKeyValuesFromMap_WithLimit(t *testing.T) {
	counts := map[string]int{
		"a": 1,
		"b": 2,
		"c": 3,
	}

	result := getTopKeyValuesFromMap(counts, 2)

	assert.NotNil(t, result)
	assert.Len(t, result, 2) // Should be limited to 2

	// Should be sorted by value descending
	assert.Equal(t, "c", result[0].Key)
	assert.Equal(t, 3, result[0].Value)
	assert.Equal(t, "b", result[1].Key)
	assert.Equal(t, 2, result[1].Value)
}

func TestService_calculateBrowserStats_FromFacets(t *testing.T) {
	result := &searcher.SearchResult{
		TotalHits: 1000,
		Facets: map[string]*searcher.Facet{
			"browser": {
				Terms: []*searcher.FacetTerm{
					{Term: "Chrome", Count: 600},
					{Term: "Firefox", Count: 300},
					{Term: "Safari", Count: 100},
				},
			},
		},
	}

	mockSearcher := &MockSearcher{}
	s := NewService(mockSearcher).(*service)

	stats := s.calculateBrowserStats(result)

	assert.NotNil(t, stats)
	assert.Len(t, stats, 3)

	// Check sorting and calculations
	assert.Equal(t, "Chrome", stats[0].Browser)
	assert.Equal(t, 600, stats[0].Count)
	assert.Equal(t, 60.0, stats[0].Percent) // 600/1000 * 100

	assert.Equal(t, "Firefox", stats[1].Browser)
	assert.Equal(t, 300, stats[1].Count)
	assert.Equal(t, 30.0, stats[1].Percent) // 300/1000 * 100
}

func TestService_calculateOSStats_FromFacets(t *testing.T) {
	result := &searcher.SearchResult{
		TotalHits: 800,
		Facets: map[string]*searcher.Facet{
			"os": {
				Terms: []*searcher.FacetTerm{
					{Term: "Windows", Count: 400},
					{Term: "macOS", Count: 250},
					{Term: "Linux", Count: 150},
				},
			},
		},
	}

	mockSearcher := &MockSearcher{}
	s := NewService(mockSearcher).(*service)

	stats := s.calculateOSStats(result)

	assert.NotNil(t, stats)
	assert.Len(t, stats, 3)

	// Check sorting and calculations
	assert.Equal(t, "Windows", stats[0].OS)
	assert.Equal(t, 400, stats[0].Count)
	assert.Equal(t, 50.0, stats[0].Percent) // 400/800 * 100

	assert.Equal(t, "macOS", stats[1].OS)
	assert.Equal(t, 250, stats[1].Count)
	assert.Equal(t, 31.25, stats[1].Percent) // 250/800 * 100
}

func TestService_GetVisitorsByCountry_Success(t *testing.T) {
	mockSearcher := &MockSearcher{}
	s := NewService(mockSearcher).(*service)

	ctx := context.Background()
	req := &VisitorsByCountryRequest{
		StartTime: 1000,
		EndTime:   2000,
		LogPaths:  []string{"/var/log/nginx/access.log"},
	}

	expectedResult := &searcher.SearchResult{
		Facets: map[string]*searcher.Facet{
			"region_code": {
				Terms: []*searcher.FacetTerm{
					{Term: "US", Count: 100},
					{Term: "CN", Count: 50},
				},
			},
		},
	}

	mockSearcher.On("Search", ctx, mock.AnythingOfType("*searcher.SearchRequest")).Return(expectedResult, nil)

	result, err := s.GetVisitorsByCountry(ctx, req)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 100, result.Data["US"])
	assert.Equal(t, 50, result.Data["CN"])
}

func TestService_GetErrorDistribution_Success(t *testing.T) {
	mockSearcher := &MockSearcher{}
	s := NewService(mockSearcher).(*service)

	ctx := context.Background()
	req := &ErrorDistributionRequest{
		StartTime: 1000,
		EndTime:   2000,
		LogPaths:  []string{"/var/log/nginx/access.log"},
	}

	expectedResult := &searcher.SearchResult{
		Facets: map[string]*searcher.Facet{
			"status": {
				Terms: []*searcher.FacetTerm{
					{Term: "404", Count: 20},
					{Term: "500", Count: 5},
				},
			},
		},
	}

	mockSearcher.On("Search", ctx, mock.MatchedBy(func(r *searcher.SearchRequest) bool {
		return r.Query == "status:[400 TO 599]"
	})).Return(expectedResult, nil)

	result, err := s.GetErrorDistribution(ctx, req)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 20, result.Data["404"])
	assert.Equal(t, 5, result.Data["500"])
}
