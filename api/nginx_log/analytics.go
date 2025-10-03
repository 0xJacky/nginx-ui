package nginx_log

import (
	"context"
	"net/http"
	"sort"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/internal/nginx_log"
	"github.com/0xJacky/Nginx-UI/internal/nginx_log/analytics"
	"github.com/0xJacky/Nginx-UI/internal/nginx_log/searcher"
	"github.com/gin-gonic/gin"
	"github.com/uozi-tech/cosy"
	"github.com/uozi-tech/cosy/logger"
)

type GeoRegionItem struct {
	Code    string  `json:"code"`
	Value   int     `json:"value"`
	Percent float64 `json:"percent"`
}

type GeoDataItem struct {
	Name    string  `json:"name"`
	Value   int     `json:"value"`
	Percent float64 `json:"percent"`
}

// AnalyticsRequest represents the request for log analytics
type AnalyticsRequest struct {
	Path      string `json:"path" form:"path"`
	StartTime int64  `json:"start_time" form:"start_time"`
	EndTime   int64  `json:"end_time" form:"end_time"`
	Limit     int    `json:"limit" form:"limit"`
}

// AdvancedSearchRequest represents the request for advanced log search
type AdvancedSearchRequest struct {
	Query     string `json:"query" form:"query"`
	LogPath   string `json:"log_path" form:"log_path"`
	StartTime int64  `json:"start_time" form:"start_time"`
	EndTime   int64  `json:"end_time" form:"end_time"`
	IP        string `json:"ip" form:"ip"`
	Method    string `json:"method" form:"method"`
	Status    []int  `json:"status" form:"status"`
	Path      string `json:"path" form:"path"`
	UserAgent string `json:"user_agent" form:"user_agent"`
	Referer   string `json:"referer" form:"referer"`
	Browser   string `json:"browser" form:"browser"`
	OS        string `json:"os" form:"os"`
	Device    string `json:"device" form:"device"`
	Limit     int    `json:"limit" form:"limit"`
	Offset    int    `json:"offset" form:"offset"`
	SortBy    string `json:"sort_by" form:"sort_by"`
	SortOrder string `json:"sort_order" form:"sort_order"`
}

// SummaryStats Structures to match the frontend's expectations for the search response
type SummaryStats struct {
	UV              int     `json:"uv"`
	PV              int     `json:"pv"`
	TotalTraffic    int64   `json:"total_traffic"`
	UniquePages     int     `json:"unique_pages"`
	AvgTrafficPerPV float64 `json:"avg_traffic_per_pv"`
}

type AdvancedSearchResponseAPI struct {
	Entries []map[string]interface{} `json:"entries"`
	Total   uint64                   `json:"total"`
	Took    int64                    `json:"took"` // Milliseconds
	Query   string                   `json:"query"`
	Summary SummaryStats             `json:"summary"`
}

// PreflightResponse represents the response for preflight query

// GetLogAnalytics provides comprehensive log analytics
func GetLogAnalytics(c *gin.Context) {
	var req AnalyticsRequest
	if !cosy.BindAndValid(c, &req) {
		return
	}

	// Get modern analytics service
	analyticsService := nginx_log.GetAnalytics()
	if analyticsService == nil {
		cosy.ErrHandler(c, nginx_log.ErrModernAnalyticsNotAvailable)
		return
	}

	// Validate log path
	if err := analyticsService.ValidateLogPath(req.Path); err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	// Build search request for log entries statistics
	searchReq := &searcher.SearchRequest{
		Limit:         req.Limit,
		UseCache:      true,
		IncludeStats:  true,
		IncludeFacets: true,
		FacetFields:   []string{"path", "ip", "user_agent", "status", "method"},
	}

	if req.StartTime > 0 {
		searchReq.StartTime = &req.StartTime
	}
	if req.EndTime > 0 {
		searchReq.EndTime = &req.EndTime
	}

	// Get log entries statistics
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	stats, err := analyticsService.GetLogEntriesStats(ctx, searchReq)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	c.JSON(http.StatusOK, stats)
}

