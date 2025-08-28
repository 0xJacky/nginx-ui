package analytics

import (
	"context"
	"testing"

	"github.com/0xJacky/Nginx-UI/internal/nginx_log/searcher"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestService_GetDashboardAnalytics_Success(t *testing.T) {
	mockSearcher := &MockSearcher{}
	s := NewService(mockSearcher)

	ctx := context.Background()
	req := &DashboardQueryRequest{
		StartTime: 1640995200, // 2022-01-01 00:00:00 UTC
		EndTime:   1641081600, // 2022-01-02 00:00:00 UTC
		LogPath:   "/var/log/nginx/access.log",
	}

	// Mock search result with more comprehensive sample data for in-memory calculation
	expectedResult := &searcher.SearchResult{
		TotalHits: 4,
		Hits: []*searcher.SearchHit{
			{
				Fields: map[string]interface{}{
					"timestamp":   float64(1640995800), // 2022-01-01 00:10:00 UTC (hour 0)
					"ip":          "192.168.1.1",
					"path":        "/api/users",
					"browser":     "Chrome",
					"os":          "Windows",
					"device_type": "Desktop",
				},
			},
			{
				Fields: map[string]interface{}{
					"timestamp":   float64(1640999400), // 2022-01-01 01:10:00 UTC (hour 1)
					"ip":          "192.168.1.2",
					"path":        "/api/posts",
					"browser":     "Firefox",
					"os":          "Linux",
					"device_type": "Desktop",
				},
			},
			{
				Fields: map[string]interface{}{
					"timestamp":   float64(1640999500), // 2022-01-01 01:11:40 UTC (hour 1)
					"ip":          "192.168.1.1",
					"path":        "/api/users",
					"browser":     "Chrome",
					"os":          "Windows",
					"device_type": "Mobile",
				},
			},
			{
				Fields: map[string]interface{}{
					"timestamp":   float64(1641082200), // 2022-01-02 00:10:00 UTC (day 2)
					"ip":          "192.168.1.3",
					"path":        "/",
					"browser":     "Chrome",
					"os":          "macOS",
					"device_type": "Desktop",
				},
			},
		},
		Facets: map[string]*searcher.Facet{
			"path_exact": {
				Terms: []*searcher.FacetTerm{
					{Term: "/api/users", Count: 2},
					{Term: "/api/posts", Count: 1},
					{Term: "/", Count: 1},
				},
			},
			"browser": {
				Terms: []*searcher.FacetTerm{
					{Term: "Chrome", Count: 3},
					{Term: "Firefox", Count: 1},
				},
			},
			"os": {
				Terms: []*searcher.FacetTerm{
					{Term: "Windows", Count: 2},
					{Term: "Linux", Count: 1},
					{Term: "macOS", Count: 1},
				},
			},
			"device_type": {
				Terms: []*searcher.FacetTerm{
					{Term: "Desktop", Count: 3},
					{Term: "Mobile", Count: 1},
				},
			},
			"ip": {
				Total: 3, // 3 unique IPs
				Terms: []*searcher.FacetTerm{
					{Term: "192.168.1.1", Count: 2},
					{Term: "192.168.1.2", Count: 1},
					{Term: "192.168.1.3", Count: 1},
				},
			},
		},
	}

	mockSearcher.On("Search", ctx, mock.AnythingOfType("*searcher.SearchRequest")).Return(expectedResult, nil)

	result, err := s.GetDashboardAnalytics(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, result)

	// Check that all arrays are initialized
	assert.NotNil(t, result.HourlyStats)
	assert.NotNil(t, result.DailyStats)
	assert.NotNil(t, result.TopURLs)
	assert.NotNil(t, result.Browsers)
	assert.NotNil(t, result.OperatingSystems)
	assert.NotNil(t, result.Devices)

	// Check TopURLs (calculated from hits)
	assert.Len(t, result.TopURLs, 3)
	assert.Equal(t, "/api/users", result.TopURLs[0].URL)
	assert.Equal(t, 2, result.TopURLs[0].Visits)

	// Check Browsers (calculated from hits)
	assert.Len(t, result.Browsers, 2)
	assert.Equal(t, "Chrome", result.Browsers[0].Browser)
	assert.Equal(t, 3, result.Browsers[0].Count)

	// Check Devices (calculated from hits)
	assert.Len(t, result.Devices, 2)
	assert.Equal(t, "Desktop", result.Devices[0].Device)
	assert.Equal(t, 3, result.Devices[0].Count)

	// Check Summary
	assert.Equal(t, 3, result.Summary.TotalUV) // 3 unique IPs
	assert.Equal(t, 4, result.Summary.TotalPV)

	mockSearcher.AssertExpectations(t)
}

func TestService_GetDashboardAnalytics_NilRequest(t *testing.T) {
	mockSearcher := &MockSearcher{}
	s := NewService(mockSearcher)

	ctx := context.Background()
	result, err := s.GetDashboardAnalytics(ctx, nil)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "request cannot be nil")
}

