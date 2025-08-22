package analytics

import (
	"context"
	"fmt"
	"sort"

	"github.com/0xJacky/Nginx-UI/internal/nginx_log/searcher"
)

func (s *service) GetTrafficStats(ctx context.Context, req *TrafficStatsRequest) (*TrafficStats, error) {
	if req == nil {
		return nil, fmt.Errorf("request cannot be nil")
	}

	searchReq := &searcher.SearchRequest{
		StartTime:    &req.StartTime,
		EndTime:      &req.EndTime,
		LogPaths:     req.LogPaths,
		Limit:        1, // We only need the total count and stats
		IncludeStats: true,
		UseCache:     true,
	}

	result, err := s.searcher.Search(ctx, searchReq)
	if err != nil {
		return nil, fmt.Errorf("failed to search for traffic stats: %w", err)
	}

	var totalBytes int64
	if result.Stats != nil {
		totalBytes = result.Stats.TotalBytes
	}

	return &TrafficStats{
		TotalRequests: int(result.TotalHits),
		TotalBytes:    totalBytes,
	}, nil
}

func (s *service) GetVisitorsByTime(ctx context.Context, req *VisitorsByTimeRequest) (*VisitorsByTime, error) {
	if req == nil {
		return nil, fmt.Errorf("request cannot be nil")
	}

	searchReq := &searcher.SearchRequest{
		StartTime:     &req.StartTime,
		EndTime:       &req.EndTime,
		LogPaths:      req.LogPaths,
		Limit:         0,
		IncludeFacets: true,
		FacetFields:   []string{"region_code"},
		FacetSize:     300, // Large enough for all countries
		UseCache:      true,
	}

	result, err := s.searcher.Search(ctx, searchReq)
	if err != nil {
		return nil, fmt.Errorf("failed to get visitors by country: %w", err)
	}

	visitorMap := make(map[int64]map[string]bool)
	interval := int64(req.IntervalSeconds)
	if interval <= 0 {
		interval = 60 // Default to 1 minute
	}

	for _, hit := range result.Hits {
		if timestampField, ok := hit.Fields["timestamp"]; ok {
			if timestampFloat, ok := timestampField.(float64); ok {
				timestamp := int64(timestampFloat)
				bucket := (timestamp / interval) * interval
				if visitorMap[bucket] == nil {
					visitorMap[bucket] = make(map[string]bool)
				}
				if ip, ok := hit.Fields["ip"].(string); ok {
					visitorMap[bucket][ip] = true
				}
			}
		}
	}

	var visitorsByTime []TimeValue
	for timestamp, ips := range visitorMap {
		visitorsByTime = append(visitorsByTime, TimeValue{
			Timestamp: timestamp,
			Value:     len(ips),
		})
	}
	sort.Slice(visitorsByTime, func(i, j int) bool {
		return visitorsByTime[i].Timestamp < visitorsByTime[j].Timestamp
	})

	return &VisitorsByTime{Data: visitorsByTime}, nil
}

func (s *service) GetVisitorsByCountry(ctx context.Context, req *VisitorsByCountryRequest) (*VisitorsByCountry, error) {
	if req == nil {
		return nil, fmt.Errorf("request cannot be nil")
	}

	searchReq := &searcher.SearchRequest{
		StartTime:     &req.StartTime,
		EndTime:       &req.EndTime,
		LogPaths:      req.LogPaths,
		Limit:         0,
		IncludeFacets: true,
		FacetFields:   []string{"region_code"},
		FacetSize:     300, // Large enough for all countries
		UseCache:      true,
	}

	result, err := s.searcher.Search(ctx, searchReq)
	if err != nil {
		return nil, fmt.Errorf("failed to get visitors by country: %w", err)
	}

	countryMap := make(map[string]int)
	if result.Facets != nil {
		if countryFacet, ok := result.Facets["region_code"]; ok {
			for _, term := range countryFacet.Terms {
				countryMap[term.Term] = term.Count
			}
		}
	}

	return &VisitorsByCountry{Data: countryMap}, nil
}

func (s *service) GetTopRequests(ctx context.Context, req *TopRequestsRequest) (*TopRequests, error) {
	if req == nil {
		return nil, fmt.Errorf("request cannot be nil")
	}

	searchReq := &searcher.SearchRequest{
		StartTime: &req.StartTime,
		EndTime:   &req.EndTime,
		LogPaths:  req.LogPaths,
		Limit:     req.Limit,
		SortBy:    req.SortBy,
		SortOrder: req.SortOrder,
		UseCache:  true,
	}

	result, err := s.searcher.Search(ctx, searchReq)
	if err != nil {
		return nil, fmt.Errorf("failed to get top requests: %w", err)
	}

	// For now, we return an empty list as the RequestInfo struct is not fully defined.
	var requests []RequestInfo

	return &TopRequests{
		Total:    int(result.TotalHits),
		Requests: requests,
	}, nil
}

