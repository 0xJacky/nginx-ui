package analytics

import (
	"context"
	"fmt"

	"github.com/0xJacky/Nginx-UI/internal/nginx_log/searcher"
	"github.com/uozi-tech/cosy/logger"
)

func (s *service) GetGeoDistribution(ctx context.Context, req *GeoQueryRequest) (*GeoDistribution, error) {
	if req == nil {
		return nil, fmt.Errorf("request cannot be nil")
	}

	if err := s.ValidateTimeRange(req.StartTime, req.EndTime); err != nil {
		return nil, fmt.Errorf("invalid time range: %w", err)
	}

	logger.Debugf("=== DEBUG GetGeoDistribution START ===")
	logger.Debugf("GetGeoDistribution - req: %+v", req)

	searchReq := &searcher.SearchRequest{
		StartTime:     &req.StartTime,
		EndTime:       &req.EndTime,
		LogPaths:      req.LogPaths,
		Limit:         0, // We only need facets.
		IncludeFacets: true,
		FacetFields:   []string{"region_code"},
		FacetSize:     300, // Large enough to cover all countries
		UseCache:      true,
	}
	logger.Debugf("GetGeoDistribution - SearchRequest: %+v", searchReq)

	result, err := s.searcher.Search(ctx, searchReq)
	if err != nil {
		logger.Debugf("GetGeoDistribution - Search failed: %v", err)
		return nil, fmt.Errorf("failed to get geo distribution: %w", err)
	}

	logger.Debugf("GetGeoDistribution - Search returned TotalHits: %d", result.TotalHits)
	logger.Debugf("GetGeoDistribution - Search returned %d facets", len(result.Facets))

	dist := &GeoDistribution{
		Countries: make(map[string]int),
	}
	if result.Facets != nil {
		if countryFacet, ok := result.Facets["region_code"]; ok {
			logger.Debugf("GetGeoDistribution - Found region_code facet with %d terms", len(countryFacet.Terms))
			for _, term := range countryFacet.Terms {
				if term.Term == "CN" {
					logger.Debugf("GetGeoDistribution - FOUND CN - Term: '%s', Count: %d", term.Term, term.Count)
				}
				logger.Debugf("GetGeoDistribution - Country term: '%s', Count: %d", term.Term, term.Count)
				dist.Countries[term.Term] = term.Count
			}
		} else {
			logger.Debugf("GetGeoDistribution - No 'region_code' facet found in result")
			for facetName := range result.Facets {
				logger.Debugf("GetGeoDistribution - Available facet: '%s'", facetName)
			}
		}
	} else {
		logger.Debugf("GetGeoDistribution - No facets in search result")
	}

	logger.Debugf("GetGeoDistribution - Final distribution has %d countries", len(dist.Countries))
	if cnCount, ok := dist.Countries["CN"]; ok {
		logger.Debugf("GetGeoDistribution - CN final count: %d", cnCount)
	}
	logger.Debugf("=== DEBUG GetGeoDistribution END ===")

	return dist, nil
}

func (s *service) GetGeoDistributionByCountry(ctx context.Context, req *GeoQueryRequest, countryCode string) (*GeoDistribution, error) {
	if req == nil {
		return nil, fmt.Errorf("request cannot be nil")
	}

	if err := s.ValidateTimeRange(req.StartTime, req.EndTime); err != nil {
		return nil, fmt.Errorf("invalid time range: %w", err)
	}

	logger.Debugf("=== DEBUG GetGeoDistributionByCountry START ===")
	logger.Debugf("GetGeoDistributionByCountry - countryCode: '%s'", countryCode)
	logger.Debugf("GetGeoDistributionByCountry - req: %+v", req)

	searchReq := &searcher.SearchRequest{
		StartTime:     &req.StartTime,
		EndTime:       &req.EndTime,
		LogPaths:      req.LogPaths,
		Countries:     []string{countryCode}, // Use proper country filter instead of text query
		Limit:         0, // We only need facets.
		IncludeFacets: true,
		FacetFields:   []string{"province"},
		FacetSize:     100, // Large enough to cover all provinces in a country
		UseCache:      true,
	}
	logger.Debugf("GetGeoDistributionByCountry - SearchRequest: %+v", searchReq)
	logger.Debugf("GetGeoDistributionByCountry - Countries filter: %v", searchReq.Countries)

	result, err := s.searcher.Search(ctx, searchReq)
	if err != nil {
		logger.Debugf("GetGeoDistributionByCountry - Search failed: %v", err)
		return nil, fmt.Errorf("failed to get geo distribution by country: %w", err)
	}

	logger.Debugf("GetGeoDistributionByCountry - Search returned TotalHits: %d", result.TotalHits)
	logger.Debugf("GetGeoDistributionByCountry - Search returned %d facets", len(result.Facets))

	dist := &GeoDistribution{
		Countries: make(map[string]int), // Reusing 'Countries' map for provinces
	}
	if result.Facets != nil {
		if provinceFacet, ok := result.Facets["province"]; ok {
			logger.Debugf("GetGeoDistributionByCountry - Found province facet with %d terms, Total: %d, Missing: %d", len(provinceFacet.Terms), provinceFacet.Total, provinceFacet.Missing)
			for _, term := range provinceFacet.Terms {
				logger.Debugf("GetGeoDistributionByCountry - Province term: '%s', Count: %d", term.Term, term.Count)
				dist.Countries[term.Term] = term.Count
			}
		} else {
			logger.Debugf("GetGeoDistributionByCountry - No 'province' facet found in result")
			for facetName, facet := range result.Facets {
				logger.Debugf("GetGeoDistributionByCountry - Available facet: '%s' (Total: %d, Missing: %d, Terms: %d)", facetName, facet.Total, facet.Missing, len(facet.Terms))
			}
		}
	} else {
		logger.Debugf("GetGeoDistributionByCountry - No facets in search result")
	}

	logger.Debugf("GetGeoDistributionByCountry - Final distribution has %d provinces", len(dist.Countries))
	logger.Debugf("=== DEBUG GetGeoDistributionByCountry END ===")

	return dist, nil
}

