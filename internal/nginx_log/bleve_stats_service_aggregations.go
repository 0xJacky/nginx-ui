package nginx_log

import (
	"context"

	"github.com/blevesearch/bleve/v2/search/query"
)

// calculateTopURLsFromBleve calculates top URLs using Bleve aggregations
func (s *BleveStatsService) calculateTopURLsFromBleve(ctx context.Context, baseQuery query.Query) ([]URLAccessStats, error) {
	results, err := s.aggregateFieldFromBleve(ctx, baseQuery, "path", extractPathField)
	if err != nil {
		return nil, err
	}

	// Take top 10 and convert to URLAccessStats format
	if len(results) > 10 {
		results = results[:10]
	}

	urlStats := make([]URLAccessStats, len(results))
	for i, result := range results {
		urlStats[i] = URLAccessStats{
			URL:     result.Field,
			Visits:  result.Count,
			Percent: result.Percent,
		}
	}

	return urlStats, nil
}

// calculateBrowserStatsFromBleve calculates browser statistics using Bleve
func (s *BleveStatsService) calculateBrowserStatsFromBleve(ctx context.Context, baseQuery query.Query) ([]BrowserAccessStats, error) {
	results, err := s.aggregateFieldFromBleve(ctx, baseQuery, "browser", extractBrowserField)
	if err != nil {
		return nil, err
	}

	// Convert to BrowserAccessStats format
	browserStats := make([]BrowserAccessStats, len(results))
	for i, result := range results {
		browserStats[i] = BrowserAccessStats{
			Browser: result.Field,
			Count:   result.Count,
			Percent: result.Percent,
		}
	}

	return browserStats, nil
}

// calculateOSStatsFromBleve calculates OS statistics using Bleve
func (s *BleveStatsService) calculateOSStatsFromBleve(ctx context.Context, baseQuery query.Query) ([]OSAccessStats, error) {
	results, err := s.aggregateFieldFromBleve(ctx, baseQuery, "os", extractOSField)
	if err != nil {
		return nil, err
	}

	// Convert to OSAccessStats format
	osStats := make([]OSAccessStats, len(results))
	for i, result := range results {
		osStats[i] = OSAccessStats{
			OS:      result.Field,
			Count:   result.Count,
			Percent: result.Percent,
		}
	}

	return osStats, nil
}

// calculateDeviceStatsFromBleve calculates device statistics using Bleve
func (s *BleveStatsService) calculateDeviceStatsFromBleve(ctx context.Context, baseQuery query.Query) ([]DeviceAccessStats, error) {
	results, err := s.aggregateFieldFromBleve(ctx, baseQuery, "device_type", extractDeviceField)
	if err != nil {
		return nil, err
	}

	// Convert to DeviceAccessStats format
	deviceStats := make([]DeviceAccessStats, len(results))
	for i, result := range results {
		deviceStats[i] = DeviceAccessStats{
			Device:  result.Field,
			Count:   result.Count,
			Percent: result.Percent,
		}
	}

	return deviceStats, nil
}