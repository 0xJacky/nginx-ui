package nginx_log

// This file has been split into multiple modules for better organization:
//
// - bleve_stats_service_core.go: Core service structure, initialization, main analytics function
// - bleve_stats_service_time.go: Time-based statistics calculations (hourly/daily)
// - bleve_stats_service_aggregations.go: URL, browser, OS, and device aggregations
// - bleve_stats_service_utils.go: Utility functions, time range queries, global service management
//
// All original functionality is preserved across these modules.
