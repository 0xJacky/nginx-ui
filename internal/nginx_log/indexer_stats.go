package nginx_log

import (
	"context"
	"crypto/sha256"
	"fmt"
	"time"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/search/query"
	"github.com/uozi-tech/cosy/logger"
)

// createStatsQueryHash creates a hash for the given query to use as cache key
func (li *LogIndexer) createStatsQueryHash(query query.Query) string {
	queryStr := fmt.Sprintf("%+v", query)
	hash := sha256.Sum256([]byte(queryStr))
	return fmt.Sprintf("stats_%x", hash[:16]) // Use first 16 bytes for shorter key
}

// getLatestFilesModTime returns the latest modification time of all registered log files
func (li *LogIndexer) getLatestFilesModTime() time.Time {
	li.mu.RLock()
	defer li.mu.RUnlock()
	
	var latest time.Time
	for _, fileInfo := range li.logPaths {
		if fileInfo.LastModified.After(latest) {
			latest = fileInfo.LastModified
		}
	}
	return latest
}

// isCacheValid checks if the cached statistics are still valid
func (li *LogIndexer) isCacheValid(cached *CachedStatsResult) bool {
	// Get current document count
	docCount, err := li.index.DocCount()
	if err != nil {
		logger.Warnf("Failed to get document count for cache validation: %v", err)
		return false
	}
	
	// Get latest file modification time
	latestModTime := li.getLatestFilesModTime()
	
	// Cache is valid if:
	// 1. Document count hasn't changed
	// 2. No files have been modified since cache was created
	// 3. Cache is not older than 5 minutes (safety fallback)
	isValid := cached.DocCount == docCount &&
		!latestModTime.After(cached.FilesModTime) &&
		time.Since(cached.LastCalculated) < 5*time.Minute
	
	if !isValid {
		logger.Infof("Cache invalid - DocCount: %d->%d, ModTime: %v->%v, Age: %v",
			cached.DocCount, docCount, cached.FilesModTime, latestModTime, time.Since(cached.LastCalculated))
	}
	
	return isValid
}

// calculateSummaryStatsFromQuery calculates summary statistics using optimized single query approach with caching
func (li *LogIndexer) calculateSummaryStatsFromQuery(ctx context.Context, query query.Query) (*SummaryStats, error) {
	// Create cache key
	cacheKey := li.createStatsQueryHash(query)
	
	// Check cache first
	if cached, found := li.statsCache.Get(cacheKey); found {
		if li.isCacheValid(cached) {
			logger.Infof("Stats cache hit for key: %s", cacheKey)
			return cached.Stats, nil
		} else {
			logger.Infof("Stats cache invalid for key: %s, recalculating", cacheKey)
			// Remove invalid cache entry
			li.statsCache.Del(cacheKey)
		}
	}
	
	logger.Infof("Stats cache miss for key: %s, calculating...", cacheKey)
	// Get total page views (PV) - just the count
	countReq := bleve.NewSearchRequest(query)
	countReq.Size = 0 // Don't fetch any documents, just get the count
	countResult, err := li.index.SearchInContext(ctx, countReq)
	if err != nil {
		return nil, fmt.Errorf("count search failed: %w", err)
	}

	pv := int(countResult.Total)
	if pv == 0 {
		return &SummaryStats{}, nil
	}

	logger.Infof("Total page views (PV): %d", pv)

	// Determine sample size for large datasets to improve performance
	var sampleSize int
	if pv <= 10000 {
		// For smaller datasets, use all results for accuracy
		sampleSize = pv
	} else {
		// For large datasets, use sampling to improve performance
		sampleSize = 10000
		logger.Infof("Using sampling for performance: %d out of %d total results", sampleSize, pv)
	}

	// Single query to get all needed fields at once
	statsReq := bleve.NewSearchRequest(query)
	statsReq.Size = sampleSize
	statsReq.From = 0
	statsReq.Fields = []string{"ip", "path", "bytes_sent"} // Get all needed fields in one query
	
	statsResult, err := li.index.SearchInContext(ctx, statsReq)
	if err != nil {
		return nil, fmt.Errorf("stats aggregation search failed: %w", err)
	}

	logger.Infof("Processing %d hits for statistics calculation", len(statsResult.Hits))

	// Calculate all statistics in a single pass
	uniqueIPs := make(map[string]bool)
	uniquePages := make(map[string]bool)
	var totalTraffic int64

	for _, hit := range statsResult.Hits {
		if fields := hit.Fields; fields != nil {
			// IP for UV calculation
			if ip := li.getStringField(fields, "ip"); ip != "" {
				uniqueIPs[ip] = true
			}

			// Path for unique pages calculation
			if path := li.getStringField(fields, "path"); path != "" {
				uniquePages[path] = true
			}

			// Bytes sent for traffic calculation
			if bytesSent := li.getFloatField(fields, "bytes_sent"); bytesSent > 0 {
				totalTraffic += int64(bytesSent)
			}
		}
	}

	// If we used sampling, scale up the traffic
	if sampleSize < pv && len(statsResult.Hits) > 0 {
		scalingFactor := float64(pv) / float64(len(statsResult.Hits))
		totalTraffic = int64(float64(totalTraffic) * scalingFactor)
		logger.Infof("Scaled traffic from sample: factor=%.2f, scaled_traffic=%d", scalingFactor, totalTraffic)
	}

	// Calculate average traffic per page view
	var avgTrafficPerPV float64
	if pv > 0 {
		avgTrafficPerPV = float64(totalTraffic) / float64(pv)
	}

	uv := len(uniqueIPs)
	uniquePagesCount := len(uniquePages)

	logger.Infof("Summary calculation results: UV=%d, PV=%d, Traffic=%d, UniquePages=%d, AvgTrafficPerPV=%.2f (from %d samples)", 
		uv, pv, totalTraffic, uniquePagesCount, avgTrafficPerPV, len(statsResult.Hits))

	stats := &SummaryStats{
		UV:              uv,
		PV:              pv,
		TotalTraffic:    totalTraffic,
		UniquePages:     uniquePagesCount,
		AvgTrafficPerPV: avgTrafficPerPV,
	}

	// Cache the results
	docCount, err := li.index.DocCount()
	if err != nil {
		logger.Warnf("Failed to get document count for caching: %v", err)
		docCount = 0 // Continue without caching on error
	}

	cachedResult := &CachedStatsResult{
		Stats:          stats,
		QueryHash:      cacheKey,
		LastCalculated: time.Now(),
		FilesModTime:   li.getLatestFilesModTime(),
		DocCount:       docCount,
	}

	// Store in cache with estimated size (small structures, so use fixed size)
	li.statsCache.Set(cacheKey, cachedResult, 1024) // 1KB estimated size
	logger.Infof("Cached stats result for key: %s", cacheKey)

	return stats, nil
}

