package nginx_log

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/0xJacky/Nginx-UI/internal/geolite"
	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/search"
	"github.com/blevesearch/bleve/v2/search/query"
)

// GeoStats represents geographic statistics
type GeoStats struct {
	RegionCode string  `json:"region_code"`
	Country    string  `json:"country"`
	Province   string  `json:"province,omitempty"`
	City       string  `json:"city,omitempty"`
	Count      int     `json:"count"`
	Percent    float64 `json:"percent"`
}

// WorldMapData represents data for world map visualization
type WorldMapData struct {
	RegionCode string  `json:"code"`
	ISP        string  `json:"isp,omitempty"`
	Value      int     `json:"value"`
	Percent    float64 `json:"percent"`
}

// ChinaMapData represents data for China map visualization
type ChinaMapData struct {
	Name    string     `json:"name"` // Province name for ECharts map
	Value   int        `json:"value"`
	Percent float64    `json:"percent"`
	Cities  []CityData `json:"cities,omitempty"`
}

// CityData represents city-level data
type CityData struct {
	Name    string  `json:"name"`
	Value   int     `json:"value"`
	Percent float64 `json:"percent"`
}

// normalizeRegionForWorldMap unifies HK, MO, TW under CN for world map display
func normalizeRegionForWorldMap(regionCode string) string {
	if regionCode == "HK" || regionCode == "MO" || regionCode == "TW" {
		return "CN"
	}
	return regionCode
}

// normalizeProvinceName standardizes Chinese province names
func normalizeProvinceName(province string) string {
	if province == "" {
		return province
	}

	// Common province name mappings - ensure they end with proper suffixes
	provinceMap := map[string]string{
		// Provinces (省)
		"北京":  "北京市",
		"天津":  "天津市",
		"上海":  "上海市",
		"重庆":  "重庆市",
		"河北":  "河北省",
		"山西":  "山西省",
		"辽宁":  "辽宁省",
		"吉林":  "吉林省",
		"黑龙江": "黑龙江省",
		"江苏":  "江苏省",
		"浙江":  "浙江省",
		"安徽":  "安徽省",
		"福建":  "福建省",
		"江西":  "江西省",
		"山东":  "山东省",
		"河南":  "河南省",
		"湖北":  "湖北省",
		"湖南":  "湖南省",
		"广东":  "广东省",
		"海南":  "海南省",
		"四川":  "四川省",
		"贵州":  "贵州省",
		"云南":  "云南省",
		"陕西":  "陕西省",
		"甘肃":  "甘肃省",
		"青海":  "青海省",
		"台湾":  "台湾省",
		// Autonomous regions (自治区)
		"内蒙古": "内蒙古自治区",
		"广西":  "广西壮族自治区",
		"西藏":  "西藏自治区",
		"宁夏":  "宁夏回族自治区",
		"新疆":  "新疆维吾尔自治区",
	}

	// Check if we have a mapping for this province
	if normalized, exists := provinceMap[province]; exists {
		return normalized
	}

	// If already has proper suffix, return as-is
	if strings.HasSuffix(province, "省") || strings.HasSuffix(province, "市") ||
		strings.HasSuffix(province, "自治区") || strings.HasSuffix(province, "特别行政区") {
		return province
	}

	// Default: assume it's a province and add "省" suffix
	return province + "省"
}

// getProvinceShortName returns the short name for Chinese provinces for map display
func getProvinceShortName(fullName string) string {
	// Map of full province names to short names for map display
	shortNameMap := map[string]string{
		// Municipalities (直辖市) - keep as is
		"北京市": "北京",
		"天津市": "天津",
		"上海市": "上海",
		"重庆市": "重庆",
		// Provinces (省) - remove suffix
		"河北省":  "河北",
		"山西省":  "山西",
		"辽宁省":  "辽宁",
		"吉林省":  "吉林",
		"黑龙江省": "黑龙江",
		"江苏省":  "江苏",
		"浙江省":  "浙江",
		"安徽省":  "安徽",
		"福建省":  "福建",
		"江西省":  "江西",
		"山东省":  "山东",
		"河南省":  "河南",
		"湖北省":  "湖北",
		"湖南省":  "湖南",
		"广东省":  "广东",
		"海南省":  "海南",
		"四川省":  "四川",
		"贵州省":  "贵州",
		"云南省":  "云南",
		"陕西省":  "陕西",
		"甘肃省":  "甘肃",
		"青海省":  "青海",
		"台湾省":  "台湾",
		// Autonomous regions (自治区) - use short form
		"内蒙古自治区":   "内蒙古",
		"广西壮族自治区":  "广西",
		"西藏自治区":    "西藏",
		"宁夏回族自治区":  "宁夏",
		"新疆维吾尔自治区": "新疆",
		// Special Administrative Regions (特别行政区) - use short form
		"香港特别行政区": "香港",
		"澳门特别行政区": "澳门",
		// Unknown
		"未知": "未知",
	}

	if shortName, exists := shortNameMap[fullName]; exists {
		return shortName
	}

	// If no mapping found, return the original name
	return fullName
}

