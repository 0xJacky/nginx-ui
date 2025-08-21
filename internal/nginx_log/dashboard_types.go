package nginx_log

// DashboardQueryRequest represents a request for dashboard analytics
type DashboardQueryRequest struct {
	StartTime int64  `json:"start_time"` // Unix timestamp
	EndTime   int64  `json:"end_time"`   // Unix timestamp
	LogPath   string `json:"log_path,omitempty"`
}

// DashboardAnalytics represents comprehensive dashboard analytics data
type DashboardAnalytics struct {
	HourlyStats      []HourlyAccessStats   `json:"hourly_stats"`
	DailyStats       []DailyAccessStats    `json:"daily_stats"`
	TopURLs          []URLAccessStats      `json:"top_urls"`
	Browsers         []BrowserAccessStats  `json:"browsers"`
	OperatingSystems []OSAccessStats       `json:"operating_systems"`
	Devices          []DeviceAccessStats   `json:"devices"`
	Summary          DashboardSummary      `json:"summary"`
}

// DashboardSummary represents summary statistics for the dashboard
type DashboardSummary struct {
	TotalUV         int     `json:"total_uv"`
	TotalPV         int     `json:"total_pv"`
	AvgDailyUV      float64 `json:"avg_daily_uv"`
	AvgDailyPV      float64 `json:"avg_daily_pv"`
	PeakHour        int     `json:"peak_hour"`
	PeakHourTraffic int     `json:"peak_hour_traffic"`
}

// HourlyAccessStats represents hourly access statistics
type HourlyAccessStats struct {
	Hour      int   `json:"hour"`
	UV        int   `json:"uv"`
	PV        int   `json:"pv"`
	Timestamp int64 `json:"timestamp"`
}

// DailyAccessStats represents daily access statistics
type DailyAccessStats struct {
	Date      string `json:"date"`
	UV        int    `json:"uv"`
	PV        int    `json:"pv"`
	Timestamp int64  `json:"timestamp"`
}

// URLAccessStats represents URL access statistics
type URLAccessStats struct {
	URL     string  `json:"url"`
	Visits  int     `json:"visits"`
	Percent float64 `json:"percent"`
}

// BrowserAccessStats represents browser usage statistics
type BrowserAccessStats struct {
	Browser string  `json:"browser"`
	Count   int     `json:"count"`
	Percent float64 `json:"percent"`
}

// OSAccessStats represents operating system usage statistics
type OSAccessStats struct {
	OS      string  `json:"os"`
	Count   int     `json:"count"`
	Percent float64 `json:"percent"`
}

// DeviceAccessStats represents device type usage statistics
type DeviceAccessStats struct {
	Device  string  `json:"device"`
	Count   int     `json:"count"`
	Percent float64 `json:"percent"`
}