func (s *service) GetErrorDistribution(ctx context.Context, req *ErrorDistributionRequest) (*ErrorDistribution, error) {
	if req == nil {
		return nil, fmt.Errorf("request cannot be nil")
	}

	searchReq := &searcher.SearchRequest{
		StartTime:     &req.StartTime,
		EndTime:       &req.EndTime,
		LogPaths:      req.LogPaths,
		Limit:         0,
		IncludeFacets: true,
		FacetFields:   []string{"status"},
		FacetSize:     200,                   // More than enough for all status codes
		Query:         "status:[400 TO 599]", // Filter for error codes
		UseCache:      true,
	}

	result, err := s.searcher.Search(ctx, searchReq)
	if err != nil {
		return nil, fmt.Errorf("failed to get error distribution: %w", err)
	}

	dist := make(map[string]int)
	if result.Facets != nil {
		if statusFacet, ok := result.Facets["status"]; ok {
			for _, term := range statusFacet.Terms {
				dist[term.Term] = term.Count
			}
		}
	}

	return &ErrorDistribution{Data: dist}, nil
}

func (s *service) GetRequestRate(ctx context.Context, req *RequestRateRequest) (*RequestRate, error) {
	if req == nil {
		return nil, fmt.Errorf("request cannot be nil")
	}

	searchReq := &searcher.SearchRequest{
		StartTime: &req.StartTime,
		EndTime:   &req.EndTime,
		LogPaths:  req.LogPaths,
		Limit:     1, // We only need total hits
		UseCache:  true,
	}

	result, err := s.searcher.Search(ctx, searchReq)
	if err != nil {
		return nil, fmt.Errorf("failed to get request rate: %w", err)
	}

	duration := req.EndTime - req.StartTime
	var rate float64
	if duration > 0 {
		rate = float64(result.TotalHits) / float64(duration)
	}

	return &RequestRate{
		TotalRequests: int(result.TotalHits),
		Rate:          rate,
	}, nil
}

func (s *service) GetBandwidthUsage(ctx context.Context, req *BandwidthUsageRequest) (*BandwidthUsage, error) {
	if req == nil {
		return nil, fmt.Errorf("request cannot be nil")
	}

	searchReq := &searcher.SearchRequest{
		StartTime:    &req.StartTime,
		EndTime:      &req.EndTime,
		LogPaths:     req.LogPaths,
		Limit:        1,
		IncludeStats: true,
		UseCache:     true,
	}

	result, err := s.searcher.Search(ctx, searchReq)
	if err != nil {
		return nil, fmt.Errorf("failed to get bandwidth usage: %w", err)
	}

	var totalBytes int64
	if result.Stats != nil {
		totalBytes = result.Stats.TotalBytes
	}

	return &BandwidthUsage{
		TotalRequests: int(result.TotalHits),
		TotalBytes:    totalBytes,
	}, nil
}

func (s *service) GetVisitorPaths(ctx context.Context, req *VisitorPathsRequest) (*VisitorPaths, error) {
	if req == nil {
		return nil, fmt.Errorf("request cannot be nil")
	}

	searchReq := &searcher.SearchRequest{
		StartTime: &req.StartTime,
		EndTime:   &req.EndTime,
		LogPaths:  req.LogPaths,
		Limit:     req.Limit,
		Offset:    req.Offset,
		Query:     fmt.Sprintf(`ip:"%s"`, req.IP),
		SortBy:    "timestamp",
		SortOrder: "asc",
		UseCache:  true,
	}

	result, err := s.searcher.Search(ctx, searchReq)
	if err != nil {
		return nil, fmt.Errorf("failed to get visitor paths: %w", err)
	}

	var paths []VisitorPath
	for _, hit := range result.Hits {
		path, _ := hit.Fields["path"].(string)
		ts, _ := hit.Fields["timestamp"].(float64)
		paths = append(paths, VisitorPath{
			Path:      path,
			Timestamp: int64(ts),
		})
	}

	return &VisitorPaths{
		Total: int(result.TotalHits),
		Paths: paths,
	}, nil
}