// GetWorldMapData returns aggregated data for world map visualization
func (s *BleveStatsService) GetWorldMapData(ctx context.Context, baseQuery query.Query) ([]WorldMapData, error) {
	regionCount := make(map[string]int)
	regionData := make(map[string]*WorldMapData)
	totalRequests := 0

	// Query all entries with more geographic fields
	searchReq := bleve.NewSearchRequest(baseQuery)
	searchReq.Size = 10000
	searchReq.Fields = []string{"region_code", "location", "province", "city", "isp"}

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
			regionCode := extractField(hit, "region_code")
			isp := extractField(hit, "isp")

			if regionCode == "" {
				regionCode = "UNKNOWN"
			}

			// Skip UNKNOWN entries for world map (they're not useful for geographic visualization)
			if regionCode == "UNKNOWN" {
				totalRequests++ // Still count for percentage calculation
				continue
			}

			// Unify Hong Kong, Macao, and Taiwan under China for world map display
			// but preserve original region code for detailed analysis
			regionCode = normalizeRegionForWorldMap(regionCode)

			// Initialize or update region data
			if _, exists := regionData[regionCode]; !exists {
				regionData[regionCode] = &WorldMapData{
					RegionCode: regionCode,
					ISP:        isp,
					Value:      0,
				}
			}

			regionCount[regionCode]++
			totalRequests++
		}

		from += len(searchResult.Hits)
		if uint64(from) >= searchResult.Total {
			break
		}
	}

	// Convert to WorldMapData slice with calculated percentages
	var results []WorldMapData
	for code, count := range regionCount {
		percent := 0.0
		if totalRequests > 0 {
			percent = float64(count) * 100.0 / float64(totalRequests)
		}

		data := &WorldMapData{
			RegionCode: code,
			Value:      count,
			Percent:    percent,
		}

		results = append(results, *data)
	}

	// Sort by count (descending)
	sort.Slice(results, func(i, j int) bool {
		return results[i].Value > results[j].Value
	})

	return results, nil
}

