package analytics

import (
	"context"
	"fmt"

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