// GetLogPreflight returns the preflight status for log indexing
func GetLogPreflight(c *gin.Context) {
	// Get optional log path parameter
	logPath := c.Query("log_path")

	// Create preflight service and perform check
	preflightService := nginx_log.NewPreflight()
	internalResponse, err := preflightService.CheckLogPreflight(logPath)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	// Convert internal response to API response
	response := PreflightResponse{
		Available:   internalResponse.Available,
		IndexStatus: internalResponse.IndexStatus,
		Message:     internalResponse.Message,
	}

	if internalResponse.TimeRange != nil {
		response.TimeRange = &TimeRange{
			Start: internalResponse.TimeRange.Start,
			End:   internalResponse.TimeRange.End,
		}
	}

	if internalResponse.FileInfo != nil {
		response.FileInfo = &FileInfo{
			Exists:       internalResponse.FileInfo.Exists,
			Readable:     internalResponse.FileInfo.Readable,
			Size:         internalResponse.FileInfo.Size,
			LastModified: internalResponse.FileInfo.LastModified,
		}
	}

	c.JSON(http.StatusOK, response)
}

// AdvancedSearchLogs provides advanced search capabilities for logs
func AdvancedSearchLogs(c *gin.Context) {
	var req AdvancedSearchRequest
	if !cosy.BindAndValid(c, &req) {
		return
	}

	searcherService := nginx_log.GetSearcher()
	if searcherService == nil {
		cosy.ErrHandler(c, nginx_log.ErrModernSearcherNotAvailable)
		return
	}

	analyticsService := nginx_log.GetAnalytics()
	if analyticsService == nil {
		cosy.ErrHandler(c, nginx_log.ErrModernAnalyticsNotAvailable)
		return
	}

	// Use default access log path if LogPath is empty
	if req.LogPath == "" {
		defaultLogPath := nginx.GetAccessLogPath()
		if defaultLogPath != "" {
			req.LogPath = defaultLogPath
			logger.Debugf("Using default access log path for search: %s", req.LogPath)
		}
	}

	// Validate log path if provided
	if req.LogPath != "" {
		if err := analyticsService.ValidateLogPath(req.LogPath); err != nil {
			cosy.ErrHandler(c, err)
			return
		}
	}

	// Build search request
	searchReq := &searcher.SearchRequest{
		Query:               req.Query,
		Limit:               req.Limit,
		Offset:              req.Offset,
		SortBy:              req.SortBy,
		SortOrder:           req.SortOrder,
		UseCache:            true,
		Timeout:             60 * time.Second, // Add timeout for large facet operations
		IncludeHighlighting: true,
		IncludeFacets:       true,                         // Re-enable facets for accurate summary stats
		FacetFields:         []string{"ip", "path_exact"}, // For UV and Unique Pages
		FacetSize:           10000,                        // Balanced: large enough for most cases, but not excessive
	}

	// If no sorting is specified, default to sorting by timestamp descending.
	if searchReq.SortBy == "" {
		searchReq.SortBy = "timestamp"
		searchReq.SortOrder = "desc"
	}

	// Expand the base log path to all physical files in the group using filesystem globbing.
	if req.LogPath != "" {
		logPaths, err := nginx_log.ExpandLogGroupPath(req.LogPath)
		if err != nil {
			logger.Warnf("Could not expand log group path %s: %v", req.LogPath, err)
			// Fallback to using the raw path when expansion fails
			searchReq.LogPaths = []string{req.LogPath}
		} else if len(logPaths) == 0 {
			// ExpandLogGroupPath succeeded but returned empty slice (file doesn't exist on filesystem)
			// Still search for historical indexed data using the requested path
			logger.Debugf("Log file %s does not exist on filesystem, but searching for historical indexed data", req.LogPath)
			searchReq.LogPaths = []string{req.LogPath}
		} else {
			searchReq.LogPaths = logPaths
		}
		logger.Debugf("Search request LogPaths: %v", searchReq.LogPaths)
	}

	// Add time filters
	if req.StartTime > 0 {
		searchReq.StartTime = &req.StartTime
	}
	if req.EndTime > 0 {
		searchReq.EndTime = &req.EndTime
	}
	// If no time range is provided, default to searching all time.
	if searchReq.StartTime == nil && searchReq.EndTime == nil {
		var startTime int64 = 0 // Unix epoch
		now := time.Now().Unix()
		searchReq.StartTime = &startTime
		searchReq.EndTime = &now
	}

	// Add field filters
	if req.IP != "" {
		searchReq.IPAddresses = []string{req.IP}
	}
	if req.Method != "" {
		searchReq.Methods = []string{req.Method}
	}
	if req.Path != "" {
		searchReq.Paths = []string{req.Path}
	}
	if req.UserAgent != "" {
		searchReq.UserAgents = []string{req.UserAgent}
	}
	if req.Referer != "" {
		searchReq.Referers = []string{req.Referer}
	}
	if req.Browser != "" {
		searchReq.Browsers = []string{req.Browser}
	}
	if req.OS != "" {
		searchReq.OSs = []string{req.OS}
	}
	if req.Device != "" {
		searchReq.Devices = []string{req.Device}
	}
	if len(req.Status) > 0 {
		searchReq.StatusCodes = req.Status
	}

	// Execute search with timeout
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Minute)
	defer cancel()

	result, err := searcherService.Search(ctx, searchReq)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	// --- Transform the searcher result to the API response structure ---

	// 1. Extract entries from hits
	entries := make([]map[string]interface{}, len(result.Hits))
	var totalTraffic int64 // Total traffic is for the entire result set, must be calculated separately if needed.
	for i, hit := range result.Hits {
		entries[i] = hit.Fields
		if bytesSent, ok := hit.Fields["bytes_sent"].(float64); ok {
			totalTraffic += int64(bytesSent)
		}
	}

	// 2. Calculate summary stats from the overall results using Counter for accuracy
	pv := int(result.TotalHits)
	var uv, uniquePages int
	var facetUV, facetUniquePages int

	// First get facet values as fallback
	if result.Facets != nil {
		if ipFacet, ok := result.Facets["ip"]; ok {
			facetUV = ipFacet.Total // .Total on a facet gives the count of unique terms
			uv = facetUV
		}
		if pathFacet, ok := result.Facets["path_exact"]; ok {
			facetUniquePages = pathFacet.Total
			uniquePages = facetUniquePages
		}
	}

	// Override with Counter results for better accuracy
	if analyticsService != nil {
		// Get cardinality counts for UV (unique IPs)
		if uvResult := getCardinalityCount(ctx, "ip", searchReq); uvResult > 0 {
			uv = uvResult
			logger.Debugf("🔢 Search endpoint - UV from Counter: %d (vs facet: %d)", uvResult, facetUV)
		}

		// Get cardinality counts for Unique Pages (unique paths)
		if upResult := getCardinalityCount(ctx, "path_exact", searchReq); upResult > 0 {
			uniquePages = upResult
			logger.Debugf("🔢 Search endpoint - Unique Pages from Counter: %d (vs facet: %d)", upResult, facetUniquePages)
		}
	}

	// Note: TotalTraffic is not available for the whole result set without a separate query.
	// We will approximate it based on the current page's average for now.
	var avgBytesOnPage float64
	if len(result.Hits) > 0 {
		avgBytesOnPage = float64(totalTraffic) / float64(len(result.Hits))
	}
	approximatedTotalTraffic := int64(avgBytesOnPage * float64(pv))

	var avgTraffic float64
	if pv > 0 {
		avgTraffic = float64(approximatedTotalTraffic) / float64(pv)
	}

	summary := SummaryStats{
		UV:              uv,
		PV:              pv,
		TotalTraffic:    approximatedTotalTraffic,
		UniquePages:     uniquePages,
		AvgTrafficPerPV: avgTraffic,
	}

	// 3. Assemble the final response
	apiResponse := AdvancedSearchResponseAPI{
		Entries: entries,
		Total:   result.TotalHits,
		Took:    result.Duration.Milliseconds(),
		Query:   req.Query,
		Summary: summary,
	}

	c.JSON(http.StatusOK, apiResponse)
}

