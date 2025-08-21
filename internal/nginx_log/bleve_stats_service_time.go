package nginx_log

import (
	"context"
	"fmt"
	"time"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/search/query"
	"github.com/uozi-tech/cosy/logger"
)

// calculateHourlyStatsFromBleve calculates 24-hour UV/PV statistics using Bleve aggregations
// Shows stats for the End Date (target day) only
func (s *BleveStatsService) calculateHourlyStatsFromBleve(ctx context.Context, baseQuery query.Query, startTime, endTime int64) ([]HourlyAccessStats, error) {
	logger.Info("BleveStatsService: Starting hourly stats calculation")

	hourStats := make(map[int]map[string]bool) // hour -> unique IPs
	hourPV := make(map[int]int)                // hour -> page views

	// Initialize all 24 hours
	for i := 0; i < 24; i++ {
		hourStats[i] = make(map[string]bool)
		hourPV[i] = 0
	}

	// Query all entries for the time range
	searchReq := bleve.NewSearchRequest(baseQuery)
	searchReq.Size = 10000 // Process in batches
	searchReq.Fields = []string{"timestamp", "ip"}
	searchReq.SortBy([]string{"timestamp"})

	from := 0
	totalProcessed := 0
	for {
		searchReq.From = from
		logger.Debugf("BleveStatsService: Executing hourly stats search, from=%d", from)

		searchResult, err := s.indexer.index.Search(searchReq)
		if err != nil {
			logger.Errorf("BleveStatsService: Failed to search logs: %v", err)
			return nil, fmt.Errorf("failed to search logs: %w", err)
		}

		logger.Debugf("BleveStatsService: Search returned %d hits, total=%d", len(searchResult.Hits), searchResult.Total)

		if len(searchResult.Hits) == 0 {
			break
		}

		// Process hits
		for _, hit := range searchResult.Hits {
			timestamp, ip := s.extractTimestampAndIP(hit)

			if timestamp != nil && ip != "" {
				// For hourly stats, only process entries from the target date (endTime)
				if endTime != 0 {
					targetTime := time.Unix(endTime, 0)
					targetDate := targetTime.Truncate(24 * time.Hour)
					entryDate := timestamp.Truncate(24 * time.Hour)
					if !entryDate.Equal(targetDate) {
						continue // Skip entries not from the target date
					}
				}

				hour := timestamp.Hour()
				hourStats[hour][ip] = true
				hourPV[hour]++
				totalProcessed++
			} else {
				logger.Debugf("BleveStatsService: Skipped hit with missing timestamp or IP - timestamp=%v, ip='%s'", timestamp, ip)
			}
		}

		from += len(searchResult.Hits)
		if uint64(from) >= searchResult.Total {
			break
		}
	}

	logger.Infof("BleveStatsService: Processed %d entries for hourly stats", totalProcessed)

	// Convert to result format
	result := make([]HourlyAccessStats, 0, 24)

	// Use endTime (target date) for hour timestamps, or current date if not specified
	var targetDate time.Time
	if endTime != 0 {
		endDateTime := time.Unix(endTime, 0)
		targetDate = endDateTime.Truncate(24 * time.Hour)
	} else {
		now := time.Now()
		targetDate = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	}

	for hour := 0; hour < 24; hour++ {
		hourTime := targetDate.Add(time.Duration(hour) * time.Hour)

		result = append(result, HourlyAccessStats{
			Hour:      hour,
			UV:        len(hourStats[hour]),
			PV:        hourPV[hour],
			Timestamp: hourTime.Unix(),
		})
	}

	return result, nil
}

// calculateDailyStatsFromBleve calculates daily UV/PV statistics using Bleve
func (s *BleveStatsService) calculateDailyStatsFromBleve(ctx context.Context, baseQuery query.Query, startTime, endTime int64) ([]DailyAccessStats, error) {
	dailyStats := make(map[string]map[string]bool) // date -> unique IPs
	dailyPV := make(map[string]int)                // date -> page views

	// Query all entries for the time range
	searchReq := bleve.NewSearchRequest(baseQuery)
	searchReq.Size = 10000 // Process in batches
	searchReq.Fields = []string{"timestamp", "ip"}
	searchReq.SortBy([]string{"timestamp"})

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

		// Process hits
		for _, hit := range searchResult.Hits {
			timestamp, ip := s.extractTimestampAndIP(hit)

			if timestamp != nil && ip != "" {
				date := timestamp.Format("2006-01-02")
				if dailyStats[date] == nil {
					dailyStats[date] = make(map[string]bool)
				}
				dailyStats[date][ip] = true
				dailyPV[date]++
			}
		}

		from += len(searchResult.Hits)
		if uint64(from) >= searchResult.Total {
			break
		}
	}

	// Generate complete date range with padding
	result := make([]DailyAccessStats, 0)

	// Use default time range if not provided
	var startDateTime, endDateTime time.Time
	if startTime == 0 || endTime == 0 {
		endDateTime = time.Now()
		startDateTime = endDateTime.AddDate(0, 0, -30) // 30 days ago
	} else {
		startDateTime = time.Unix(startTime, 0)
		endDateTime = time.Unix(endTime, 0)
	}

	currentDate := startDateTime.Truncate(24 * time.Hour)
	for currentDate.Before(endDateTime) || currentDate.Equal(endDateTime.Truncate(24*time.Hour)) {
		dateKey := currentDate.Format("2006-01-02")

		if ips, exists := dailyStats[dateKey]; exists {
			result = append(result, DailyAccessStats{
				Date:      dateKey,
				UV:        len(ips),
				PV:        dailyPV[dateKey],
				Timestamp: currentDate.Unix(),
			})
		} else {
			// Pad with zeros for dates without data
			result = append(result, DailyAccessStats{
				Date:      dateKey,
				UV:        0,
				PV:        0,
				Timestamp: currentDate.Unix(),
			})
		}

		currentDate = currentDate.AddDate(0, 0, 1)
	}

	return result, nil
}