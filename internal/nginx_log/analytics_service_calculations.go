package nginx_log

import (
	"sort"
	"time"
)

// calculateHourlyStats calculates UV/PV statistics for each hour of the day
func (s *AnalyticsService) calculateHourlyStats(entries []*AccessLogEntry, startTime, endTime int64) []HourlyAccessStats {
	// Create map to aggregate stats by hour (0-23)
	hourStats := make(map[int]map[string]bool) // hour -> set of unique IPs
	hourPV := make(map[int]int)                // hour -> page view count

	// Initialize all 24 hours
	for i := 0; i < 24; i++ {
		hourStats[i] = make(map[string]bool)
		hourPV[i] = 0
	}

	// Process entries
	for _, entry := range entries {
		entryTime := time.Unix(entry.Timestamp, 0)
		hour := entryTime.Hour()

		// Count unique visitors (UV)
		hourStats[hour][entry.IP] = true

		// Count page views (PV)
		hourPV[hour]++
	}

	// Convert to result format - always return 24 hours
	result := make([]HourlyAccessStats, 0, 24)
	for hour := 0; hour < 24; hour++ {
		// Create timestamp for this hour today
		now := time.Now()
		hourTime := time.Date(now.Year(), now.Month(), now.Day(), hour, 0, 0, 0, now.Location())

		result = append(result, HourlyAccessStats{
			Hour:      hour,
			UV:        len(hourStats[hour]),
			PV:        hourPV[hour],
			Timestamp: hourTime.Unix(),
		})
	}

	return result
}

// calculateDailyStats calculates daily UV/PV statistics for the time range with padding
func (s *AnalyticsService) calculateDailyStats(entries []*AccessLogEntry, startTime, endTime int64) []DailyAccessStats {
	// Create map to aggregate stats by date
	dailyStats := make(map[string]map[string]bool) // date -> set of unique IPs
	dailyPV := make(map[string]int)                // date -> page view count

	// Process entries
	for _, entry := range entries {
		entryTime := time.Unix(entry.Timestamp, 0)
		date := entryTime.Format("2006-01-02")

		if dailyStats[date] == nil {
			dailyStats[date] = make(map[string]bool)
		}

		// Count unique visitors
		dailyStats[date][entry.IP] = true

		// Count page views
		dailyPV[date]++
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

	return result
}

// calculateTopURLs calculates the most visited URLs
func (s *AnalyticsService) calculateTopURLs(entries []*AccessLogEntry) []URLAccessStats {
	urlCount := make(map[string]int)
	totalRequests := len(entries)

	// Count URL visits
	for _, entry := range entries {
		urlCount[entry.Path]++
	}

	// Convert to slice and sort
	var urlStats []URLAccessStats
	for url, count := range urlCount {
		percent := 0.0
		if totalRequests > 0 {
			percent = float64(count) * 100.0 / float64(totalRequests)
		}

		urlStats = append(urlStats, URLAccessStats{
			URL:     url,
			Visits:  count,
			Percent: percent,
		})
	}

	// Sort by visits (descending)
	sort.Slice(urlStats, func(i, j int) bool {
		return urlStats[i].Visits > urlStats[j].Visits
	})

	// Limit to top 10
	if len(urlStats) > 10 {
		urlStats = urlStats[:10]
	}

	return urlStats
}

// calculateBrowserStats calculates browser usage statistics
func (s *AnalyticsService) calculateBrowserStats(entries []*AccessLogEntry) []BrowserAccessStats {
	commonStats := calculateCommonStats(entries, func(entry *AccessLogEntry) string {
		return entry.Browser
	})

	// Convert to BrowserAccessStats format
	result := make([]BrowserAccessStats, len(commonStats))
	for i, stat := range commonStats {
		result[i] = BrowserAccessStats{
			Browser: stat.Name,
			Count:   stat.Count,
			Percent: stat.Percent,
		}
	}

	return result
}

// calculateOSStats calculates operating system usage statistics
func (s *AnalyticsService) calculateOSStats(entries []*AccessLogEntry) []OSAccessStats {
	commonStats := calculateCommonStats(entries, func(entry *AccessLogEntry) string {
		return entry.OS
	})

	// Convert to OSAccessStats format
	result := make([]OSAccessStats, len(commonStats))
	for i, stat := range commonStats {
		result[i] = OSAccessStats{
			OS:      stat.Name,
			Count:   stat.Count,
			Percent: stat.Percent,
		}
	}

	return result
}

// calculateDeviceStats calculates device type usage statistics
func (s *AnalyticsService) calculateDeviceStats(entries []*AccessLogEntry) []DeviceAccessStats {
	commonStats := calculateCommonStats(entries, func(entry *AccessLogEntry) string {
		return entry.DeviceType
	})

	// Convert to DeviceAccessStats format
	result := make([]DeviceAccessStats, len(commonStats))
	for i, stat := range commonStats {
		result[i] = DeviceAccessStats{
			Device:  stat.Name,
			Count:   stat.Count,
			Percent: stat.Percent,
		}
	}

	return result
}