// GetLogEntries provides simple log entry retrieval
func GetLogEntries(c *gin.Context) {
	var req struct {
		Path  string `json:"path" form:"path"`
		Limit int    `json:"limit" form:"limit"`
		Tail  bool   `json:"tail" form:"tail"` // Get latest entries
	}

	if !cosy.BindAndValid(c, &req) {
		return
	}

	searcherService := nginx_log.GetSearcher()
	if searcherService == nil {
		cosy.ErrHandler(c, nginx_log.ErrModernSearcherNotAvailable)
		return
	}

	analyticsService := nginx_log.GetAnalytics()
	if analyticsService == nil {
		cosy.ErrHandler(c, nginx_log.ErrModernAnalyticsNotAvailable)
		return
	}

	// Validate log path
	if err := analyticsService.ValidateLogPath(req.Path); err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	// Set default limit
	if req.Limit == 0 {
		req.Limit = 100
	}

	// Build search request
	searchReq := &searcher.SearchRequest{
		Limit:     req.Limit,
		UseCache:  false, // Don't cache simple entry requests
		SortBy:    "timestamp",
		SortOrder: "desc", // Latest first by default
	}

	if req.Tail {
		searchReq.SortOrder = "desc" // Latest entries first
	} else {
		searchReq.SortOrder = "asc" // Oldest entries first
	}

	// Execute search
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	result, err := searcherService.Search(ctx, searchReq)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	// Convert search hits to simple entries format
	var entries []map[string]interface{}
	for _, hit := range result.Hits {
		entries = append(entries, hit.Fields)
	}

	c.JSON(http.StatusOK, AnalyticsResponse{
		Entries: entries,
		Count:   len(entries),
	})
}

