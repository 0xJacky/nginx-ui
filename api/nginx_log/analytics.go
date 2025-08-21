package nginx_log

import (
	"context"
	"net/http"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/internal/nginx_log"
	"github.com/gin-gonic/gin"
	"github.com/uozi-tech/cosy"
	"github.com/uozi-tech/cosy/logger"
)

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

// PreflightResponse represents the response for preflight query
type PreflightResponse struct {
	StartTime   *int64 `json:"start_time,omitempty"`
	EndTime     *int64 `json:"end_time,omitempty"`
	Available   bool   `json:"available"`
	IndexStatus string `json:"index_status"`
}

// GetLogAnalytics provides comprehensive log analytics
func GetLogAnalytics(c *gin.Context) {
	var req AnalyticsRequest
	if !cosy.BindAndValid(c, &req) {
		return
	}

	// Get analytics service
	service := nginx_log.GetAnalyticsService()
	if service == nil {
		cosy.ErrHandler(c, nginx_log.ErrAnalyticsServiceNotAvailable)
		return
	}

	// Validate log path
	if err := service.ValidateLogPath(req.Path); err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	// Analyze log file
	analytics, err := service.AnalyzeLogFile(req.Path)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	c.JSON(http.StatusOK, analytics)
}

// GetLogPreflight returns the preflight status for log indexing
func GetLogPreflight(c *gin.Context) {
	service := nginx_log.GetAnalyticsService()
	if service == nil {
		cosy.ErrHandler(c, nginx_log.ErrAnalyticsServiceNotAvailable)
		return
	}

	// Get optional log path parameter
	logPath := c.Query("log_path")

	// Use default access log path if logPath is empty
	if logPath == "" {
		defaultLogPath := nginx.GetAccessLogPath()
		if defaultLogPath != "" {
			logPath = defaultLogPath
			logger.Debugf("Using default access log path for preflight: %s", logPath)
		}
	}

	// Use service method to get preflight status
	result, err := service.GetPreflightStatus(logPath)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	// Convert internal result to API response
	response := PreflightResponse{
		StartTime:   &result.StartTime,
		EndTime:     &result.EndTime,
		Available:   result.Available,
		IndexStatus: result.IndexStatus,
	}

	logger.Debugf("Preflight response: log_path=%s, available=%v, index_status=%s", 
		logPath, result.Available, result.IndexStatus)

	c.JSON(http.StatusOK, response)
}

// Note: GetIndexStatus function removed - index status is now included in GetLogList response

