package analytics

import (
	"context"
	"fmt"

	"github.com/0xJacky/Nginx-UI/internal/nginx_log/searcher"
	"github.com/0xJacky/Nginx-UI/internal/nginx_log/utils"
)

// Service defines the interface for analytics operations
type Service interface {
	GetDashboardAnalytics(ctx context.Context, req *DashboardQueryRequest) (*DashboardAnalytics, error)

	GetLogEntriesStats(ctx context.Context, req *searcher.SearchRequest) (*EntriesStats, error)

	GetGeoDistribution(ctx context.Context, req *GeoQueryRequest) (*GeoDistribution, error)
	GetGeoDistributionByCountry(ctx context.Context, req *GeoQueryRequest, countryCode string) (*GeoDistribution, error)
	GetTopCountries(ctx context.Context, req *GeoQueryRequest) ([]CountryStats, error)
	GetTopCities(ctx context.Context, req *GeoQueryRequest) ([]CityStats, error)
	GetGeoStatsForIP(ctx context.Context, req *GeoQueryRequest, ip string) (*CityStats, error)

	GetTopPaths(ctx context.Context, req *TopListRequest) ([]KeyValue, error)
	GetTopIPs(ctx context.Context, req *TopListRequest) ([]KeyValue, error)
	GetTopUserAgents(ctx context.Context, req *TopListRequest) ([]KeyValue, error)

	ValidateLogPath(logPath string) error
	ValidateTimeRange(startTime, endTime int64) error
}

// service implements the Service interface
type service struct {
	searcher           searcher.Searcher
	cardinalityCounter *searcher.CardinalityCounter
}

// NewService creates a new analytics service
func NewService(s searcher.Searcher) Service {
	// Try to extract shards from distributed searcher for cardinality counting
	var cardinalityCounter *searcher.CardinalityCounter

	if ds, ok := s.(*searcher.DistributedSearcher); ok {
		shards := ds.GetShards()

		if len(shards) > 0 {
			cardinalityCounter = searcher.NewCardinalityCounter(shards)
		}
	}

	return &service{
		searcher:           s,
		cardinalityCounter: cardinalityCounter,
	}
}

// getCardinalityCounter dynamically creates or returns a cardinality counter
// This is necessary because shards may be updated after service initialization
func (s *service) getCardinalityCounter() *searcher.CardinalityCounter {
	// If we already have a cardinality counter and it's still valid, use it
	if s.cardinalityCounter != nil {
		return s.cardinalityCounter
	}

	// Try to create a new cardinality counter from current shards
	if ds, ok := s.searcher.(*searcher.DistributedSearcher); ok {
		shards := ds.GetShards()
		if len(shards) > 0 {
			// Update our cached cardinality counter
			s.cardinalityCounter = searcher.NewCardinalityCounter(shards)
			return s.cardinalityCounter
		}
	}

	return nil
}

// ValidateLogPath validates the log path against whitelist
func (s *service) ValidateLogPath(logPath string) error {
	if logPath == "" {
		return nil // Empty path is acceptable for global search
	}
	if !utils.IsValidLogPath(logPath) {
		return fmt.Errorf("log path is not under whitelist")
	}
	return nil
}

// ValidateTimeRange validates the time range parameters
func (s *service) ValidateTimeRange(startTime, endTime int64) error {
	if startTime < 0 || endTime < 0 {
		return fmt.Errorf("time values cannot be negative")
	}

	if startTime > 0 && endTime > 0 && startTime >= endTime {
		return fmt.Errorf("start time must be before end time")
	}

	return nil
}

// buildBaseSearchRequest builds a base search request with common parameters
func (s *service) buildBaseSearchRequest(startTime, endTime int64, logPath string) *searcher.SearchRequest {
	req := &searcher.SearchRequest{
		Limit:    DefaultLimit,
		Offset:   0,
		UseCache: true,
	}

	if startTime > 0 {
		req.StartTime = &startTime
	}

	if endTime > 0 {
		req.EndTime = &endTime
	}

	if logPath != "" {
		req.LogPaths = []string{logPath}
	}

	return req
}

// validateAndNormalizeSearchRequest validates and normalizes a search request
func (s *service) validateAndNormalizeSearchRequest(req *searcher.SearchRequest) error {
	if req == nil {
		return fmt.Errorf("request cannot be nil")
	}

	if req.Limit <= 0 {
		req.Limit = DefaultLimit
	}

	if req.Limit > MaxLimit {
		req.Limit = MaxLimit
	}

	if req.Offset < 0 {
		req.Offset = 0
	}

	return nil
}