// DashboardRequest represents the request for dashboard analytics
type DashboardRequest struct {
	LogPath   string `json:"log_path" form:"log_path"`
	StartDate string `json:"start_date" form:"start_date"` // Format: 2006-01-02
	EndDate   string `json:"end_date" form:"end_date"`     // Format: 2006-01-02
}

// HourlyStats represents hourly UV/PV statistics
type HourlyStats struct {
	Hour      int   `json:"hour"`      // 0-23
	UV        int   `json:"uv"`        // Unique visitors (unique IPs)
	PV        int   `json:"pv"`        // Page views (total requests)
	Timestamp int64 `json:"timestamp"` // Unix timestamp for the hour
}

// DailyStats represents daily access statistics
type DailyStats struct {
	Date      string `json:"date"`      // YYYY-MM-DD format
	UV        int    `json:"uv"`        // Unique visitors
	PV        int    `json:"pv"`        // Page views
	Timestamp int64  `json:"timestamp"` // Unix timestamp for the day
}

// URLStats represents URL access statistics
type URLStats struct {
	URL     string  `json:"url"`
	Visits  int     `json:"visits"`
	Percent float64 `json:"percent"`
}

// BrowserStats represents browser statistics
type BrowserStats struct {
	Browser string  `json:"browser"`
	Count   int     `json:"count"`
	Percent float64 `json:"percent"`
}

// OSStats represents operating system statistics
type OSStats struct {
	OS      string  `json:"os"`
	Count   int     `json:"count"`
	Percent float64 `json:"percent"`
}

// DeviceStats represents device type statistics
type DeviceStats struct {
	Device  string  `json:"device"`
	Count   int     `json:"count"`
	Percent float64 `json:"percent"`
}

