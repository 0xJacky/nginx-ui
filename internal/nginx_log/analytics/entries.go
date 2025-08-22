package analytics

import (
	"context"
	"fmt"
	"sort"

	"github.com/0xJacky/Nginx-UI/internal/nginx_log/searcher"
)

func (s *service) GetLogEntriesStats(ctx context.Context, req *searcher.SearchRequest) (*EntriesStats, error) {
	if req == nil {
		return nil, fmt.Errorf("request cannot be nil")
	}

	// Ensure facets are included for stats calculation
	req.IncludeFacets = true
	req.FacetFields = []string{"status", "method", "path_exact", "ip", "user_agent"}
	req.FacetSize = 10 // Top 10 for lists

	result, err := s.searcher.Search(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to search logs for entries stats: %w", err)
	}

	stats := &EntriesStats{
		TotalEntries:   int64(result.TotalHits),
		StatusCodeDist: make(map[string]int),
		MethodDist:     make(map[string]int),
		TopPaths:       make([]KeyValue, 0),
		TopIPs:         make([]KeyValue, 0),
		TopUserAgents:  make([]KeyValue, 0),
	}

	if result.Facets != nil {
		if statusFacet, ok := result.Facets["status"]; ok {
			for _, term := range statusFacet.Terms {
				stats.StatusCodeDist[term.Term] = term.Count
			}
		}
		if methodFacet, ok := result.Facets["method"]; ok {
			for _, term := range methodFacet.Terms {
				stats.MethodDist[term.Term] = term.Count
			}
		}
		if pathFacet, ok := result.Facets["path_exact"]; ok {
			for _, term := range pathFacet.Terms {
				stats.TopPaths = append(stats.TopPaths, KeyValue{Key: term.Term, Value: term.Count})
			}
		}
		if ipFacet, ok := result.Facets["ip"]; ok {
			for _, term := range ipFacet.Terms {
				stats.TopIPs = append(stats.TopIPs, KeyValue{Key: term.Term, Value: term.Count})
			}
		}
		if uaFacet, ok := result.Facets["user_agent"]; ok {
			for _, term := range uaFacet.Terms {
				stats.TopUserAgents = append(stats.TopUserAgents, KeyValue{Key: term.Term, Value: term.Count})
			}
		}
	}

	// Populate stats if available
	if result.Stats != nil {
		stats.BytesStats = &BytesStatistics{
			Total:   result.Stats.TotalBytes,
			Average: result.Stats.AvgBytes,
			Min:     result.Stats.MinBytes,
			Max:     result.Stats.MaxBytes,
		}
		stats.ResponseTimeStats = &ResponseTimeStatistics{
			Average: result.Stats.AvgReqTime,
			Min:     result.Stats.MinReqTime,
			Max:     result.Stats.MaxReqTime,
		}
	}

	return stats, nil
}

// getTopKeyValuesFromMap is a helper to convert a map of counts to a sorted KeyValue slice.
func getTopKeyValuesFromMap(counts map[string]int, limit int) []KeyValue {
	kvs := make([]KeyValue, 0, len(counts))
	for k, v := range counts {
		kvs = append(kvs, KeyValue{Key: k, Value: v})
	}

	sort.Slice(kvs, func(i, j int) bool {
		return kvs[i].Value > kvs[j].Value
	})

	if limit > 0 && len(kvs) > limit {
		return kvs[:limit]
	}
	return kvs
}

func (s *service) GetTopPaths(ctx context.Context, req *TopListRequest) ([]KeyValue, error) {
	if req == nil {
		return nil, fmt.Errorf("request cannot be nil")
	}

	if err := s.ValidateTimeRange(req.StartTime, req.EndTime); err != nil {
		return nil, err
	}

	searchReq := &searcher.SearchRequest{
		StartTime:     &req.StartTime,
		EndTime:       &req.EndTime,
		LogPaths:      []string{req.LogPath},
		Limit:         0, // We only need facets
		IncludeFacets: true,
		FacetFields:   []string{"path_exact"},
		FacetSize:     req.Limit,
		UseCache:      true,
	}

	result, err := s.searcher.Search(ctx, searchReq)
	if err != nil {
		return nil, fmt.Errorf("failed to get top paths: %w", err)
	}

	topPaths := make([]KeyValue, 0)
	if result.Facets != nil {
		if pathFacet, ok := result.Facets["path_exact"]; ok {
			for _, term := range pathFacet.Terms {
				topPaths = append(topPaths, KeyValue{Key: term.Term, Value: term.Count})
			}
		}
	}

	return topPaths, nil
}

func (s *service) GetTopIPs(ctx context.Context, req *TopListRequest) ([]KeyValue, error) {
	if req == nil {
		return nil, fmt.Errorf("request cannot be nil")
	}

	if err := s.ValidateTimeRange(req.StartTime, req.EndTime); err != nil {
		return nil, err
	}

	searchReq := &searcher.SearchRequest{
		StartTime:     &req.StartTime,
		EndTime:       &req.EndTime,
		LogPaths:      []string{req.LogPath},
		Limit:         0,
		IncludeFacets: true,
		FacetFields:   []string{"ip"},
		FacetSize:     req.Limit,
		UseCache:      true,
	}

	result, err := s.searcher.Search(ctx, searchReq)
	if err != nil {
		return nil, fmt.Errorf("failed to get top IPs: %w", err)
	}

	topIPs := make([]KeyValue, 0)
	if result.Facets != nil {
		if ipFacet, ok := result.Facets["ip"]; ok {
			for _, term := range ipFacet.Terms {
				topIPs = append(topIPs, KeyValue{Key: term.Term, Value: term.Count})
			}
		}
	}

	return topIPs, nil
}

func (s *service) GetTopUserAgents(ctx context.Context, req *TopListRequest) ([]KeyValue, error) {
	if req == nil {
		return nil, fmt.Errorf("request cannot be nil")
	}

	if err := s.ValidateTimeRange(req.StartTime, req.EndTime); err != nil {
		return nil, err
	}

	searchReq := &searcher.SearchRequest{
		StartTime:     &req.StartTime,
		EndTime:       &req.EndTime,
		LogPaths:      []string{req.LogPath},
		Limit:         0,
		IncludeFacets: true,
		FacetFields:   []string{"user_agent"},
		FacetSize:     req.Limit,
		UseCache:      true,
	}

	result, err := s.searcher.Search(ctx, searchReq)
	if err != nil {
		return nil, fmt.Errorf("failed to get top user agents: %w", err)
	}

	topUserAgents := make([]KeyValue, 0)
	if result.Facets != nil {
		if uaFacet, ok := result.Facets["user_agent"]; ok {
			for _, term := range uaFacet.Terms {
				topUserAgents = append(topUserAgents, KeyValue{Key: term.Term, Value: term.Count})
			}
		}
	}

	return topUserAgents, nil
}
