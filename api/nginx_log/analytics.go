package nginx_log

import (
	"context"
	"net/http"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/nginx_log"
	"github.com/gin-gonic/gin"
	"github.com/uozi-tech/cosy"
)

// AnalyticsRequest represents the request for log analytics
type AnalyticsRequest struct {
	Path      string    `json:"path" form:"path"`
	StartTime time.Time `json:"start_time" form:"start_time"`
	EndTime   time.Time `json:"end_time" form:"end_time"`
	Limit     int       `json:"limit" form:"limit"`
}

// AdvancedSearchRequest represents the request for advanced log search
type AdvancedSearchRequest struct {
	Query     string    `json:"query" form:"query"`
	LogPath   string    `json:"log_path" form:"log_path"`
	StartTime time.Time `json:"start_time" form:"start_time"`
	EndTime   time.Time `json:"end_time" form:"end_time"`
	IP        string    `json:"ip" form:"ip"`
	Method    string    `json:"method" form:"method"`
	Status    []int     `json:"status" form:"status"`
	Path      string    `json:"path" form:"path"`
	UserAgent string    `json:"user_agent" form:"user_agent"`
	Referer   string    `json:"referer" form:"referer"`
	Browser   string    `json:"browser" form:"browser"`
	OS        string    `json:"os" form:"os"`
	Device    string    `json:"device" form:"device"`
	Limit     int       `json:"limit" form:"limit"`
	Offset    int       `json:"offset" form:"offset"`
	SortBy    string    `json:"sort_by" form:"sort_by"`
	SortOrder string    `json:"sort_order" form:"sort_order"`
}

// PreflightResponse represents the response for preflight query
type PreflightResponse struct {
	StartTime   *time.Time `json:"start_time,omitempty"`
	EndTime     *time.Time `json:"end_time,omitempty"`
	Available   bool       `json:"available"`
	IndexStatus string     `json:"index_status"`
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

	var start, end time.Time
	var indexStatus string
	
	// Check real indexing status using IndexingStatusManager
	statusManager := nginx_log.GetIndexingStatusManager()
	isCurrentlyIndexing := statusManager.IsIndexing()
	
	if logPath != "" {
		// Validate log path exists
		if err := service.ValidateLogPath(logPath); err != nil {
			cosy.ErrHandler(c, err)
			return
		}

		// Get time range for specific log file
		start, end = service.GetTimeRangeForPath(logPath)
		
		// Check if this specific file is being indexed
		if nginx_log.IsFileIndexing(logPath) {
			indexStatus = "indexing"
		} else if !start.IsZero() && !end.IsZero() {
			// Trust persistence data - if we have time range, index is ready
			indexStatus = "ready"
		} else {
			indexStatus = "not_indexed"
		}
	} else {
		// Get time range for all indexed logs
		start, end = service.GetTimeRange()
		
		// Use global indexing status
		if isCurrentlyIndexing {
			indexStatus = "indexing"
		} else if !start.IsZero() && !end.IsZero() {
			// Trust persistence data - if we have global time range, index is ready
			indexStatus = "ready"
		} else {
			indexStatus = "not_indexed"
		}
	}

	var startPtr, endPtr *time.Time
	if !start.IsZero() {
		startPtr = &start
	}
	if !end.IsZero() {
		endPtr = &end
	}

	response := PreflightResponse{
		StartTime:   startPtr,
		EndTime:     endPtr,
		Available:   !start.IsZero() && !end.IsZero(),
		IndexStatus: indexStatus,
	}

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
