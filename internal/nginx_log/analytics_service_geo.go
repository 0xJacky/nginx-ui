package nginx_log

import (
	"context"
	"fmt"
	"time"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/search/query"
	"github.com/uozi-tech/cosy/logger"
)

// GetWorldMapData returns geographic data for world map visualization
func (s *AnalyticsService) GetWorldMapData(ctx context.Context, logPath string, startTime, endTime time.Time) ([]WorldMapData, error) {
	if s.indexer == nil {
		return nil, fmt.Errorf("indexer not available")
	}

	if !IsLogPathUnderWhiteList(logPath) {
		return nil, fmt.Errorf("log path is not under whitelist")
	}

	// Get stats service
	statsService := NewBleveStatsService()
	statsService.SetIndexer(s.indexer)

	// Build base query
	baseQuery, err := s.buildTimeRangeQuery(logPath, startTime, endTime)
	if err != nil {
		return nil, fmt.Errorf("failed to build query: %w", err)
	}

	// Get world map data from Bleve stats service
	return statsService.GetWorldMapData(ctx, baseQuery)
}

// GetChinaMapData returns geographic data for China map visualization
func (s *AnalyticsService) GetChinaMapData(ctx context.Context, logPath string, startTime, endTime time.Time) ([]ChinaMapData, error) {
	if s.indexer == nil {
		return nil, fmt.Errorf("indexer not available")
	}

	if !IsLogPathUnderWhiteList(logPath) {
		return nil, fmt.Errorf("log path is not under whitelist")
	}

	// Get stats service
	statsService := NewBleveStatsService()
	statsService.SetIndexer(s.indexer)

	// Build base query
	baseQuery, err := s.buildTimeRangeQuery(logPath, startTime, endTime)
	if err != nil {
		return nil, fmt.Errorf("failed to build query: %w", err)
	}

	// Get China map data from Bleve stats service
	return statsService.GetChinaMapData(ctx, baseQuery)
}

// GetGeoStats returns geographic statistics
func (s *AnalyticsService) GetGeoStats(ctx context.Context, logPath string, startTime, endTime time.Time, limit int) ([]GeoStats, error) {
	if s.indexer == nil {
		return nil, fmt.Errorf("indexer not available")
	}

	if !IsLogPathUnderWhiteList(logPath) {
		return nil, fmt.Errorf("log path is not under whitelist")
	}

	// Get stats service
	statsService := NewBleveStatsService()
	statsService.SetIndexer(s.indexer)

	// Build base query
	baseQuery, err := s.buildTimeRangeQuery(logPath, startTime, endTime)
	if err != nil {
		return nil, fmt.Errorf("failed to build query: %w", err)
	}

	// Get geographic statistics from Bleve stats service
	return statsService.GetGeoStats(ctx, baseQuery, limit)
}

// buildTimeRangeQuery builds a query for the given time range and log path
func (s *AnalyticsService) buildTimeRangeQuery(logPath string, startTime, endTime time.Time) (query.Query, error) {
	var queries []query.Query

	// Add file path filter if specified
	if logPath != "" {
		filePathQuery := bleve.NewTermQuery(logPath)
		filePathQuery.SetField("file_path")
		queries = append(queries, filePathQuery)
	}

	// Add time range filter if specified
	if !startTime.IsZero() || !endTime.IsZero() {
		var start, end *float64
		
		if !startTime.IsZero() {
			startFloat := float64(startTime.Unix())
			start = &startFloat
		}
		
		if !endTime.IsZero() {
			endFloat := float64(endTime.Unix())
			end = &endFloat
		}
		
		numericQuery := bleve.NewNumericRangeQuery(start, end)
		numericQuery.SetField("timestamp")
		queries = append(queries, numericQuery)
		
		logger.Debugf("Time range query: start=%v (%v), end=%v (%v)", startTime, start, endTime, end)
	}

	// Combine queries
	switch len(queries) {
	case 0:
		return bleve.NewMatchAllQuery(), nil
	case 1:
		return queries[0], nil
	default:
		return bleve.NewConjunctionQuery(queries...), nil
	}
}