// DashboardResponse represents the dashboard analytics response
type DashboardResponse struct {
	HourlyStats      []HourlyStats  `json:"hourly_stats"`      // 24-hour UV/PV data
	DailyStats       []DailyStats   `json:"daily_stats"`       // Monthly trend data
	TopURLs          []URLStats     `json:"top_urls"`          // TOP 10 URLs
	Browsers         []BrowserStats `json:"browsers"`          // Browser statistics
	OperatingSystems []OSStats      `json:"operating_systems"` // OS statistics
	Devices          []DeviceStats  `json:"devices"`           // Device statistics
	Summary          struct {
		TotalUV         int     `json:"total_uv"`          // Total unique visitors
		TotalPV         int     `json:"total_pv"`          // Total page views
		AvgDailyUV      float64 `json:"avg_daily_uv"`      // Average daily UV
		AvgDailyPV      float64 `json:"avg_daily_pv"`      // Average daily PV
		PeakHour        int     `json:"peak_hour"`         // Peak traffic hour (0-23)
		PeakHourTraffic int     `json:"peak_hour_traffic"` // Peak hour PV count
	} `json:"summary"`
}

// GetDashboardAnalytics provides comprehensive dashboard analytics from modern analytics service
func GetDashboardAnalytics(c *gin.Context) {
	var req DashboardRequest

	// Parse JSON body for POST request
	if err := c.ShouldBindJSON(&req); err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	logger.Debugf("Dashboard API received log_path: '%s', start_date: '%s', end_date: '%s'", req.LogPath, req.StartDate, req.EndDate)

	analyticsService := nginx_log.GetAnalytics()
	if analyticsService == nil {
		cosy.ErrHandler(c, nginx_log.ErrModernAnalyticsNotAvailable)
		return
	}

	// Use default access log path if LogPath is empty
	if req.LogPath == "" {
		defaultLogPath := nginx.GetAccessLogPath()
		if defaultLogPath != "" {
			req.LogPath = defaultLogPath
			logger.Debugf("Using default access log path: %s", req.LogPath)
		}
	}

	// Validate log path if provided
	if req.LogPath != "" {
		if err := analyticsService.ValidateLogPath(req.LogPath); err != nil {
			cosy.ErrHandler(c, err)
			return
		}
	}

	// Parse and validate date strings
	var startTime, endTime time.Time
	var err error

	if req.StartDate != "" {
		startTime, err = time.Parse("2006-01-02", req.StartDate)
		if err != nil {
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid start_date format, expected YYYY-MM-DD: " + err.Error()})
			return
		}
		// Convert to UTC for consistent processing
		startTime = startTime.UTC()
	}

	if req.EndDate != "" {
		endTime, err = time.Parse("2006-01-02", req.EndDate)
		if err != nil {
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid end_date format, expected YYYY-MM-DD: " + err.Error()})
			return
		}
		// Set end time to end of day and convert to UTC
		endTime = endTime.Add(23*time.Hour + 59*time.Minute + 59*time.Second).UTC()
	}

	// Set default time range if not provided (last 30 days)
	if startTime.IsZero() || endTime.IsZero() {
		endTime = time.Now()
		startTime = endTime.AddDate(0, 0, -30) // 30 days ago
	}

	// Get dashboard analytics with timeout
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	logger.Debugf("Dashboard request for log_path: %s, parsed start_time: %v, end_time: %v", req.LogPath, startTime, endTime)

	// Use main_log_path field for efficient log group queries instead of expanding file paths
	// This provides much better performance by using indexed field filtering
	logger.Debugf("Dashboard querying log group with main_log_path: %s", req.LogPath)

	// Build dashboard query request
	dashboardReq := &analytics.DashboardQueryRequest{
		LogPath:   req.LogPath,
		LogPaths:  []string{req.LogPath}, // Use single main log path
		StartTime: startTime.Unix(),
		EndTime:   endTime.Unix(),
	}
	logger.Debugf("Query parameters - LogPath='%s', StartTime=%v, EndTime=%v",
		dashboardReq.LogPath, dashboardReq.StartTime, dashboardReq.EndTime)

	// Get analytics from modern analytics service
	result, err := analyticsService.GetDashboardAnalytics(ctx, dashboardReq)

	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	logger.Debugf("Successfully retrieved dashboard analytics")

	// Debug: Log summary of results
	if result != nil {
		logger.Debugf("Results summary - TotalUV=%d, TotalPV=%d, HourlyStats=%d, DailyStats=%d, TopURLs=%d",
			result.Summary.TotalUV, result.Summary.TotalPV,
			len(result.HourlyStats), len(result.DailyStats), len(result.TopURLs))
	} else {
		logger.Debugf("Analytics result is nil")
	}

	c.JSON(http.StatusOK, result)
}