func (s *service) GetTopCountries(ctx context.Context, req *GeoQueryRequest) ([]CountryStats, error) {
	if req == nil {
		return nil, fmt.Errorf("request cannot be nil")
	}

	if err := s.ValidateTimeRange(req.StartTime, req.EndTime); err != nil {
		return nil, fmt.Errorf("invalid time range: %w", err)
	}

	searchReq := &searcher.SearchRequest{
		StartTime:     &req.StartTime,
		EndTime:       &req.EndTime,
		LogPaths:      req.LogPaths,
		Limit:         0, // We only need facets
		IncludeFacets: true,
		FacetFields:   []string{"region_code"},
		FacetSize:     req.Limit, // Use the requested limit for facet size
		UseCache:      true,
	}

	result, err := s.searcher.Search(ctx, searchReq)
	if err != nil {
		return nil, fmt.Errorf("failed to get top countries: %w", err)
	}

	var stats []CountryStats
	if result.Facets != nil {
		if countryFacet, ok := result.Facets["region_code"]; ok {
			for _, term := range countryFacet.Terms {
				stats = append(stats, CountryStats{
					Country:  term.Term,
					Requests: term.Count,
				})
			}
		}
	}

	// Facets are already sorted by count descending from bleve
	return stats, nil
}

func (s *service) GetTopCities(ctx context.Context, req *GeoQueryRequest) ([]CityStats, error) {
	if req == nil {
		return nil, fmt.Errorf("request cannot be nil")
	}

	if err := s.ValidateTimeRange(req.StartTime, req.EndTime); err != nil {
		return nil, fmt.Errorf("invalid time range: %w", err)
	}

	searchReq := &searcher.SearchRequest{
		StartTime:     &req.StartTime,
		EndTime:       &req.EndTime,
		LogPaths:      req.LogPaths,
		Limit:         0, // We only need facets
		IncludeFacets: true,
		FacetFields:   []string{"city"},
		FacetSize:     req.Limit,
		UseCache:      true,
	}

	result, err := s.searcher.Search(ctx, searchReq)
	if err != nil {
		return nil, fmt.Errorf("failed to get top cities: %w", err)
	}

	var stats []CityStats
	if result.Facets != nil {
		if cityFacet, ok := result.Facets["city"]; ok {
			totalHits := int(result.TotalHits)
			for _, term := range cityFacet.Terms {
				percent := float64(term.Count) / float64(totalHits) * 100
				stats = append(stats, CityStats{
					City:    term.Term,
					Count:   term.Count,
					Percent: percent,
				})
			}
		}
	}

	return stats, nil
}

func (s *service) GetGeoStatsForIP(ctx context.Context, req *GeoQueryRequest, ip string) (*CityStats, error) {
	if req == nil {
		return nil, fmt.Errorf("request cannot be nil")
	}

	if ip == "" {
		return nil, fmt.Errorf("IP address cannot be empty")
	}

	if err := s.ValidateTimeRange(req.StartTime, req.EndTime); err != nil {
		return nil, fmt.Errorf("invalid time range: %w", err)
	}

	searchReq := &searcher.SearchRequest{
		StartTime:     &req.StartTime,
		EndTime:       &req.EndTime,
		LogPaths:      req.LogPaths,
		Limit:         0,
		IncludeFacets: true,
		FacetFields:   []string{"country", "country_code", "city"},
		FacetSize:     10,
		Query:         fmt.Sprintf(`ip:"%s"`, ip),
		UseCache:      true,
	}

	result, err := s.searcher.Search(ctx, searchReq)
	if err != nil {
		return nil, fmt.Errorf("failed to get geo stats for IP: %w", err)
	}

	if result.TotalHits == 0 {
		return nil, fmt.Errorf("no data found for IP %s", ip)
	}

	if result.Facets == nil {
		return nil, fmt.Errorf("could not extract geo information for IP %s", ip)
	}

	stats := &CityStats{
		Count:   int(result.TotalHits),
		Percent: 100.0, // 100% for single IP
	}

	if countryFacet, ok := result.Facets["country"]; ok && len(countryFacet.Terms) > 0 {
		stats.Country = countryFacet.Terms[0].Term
	}

	if countryCodeFacet, ok := result.Facets["country_code"]; ok && len(countryCodeFacet.Terms) > 0 {
		stats.CountryCode = countryCodeFacet.Terms[0].Term
	}

	if cityFacet, ok := result.Facets["city"]; ok && len(cityFacet.Terms) > 0 {
		stats.City = cityFacet.Terms[0].Term
	}

	return stats, nil
}
