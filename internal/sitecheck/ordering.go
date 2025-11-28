package sitecheck

import (
	"sort"
	"strings"

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
	hasCustomOrder := false
	for _, config := range configs {
		orderMap[config.GetURL()] = config.CustomOrder
		if config.CustomOrder != 0 {
			hasCustomOrder = true
		}
	}

	if !hasCustomOrder {
		return applyDefaultOrdering(sites)
	}

	// Sort sites based on custom order, with fallback to default ordering
	sort.Slice(sites, func(i, j int) bool {
		urlI := sites[i].GetURL()
		urlJ := sites[j].GetURL()
		orderI, hasOrderI := orderMap[urlI]
		orderJ, hasOrderJ := orderMap[urlJ]

		// If both have custom order, use custom order
		if hasOrderI && hasOrderJ {
			if orderI != orderJ {
				return orderI < orderJ
			}
			return compareByName(sites[i], sites[j])
		}

		// If only one has custom order, it comes first
		if hasOrderI && !hasOrderJ {
			return true
		}
		if !hasOrderI && hasOrderJ {
			return false
		}

		// If neither has custom order, use default ordering
		return compareByName(sites[i], sites[j])
	})

	return sites
}

// applyDefaultOrdering applies the default stable sorting
func applyDefaultOrdering(sites []*SiteInfo) []*SiteInfo {
	sort.SliceStable(sites, func(i, j int) bool {
		return compareByName(sites[i], sites[j])
	})
	return sites
}

func compareByName(a, b *SiteInfo) bool {
	nameA := strings.ToLower(strings.TrimSpace(a.Name))
	nameB := strings.ToLower(strings.TrimSpace(b.Name))
	if nameA != nameB {
		return nameA < nameB
	}

	urlA := strings.ToLower(strings.TrimSpace(a.GetURL()))
	urlB := strings.ToLower(strings.TrimSpace(b.GetURL()))
	return urlA < urlB
}