// GetWorldMapData provides geographic data for world map visualization
func GetWorldMapData(c *gin.Context) {
	var req AnalyticsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	logger.Debugf("=== DEBUG GetWorldMapData START ===")
	logger.Debugf("WorldMapData request - Path: '%s', StartTime: %d, EndTime: %d, Limit: %d",
		req.Path, req.StartTime, req.EndTime, req.Limit)

	analyticsService := nginx_log.GetAnalytics()
	if analyticsService == nil {
		cosy.ErrHandler(c, nginx_log.ErrModernAnalyticsNotAvailable)
		return
	}

	// Use default access log path if Path is empty
	if req.Path == "" {
		defaultLogPath := nginx.GetAccessLogPath()
		if defaultLogPath != "" {
			req.Path = defaultLogPath
			logger.Debugf("Using default access log path for world map: %s", req.Path)
		}
	}

	// Validate log path if provided
	if req.Path != "" {
		if err := analyticsService.ValidateLogPath(req.Path); err != nil {
			cosy.ErrHandler(c, err)
			return
		}
	}

	// Use main_log_path field for efficient log group queries instead of expanding file paths
	logger.Debugf("WorldMapData - Using main_log_path field for log group: %s", req.Path)

	// Get world map data with timeout
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	geoReq := &analytics.GeoQueryRequest{
		StartTime:      req.StartTime,
		EndTime:        req.EndTime,
		LogPath:        req.Path,
		LogPaths:       []string{req.Path}, // Use single main log path
		UseMainLogPath: true,              // Use main_log_path field for efficient queries
		Limit:          req.Limit,
	}
	logger.Debugf("WorldMapData - GeoQueryRequest: %+v", geoReq)

	data, err := analyticsService.GetGeoDistribution(ctx, geoReq)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	logger.Debugf("WorldMapData - GetGeoDistribution returned data with %d countries", len(data.Countries))
	for code, count := range data.Countries {
		if code == "CN" {
			logger.Debugf("WorldMapData - CN country count: %d", count)
		}
		logger.Debugf("WorldMapData - Country: '%s', Count: %d", code, count)
	}

	// Transform map to slice for frontend chart compatibility, calculate percentages, and sort.
	chartData := make([]GeoRegionItem, 0, len(data.Countries))
	totalValue := 0
	for _, value := range data.Countries {
		totalValue += value
	}
	logger.Debugf("WorldMapData - Total value calculated: %d", totalValue)

	for code, value := range data.Countries {
		percent := 0.0
		if totalValue > 0 {
			percent = (float64(value) / float64(totalValue)) * 100
		}
		chartData = append(chartData, GeoRegionItem{Code: code, Value: value, Percent: percent})
	}

	// Sort by value descending
	sort.Slice(chartData, func(i, j int) bool {
		return chartData[i].Value > chartData[j].Value
	})

	logger.Debugf("WorldMapData - Final response data contains %d items with total value %d", len(chartData), totalValue)
	for i, item := range chartData {
		if item.Code == "CN" {
			logger.Debugf("WorldMapData - FOUND CN - [%d] Code: '%s', Value: %d, Percent: %.2f%%", i, item.Code, item.Value, item.Percent)
		}
		logger.Debugf("WorldMapData - [%d] Code: '%s', Value: %d, Percent: %.2f%%", i, item.Code, item.Value, item.Percent)
	}
	logger.Debugf("=== DEBUG GetWorldMapData END ===")

	c.JSON(http.StatusOK, GeoRegionResponse{
		Data: chartData,
	})
}