// AdvancedSearchLogs provides advanced search capabilities for logs
func AdvancedSearchLogs(c *gin.Context) {
	var req AdvancedSearchRequest
	if !cosy.BindAndValid(c, &req) {
		return
	}

	service := nginx_log.GetAnalyticsService()
	if service == nil {
		cosy.ErrHandler(c, nginx_log.ErrAnalyticsServiceNotAvailable)
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
		if err := service.ValidateLogPath(req.LogPath); err != nil {
			cosy.ErrHandler(c, err)
			return
		}
	}

	// Build query request
	queryReq := &nginx_log.QueryRequest{
		StartTime: req.StartTime,
		EndTime:   req.EndTime,
		Query:     req.Query,
		IP:        req.IP,
		Method:    req.Method,
		Path:      req.Path,
		UserAgent: req.UserAgent,
		Referer:   req.Referer,
		Browser:   req.Browser,
		OS:        req.OS,
		Device:    req.Device,
		Limit:     req.Limit,
		Offset:    req.Offset,
		SortBy:    req.SortBy,
		SortOrder: req.SortOrder,
		LogPath:   req.LogPath,
	}

	// Add status filter if provided
	if len(req.Status) > 0 {
		queryReq.Status = req.Status
	}

	// Execute search with timeout
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	result, err := service.SearchLogs(ctx, queryReq)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	c.JSON(http.StatusOK, result)
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

	service := nginx_log.GetAnalyticsService()
	if service == nil {
		cosy.ErrHandler(c, nginx_log.ErrAnalyticsServiceNotAvailable)
		return
	}

	// Validate log path
	if err := service.ValidateLogPath(req.Path); err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	// Get log entries
	entries, err := service.GetLogEntries(req.Path, req.Limit, req.Tail)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"entries": entries,
		"count":   len(entries),
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

// GetDashboardAnalytics provides comprehensive dashboard analytics from Bleve aggregations
func GetDashboardAnalytics(c *gin.Context) {
	var req DashboardRequest

	// Parse JSON body for POST request
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON request body: " + err.Error()})
		return
	}

	logger.Debugf("Dashboard API received log_path: '%s', start_date: '%s', end_date: '%s'", req.LogPath, req.StartDate, req.EndDate)

	service := nginx_log.GetAnalyticsService()
	if service == nil {
		cosy.ErrHandler(c, nginx_log.ErrAnalyticsServiceNotAvailable)
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
		if err := service.ValidateLogPath(req.LogPath); err != nil {
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
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid start_date format, expected YYYY-MM-DD: " + err.Error()})
			return
		}
	}

	if req.EndDate != "" {
		endTime, err = time.Parse("2006-01-02", req.EndDate)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid end_date format, expected YYYY-MM-DD: " + err.Error()})
			return
		}
		// Set end time to end of day
		endTime = endTime.Add(23*time.Hour + 59*time.Minute + 59*time.Second)
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

	// Debug: Check time range from Bleve for this file
	debugStart, debugEnd := service.GetTimeRangeFromSummaryStatsForPath(req.LogPath)
	logger.Debugf("Bleve time range for %s - start=%v, end=%v", req.LogPath, debugStart, debugEnd)
	
	// Debug: Log exact query parameters
	queryRequest := &nginx_log.DashboardQueryRequest{
		LogPath:   req.LogPath,
		StartTime: startTime.Unix(),
		EndTime:   endTime.Unix(),
	}
	logger.Debugf("Query parameters - LogPath='%s', StartTime=%v, EndTime=%v", 
		queryRequest.LogPath, queryRequest.StartTime, queryRequest.EndTime)

	// Get analytics from Bleve aggregations
	analytics, err := service.GetDashboardAnalyticsFromStats(ctx, queryRequest)

	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	logger.Debugf("Successfully retrieved dashboard analytics from Bleve aggregations")
	
	// Debug: Log summary of results
	if analytics != nil {
		logger.Debugf("Results summary - TotalUV=%d, TotalPV=%d, HourlyStats=%d, DailyStats=%d, TopURLs=%d", 
			analytics.Summary.TotalUV, analytics.Summary.TotalPV, 
			len(analytics.HourlyStats), len(analytics.DailyStats), len(analytics.TopURLs))
	} else {
		logger.Debugf("Analytics result is nil")
	}
	
	c.JSON(http.StatusOK, analytics)
}

// GetWorldMapData provides geographic data for world map visualization
func GetWorldMapData(c *gin.Context) {
	var req AnalyticsRequest
	if !cosy.BindAndValid(c, &req) {
		return
	}

	service := nginx_log.GetAnalyticsService()
	if service == nil {
		cosy.ErrHandler(c, nginx_log.ErrAnalyticsServiceNotAvailable)
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
		if err := service.ValidateLogPath(req.Path); err != nil {
			cosy.ErrHandler(c, err)
			return
		}
	}

	// Get world map data with timeout
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	data, err := service.GetWorldMapData(ctx, req.Path, time.Unix(req.StartTime, 0), time.Unix(req.EndTime, 0))
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": data,
	})
}

// GetChinaMapData provides geographic data for China map visualization
func GetChinaMapData(c *gin.Context) {
	var req AnalyticsRequest
	if !cosy.BindAndValid(c, &req) {
		return
	}

	service := nginx_log.GetAnalyticsService()
	if service == nil {
		cosy.ErrHandler(c, nginx_log.ErrAnalyticsServiceNotAvailable)
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
		if err := service.ValidateLogPath(req.Path); err != nil {
			cosy.ErrHandler(c, err)
			return
		}
	}

	// Get China map data with timeout
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	data, err := service.GetChinaMapData(ctx, req.Path, time.Unix(req.StartTime, 0), time.Unix(req.EndTime, 0))
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": data,
	})
}

// GetGeoStats provides geographic statistics
func GetGeoStats(c *gin.Context) {
	var req AnalyticsRequest
	if !cosy.BindAndValid(c, &req) {
		return
	}

	service := nginx_log.GetAnalyticsService()
	if service == nil {
		cosy.ErrHandler(c, nginx_log.ErrAnalyticsServiceNotAvailable)
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
		if err := service.ValidateLogPath(req.Path); err != nil {
			cosy.ErrHandler(c, err)
			return
		}
	}

	// Set default limit if not provided
	if req.Limit == 0 {
		req.Limit = 20
	}

	// Get geographic statistics with timeout
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	stats, err := service.GetGeoStats(ctx, req.Path, time.Unix(req.StartTime, 0), time.Unix(req.EndTime, 0), req.Limit)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"stats": stats,
	})
}
