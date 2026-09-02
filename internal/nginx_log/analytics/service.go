package analytics

import (
	"context"
	"fmt"
	"sync"

	"github.com/0xJacky/Nginx-UI/internal/nginx_log/searcher"
	"github.com/0xJacky/Nginx-UI/internal/nginx_log/utils"
	"github.com/blevesearch/bleve/v2"
)

// Service defines the interface for analytics operations
type Service interface {
	GetDashboardAnalytics(ctx context.Context, req *DashboardQueryRequest) (*DashboardAnalytics, error)

	GetLogEntriesStats(ctx context.Context, req *searcher.SearchRequest) (*EntriesStats, error)

	GetGeoDistribution(ctx context.Context, req *GeoQueryRequest) (*GeoDistribution, error)
	GetGeoDistributionByCountry(ctx context.Context, req *GeoQueryRequest, countryCode string) (*GeoDistribution, error)
	GetGeoDistributionByProvince(ctx context.Context, req *GeoQueryRequest, countryCode, province string) (*GeoDistribution, error)
	GetTopCountries(ctx context.Context, req *GeoQueryRequest) ([]CountryStats, error)

	ValidateLogPath(logPath string) error
	ValidateTimeRange(startTime, endTime int64) error

	Stop() error
}

// service implements the Service interface
type service struct {
	searcher searcher.SearcherInterface

	counterMu          sync.Mutex
	cardinalityCounter *searcher.Counter
	counterShards      []bleve.Index // Shards the counter was built from, to detect swaps
}

// NewService creates a new analytics service
func NewService(s searcher.SearcherInterface) Service {
	// The cardinality counter is created lazily on first use so it always
	// reflects the searcher's current shards.
	return &service{
		searcher: s,
	}
}

// Stop gracefully stops the analytics service and its components
func (s *service) Stop() error {
	s.counterMu.Lock()
	defer s.counterMu.Unlock()
	if s.cardinalityCounter != nil {
		counter := s.cardinalityCounter
		s.cardinalityCounter = nil
		s.counterShards = nil
		return counter.Stop()
	}
	return nil
}

// getCardinalityCounter returns a cardinality counter built from the
// searcher's current shards. After an index rebuild the searcher swaps its
// shards (and closes the old ones), so a counter built earlier would query
// stale shard handles; rebuild it whenever the shard set changes.
func (s *service) getCardinalityCounter() *searcher.Counter {
	ds, ok := s.searcher.(*searcher.Searcher)
	if !ok {
		return nil
	}

	shards := ds.GetShards()
	if len(shards) == 0 {
		return nil
	}

	s.counterMu.Lock()
	defer s.counterMu.Unlock()

	if s.cardinalityCounter != nil && sameShards(s.counterShards, shards) {
		return s.cardinalityCounter
	}

	if s.cardinalityCounter != nil {
		// Closing the counter only closes its IndexAlias wrapper, not the
		// underlying shards, so this is safe while searches are in flight.
		_ = s.cardinalityCounter.Stop()
	}

	s.cardinalityCounter = searcher.NewCounter(shards)
	s.counterShards = append([]bleve.Index(nil), shards...)
	return s.cardinalityCounter
}

// sameShards reports whether two shard slices contain the same indexes in order.
func sameShards(a, b []bleve.Index) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
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
