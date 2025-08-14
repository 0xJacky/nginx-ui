package sitecheck

import (
	"sort"

	"github.com/0xJacky/Nginx-UI/query"
	"github.com/uozi-tech/cosy/logger"
)

// applyCustomOrdering applies custom ordering from database to sites
func applyCustomOrdering(sites []*SiteInfo) []*SiteInfo {
	if len(sites) == 0 {
		return sites
	}

	// Get custom ordering from database
	sc := query.SiteConfig
	configs, err := sc.Find()
	if err != nil {
		logger.Errorf("Failed to get site configs for ordering: %v", err)
		// Fall back to default ordering
		return applyDefaultOrdering(sites)
	}

	// Create a map of URL to custom order
	orderMap := make(map[string]int)
	for _, config := range configs {
		orderMap[config.GetURL()] = config.CustomOrder
	}

	// Sort sites based on custom order, with fallback to default ordering
	sort.Slice(sites, func(i, j int) bool {
		orderI, hasOrderI := orderMap[sites[i].URL]
		orderJ, hasOrderJ := orderMap[sites[j].URL]

		// If both have custom order, use custom order
		if hasOrderI && hasOrderJ {
			return orderI < orderJ
		}

		// If only one has custom order, it comes first
		if hasOrderI && !hasOrderJ {
			return true
		}
		if !hasOrderI && hasOrderJ {
			return false
		}

		// If neither has custom order, use default ordering
		return defaultCompare(sites[i], sites[j])
	})

	return sites
}

// applyDefaultOrdering applies the default stable sorting
func applyDefaultOrdering(sites []*SiteInfo) []*SiteInfo {
	sort.Slice(sites, func(i, j int) bool {
		return defaultCompare(sites[i], sites[j])
	})
	return sites
}

// defaultCompare implements the default site comparison logic
func defaultCompare(a, b *SiteInfo) bool {
	// Primary sort: by status (online > checking > error > offline)
	statusPriority := map[string]int{
		"online":   4,
		"checking": 3,
		"error":    2,
		"offline":  1,
	}

	priorityA := statusPriority[a.Status]
	priorityB := statusPriority[b.Status]

	if priorityA != priorityB {
		return priorityA > priorityB
	}

	// Secondary sort: by response time (faster first, for online sites)
	if a.Status == "online" && b.Status == "online" {
		if a.ResponseTime != b.ResponseTime {
			return a.ResponseTime < b.ResponseTime
		}
	}

	// Tertiary sort: by name (alphabetical, stable)
	if a.Name != b.Name {
		return a.Name < b.Name
	}

	// Final sort: by URL (for complete stability)
	return a.URL < b.URL
}
