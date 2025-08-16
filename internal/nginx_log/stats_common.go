package nginx_log

import "sort"

// CommonAccessStats represents generic access statistics
type CommonAccessStats struct {
	Name    string  `json:"name"`
	Count   int     `json:"count"`
	Percent float64 `json:"percent"`
}

// calculateCommonStats is a function to calculate statistics for any field
func calculateCommonStats(entries []*AccessLogEntry, extractField func(*AccessLogEntry) string) []CommonAccessStats {
	fieldCount := make(map[string]int)
	totalRequests := len(entries)

	// Count field occurrences
	for _, entry := range entries {
		field := extractField(entry)
		if field == "" {
			field = "Unknown"
		}
		fieldCount[field]++
	}

	// Convert to slice and sort
	var stats []CommonAccessStats
	for name, count := range fieldCount {
		percent := 0.0
		if totalRequests > 0 {
			percent = float64(count) * 100.0 / float64(totalRequests)
		}

		stats = append(stats, CommonAccessStats{
			Name:    name,
			Count:   count,
			Percent: percent,
		})
	}

	// Sort by count (descending)
	sort.Slice(stats, func(i, j int) bool {
		return stats[i].Count > stats[j].Count
	})

	return stats
}

// calculateAverage calculates average from slice of values
func calculateAverage(values []int) float64 {
	if len(values) == 0 {
		return 0.0
	}
	
	sum := 0
	for _, v := range values {
		sum += v
	}
	return float64(sum) / float64(len(values))
}

// findMax finds maximum value and its index
func findMax(values []int) (maxValue, maxIndex int) {
	if len(values) == 0 {
		return 0, -1
	}
	
	maxValue = values[0]
	maxIndex = 0
	
	for i, v := range values[1:] {
		if v > maxValue {
			maxValue = v
			maxIndex = i + 1
		}
	}
	return maxValue, maxIndex
}