func TestService_GetDashboardAnalytics_InvalidTimeRange(t *testing.T) {
	mockSearcher := &MockSearcher{}
	s := NewService(mockSearcher)

	ctx := context.Background()
	req := &DashboardQueryRequest{
		StartTime: 2000,
		EndTime:   1000, // End before start
		LogPath:   "/var/log/nginx/access.log",
	}

	result, err := s.GetDashboardAnalytics(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "invalid time range")
}

func TestService_GetDashboardAnalytics_SearchError(t *testing.T) {
	mockSearcher := &MockSearcher{}
	s := NewService(mockSearcher)

	ctx := context.Background()
	req := &DashboardQueryRequest{
		StartTime: 1000,
		EndTime:   2000,
		LogPath:   "/var/log/nginx/access.log",
	}

	mockSearcher.On("Search", ctx, mock.AnythingOfType("*searcher.SearchRequest")).Return(nil, assert.AnError)

	result, err := s.GetDashboardAnalytics(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to search logs")

	mockSearcher.AssertExpectations(t)
}

func TestService_GetDashboardAnalytics_EmptyResult(t *testing.T) {
	mockSearcher := &MockSearcher{}
	s := NewService(mockSearcher)

	ctx := context.Background()
	req := &DashboardQueryRequest{
		StartTime: 1000,
		EndTime:   2000,
		LogPath:   "/var/log/nginx/access.log",
	}

	// Empty search result
	expectedResult := &searcher.SearchResult{
		TotalHits: 0,
		Hits:      []*searcher.SearchHit{},
	}

	mockSearcher.On("Search", ctx, mock.AnythingOfType("*searcher.SearchRequest")).Return(expectedResult, nil)

	result, err := s.GetDashboardAnalytics(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	
	// All arrays should not be nil, but can be empty
	assert.NotNil(t, result.HourlyStats)
	assert.NotNil(t, result.DailyStats)
	assert.Len(t, result.TopURLs, 0)
	assert.Len(t, result.Browsers, 0)
	assert.Len(t, result.OperatingSystems, 0)
	assert.Len(t, result.Devices, 0)

	// Summary should have zero values
	assert.Equal(t, 0, result.Summary.TotalUV)
	assert.Equal(t, 0, result.Summary.TotalPV)

	mockSearcher.AssertExpectations(t)
}

func TestService_calculateHourlyStats(t *testing.T) {
	mockSearcher := &MockSearcher{}
	s := NewService(mockSearcher).(*service)

	// Create test data spanning multiple hours
	result := &searcher.SearchResult{
		TotalHits: 3,
		Hits: []*searcher.SearchHit{
			{
				Fields: map[string]interface{}{
					"timestamp": float64(1640995800), // 2022-01-01 00:10:00 UTC (hour 0)
					"ip":        "192.168.1.1",
				},
			},
			{
				Fields: map[string]interface{}{
					"timestamp": float64(1640999400), // 2022-01-01 01:10:00 UTC (hour 1)
					"ip":        "192.168.1.2",
				},
			},
			{
				Fields: map[string]interface{}{
					"timestamp": float64(1640999500), // 2022-01-01 01:11:40 UTC (hour 1)
					"ip":        "192.168.1.1", // Same IP as first hit
				},
			},
		},
	}

	startTime := int64(1640995200) // 2022-01-01 00:00:00 UTC
	endTime := int64(1641006000)   // 2022-01-01 03:00:00 UTC (extended range)

	stats := s.calculateHourlyStats(result, startTime, endTime)

	assert.NotNil(t, stats)
	assert.GreaterOrEqual(t, len(stats), 3) // Should have at least 3 hours

	// Find stats with actual data (non-zero PV)
	var statsWithData []*HourlyAccessStats
	for i := range stats {
		if stats[i].PV > 0 {
			statsWithData = append(statsWithData, &stats[i])
		}
	}

	assert.Len(t, statsWithData, 2) // Should have 2 hours with data

	// First hour with data should have 1 PV and 1 UV
	assert.Equal(t, 1, statsWithData[0].PV)
	assert.Equal(t, 1, statsWithData[0].UV)

	// Second hour with data should have 2 PV and 2 UV  
	assert.Equal(t, 2, statsWithData[1].PV)
	assert.Equal(t, 2, statsWithData[1].UV)
}

func TestService_calculateDailyStats(t *testing.T) {
	mockSearcher := &MockSearcher{}
	s := NewService(mockSearcher).(*service)

	// Create test data spanning multiple days
	result := &searcher.SearchResult{
		TotalHits: 3,
		Hits: []*searcher.SearchHit{
			{
				Fields: map[string]interface{}{
					"timestamp": float64(1640995800), // 2022-01-01 00:10:00 UTC
					"ip":        "192.168.1.1",
				},
			},
			{
				Fields: map[string]interface{}{
					"timestamp": float64(1641082200), // 2022-01-02 00:10:00 UTC
					"ip":        "192.168.1.2",
				},
			},
			{
				Fields: map[string]interface{}{
					"timestamp": float64(1641082800), // 2022-01-02 00:20:00 UTC
					"ip":        "192.168.1.1", // Same IP as first hit
				},
			},
		},
	}

	startTime := int64(1640995200) // 2022-01-01 00:00:00 UTC
	endTime := int64(1641168000)   // 2022-01-03 00:00:00 UTC

	stats := s.calculateDailyStats(result, startTime, endTime)

	assert.NotNil(t, stats)
	assert.Len(t, stats, 3) // Should have 3 days because we initialize for the full range

	// Verify stats are sorted by timestamp
	for i := 1; i < len(stats); i++ {
		assert.LessOrEqual(t, stats[i-1].Timestamp, stats[i].Timestamp)
	}

	// Find the days with data
	var day1Stats, day2Stats *DailyAccessStats
	for i := range stats {
		if stats[i].Date == "2022-01-01" {
			day1Stats = &stats[i]
		} else if stats[i].Date == "2022-01-02" {
			day2Stats = &stats[i]
		}
	}

	assert.NotNil(t, day1Stats)
	assert.NotNil(t, day2Stats)

	// Day 1 should have 1 PV and 1 UV
	assert.Equal(t, 1, day1Stats.PV)
	assert.Equal(t, 1, day1Stats.UV)

	// Day 2 should have 2 PV and 2 UV
	assert.Equal(t, 2, day2Stats.PV)
	assert.Equal(t, 2, day2Stats.UV)
}

// Test for the generic top field stats calculator
func Test_calculateTopFieldStats(t *testing.T) {
	facet := &searcher.Facet{
		Terms: []*searcher.FacetTerm{
			{Term: "/a", Count: 3},
			{Term: "/b", Count: 2},
			{Term: "/c", Count: 1},
		},
	}

	stats := calculateTopFieldStats(facet, 6, func(term string, count int, percent float64) URLAccessStats {
		return URLAccessStats{URL: term, Visits: count, Percent: percent}
	})

	assert.NotNil(t, stats)
	assert.Len(t, stats, 3) // Should have all 3 terms from facet

	// Should be sorted by visits descending
	assert.Equal(t, "/a", stats[0].URL)
	assert.Equal(t, 3, stats[0].Visits)
	assert.InDelta(t, 50.0, stats[0].Percent, 0.01) // 3/6

	assert.Equal(t, "/b", stats[1].URL)
	assert.Equal(t, 2, stats[1].Visits)
	assert.InDelta(t, 33.33, stats[1].Percent, 0.01) // 2/6
}

func TestService_calculateDashboardSummary(t *testing.T) {
	mockSearcher := &MockSearcher{}
	s := NewService(mockSearcher).(*service)

	analytics := &DashboardAnalytics{
		HourlyStats: []HourlyAccessStats{
			{Hour: 0, UV: 10, PV: 100},
			{Hour: 1, UV: 20, PV: 200}, // Peak hour
			{Hour: 2, UV: 15, PV: 150},
		},
		DailyStats: []DailyAccessStats{
			{Date: "2022-01-01", UV: 30, PV: 300},
			{Date: "2022-01-02", UV: 25, PV: 250},
		},
	}

	result := &searcher.SearchResult{
		TotalHits: 550,
		Hits: []*searcher.SearchHit{
			{Fields: map[string]interface{}{"ip": "192.168.1.1"}},
			{Fields: map[string]interface{}{"ip": "192.168.1.2"}},
			{Fields: map[string]interface{}{"ip": "192.168.1.1"}}, // Duplicate
		},
		Facets: map[string]*searcher.Facet{
			"ip": {
				Total: 2, // 2 unique IPs
				Terms: []*searcher.FacetTerm{
					{Term: "192.168.1.1", Count: 2},
					{Term: "192.168.1.2", Count: 1},
				},
			},
		},
	}

	summary := s.calculateDashboardSummary(analytics, result)

	assert.Equal(t, 2, summary.TotalUV)   // 2 unique IPs from hits
	assert.Equal(t, 550, summary.TotalPV) // Total hits from result

	// Average daily values (2 days)
	assert.InDelta(t, 1.0, summary.AvgDailyUV, 0.01) // 2 total UV / 2 days = 1
	assert.InDelta(t, 275.0, summary.AvgDailyPV, 0.01) // (300 + 250) / 2

	// Peak hour should be hour 1 with 200 PV
	assert.Equal(t, 1, summary.PeakHour)
	assert.Equal(t, 200, summary.PeakHourTraffic)
}