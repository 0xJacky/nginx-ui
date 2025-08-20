package nginx_log

import (
	"context"
	"fmt"
	"sort"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/search"
	"github.com/blevesearch/bleve/v2/search/query"
)

// BleveFieldExtractor defines how to extract a field value from a search hit
type BleveFieldExtractor func(hit *search.DocumentMatch) string

// BleveAggregationResult represents the result of field aggregation
type BleveAggregationResult struct {
	Field         string
	Count         int
	TotalRequests int
	Percent       float64
}

// aggregateFieldFromBleve performs field aggregation using Bleve search
func (s *BleveStatsService) aggregateFieldFromBleve(ctx context.Context, baseQuery query.Query, fieldName string, extractor BleveFieldExtractor) ([]BleveAggregationResult, error) {
	fieldCount := make(map[string]int)
	totalRequests := 0

	// Query all entries
	searchReq := bleve.NewSearchRequest(baseQuery)
	searchReq.Size = 10000
	searchReq.Fields = []string{fieldName}

	from := 0
	for {
		searchReq.From = from
		searchResult, err := s.indexer.index.Search(searchReq)
		if err != nil {
			return nil, fmt.Errorf("failed to search logs: %w", err)
		}

		if len(searchResult.Hits) == 0 {
			break
		}

		for _, hit := range searchResult.Hits {
			fieldValue := extractor(hit)
			if fieldValue == "" {
				fieldValue = "Unknown"
			}
			fieldCount[fieldValue]++
			totalRequests++
		}

		from += len(searchResult.Hits)
		if uint64(from) >= searchResult.Total {
			break
		}
	}

	// Convert to slice and sort
	var results []BleveAggregationResult
	for field, count := range fieldCount {
		percent := 0.0
		if totalRequests > 0 {
			percent = float64(count) * 100.0 / float64(totalRequests)
		}

		results = append(results, BleveAggregationResult{
			Field:         field,
			Count:         count,
			TotalRequests: totalRequests,
			Percent:       percent,
		})
	}

	// Sort by count (descending)
	sort.Slice(results, func(i, j int) bool {
		return results[i].Count > results[j].Count
	})

	return results, nil
}

// Standard field extractors
func extractPathField(hit *search.DocumentMatch) string {
	if pathField, ok := hit.Fields["path"]; ok {
		if path, ok := pathField.(string); ok && path != "" {
			return path
		}
	}
	return ""
}

func extractBrowserField(hit *search.DocumentMatch) string {
	if browserField, ok := hit.Fields["browser"]; ok {
		if browser, ok := browserField.(string); ok && browser != "" {
			return browser
		}
	}
	return ""
}

func extractOSField(hit *search.DocumentMatch) string {
	if osField, ok := hit.Fields["os"]; ok {
		if os, ok := osField.(string); ok && os != "" {
			return os
		}
	}
	return ""
}

func extractDeviceField(hit *search.DocumentMatch) string {
	if deviceField, ok := hit.Fields["device_type"]; ok {
		if device, ok := deviceField.(string); ok && device != "" {
			return device
		}
	}
	return ""
}