// GetChinaMapData provides geographic data for China map visualization
func GetChinaMapData(c *gin.Context) {
	var req AnalyticsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	logger.Debugf("=== DEBUG GetChinaMapData START ===")
	logger.Debugf("ChinaMapData request - Path: '%s', StartTime: %d, EndTime: %d, Limit: %d",
		req.Path, req.StartTime, req.EndTime, req.Limit)

	analyticsService := nginx_log.GetAnalytics()
	if analyticsService == nil {
		cosy.ErrHandler(c, nginx_log.ErrModernAnalyticsNotAvailable)
		return
	}

	// Use default access log path if Path is empty
	if req.Path == "" {
		defaultLogPath := nginx.GetAccessLogPath()
		if defaultLogPath != "" {
			req.Path = defaultLogPath
			logger.Debugf("Using default access log path for China map: %s", req.Path)
		}
	}

	// Validate log path if provided
	if req.Path != "" {
		if err := analyticsService.ValidateLogPath(req.Path); err != nil {
			cosy.ErrHandler(c, err)
			return
		}
	}

	// Use main_log_path field for efficient log group queries instead of expanding file paths
	logger.Debugf("ChinaMapData - Using main_log_path field for log group: %s", req.Path)

	// Get China map data with timeout
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	geoReq := &analytics.GeoQueryRequest{
		StartTime:      req.StartTime,
		EndTime:        req.EndTime,
		LogPath:        req.Path,
		LogPaths:       []string{req.Path}, // Use single main log path
		UseMainLogPath: true,              // Use main_log_path field for efficient queries
		Limit:          req.Limit,
	}
	logger.Debugf("ChinaMapData - GeoQueryRequest: %+v", geoReq)

	// Get distribution specifically for China (country code "CN")
	logger.Debugf("ChinaMapData - About to call GetGeoDistributionByCountry with country code 'CN'")
	data, err := analyticsService.GetGeoDistributionByCountry(ctx, geoReq, "CN")
	if err != nil {
		logger.Debugf("ChinaMapData - GetGeoDistributionByCountry returned error: %v", err)
		cosy.ErrHandler(c, err)
		return
	}

	logger.Debugf("ChinaMapData - GetGeoDistributionByCountry returned data with %d provinces", len(data.Countries))
	for name, count := range data.Countries {
		logger.Debugf("ChinaMapData - Province: '%s', Count: %d", name, count)
	}

	// Transform map to slice for frontend chart compatibility, calculate percentages, and sort.
	chartData := make([]GeoDataItem, 0, len(data.Countries))
	totalValue := 0
	for _, value := range data.Countries {
		totalValue += value
	}
	logger.Debugf("ChinaMapData - Total value calculated: %d", totalValue)

	for name, value := range data.Countries {
		percent := 0.0
		if totalValue > 0 {
			percent = (float64(value) / float64(totalValue)) * 100
		}
		chartData = append(chartData, GeoDataItem{Name: name, Value: value, Percent: percent})
	}

	// Sort by value descending
	sort.Slice(chartData, func(i, j int) bool {
		return chartData[i].Value > chartData[j].Value
	})

	logger.Debugf("ChinaMapData - Final response data contains %d items with total value %d", len(chartData), totalValue)
	for i, item := range chartData {
		logger.Debugf("ChinaMapData - [%d] Name: '%s', Value: %d, Percent: %.2f%%", i, item.Name, item.Value, item.Percent)
	}
	logger.Debugf("=== DEBUG GetChinaMapData END ===")

	c.JSON(http.StatusOK, GeoDataResponse{
		Data: chartData,
	})
}