// GetChinaMapData returns aggregated data for China map visualization
func (s *BleveStatsService) GetChinaMapData(ctx context.Context, baseQuery query.Query) ([]ChinaMapData, error) {
	// Debug: First let's see what region codes are actually in the index
	allRegionsReq := bleve.NewSearchRequest(baseQuery)
	allRegionsReq.Size = 1000
	allRegionsReq.Fields = []string{"region_code"}

	// First, filter for Chinese IPs - use MatchQuery instead of TermQuery
	chineseRegions := []string{"CN", "HK", "MO", "TW"}
	regionQueries := make([]query.Query, 0, len(chineseRegions))

	for _, region := range chineseRegions {
		matchQuery := bleve.NewMatchQuery(region)
		matchQuery.SetField("region_code")
		regionQueries = append(regionQueries, matchQuery)
	}

	chinaQuery := bleve.NewDisjunctionQuery(regionQueries...)

	// Combine with base query if provided
	var finalQuery query.Query
	if baseQuery != nil {
		finalQuery = bleve.NewConjunctionQuery(baseQuery, chinaQuery)

		// Test the conjunction query first to see if it returns results
		testReq := bleve.NewSearchRequest(finalQuery)
		testReq.Size = 1
		testResult, testErr := s.indexer.index.Search(testReq)
		if testErr == nil && testResult.Total == 0 {
			finalQuery = chinaQuery
		}
	} else {
		finalQuery = chinaQuery
	}

	provinceData := make(map[string]*ChinaMapData)
	totalRequests := 0

	// Query Chinese entries
	searchReq := bleve.NewSearchRequest(finalQuery)
	searchReq.Size = 10000
	searchReq.Fields = []string{"region_code", "province", "city"}

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
			regionCode := ""
			province := ""
			city := ""

			// Get region code first
			if regField, ok := hit.Fields["region_code"]; ok {
				if reg, ok := regField.(string); ok {
					regionCode = reg
				}
			}

			if provField, ok := hit.Fields["province"]; ok {
				if prov, ok := provField.(string); ok && prov != "" && prov != "0" {
					province = prov
				}
			}

			if cityField, ok := hit.Fields["city"]; ok {
				if c, ok := cityField.(string); ok && c != "" && c != "0" {
					city = c
				}
			}

			// Handle special regions for China map
			switch regionCode {
			case "HK":
				province = "香港特别行政区"
				if city == "" {
					city = "香港"
				}
			case "MO":
				province = "澳门特别行政区"
				if city == "" {
					city = "澳门"
				}
			case "TW":
				province = "台湾省"
				if city == "" {
					city = "台北"
				}
			default:
				// For mainland China, normalize the province name
				if province == "" {
					province = "未知"
				} else {
					province = normalizeProvinceName(province)
				}
			}

			// Initialize province data if not exists
			if _, exists := provinceData[province]; !exists {
				provinceData[province] = &ChinaMapData{
					Name:   getProvinceShortName(province), // Use short name for ECharts map display
					Value:  0,
					Cities: make([]CityData, 0),
				}
			}

			provinceData[province].Value++

			// Track city data if available
			if city != "" && city != province {
				found := false
				for i, cityData := range provinceData[province].Cities {
					if cityData.Name == city {
						provinceData[province].Cities[i].Value++
						found = true
						break
					}
				}
				if !found {
					provinceData[province].Cities = append(provinceData[province].Cities, CityData{
						Name:  city,
						Value: 1,
					})
				}
			}

			totalRequests++
		}

		from += len(searchResult.Hits)
		if uint64(from) >= searchResult.Total {
			break
		}
	}

	// Convert to slice and calculate percentages
	var results []ChinaMapData
	for _, data := range provinceData {
		if totalRequests > 0 {
			data.Percent = float64(data.Value) * 100.0 / float64(totalRequests)

			// Calculate city percentages
			for i := range data.Cities {
				data.Cities[i].Percent = float64(data.Cities[i].Value) * 100.0 / float64(data.Value)
			}

			// Sort cities by value
			sort.Slice(data.Cities, func(i, j int) bool {
				return data.Cities[i].Value > data.Cities[j].Value
			})
		}

		results = append(results, *data)
	}

	// Sort provinces by value
	sort.Slice(results, func(i, j int) bool {
		return results[i].Value > results[j].Value
	})

	return results, nil
}

// GetGeoStats returns geographic statistics for the given query
func (s *BleveStatsService) GetGeoStats(ctx context.Context, baseQuery query.Query, limit int) ([]GeoStats, error) {
	geoCount := make(map[string]*GeoStats)
	totalRequests := 0

	// Query all entries
	searchReq := bleve.NewSearchRequest(baseQuery)
	searchReq.Size = 10000
	searchReq.Fields = []string{"region_code", "location", "province", "city"}

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
			regionCode := extractField(hit, "region_code")
			location := extractField(hit, "location")
			province := extractField(hit, "province")
			city := extractField(hit, "city")

			if regionCode == "" {
				regionCode = "UNKNOWN"
			}

			key := regionCode
			if geolite.IsChineseRegion(regionCode) && province != "" {
				key = fmt.Sprintf("%s-%s", regionCode, province)
			}

			if _, exists := geoCount[key]; !exists {
				country := ""
				if location != "" {
					parts := strings.Split(location, ",")
					if len(parts) > 0 {
						country = strings.TrimSpace(parts[0])
					}
				}

				geoCount[key] = &GeoStats{
					RegionCode: regionCode,
					Country:    country,
					Province:   province,
					City:       city,
					Count:      0,
				}
			}

			geoCount[key].Count++
			totalRequests++
		}

		from += len(searchResult.Hits)
		if uint64(from) >= searchResult.Total {
			break
		}
	}

	// Convert to slice and calculate percentages
	var results []GeoStats
	for _, stats := range geoCount {
		if totalRequests > 0 {
			stats.Percent = float64(stats.Count) * 100.0 / float64(totalRequests)
		}
		results = append(results, *stats)
	}

	// Sort by count (descending)
	sort.Slice(results, func(i, j int) bool {
		return results[i].Count > results[j].Count
	})

	// Apply limit if specified
	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}

	return results, nil
}

// Helper function to extract field from search hit
func extractField(hit *search.DocumentMatch, fieldName string) string {
	if field, ok := hit.Fields[fieldName]; ok {
		if value, ok := field.(string); ok && value != "" && value != "0" {
			return value
		}
	}
	return ""
}