// invalidateStatsCache clears the statistics cache when data changes
func (li *LogIndexer) invalidateStatsCache() {
	// Clear all stats cache entries since we don't know which queries might be affected
	li.statsCache.Clear()
	logger.Infof("Statistics cache invalidated due to data changes")
}

// GetStatsCacheStatus returns statistics about the stats cache for monitoring
func (li *LogIndexer) GetStatsCacheStatus() map[string]interface{} {
	metrics := li.statsCache.Metrics
	return map[string]interface{}{
		"hits":        metrics.Hits(),
		"misses":      metrics.Misses(),
		"cost_added":  metrics.CostAdded(),
		"cost_evicted": metrics.CostEvicted(),
		"sets_dropped": metrics.SetsDropped(),
		"sets_rejected": metrics.SetsRejected(),
		"gets_kept":   metrics.GetsKept(),
		"gets_dropped": metrics.GetsDropped(),
	}
}

// calculateSummaryStats calculates summary statistics from the given entries (kept for compatibility)
func (li *LogIndexer) calculateSummaryStats(entries []*AccessLogEntry) *SummaryStats {
	if len(entries) == 0 {
		return &SummaryStats{}
	}

	// UV: Unique Visitors (unique IPs)
	uniqueIPs := make(map[string]bool)
	for _, entry := range entries {
		if entry.IP != "" {
			uniqueIPs[entry.IP] = true
		}
	}
	uv := len(uniqueIPs)

	// PV: Page Views (total requests)
	pv := len(entries)

	// Traffic: Total bytes sent
	var totalTraffic int64
	for _, entry := range entries {
		totalTraffic += entry.BytesSent
	}

	// Unique pages visited
	uniquePages := make(map[string]bool)
	for _, entry := range entries {
		if entry.Path != "" {
			uniquePages[entry.Path] = true
		}
	}

	// Average traffic per page view
	var avgTrafficPerPV float64
	if pv > 0 {
		avgTrafficPerPV = float64(totalTraffic) / float64(pv)
	}

	return &SummaryStats{
		UV:              uv,
		PV:              pv,
		TotalTraffic:    totalTraffic,
		UniquePages:     len(uniquePages),
		AvgTrafficPerPV: avgTrafficPerPV,
	}
}