// GetGeoStats provides geographic statistics
func GetGeoStats(c *gin.Context) {
	var req AnalyticsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid JSON request body: " + err.Error()})
		return
	}

	analyticsService := nginx_log.GetAnalytics()
	if analyticsService == nil {
		cosy.ErrHandler(c, nginx_log.ErrModernAnalyticsNotAvailable)
		return
	}

	// Use default access log path if Path is empty
	if req.Path == "" {
		defaultLogPath := nginx.GetAccessLogPath()
		if defaultLogPath != "" {
			req.Path = defaultLogPath
			logger.Debugf("Using default access log path for geo stats: %s", req.Path)
		}
	}

	// Validate log path if provided
	if req.Path != "" {
		if err := analyticsService.ValidateLogPath(req.Path); err != nil {
			cosy.ErrHandler(c, err)
			return
		}
	}

	// Use main_log_path field for efficient log group queries instead of expanding file paths
	logger.Debugf("GeoStats - Using main_log_path field for log group: %s", req.Path)

	// Set default limit if not provided
	if req.Limit == 0 {
		req.Limit = 20
	}

	// Get geographic statistics with timeout
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	geoReq := &analytics.GeoQueryRequest{
		StartTime:      req.StartTime,
		EndTime:        req.EndTime,
		LogPath:        req.Path,
		LogPaths:       []string{req.Path}, // Use single main log path
		UseMainLogPath: true,              // Use main_log_path field for efficient queries
		Limit:          req.Limit,
	}

	stats, err := analyticsService.GetTopCountries(ctx, geoReq)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	// Convert to []interface{} for JSON serialization
	statsInterface := make([]interface{}, len(stats))
	for i, stat := range stats {
		statsInterface[i] = stat
	}
	
	c.JSON(http.StatusOK, GeoStatsResponse{
		Stats: statsInterface,
	})
}

// getCardinalityCount is a helper function to get accurate cardinality counts using the analytics service
func getCardinalityCount(ctx context.Context, field string, searchReq *searcher.SearchRequest) int {
	logger.Debugf("🔍 getCardinalityCount: Starting cardinality count for field '%s'", field)

	// Create a CardinalityRequest from the SearchRequest
	cardReq := &searcher.CardinalityRequest{
		Field:          field,
		StartTime:      searchReq.StartTime,
		EndTime:        searchReq.EndTime,
		LogPaths:       searchReq.LogPaths,
		UseMainLogPath: searchReq.UseMainLogPath, // Use main_log_path field if enabled
	}
	logger.Debugf("🔍 CardinalityRequest: Field=%s, StartTime=%v, EndTime=%v, LogPaths=%v",
		cardReq.Field, cardReq.StartTime, cardReq.EndTime, cardReq.LogPaths)

	// Try to get the searcher to access cardinality counter
	searcherService := nginx_log.GetSearcher()
	if searcherService == nil {
		logger.Debugf("🚨 getCardinalityCount: ModernSearcher not available for field %s", field)
		return 0
	}
	logger.Debugf("🔍 getCardinalityCount: ModernSearcher available, type: %T", searcherService)

	// Use searcher to access Counter
	if searcherService != nil {
		ds := searcherService
		logger.Debugf("🔍 getCardinalityCount: Successfully cast to Searcher")
		shards := ds.GetShards()
		logger.Debugf("🔍 getCardinalityCount: Retrieved %d shards", len(shards))
		if len(shards) > 0 {
			// Check shard health
			for i, shard := range shards {
				logger.Debugf("🔍 getCardinalityCount: Shard %d: %v", i, shard != nil)
			}

			cardinalityCounter := searcher.NewCounter(shards)
			logger.Debugf("🔍 getCardinalityCount: Created Counter")
			result, err := cardinalityCounter.Count(ctx, cardReq)
			if err != nil {
				logger.Debugf("🚨 getCardinalityCount: Counter failed for field %s: %v", field, err)
				return 0
			}

			if result.Error != "" {
				logger.Debugf("🚨 getCardinalityCount: Counter returned error for field %s: %s", field, result.Error)
				return 0
			}

			logger.Debugf("✅ getCardinalityCount: Successfully got cardinality for field %s: %d", field, result.Cardinality)
			return int(result.Cardinality)
		} else {
			logger.Debugf("🚨 getCardinalityCount: Searcher has no shards for field %s", field)
		}
	} else {
		logger.Debugf("🚨 getCardinalityCount: Searcher is not Searcher (type: %T) for field %s", searcherService, field)
	}

	return 0
}
